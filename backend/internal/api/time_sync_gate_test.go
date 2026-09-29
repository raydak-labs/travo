package api

import (
	"net/http"
	"testing"
	"time"
)

// An unauthenticated caller must not be able to park the clock in 1970 while
// the gate is open: that keeps JWT exp checks valid forever.
func TestTimeSync_RejectsImplausibleTargetWindow(t *testing.T) {
	floor := time.Now().Add(time.Hour) // gate stays open: clock looks implausible
	for _, name := range []string{"epoch", "y2030"} {
		t.Run(name, func(t *testing.T) {
			app, deps := setupTestApp(t)
			deps.TimeSyncMinPlausible = floor
			deps.TimeSyncGate = NewTimeSyncGate(false)
			deps.TimeSyncSetTime = func(epochSec int64) error {
				t.Fatal("SetTime must not be called")
				return nil
			}

			var client int64
			if name == "epoch" {
				client = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
			} else {
				client = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
			}
			resp := postTimeSync(t, app, "", client)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400 for out-of-window client time, got %d", resp.StatusCode)
			}
		})
	}
}

// Once the clock has been observed plausible the gate latches: rewinding the
// clock (by any means) cannot reopen unauthenticated time sync.
func TestTimeSyncGate_LatchesPlausible(t *testing.T) {
	floor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g := NewTimeSyncGate(false)

	if g.UnauthBlocked(floor.Add(-24*time.Hour), floor) {
		t.Fatal("gate must stay open while the clock is implausible")
	}
	// Clock becomes plausible.
	if !g.UnauthBlocked(floor.Add(24*time.Hour), floor) {
		t.Fatal("gate must close once the clock looks plausible")
	}
	// Attacker rewinds the clock to 1970 and retries.
	if !g.UnauthBlocked(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), floor) {
		t.Fatal("gate must stay closed after rewinding the clock")
	}
	if !g.Plausible() {
		t.Error("Plausible() should report the latched state")
	}
}

func TestTimeSyncGate_ClockSetLatches(t *testing.T) {
	floor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g := NewTimeSyncGate(false)
	if g.Plausible() {
		t.Fatal("new gate must not be latched")
	}
	g.NoteClockSet(floor.Add(-365*24*time.Hour), floor)
	if g.Plausible() {
		t.Error("a still-implausible clock must not latch the gate")
	}
	g.NoteClockSet(floor, floor)
	if !g.Plausible() {
		t.Error("adopting a plausible clock must latch the gate")
	}
}

func TestValidateClientTimeWindow(t *testing.T) {
	floor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := validateClientTimeWindow(floor, floor); err != nil {
		t.Errorf("floor itself must be accepted: %v", err)
	}
	if err := validateClientTimeWindow(floor.Add(24*time.Hour), floor); err != nil {
		t.Errorf("a day ahead must be accepted: %v", err)
	}
	if err := validateClientTimeWindow(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), floor); err == nil {
		t.Error("1970 must be rejected")
	}
	if err := validateClientTimeWindow(floor.Add(10000*24*time.Hour), floor); err == nil {
		t.Error("far-future clock must be rejected")
	}
}
