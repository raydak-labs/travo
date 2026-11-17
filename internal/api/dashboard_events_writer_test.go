package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/auth"
	"github.com/blackdark-org/travo-greenfield/internal/throughput"
)

type recordedWrite struct {
	frame string
	at    time.Time
}

type recordingResponseWriter struct {
	mu        sync.Mutex
	header    http.Header
	deadlines []time.Time
	writes    []recordedWrite
	flushes   int
	flushErr  error
	changed   chan struct{}
}

func newRecordingResponseWriter() *recordingResponseWriter {
	return &recordingResponseWriter{header: make(http.Header), changed: make(chan struct{}, 100)}
}

func (w *recordingResponseWriter) Header() http.Header { return w.header }
func (w *recordingResponseWriter) WriteHeader(int)     {}
func (w *recordingResponseWriter) SetWriteDeadline(at time.Time) error {
	w.mu.Lock()
	w.deadlines = append(w.deadlines, at)
	w.mu.Unlock()
	w.signal()
	return nil
}
func (w *recordingResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.writes = append(w.writes, recordedWrite{frame: string(p), at: time.Now()})
	w.mu.Unlock()
	w.signal()
	return len(p), nil
}
func (w *recordingResponseWriter) Flush() {
	w.mu.Lock()
	w.flushes++
	w.mu.Unlock()
	w.signal()
}
func (w *recordingResponseWriter) FlushError() error {
	w.Flush()
	return w.flushErr
}
func (w *recordingResponseWriter) signal() {
	select {
	case w.changed <- struct{}{}:
	default:
	}
}
func (w *recordingResponseWriter) snapshot() (deadlines []time.Time, body string, flushes int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	deadlines = append(deadlines, w.deadlines...)
	var frames strings.Builder
	for _, write := range w.writes {
		frames.WriteString(write.frame)
	}
	return deadlines, frames.String(), w.flushes
}

func TestDashboardSSEWriterDeadlineAndSessionChecks(t *testing.T) {
	w := newRecordingResponseWriter()
	checks := 0
	writer := dashboardSSEWriter{
		writer: w, controller: http.NewResponseController(w), timeout: 5 * time.Second,
		sessionValid: func() bool { checks++; return checks != 2 },
	}
	if !writer.write([]byte(": connected\n\n")) {
		t.Fatal("valid session write failed")
	}
	if writer.write([]byte(": heartbeat\n\n")) {
		t.Fatal("write continued after session revocation")
	}
	deadlines, body, flushes := w.snapshot()
	if len(deadlines) != 2 || deadlines[0].IsZero() || !deadlines[1].IsZero() {
		t.Fatalf("deadlines = %v, want a 5-second deadline then cleared", deadlines)
	}
	if body != ": connected\n\n" || flushes != 1 || checks != 2 {
		t.Fatalf("body %q, flushes %d, session checks %d", body, flushes, checks)
	}
}

func TestDashboardSSEWriterDoesNotClearFailedFlush(t *testing.T) {
	w := newRecordingResponseWriter()
	w.flushErr = errors.New("blocked")
	writer := dashboardSSEWriter{
		writer: w, controller: http.NewResponseController(w), timeout: time.Second,
		sessionValid: func() bool { return true },
	}
	if writer.write([]byte(": heartbeat\n\n")) {
		t.Fatal("flush error was ignored")
	}
	deadlines, _, _ := w.snapshot()
	if len(deadlines) != 1 || deadlines[0].IsZero() {
		t.Fatalf("deadline was cleared after failed flush: %v", deadlines)
	}
}

func TestDashboardEventsIdleHeartbeat(t *testing.T) {
	store := auth.NewStore(zeroEntropy())
	clock := &fakeClock{t: time.Now()}
	token, _, err := store.Create(clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := &server{sessions: store, clock: clock}
	svc := newDashboardEventService(context.Background(), 8, nil)
	sub, err := svc.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	w := newRecordingResponseWriter()
	heartbeat := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.serveDashboardEvents(ctx, w, sub, authed{token: token}, heartbeat, time.Second)
		close(done)
	}()
	heartbeat <- time.Now()
	waitForWrite(t, w, func(body string) bool { return body == ": connected\n\n: heartbeat\n\n" })
	cancel()
	<-done
}

type blockingResponseWriter struct {
	*recordingResponseWriter
	entered  chan struct{}
	armed    bool
	deadline time.Time
}

func (w *blockingResponseWriter) SetWriteDeadline(at time.Time) error {
	w.mu.Lock()
	w.deadline = at
	w.mu.Unlock()
	return w.recordingResponseWriter.SetWriteDeadline(at)
}
func (w *blockingResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	armed := w.armed
	deadline := w.deadline
	w.mu.Unlock()
	if armed {
		select {
		case w.entered <- struct{}{}:
		default:
		}
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		<-timer.C
		return 0, errors.New("write deadline exceeded")
	}
	return w.recordingResponseWriter.Write(p)
}
func (w *blockingResponseWriter) arm() {
	w.mu.Lock()
	w.armed = true
	w.mu.Unlock()
}

func TestDashboardEventsSlowSubscriberDoesNotBlockHealthy(t *testing.T) {
	svc := newDashboardEventService(context.Background(), 8, []dashboardEventSource{{
		name: "throughput", interval: time.Hour,
		read: func(ctx context.Context) (any, error) { <-ctx.Done(); return nil, ctx.Err() },
	}})
	slowSub, _ := svc.subscribe()
	healthySub, _ := svc.subscribe()
	store := auth.NewStore(zeroEntropy())
	clock := &fakeClock{t: time.Now()}
	token, _, _ := store.Create(clock.Now())
	s := &server{sessions: store, clock: clock}
	slow := &blockingResponseWriter{recordingResponseWriter: newRecordingResponseWriter(), entered: make(chan struct{}, 1)}
	healthy := newRecordingResponseWriter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slowDone := make(chan struct{})
	healthyDone := make(chan struct{})
	go func() {
		s.serveDashboardEvents(ctx, slow, slowSub, authed{token: token}, nil, 20*time.Millisecond)
		close(slowDone)
	}()
	go func() {
		s.serveDashboardEvents(ctx, healthy, healthySub, authed{token: token}, nil, 20*time.Millisecond)
		close(healthyDone)
	}()
	<-slow.changed
	<-healthy.changed
	slow.arm()
	run := svc.currentRun()
	svc.publish(run, 0, dashboardEvent{name: "throughput", data: throughputBody{Interfaces: []throughput.Rate{{Name: "one"}}}})

	select {
	case <-slow.entered:
	case <-time.After(time.Second):
		t.Fatal("slow subscriber did not enter its blocked write")
	}
	svc.publish(run, 0, dashboardEvent{name: "throughput", data: throughputBody{Interfaces: []throughput.Rate{{Name: "two"}}}})
	select {
	case <-slowDone:
	case <-time.After(time.Second):
		t.Fatal("slow subscriber was not disconnected at its write deadline")
	}
	select {
	case <-healthyDone:
		t.Fatal("healthy subscriber was disconnected with the slow one")
	default:
	}
	waitForWrite(t, healthy, func(body string) bool { return strings.Contains(body, `"name":"two"`) })
	cancel()
	<-healthyDone
	_, body, _ := healthy.snapshot()
	if !strings.Contains(body, `"name":"two"`) {
		t.Fatalf("healthy subscriber missed the newest event: %q", body)
	}
}

func waitForWrite(t *testing.T, w *recordingResponseWriter, done func(string) bool) {
	t.Helper()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for {
		_, body, _ := w.snapshot()
		if done(body) {
			return
		}
		select {
		case <-w.changed:
		case <-timeout.C:
			t.Fatalf("timed out waiting for write; body = %q", body)
		}
	}
}
