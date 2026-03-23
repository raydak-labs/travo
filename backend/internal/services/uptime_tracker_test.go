package services

import (
	"testing"
	"time"
)

func TestUptimeTracker_RecordsFirstEvent(t *testing.T) {
	prober := &MockHTTPProber{StatusCode: 204, Body: ""}
	tracker := NewUptimeTracker(prober)

	tracker.check()

	events := tracker.GetUptimeLog()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].State != "connected" {
		t.Errorf("expected state 'connected', got %q", events[0].State)
	}
}

func TestUptimeTracker_NoEventOnNoStateChange(t *testing.T) {
	prober := &MockHTTPProber{StatusCode: 204, Body: ""}
	tracker := NewUptimeTracker(prober)

	tracker.check()
	tracker.check()

	events := tracker.GetUptimeLog()
	if len(events) != 1 {
		t.Errorf("expected 1 event (no duplicate on same state), got %d", len(events))
	}
}

func TestUptimeTracker_RecordsStateTransitions(t *testing.T) {
	prober := &MockHTTPProber{StatusCode: 204, Body: ""}
	tracker := NewUptimeTracker(prober)

	// connected
	tracker.check()
	// disconnected
	prober.StatusCode = 500
	tracker.check()
	// reconnected
	prober.StatusCode = 204
	tracker.check()

	events := tracker.GetUptimeLog()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	// newest first
	if events[0].State != "connected" {
		t.Errorf("events[0] expected 'connected', got %q", events[0].State)
	}
	if events[1].State != "disconnected" {
		t.Errorf("events[1] expected 'disconnected', got %q", events[1].State)
	}
	if events[2].State != "connected" {
		t.Errorf("events[2] expected 'connected', got %q", events[2].State)
	}
}

func TestUptimeTracker_RingBufferCapAtMax(t *testing.T) {
	prober := &MockHTTPProber{StatusCode: 204, Body: ""}
	tracker := NewUptimeTracker(prober)

	// Force maxUptimeEvents+2 transitions by alternating state
	for i := 0; i < maxUptimeEvents+2; i++ {
		if i%2 == 0 {
			prober.StatusCode = 204
		} else {
			prober.StatusCode = 500
		}
		tracker.check()
	}

	events := tracker.GetUptimeLog()
	if len(events) > maxUptimeEvents {
		t.Errorf("expected at most %d events, got %d", maxUptimeEvents, len(events))
	}
}

func TestUptimeTracker_StartStop(t *testing.T) {
	prober := &MockHTTPProber{StatusCode: 204, Body: ""}
	tracker := NewUptimeTracker(prober)
	tracker.CheckInterval = 10 * time.Millisecond

	tracker.Start()
	time.Sleep(50 * time.Millisecond)
	tracker.Stop()

	events := tracker.GetUptimeLog()
	if len(events) == 0 {
		t.Error("expected at least one event after Start()")
	}
}
