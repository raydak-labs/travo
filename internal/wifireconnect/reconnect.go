// Package wifireconnect retries the configured uplink STA after it lost its
// association, at most MaxAttempts times (docs/adr/0014). It never switches to
// another saved uplink and never writes UCI: the operator does that with Select.
package wifireconnect

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

const (
	// MaxAttempts caps recoveries until wwan holds an IPv4 address again.
	MaxAttempts = 5
	// Interval is the worker tick; an attempt needs two disconnected ticks in a row,
	// so wpa_supplicant gets one full interval to re-associate on its own.
	Interval = time.Minute
	// GuardName is held while `ifup wwan` runs (invariants §1).
	GuardName = "wifi-reconnect-in-progress"
)

// ErrUnavailable means the persisted counter is unreadable; the worker stays idle.
var ErrUnavailable = errors.New("WiFi reconnect state unavailable")

// State is the reconnect phase shown to the operator.
type State string

const (
	StateIdle      State = "idle"
	StateRetrying  State = "retrying"
	StateExhausted State = "exhausted"
)

// Status is the public reconnect view. It never carries credentials.
type Status struct {
	State       State `json:"state" enum:"idle,retrying,exhausted"`
	Attempts    int   `json:"attempts" minimum:"0"`
	MaxAttempts int   `json:"max_attempts" minimum:"1"`
}

// record is the persisted counter. A restart or redeploy never resets it.
type record struct {
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"last_attempt,omitzero"`
}

// Uplink reads the managed STA (internal/wifi).
type Uplink interface {
	UplinkEnabled(context.Context) (bool, error)
	AssociatedUplink(context.Context) (wifi.Association, error)
}

// Guards is the slice of guard.Dir the worker needs.
type Guards interface {
	Stale() ([]guard.Info, error)
	Acquire(string) error
	Release(string) error
}

// Observation is one tick's read of the uplink. Readable=false means "do nothing".
type Observation struct {
	Readable   bool
	Enabled    bool
	Associated bool
	Addressed  bool
}

// Action is what one tick does.
type Action int

const (
	ActionNone Action = iota
	ActionClear
	ActionAttempt
)

// Decide is the pure policy: clear on a proven address, attempt only when the
// enabled STA was proven disconnected on this and the previous tick and the cap allows.
func Decide(attempts int, obs Observation, wasDown bool) Action {
	switch {
	case !obs.Readable:
		return ActionNone
	case obs.Addressed:
		if attempts > 0 {
			return ActionClear
		}
		return ActionNone
	case !obs.Enabled || obs.Associated || !wasDown || attempts >= MaxAttempts:
		return ActionNone
	}
	return ActionAttempt
}

func (o Observation) down() bool { return o.Readable && o.Enabled && !o.Associated && !o.Addressed }

// Service owns the counter file and runs one tick at a time.
type Service struct {
	path      string
	uplink    Uplink
	ubus      device.Ubus
	guards    Guards
	reconnect device.UplinkReconnector
	now       func() time.Time
	writeJSON func(string, any, os.FileMode) error
	mu        sync.Mutex
	wasDown   bool
}

func New(path string, uplink Uplink, ubus device.Ubus, guards Guards, reconnect device.UplinkReconnector, now func() time.Time) *Service {
	return &Service{path: path, uplink: uplink, ubus: ubus, guards: guards, reconnect: reconnect, now: now, writeJSON: atomicfile.WriteJSON}
}

// load fails closed: only an absent file reads as zero attempts.
func (s *Service) load() (record, error) {
	var r record
	err := atomicfile.ReadJSON(s.path, &r)
	if errors.Is(err, os.ErrNotExist) {
		return record{}, nil
	}
	if err != nil || r.Attempts < 0 {
		return record{}, ErrUnavailable
	}
	return r, nil
}

// Status reads the persisted counter. GET never mutates.
func (s *Service) Status(context.Context) (Status, error) {
	r, err := s.load()
	if err != nil {
		return Status{}, err
	}
	st := Status{State: StateIdle, Attempts: r.Attempts, MaxAttempts: MaxAttempts}
	switch {
	case r.Attempts >= MaxAttempts:
		st.State = StateExhausted
	case r.Attempts > 0:
		st.State = StateRetrying
	}
	return st, nil
}

func (s *Service) observe(ctx context.Context) Observation {
	enabled, err := s.uplink.UplinkEnabled(ctx)
	if err != nil {
		return Observation{}
	}
	assoc, err := s.uplink.AssociatedUplink(ctx)
	if err != nil {
		return Observation{}
	}
	raw, err := s.ubus.Call(ctx, "network.interface.wwan", "status", nil)
	if errors.Is(err, device.ErrNotFound) && !enabled {
		return Observation{Readable: true}
	}
	if err != nil {
		return Observation{}
	}
	iface, err := facts.ParseInterfaceStatus(raw)
	if err != nil {
		return Observation{}
	}
	return Observation{Readable: true, Enabled: enabled, Associated: assoc != wifi.Association{}, Addressed: len(iface.IPv4) > 0}
}

// Tick runs one observation and at most one recovery. It is the lifecycle worker body.
func (s *Service) Tick(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	obs := s.observe(ctx)
	wasDown := s.wasDown
	// A down STA under a guard is a live change at work (a join or select takes wwan
	// down for seconds), not an outage: it must not start the two-tick count.
	guardedNow := s.guarded()
	s.wasDown = obs.down() && !guardedNow
	r, err := s.load()
	if err != nil {
		slog.Warn("wifi reconnect: counter unreadable, staying idle")
		return
	}
	switch Decide(r.Attempts, obs, wasDown) {
	case ActionClear:
		if s.writeJSON(s.path, record{}, 0o600) != nil {
			slog.Warn("wifi reconnect: could not clear counter")
		}
	case ActionAttempt:
		if guardedNow {
			return // a guard seen this tick wins even if released before attempt re-checks
		}
		s.attempt(ctx, r)
		s.wasDown = false // the next attempt waits two more ticks
	}
}

// guarded fails closed: an unreadable guard dir counts as guarded.
func (s *Service) guarded() bool {
	active, err := s.guards.Stale()
	return err != nil || len(active) > 0
}

// attempt persists the counter BEFORE acting, so a crash mid-recovery still counts.
func (s *Service) attempt(ctx context.Context, r record) {
	if s.guarded() {
		return // another live-state operation (or a stuck reconnect) owns the router
	}
	r.Attempts++
	r.LastAttempt = s.now().UTC()
	if s.writeJSON(s.path, r, 0o600) != nil {
		slog.Warn("wifi reconnect: could not persist counter, not retrying")
		return
	}
	if s.guards.Acquire(GuardName) != nil {
		return
	}
	if err := s.reconnect.ReconnectUplink(ctx); err != nil {
		slog.Warn("wifi reconnect: ifup wwan failed; guard kept", "attempt", r.Attempts, "err", err)
		return
	}
	if err := s.guards.Release(GuardName); err != nil {
		slog.Warn("wifi reconnect: guard release failed", "err", err)
	}
	slog.Info("wifi reconnect: retried uplink", "attempt", r.Attempts, "max", MaxAttempts)
}
