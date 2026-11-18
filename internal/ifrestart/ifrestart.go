// Package ifrestart restarts one uplink interface with `ifup <name>` under a guard and
// reports whether it came back within a bounded wait. There is no stop: a stopped uplink
// has no rollback. LAN and guest are never restartable (lockout), and `wifi` never runs
// (device-facts §3).
package ifrestart

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// GuardName is held from before `ifup` until the interface is up again or Wait ran out.
const GuardName = "interface-restart-in-progress"

// Restartable are the uplinks (invariants §7). The admin is reachable on br-lan only, so
// restarting one never cuts the caller off; a STA restart is lockout-checked for its radio.
var Restartable = []string{"wan", "wwan", "usbtether"}

const (
	DefaultWait = 30 * time.Second
	DefaultPoll = time.Second
)

var (
	ErrNotRestartable = errors.New("only uplink interfaces can be restarted")
	ErrNotFound       = errors.New("the interface does not exist")
	ErrTxnPending     = errors.New("a transaction awaits confirmation")
	// ErrBusy: another restart runs, or another live-state guard is held.
	ErrBusy        = errors.New("another live-state operation is in progress")
	ErrUnavailable = errors.New("interface restart is unavailable")
	// ErrFailed: ifup itself failed; the guard stays so the failure is visible.
	ErrFailed = errors.New("the interface restart failed")
)

// Guards is the slice of guard.Dir the service uses.
type Guards interface {
	Stale() ([]guard.Info, error)
	Acquire(name string) error
	Release(name string) error
}

// STALockout refuses a STA restart that would drop the caller's AP (wifi.Service).
type STALockout interface {
	CheckSTARestart(ctx context.Context, ip netip.Addr, acknowledge bool) error
}

// Request is one restart; Caller is the request's source address.
type Request struct {
	Name               string
	Caller             netip.Addr
	AcknowledgeLockout bool
}

// Outcome is the restart after the wait. Up=false means the interface was not back within
// Wait (no carrier, no lease); the restart itself was issued.
type Outcome struct {
	Interface      string `json:"interface"`
	Up             bool   `json:"up" doc:"The interface came up again within the wait"`
	ElapsedSeconds int64  `json:"elapsed_seconds" doc:"Seconds from the restart until up, or the whole wait"`
}

// Service restarts uplinks. Lockout may be nil on a device without WiFi.
type Service struct {
	Device  device.InterfaceRestarter
	Ubus    device.Ubus
	Guards  Guards
	Lockout STALockout
	Now     func() time.Time
	// Sleep waits d or until ctx ends; Wait and Poll bound the up check.
	Sleep      func(ctx context.Context, d time.Duration) error
	Wait, Poll time.Duration
	mu         sync.Mutex
}

// Restart validates, takes the guard, runs ifup and waits for the interface to come back.
func (s *Service) Restart(ctx context.Context, req Request) (Outcome, error) {
	if !slices.Contains(Restartable, req.Name) {
		return Outcome{}, ErrNotRestartable
	}
	if !s.mu.TryLock() {
		return Outcome{}, ErrBusy
	}
	defer s.mu.Unlock()
	if _, err := s.status(ctx, req.Name); err != nil {
		return Outcome{}, err
	}
	if req.Name == "wwan" && s.Lockout != nil {
		if err := s.Lockout.CheckSTARestart(ctx, req.Caller, req.AcknowledgeLockout); err != nil {
			return Outcome{}, err
		}
	}
	if err := s.acquire(); err != nil {
		return Outcome{}, err
	}
	start := s.Now()
	if err := s.Device.RestartInterface(ctx, req.Name); err != nil {
		return Outcome{}, errors.Join(ErrFailed, err) // guard kept: the next explicit restart clears it
	}
	up := s.waitUp(ctx, req.Name, start)
	res := Outcome{Interface: req.Name, Up: up, ElapsedSeconds: int64(s.Now().Sub(start).Round(time.Second) / time.Second)}
	if err := s.Guards.Release(GuardName); err != nil {
		return res, fmt.Errorf("release guard: %w", err)
	}
	return res, nil
}

// acquire refuses while a transaction or another live-state guard is held. Under s.mu our
// own guard can only be a crash leftover: this explicit restart is its retry path.
func (s *Service) acquire() error {
	held, err := s.Guards.Stale()
	if err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	for _, g := range held {
		switch g.Name {
		case txn.GuardName:
			return ErrTxnPending
		case GuardName:
			if err := s.Guards.Release(GuardName); err != nil {
				return errors.Join(ErrUnavailable, err)
			}
		default:
			return ErrBusy
		}
	}
	if err := s.Guards.Acquire(GuardName); err != nil {
		return errors.Join(ErrUnavailable, err) // a failed guard write aborts (invariants §1)
	}
	return nil
}

// waitUp polls netifd until the interface is up with an uptime that began after start,
// so the pre-restart state never reads as "back".
func (s *Service) waitUp(ctx context.Context, name string, start time.Time) bool {
	for {
		elapsed := s.Now().Sub(start)
		if st, err := s.status(ctx, name); err == nil && st.Up && time.Duration(st.Uptime)*time.Second <= elapsed {
			return true
		}
		if elapsed >= s.Wait || s.Sleep(ctx, s.Poll) != nil {
			return false
		}
	}
}

func (s *Service) status(ctx context.Context, name string) (facts.InterfaceStatus, error) {
	raw, err := s.Ubus.Call(ctx, "network.interface."+name, "status", nil)
	if errors.Is(err, device.ErrNotFound) {
		return facts.InterfaceStatus{}, ErrNotFound
	}
	if err != nil {
		return facts.InterfaceStatus{}, errors.Join(ErrUnavailable, err)
	}
	st, err := facts.ParseInterfaceStatus(raw)
	if err != nil {
		return facts.InterfaceStatus{}, errors.Join(ErrUnavailable, err)
	}
	return st, nil
}

// SleepContext is the production Sleep.
func SleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
