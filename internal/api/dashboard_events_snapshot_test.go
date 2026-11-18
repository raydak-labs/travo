package api

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// A subscriber that joins a running stream gets the last value of each source at once,
// instead of waiting up to one interval for the next tick (#93).
func TestDashboardEventServicePrimesLateSubscriber(t *testing.T) {
	daemon, cancel := context.WithCancel(context.Background())
	defer cancel()
	var reads atomic.Int32
	sources := []dashboardEventSource{
		{name: "resources", interval: time.Hour, read: func(context.Context) (any, error) {
			reads.Add(1)
			return "snapshot", nil
		}},
		{name: "throughput", interval: time.Hour, read: func(ctx context.Context) (any, error) {
			<-ctx.Done() // never published: the late subscriber must not get a placeholder
			return nil, ctx.Err()
		}},
	}
	svc := newDashboardEventService(daemon, 8, sources)
	first, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer first.close()
	waitEvent(t, first, "resources")

	late, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer late.close()
	select {
	case <-late.wake:
	case <-time.After(time.Second):
		t.Fatal("late subscriber was not woken with the last snapshot")
	}
	event, ok := late.take()
	if !ok || event.name != "resources" || event.data != "snapshot" {
		t.Fatalf("late subscriber got %+v (ok=%v), want the last resources event", event, ok)
	}
	if extra, ok := late.take(); ok {
		t.Fatalf("late subscriber got an unpublished source: %+v", extra)
	}
	if got := reads.Load(); got != 1 {
		t.Fatalf("priming read the source again: %d reads, want 1", got)
	}
}

// A new run after the last subscriber left must not replay the stopped run's values.
func TestDashboardEventServiceRestartDropsOldSnapshot(t *testing.T) {
	daemon, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	sources := []dashboardEventSource{{name: "resources", interval: time.Hour, read: func(ctx context.Context) (any, error) {
		if calls.Add(1) == 1 {
			return "old", nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}}}
	svc := newDashboardEventService(daemon, 8, sources)
	first, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	waitEvent(t, first, "resources")
	run := svc.currentRun()
	first.close()
	waitProducers(t, run)

	second, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer second.close()
	if event, ok := second.take(); ok {
		t.Fatalf("restarted stream replayed %+v from the stopped run", event)
	}
}

func waitEvent(t *testing.T, sub *dashboardSubscription, name string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		if event, ok := sub.take(); ok && event.name == name {
			return
		}
		select {
		case <-sub.wake:
		case <-deadline:
			t.Fatalf("no %s event", name)
		}
	}
}
