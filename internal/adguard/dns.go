package adguard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
)

// DNSLayer is "default DNS" on: dnsmasq forwards everything to AGH (ADR 0006).
var DNSLayer = dnslayer.Layer{Name: dnslayer.AdGuard, Servers: []string{LoopbackDNS + "#" + DNSPort}, NoResolv: true}

const (
	// FailThreshold consecutive probe failures pop the layer (fail open).
	FailThreshold = 3
	// HoldDown of continuously passing probes re-enables a failed-open layer by itself.
	HoldDown = 10 * time.Minute
)

// ErrNotAnswering refuses "default DNS" on while AGH does not resolve: DNS stays at base.
var ErrNotAnswering = errors.New("AdGuard Home is not resolving; DNS stays at the uplink resolver")

// Layers is the slice of dnslayer.Service the controller uses; it never writes dnsmasq itself.
type Layers interface {
	Push(context.Context, dnslayer.Layer) error
	Pop(context.Context, dnslayer.Name) error
	Adopt(context.Context, dnslayer.Layer) (bool, error)
	Status() (dnslayer.Status, error)
}

// dnsState is persisted only on user choices and fail-open events (rare flash writes).
type dnsState struct {
	// UserOff: the user turned default DNS off; nothing re-enables it but the user.
	UserOff bool `json:"user_off,omitempty"`
	// FailOpenSince: the probe popped the layer; it returns on confirm or after HoldDown.
	FailOpenSince *time.Time `json:"fail_open_since,omitempty"`
}

// DNS owns the `adguard` layer: default on once AGH resolves, fail open when it stops.
type DNS struct {
	Layers    Layers
	StatePath string
	// Probe resolves a fresh name via 127.0.0.1:5353 (UDPProbe in production).
	Probe func(context.Context) error
	// HasRoute reads the kernel default route: without an uplink every probe would fail.
	HasRoute func(context.Context) (bool, error)
	Now      func() time.Time

	mu           sync.Mutex
	adopted      bool
	failures     int
	passingSince time.Time
}

// DNSStatus is what the UI shows.
type DNSStatus struct {
	DefaultDNS    bool       `json:"default_dns" doc:"dnsmasq forwards to AdGuard Home (the adguard DNS layer is active)"`
	FailOpenSince *time.Time `json:"fail_open_since" nullable:"true" doc:"Set while the layer is off because AdGuard Home stopped resolving"`
}

// Status never mutates.
func (d *DNS) Status() (DNSStatus, error) {
	ls, err := d.Layers.Status()
	if err != nil {
		return DNSStatus{}, err
	}
	st, err := d.load()
	if err != nil {
		return DNSStatus{}, err
	}
	return DNSStatus{DefaultDNS: slices.Contains(ls.Layers, dnslayer.AdGuard), FailOpenSince: st.FailOpenSince}, nil
}

// SetDefault is the user's toggle. On needs a passing probe first and clears a fail-open.
func (d *DNS) SetDefault(ctx context.Context, on bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.failures, d.passingSince = 0, time.Time{}
	if err := d.adopt(ctx); err != nil {
		return err
	}
	if !on {
		if err := d.Layers.Pop(ctx, dnslayer.AdGuard); err != nil {
			return err
		}
		return d.save(dnsState{UserOff: true})
	}
	if err := d.Probe(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrNotAnswering, err)
	}
	if err := d.Layers.Push(ctx, DNSLayer); err != nil {
		return err
	}
	return d.save(dnsState{})
}

// Allow clears a user "off" without pushing; Tick pushes after the next passing probe.
func (d *DNS) Allow() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	st, err := d.load()
	if err != nil || !st.UserOff {
		return err
	}
	st.UserOff = false
	return d.save(st)
}

// Tick is one liveness round (lifecycle worker). It pushes the layer the first time AGH
// resolves, pops it after FailThreshold failures, and re-pushes after HoldDown of passes.
func (d *DNS) Tick(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.adopt(ctx); err != nil {
		return err
	}
	ls, err := d.Layers.Status()
	if err != nil {
		return err
	}
	// An unfinished DNS change belongs to dnslayer's retry; a portal hides the Internet.
	if ls.Pending || ls.Corrupt != "" || slices.Contains(ls.Layers, dnslayer.Captive) {
		return nil
	}
	if up, err := d.HasRoute(ctx); err != nil || !up {
		d.failures, d.passingSince = 0, time.Time{}
		return err
	}
	st, err := d.load()
	if err != nil {
		return err
	}
	perr := d.Probe(ctx)
	switch {
	case slices.Contains(ls.Layers, dnslayer.AdGuard):
		return d.watchActive(ctx, perr)
	case st.UserOff:
		return nil
	case perr != nil:
		d.passingSince = time.Time{}
		return nil
	case st.FailOpenSince != nil && d.holding():
		return nil
	}
	if err := d.Layers.Push(ctx, DNSLayer); err != nil {
		return err
	}
	slog.Info("adguard: default DNS on", "after_fail_open", st.FailOpenSince != nil)
	d.passingSince = time.Time{}
	return d.save(dnsState{})
}

// adopt records forwarding written before the stack existed (old Travo) as the layer, once.
func (d *DNS) adopt(ctx context.Context) error {
	if d.adopted {
		return nil
	}
	if ls, err := d.Layers.Status(); err != nil || ls.Pending {
		return err // Adopt skips while a change is unfinished; try again next time
	}
	if _, err := d.Layers.Adopt(ctx, DNSLayer); err != nil {
		return fmt.Errorf("adguard: adopt existing forwarding: %w", err)
	}
	d.adopted = true
	return nil
}

func (d *DNS) watchActive(ctx context.Context, perr error) error {
	if perr == nil {
		d.failures = 0
		return nil
	}
	d.failures++
	if d.failures < FailThreshold {
		return nil
	}
	d.failures = 0
	if err := d.Layers.Pop(ctx, dnslayer.AdGuard); err != nil {
		return fmt.Errorf("adguard: fail open: %w", err)
	}
	now := d.Now().UTC()
	slog.Warn("adguard: failed open; DNS uses the uplink resolver", "probe", perr)
	return d.save(dnsState{FailOpenSince: &now})
}

// holding is true until probes have passed continuously for HoldDown.
func (d *DNS) holding() bool {
	now := d.Now()
	if d.passingSince.IsZero() {
		d.passingSince = now
	}
	return now.Sub(d.passingSince) < HoldDown
}

func (d *DNS) load() (dnsState, error) {
	var st dnsState
	err := atomicfile.ReadJSON(d.StatePath, &st)
	if errors.Is(err, os.ErrNotExist) {
		return dnsState{}, nil
	}
	return st, err
}

func (d *DNS) save(st dnsState) error {
	if st == (dnsState{}) {
		if err := os.Remove(d.StatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return atomicfile.WriteJSON(d.StatePath, st, 0o600)
}
