// Package captive assists with hotel portals: a time-boxed DNS bypass on the layer stack
// (invariants §8, §12). Detection lives in internal/connectivity.
package captive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
)

// RestoreAfter is how long a DNS bypass stays before the worker pops it (invariants §8).
const RestoreAfter = 5 * time.Minute

var (
	ErrNoUplink    = errors.New("no kernel default route")
	ErrNoUplinkDNS = errors.New("active uplink has no DNS servers")
)

// Layers is the slice of dnslayer.Service the bypass uses.
type Layers interface {
	Push(context.Context, dnslayer.Layer) error
	Pop(context.Context, dnslayer.Name) error
	Status() (dnslayer.Status, error)
}

// BypassStatus is the read-only view for the dashboard.
type BypassStatus struct {
	DNSBypass bool `json:"dns_bypass" doc:"The captive DNS layer is on the stack"`
	// RestoreInSeconds counts down on the daemon's monotonic clock; clients never compare clocks.
	RestoreInSeconds *int `json:"restore_in_seconds" nullable:"true" doc:"Seconds until the bypass is restored automatically; null when not armed"`
	Pending          bool `json:"pending" doc:"A DNS layer change did not finish; the bypass may still be in force"`
}

// Bypass pushes and pops the captive layer and owns its auto-restore deadline.
// The deadline is memory-only (no flash writes); after a restart Tick re-arms it.
type Bypass struct {
	layers Layers
	ubus   device.Ubus
	routes device.DefaultRouteReader
	now    func() time.Time

	op       sync.Mutex // serialises Enable, Restore and Tick; Status never waits on it
	mu       sync.Mutex
	deadline time.Time // zero = not armed
}

func NewBypass(layers Layers, ubus device.Ubus, routes device.DefaultRouteReader, now func() time.Time) *Bypass {
	return &Bypass{layers: layers, ubus: ubus, routes: routes, now: now}
}

// Enable points dnsmasq at the active uplink's resolvers with rebind protection off,
// and (re)arms the auto-restore deadline. Enabling again extends the deadline.
func (b *Bypass) Enable(ctx context.Context) error {
	servers, err := b.uplinkDNS(ctx)
	if err != nil {
		return err
	}
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.layers.Push(ctx, dnslayer.Layer{Name: dnslayer.Captive, Servers: servers, DisableRebind: true}); err != nil {
		return err
	}
	b.arm(b.now().Add(RestoreAfter))
	return nil
}

func (b *Bypass) arm(deadline time.Time) {
	b.mu.Lock()
	b.deadline = deadline
	b.mu.Unlock()
}

func (b *Bypass) armed() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.deadline
}

// Restore pops the captive layer. On failure the dnslayer guard stays and Status reports
// pending: success is never reported while the bypass could still be in force.
func (b *Bypass) Restore(ctx context.Context) error {
	b.op.Lock()
	defer b.op.Unlock()
	return b.restoreLocked(ctx)
}

func (b *Bypass) restoreLocked(ctx context.Context) error {
	if err := b.layers.Pop(ctx, dnslayer.Captive); err != nil {
		return err
	}
	b.arm(time.Time{})
	return nil
}

// Tick is the auto-restore worker step. It reads the stack (never writes) unless the
// deadline passed; a captive layer without a deadline (daemon restart) gets a fresh one.
func (b *Bypass) Tick(ctx context.Context) {
	b.op.Lock()
	defer b.op.Unlock()
	st, err := b.layers.Status()
	if err != nil || st.Corrupt != "" {
		return // fail closed: never mutate on an unreadable stack
	}
	if !slices.Contains(st.Layers, dnslayer.Captive) {
		b.arm(time.Time{})
		return
	}
	now, deadline := b.now(), b.armed()
	if deadline.IsZero() {
		b.arm(now.Add(RestoreAfter))
		return
	}
	if now.Before(deadline) {
		return
	}
	if err := b.restoreLocked(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("captive: DNS bypass auto-restore failed; retrying", "err", err)
	}
}

// Status never mutates.
func (b *Bypass) Status() BypassStatus {
	st, err := b.layers.Status()
	if err != nil || st.Corrupt != "" {
		// Unreadable stack: the bypass may be in force. Fail closed, never "absent".
		return BypassStatus{Pending: true}
	}
	out := BypassStatus{DNSBypass: slices.Contains(st.Layers, dnslayer.Captive), Pending: st.Pending}
	deadline := b.armed()
	if out.DNSBypass && !deadline.IsZero() {
		left := max(0, int((deadline.Sub(b.now())+time.Second-1)/time.Second))
		out.RestoreInSeconds = &left
	}
	return out
}

// uplinkDNS returns the resolvers of the netifd interface carrying the kernel default
// route (route truth from the kernel, device-facts §1).
func (b *Bypass) uplinkDNS(ctx context.Context) ([]string, error) {
	route, found, err := b.routes.DefaultIPv4Route(ctx)
	if err != nil {
		return nil, fmt.Errorf("captive: %w", err)
	}
	if !found {
		return nil, ErrNoUplink
	}
	raw, err := b.ubus.Call(ctx, "network.interface", "dump", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("captive: %w", err)
	}
	ifs, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return nil, fmt.Errorf("captive: %w", err)
	}
	for _, it := range ifs {
		if it.Up && it.L3Device == route.Device && len(it.DNSServers) > 0 {
			return it.DNSServers, nil
		}
	}
	return nil, ErrNoUplinkDNS
}
