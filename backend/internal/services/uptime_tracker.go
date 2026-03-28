package services

import (
	"sync"
	"time"
)

const maxUptimeEvents = 100

// UptimeEvent represents a single connectivity state transition.
type UptimeEvent struct {
	Timestamp int64  `json:"timestamp"` // Unix milliseconds
	State     string `json:"state"`     // "connected" or "disconnected"
}

// UptimeTracker periodically checks internet reachability and records
// state transitions in a fixed-size ring buffer.
type UptimeTracker struct {
	prober        HTTPProber
	mu            sync.RWMutex
	events        []UptimeEvent
	lastState     *bool // nil = unknown (first run)
	CheckInterval time.Duration
	stopCh        chan struct{}
}

// NewUptimeTracker creates a new UptimeTracker using the given prober.
func NewUptimeTracker(prober HTTPProber) *UptimeTracker {
	return &UptimeTracker{
		prober:        prober,
		events:        make([]UptimeEvent, 0, maxUptimeEvents),
		CheckInterval: 30 * time.Second,
		stopCh:        make(chan struct{}),
	}
}

// Start begins periodic connectivity checks in the background.
func (t *UptimeTracker) Start() {
	go func() {
		t.check()
		ticker := time.NewTicker(t.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.check()
			case <-t.stopCh:
				return
			}
		}
	}()
}

// Stop halts the periodic checks.
func (t *UptimeTracker) Stop() {
	close(t.stopCh)
}

// GetUptimeLog returns all recorded state transitions, newest first.
func (t *UptimeTracker) GetUptimeLog() []UptimeEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]UptimeEvent, len(t.events))
	copy(result, t.events)
	// Reverse to return newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func (t *UptimeTracker) check() {
	code, _, _, err := t.prober.Do(captiveProbeURL)
	connected := err == nil && code == 204

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.lastState != nil && *t.lastState == connected {
		return // no state change
	}

	state := "disconnected"
	if connected {
		state = "connected"
	}
	t.events = append(t.events, UptimeEvent{
		Timestamp: time.Now().UnixMilli(),
		State:     state,
	})
	if len(t.events) > maxUptimeEvents {
		t.events = t.events[len(t.events)-maxUptimeEvents:]
	}
	c := connected
	t.lastState = &c
}
