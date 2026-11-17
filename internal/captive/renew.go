package captive

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// RenewGuard is held while a wwan DHCP renew is unproven (invariants §1, §12).
const RenewGuard = "captive-dhcp-renew-in-progress"

const (
	// renewSettle lets netifd act on the renew before the route is read as proof.
	renewSettle = 2 * time.Second
	// RenewWait bounds the wait for a kernel default route via wwan after the renew.
	RenewWait = 10 * time.Second
	renewPoll = 500 * time.Millisecond
)

var (
	ErrRenewNotApplicable = errors.New("DHCP renew applies only to the wwan uplink without internet")
	ErrRenewBusy          = errors.New("a previous DHCP renew did not finish")
	ErrRenewUnproven      = errors.New("no default route via wwan after the DHCP renew")
)

// RenewState is the dashboard's view of the renew action.
type RenewState string

const (
	RenewAvailable     RenewState = "available"
	RenewNotApplicable RenewState = "not_applicable"
	// RenewBlocked: the guard of an unproven renew is still present (the feature skips).
	RenewBlocked RenewState = "blocked"
)

// Connectivity is the slice of the shared connectivity service the renew uses.
type Connectivity interface {
	Status(context.Context) connectivity.Status
	Invalidate()
}

// Renewer renews the wwan DHCP lease for portals that need a fresh lease
// (device-facts §7). It never takes interfaces down and never calls `wifi`.
type Renewer struct {
	guards guard.Dir
	ubus   device.Ubus
	lease  device.LeaseRenewer
	routes device.DefaultRouteReader
	conn   Connectivity
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error

	inflight sync.Mutex // one renew at a time in this process
}

func NewRenewer(guards guard.Dir, ubus device.Ubus, lease device.LeaseRenewer, routes device.DefaultRouteReader, conn Connectivity, now func() time.Time, sleep func(context.Context, time.Duration) error) *Renewer {
	return &Renewer{guards: guards, ubus: ubus, lease: lease, routes: routes, conn: conn, now: now, sleep: sleep}
}

// Sleep waits d or until ctx ends (production sleep for NewRenewer).
func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Availability is read-only: it uses the connectivity snapshot the caller already has
// and reads netifd only when the internet is not online.
func (r *Renewer) Availability(ctx context.Context, st connectivity.Status) RenewState {
	if held, err := r.held(); err != nil || held {
		return RenewBlocked // an unreadable guard dir fails closed
	}
	if st.ActiveInterface == nil || (st.Internet != connectivity.Offline && st.Internet != connectivity.Captive) {
		return RenewNotApplicable
	}
	if dev, err := r.wwanDevice(ctx); err != nil || dev != *st.ActiveInterface {
		return RenewNotApplicable
	}
	return RenewAvailable
}

// Renew asks netifd to renew the wwan lease, then proves a kernel default route via wwan
// within RenewWait. Guard first; released when nothing happened or the route is proven.
// A guard kept by an earlier unproven renew blocks nothing automated (there is no
// automatic renew in 0.1); this explicit request is its retry path.
func (r *Renewer) Renew(ctx context.Context) error {
	if !r.inflight.TryLock() {
		return ErrRenewBusy
	}
	defer r.inflight.Unlock()
	held, err := r.held()
	if err != nil {
		return fmt.Errorf("captive: renew guard: %w", err)
	}
	if !held {
		if err := r.guards.Acquire(RenewGuard); errors.Is(err, guard.ErrActive) {
			return ErrRenewBusy // another process holds it
		} else if err != nil {
			return fmt.Errorf("captive: renew guard: %w", err)
		}
	}
	release := func(err error) error {
		return errors.Join(err, r.guards.Release(RenewGuard))
	}
	wwan, err := r.wwanDevice(ctx)
	if err != nil {
		return release(fmt.Errorf("captive: %w", err))
	}
	route, found, err := r.routes.DefaultIPv4Route(ctx)
	if err != nil {
		return release(fmt.Errorf("captive: %w", err))
	}
	if !found || route.Device != wwan {
		return release(ErrRenewNotApplicable)
	}
	if st := r.conn.Status(ctx); st.Internet != connectivity.Offline && st.Internet != connectivity.Captive {
		return release(ErrRenewNotApplicable)
	}
	if err := r.lease.RenewLease(ctx, "wwan"); err != nil {
		// A timeout or odd reply may still have reached netifd: outcome unknown, keep the guard.
		return fmt.Errorf("%w: renew: %v (guard kept)", ErrRenewUnproven, err)
	}
	// From here the lease may have changed: only proof releases the guard. A caller that
	// goes away must not strand the guard, so the bounded wait ignores its cancellation.
	ctx = context.WithoutCancel(ctx)
	defer r.conn.Invalidate()
	if err := r.sleep(ctx, renewSettle); err != nil {
		return fmt.Errorf("%w: %v (guard kept)", ErrRenewUnproven, err)
	}
	deadline := r.now().Add(RenewWait - renewSettle)
	for {
		route, found, err := r.routes.DefaultIPv4Route(ctx)
		if err == nil && found && route.Device == wwan {
			return r.guards.Release(RenewGuard)
		}
		if !r.now().Before(deadline) {
			return fmt.Errorf("%w (guard kept)", ErrRenewUnproven)
		}
		if err := r.sleep(ctx, renewPoll); err != nil {
			return fmt.Errorf("%w: %v (guard kept)", ErrRenewUnproven, err)
		}
	}
}

func (r *Renewer) held() (bool, error) {
	g, err := r.guards.Stale()
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(g, func(i guard.Info) bool { return i.Name == RenewGuard }), nil
}

// wwanDevice is the kernel device netifd runs wwan on, when wwan is up.
func (r *Renewer) wwanDevice(ctx context.Context) (string, error) {
	raw, err := r.ubus.Call(ctx, "network.interface", "dump", map[string]any{})
	if err != nil {
		return "", err
	}
	ifs, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return "", err
	}
	for _, it := range ifs {
		if it.Name == "wwan" && it.Up && it.L3Device != "" {
			return it.L3Device, nil
		}
	}
	return "", ErrRenewNotApplicable
}
