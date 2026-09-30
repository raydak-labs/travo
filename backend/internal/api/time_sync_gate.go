package api

import (
	"fmt"
	"sync"
	"time"
)

// Bounds for the clock value an *unauthenticated* caller may install.
//
// There is no RTC on most of these devices, so before the first successful
// sync the only trustworthy reference is the binary build time (the
// "minimum plausible clock"). The window around it stops an attacker from
// parking the clock in 1970 (which keeps JWT `exp` checks valid forever) or in
// a far-future year (which breaks TLS, cron and hwclock and permanently locks
// out the pre-login recovery path).
//
// The window is deliberately WIDE, and the real protection is that it is
// single-use. floor is the build time, not "now", and the build-to-install lag
// is unbounded: a user who installs a release three months old on an RTC-less
// device posts a clock that is legitimately months past floor. A tight upper
// bound (a few days was tried) does not close the attack — it just breaks the
// documented pre-login recovery path, on a device where losing the clock also
// makes JWT `exp` meaningless, leaving no way back in.
//
// What stops a repeated attack is NoteClockSet latching the gate shut after the
// FIRST accepted unauthenticated sync. That is a single bounded jump, on a
// device whose clock was already unusable, to a client on the LAN — an
// availability nuisance, not an auth bypass, since a forward jump can only
// shorten a token's remaining life and a backward one is refused outright.
const (
	timeSyncMinClientOffset = -30 * 24 * time.Hour
	timeSyncMaxClientOffset = 400 * 24 * time.Hour
)

// TimeSyncGate makes the "clock was plausible" decision sticky.
//
// The naive check (`now < buildTime` ⇒ allow unauthenticated sync) is
// self-defeating: whoever can move the clock also controls the gate, so
// rewinding to 1970 re-opens an unauthenticated endpoint forever. Once the
// clock has been observed plausible the gate latches shut, and no clock value
// can reopen it.
type TimeSyncGate struct {
	mu        sync.Mutex
	plausible bool
}

// NewTimeSyncGate returns a gate seeded with the current clock plausibility.
func NewTimeSyncGate(clockPlausible bool) *TimeSyncGate {
	return &TimeSyncGate{plausible: clockPlausible}
}

// UnauthBlocked reports whether unauthenticated time sync must be refused,
// latching the gate whenever the clock looks plausible.
func (g *TimeSyncGate) UnauthBlocked(now, floor time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !now.Before(floor) {
		g.plausible = true
	}
	return g.plausible
}

// NoteClockSet latches the gate after a successful clock change.
//
// Latching on *every* accepted write — not only on writes that land at or
// after the build time — is what closes the pin-the-clock-lowly attack. If the
// gate only latched for plausible values, an attacker could keep setting the
// clock to floor-61s, floor-121s, ... forever (the lower bound is 30 days
// wide, so this is always available). The gate would never latch, the
// unauthenticated clock-set primitive would stay reachable indefinitely, and
// the endpoint the gate exists to protect would never close. The only cost is
// that a device which legitimately needed several successive corrections gets
// one fewer of them, and re-authenticating is the documented way to set the
// clock again.
func (g *TimeSyncGate) NoteClockSet() {
	g.mu.Lock()
	g.plausible = true
	g.mu.Unlock()
}

// Plausible reports the latched state.
func (g *TimeSyncGate) Plausible() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.plausible
}

// validateClientTimeWindow rejects clock values too far from the minimum
// plausible clock. Authenticated callers bypass this (an operator with a token
// is allowed to set any time); unauthenticated ones are not.
func validateClientTimeWindow(client, floor time.Time) error {
	if client.Before(floor.Add(timeSyncMinClientOffset)) {
		return fmt.Errorf("client time %s is before the allowed window (%s)",
			client.UTC().Format(time.RFC3339), floor.Add(timeSyncMinClientOffset).UTC().Format(time.RFC3339))
	}
	if client.After(floor.Add(timeSyncMaxClientOffset)) {
		return fmt.Errorf("client time %s is after the allowed window (%s)",
			client.UTC().Format(time.RFC3339), floor.Add(timeSyncMaxClientOffset).UTC().Format(time.RFC3339))
	}
	return nil
}
