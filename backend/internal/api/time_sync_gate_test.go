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
	// Any accepted write latches, including one that lands below the floor.
	// Otherwise an attacker can keep pinning the clock just under the build
	// time (floor-61s, floor-121s, …) and the unauthenticated primitive the
	// gate exists to close stays reachable forever.
	g.NoteClockSet()
	if !g.Plausible() {
		t.Error("adopting a clock value must latch the gate")
	}
	if !g.UnauthBlocked(floor.Add(-24*time.Hour), floor) {
		t.Error("gate must stay closed after any successful clock set")
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
	// The window must stay wide enough to cover the build-to-install lag. floor
	// is the BUILD time, so a user installing an old release on an RTC-less
	// device legitimately posts a clock months past it, and rejecting that
	// breaks the only pre-login recovery path such a device has.
	if err := validateClientTimeWindow(floor.Add(90*24*time.Hour), floor); err != nil {
		t.Errorf("a 3-month build-to-install lag must still be recoverable: %v", err)
	}
	if err := validateClientTimeWindow(floor.Add(10000*24*time.Hour), floor); err == nil {
		t.Error("far-future clock must be rejected")
	}
}
