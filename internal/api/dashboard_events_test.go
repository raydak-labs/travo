package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/throughput"
)

func TestDashboardEventServiceSharesProducersAndStops(t *testing.T) {
	daemon, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan string, 6)
	calls := map[string]*atomic.Int32{}
	sources := make([]dashboardEventSource, 0, 3)
	for _, name := range []string{"connectivity", "resources", "throughput"} {
		calls[name] = &atomic.Int32{}
		sources = append(sources, dashboardEventSource{
			name: name, interval: time.Hour,
			read: func(ctx context.Context) (any, error) {
				calls[name].Add(1)
				started <- name
				<-ctx.Done()
				return nil, ctx.Err()
			},
		})
	}
	svc := newDashboardEventService(daemon, 8, sources)
	first, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	for range sources {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("producers did not read immediately")
		}
	}
	second, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	for name, count := range calls {
		if got := count.Load(); got != 1 {
			t.Fatalf("%s called %d times for two subscribers, want one shared read", name, got)
		}
	}
	run := svc.currentRun()
	first.close()
	if run.ctx.Err() != nil {
		t.Fatal("last-but-one unsubscribe stopped the shared producers")
	}
	second.close()
	waitProducers(t, run)
	for name, count := range calls {
		if got := count.Load(); got != 1 {
			t.Fatalf("%s read again after cancellation: %d", name, got)
		}
	}

	third, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer third.close()
	for range sources {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("a new first subscriber did not restart every producer")
		}
	}
	for name, count := range calls {
		if got := count.Load(); got != 2 {
			t.Fatalf("%s calls after restart = %d, want 2", name, got)
		}
	}
}

func TestDashboardEventServiceRestartWaitsForPreviousRead(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{}, 1)
	defer close(releaseFirst)
	secondStarted := make(chan struct{})
	var calls atomic.Int32
	svc := newDashboardEventService(context.Background(), 8, []dashboardEventSource{{
		name: "connectivity", interval: time.Hour,
		read: func(context.Context) (any, error) {
			if calls.Add(1) == 1 {
				close(firstStarted)
				<-releaseFirst // Simulate a source that unwinds slowly after cancellation.
				return nil, context.Canceled
			}
			close(secondStarted)
			return "ok", nil
		},
	}})
	first, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first producer did not start")
	}
	firstRun := svc.currentRun()
	first.close()

	type subscriptionResult struct {
		sub *dashboardSubscription
		err error
	}
	subscribed := make(chan subscriptionResult, 1)
	go func() {
		sub, err := svc.subscribe()
		subscribed <- subscriptionResult{sub: sub, err: err}
	}()
	var second *dashboardSubscription
	select {
	case result := <-subscribed:
		if result.err != nil {
			t.Fatal(result.err)
		}
		second = result.sub
	case <-time.After(time.Second):
		t.Fatal("resubscribe blocked on the previous read")
	}
	defer second.close()
	secondRun := svc.currentRun()
	select {
	case <-secondStarted:
		t.Fatal("replacement producer overlapped the previous read")
	case <-time.After(50 * time.Millisecond):
	}

	releaseFirst <- struct{}{}
	waitProducers(t, firstRun)
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("replacement producer did not start after the previous read drained")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("read calls = %d, want 2", got)
	}
	second.close()
	waitProducers(t, secondRun)
}

func TestDashboardEventServicePublishesWithoutSlowSubscriber(t *testing.T) {
	svc := newDashboardEventService(context.Background(), 8, []dashboardEventSource{{
		name: "throughput", interval: time.Hour,
		read: func(context.Context) (any, error) { return throughputBody{Interfaces: []throughput.Rate{}}, nil },
	}})
	slow, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer slow.close()
	healthy, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer healthy.close()
	select {
	case <-healthy.wake:
	case <-time.After(time.Second):
		t.Fatal("initial read was not published")
	}
	if event, ok := healthy.take(); !ok || event.name != "throughput" {
		t.Fatalf("healthy initial event = %+v, %v", event, ok)
	}

	run := svc.currentRun()
	published := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			svc.publish(run, 0, dashboardEvent{name: "throughput", data: throughputBody{Interfaces: []throughput.Rate{{Name: fmt.Sprint(i)}}}})
		}
		close(published)
	}()
	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("publishing blocked on a subscriber that was not reading")
	}
	event, ok := healthy.take()
	if !ok {
		t.Fatal("healthy subscriber has no pending value")
	}
	body := event.data.(throughputBody)
	if got := body.Interfaces[0].Name; got != "99" {
		t.Fatalf("replaceable slot contains %q, want latest value 99", got)
	}
	// The slow subscriber has the same bounded slot; no unbounded queue accumulated.
	slowEvent, ok := slow.take()
	if !ok || slowEvent.data.(throughputBody).Interfaces[0].Name != "99" {
		t.Fatalf("slow subscriber slot = %+v, %v", slowEvent, ok)
	}
}

func TestDashboardEventServiceCapacityAndDaemonCancellation(t *testing.T) {
	daemon, cancel := context.WithCancel(context.Background())
	svc := newDashboardEventService(daemon, 8, nil)
	subs := make([]*dashboardSubscription, 0, 8)
	for i := 0; i < 8; i++ {
		sub, err := svc.subscribe()
		if err != nil {
			t.Fatalf("subscriber %d: %v", i, err)
		}
		subs = append(subs, sub)
	}
	if _, err := svc.subscribe(); !errors.Is(err, errStreamCapacity) {
		t.Fatalf("ninth subscriber error = %v, want stream capacity", err)
	}
	cancel()
	for _, sub := range subs {
		if _, ok := sub.take(); ok {
			t.Fatal("unexpected event")
		}
		select {
		case <-svc.daemon.Done():
		case <-time.After(time.Second):
			t.Fatal("daemon did not cancel")
		}
		sub.close()
	}
	if _, err := svc.subscribe(); !errors.Is(err, errStreamClosed) {
		t.Fatalf("subscribe after shutdown = %v", err)
	}
}

func TestDashboardEventsThroughProductionMiddleware(t *testing.T) {
	link := "phy0-sta0"
	carrier := true
	snapshot := &connectivitySnapshot{ActiveInterface: &link, Carrier: &carrier, Internet: connectivity.Online}
	rates := &throughputSnapshot{ready: true, rates: []throughput.Rate{{Name: "phy0-sta0"}}}
	e := newEnvWithDashboard(t, nil, rates, snapshot, func(d *Deps) { d.Alerts = fakeAlerts{} })
	mock.SeedSystem(e.dev)
	if rec := e.do(req{method: http.MethodGet, path: "/api/v1/dashboard/events"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("events before login: %d %s", rec.Code, rec.Body)
	}
	cookie, _ := e.login()
	ts := httptest.NewServer(e.h)
	defer ts.Close()
	resp := openEventStream(t, ts.URL, cookie)
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	reader := bufio.NewReader(resp.Body)
	if frame := readSSEFrame(t, reader); frame != ": connected\n\n" {
		t.Fatalf("initial frame = %q", frame)
	}

	events := map[string]map[string]any{}
	for len(events) < 4 {
		frame := readSSEFrame(t, reader)
		name, data := parseSSEEvent(t, frame)
		events[name] = data
	}
	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "connectivity", path: "/api/v1/dashboard/connectivity"},
		{name: "resources", path: "/api/v1/system/info"},
		{name: "throughput", path: "/api/v1/dashboard/throughput"},
		{name: "alerts", path: "/api/v1/system/alerts"},
	} {
		rec := e.do(req{method: http.MethodGet, path: tc.path, cookie: cookie})
		var want map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &want); err != nil {
			t.Fatalf("GET %s: %v", tc.path, err)
		}
		if jsonString(events[tc.name]) != jsonString(want) {
			t.Errorf("%s event = %s, GET = %s", tc.name, jsonString(events[tc.name]), jsonString(want))
		}
	}
}

func TestDashboardEventsSourceUnavailable(t *testing.T) {
	e := newEnv(t) // resources, throughput and alerts are unavailable
	cookie, _ := e.login()
	ts := httptest.NewServer(e.h)
	defer ts.Close()
	resp := openEventStream(t, ts.URL, cookie)
	reader := bufio.NewReader(resp.Body)
	_ = readSSEFrame(t, reader)
	unavailable := map[string]bool{}
	for len(unavailable) < 3 {
		name, data := parseSSEEvent(t, readSSEFrame(t, reader))
		if name != "source_unavailable" {
			continue
		}
		source, _ := data["source"].(string)
		unavailable[source] = true
	}
	resp.Body.Close()
	if !unavailable["resources"] || !unavailable["throughput"] || !unavailable["alerts"] {
		t.Fatalf("source_unavailable events = %v", unavailable)
	}
}

func TestDashboardEventsCapacity(t *testing.T) {
	e := newEnv(t)
	cookie, _ := e.login()
	ts := httptest.NewServer(e.h)
	defer ts.Close()
	for i := 0; i < 8; i++ {
		stream := openEventStream(t, ts.URL, cookie)
		defer stream.Body.Close()
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/dashboard/events", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	ninth, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(ninth.Body)
	ninth.Body.Close()
	if ninth.StatusCode != http.StatusServiceUnavailable || ninth.Header.Get("Content-Type") != "application/json" || !strings.Contains(string(body), `"code":"stream_capacity"`) {
		t.Fatalf("ninth stream: %d %s %s", ninth.StatusCode, ninth.Header.Get("Content-Type"), body)
	}
}

func TestDashboardEventsRevocationAndDaemonShutdown(t *testing.T) {
	t.Run("revoked cookie closes on the next event", func(t *testing.T) {
		rates := &throughputSnapshot{ready: true, rates: []throughput.Rate{}}
		e := newEnvWithDevices(t, nil, rates)
		cookie, _ := e.login()
		ts := httptest.NewServer(e.h)
		defer ts.Close()
		resp := openEventStream(t, ts.URL, cookie)
		reader := bufio.NewReader(resp.Body)
		_ = readSSEFrame(t, reader)
		e.store.Revoke(cookie)
		for {
			frame := readSSEFrameAllowEOF(t, reader)
			if frame == "" {
				return
			}
		}
	})

	t.Run("daemon cancellation closes streams before HTTP drain", func(t *testing.T) {
		daemon, cancel := context.WithCancel(context.Background())
		e := newEnvWithDashboard(t, nil, &throughputSnapshot{ready: true}, nil, func(d *Deps) { d.DaemonContext = daemon })
		cookie, _ := e.login()
		ts := httptest.NewServer(e.h)
		resp := openEventStream(t, ts.URL, cookie)
		_ = readSSEFrame(t, bufio.NewReader(resp.Body))
		cancel()
		drained := make(chan error, 1)
		go func() {
			ctx, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			drained <- ts.Config.Shutdown(ctx)
		}()
		select {
		case err := <-drained:
			if err != nil {
				t.Fatalf("HTTP drain waited for the stream: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("HTTP shutdown did not drain after daemon cancellation")
		}
		resp.Body.Close()
		ts.Close()
	})
}
