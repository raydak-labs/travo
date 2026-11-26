// Package trust decides whether a request source may reach the admin API (ADR 0008):
// br-lan subnets and loopback only. Uplink, guest and unknown sources are refused.
package trust

import (
	"context"
	"net/netip"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// Allowed is pure: remote must be loopback or inside one of lanNets.
func Allowed(remote netip.Addr, lanNets []netip.Prefix) bool {
	remote = remote.Unmap() // dual-stack listeners report v4 peers as ::ffff:a.b.c.d
	if !remote.IsValid() {
		return false
	}
	if remote.IsLoopback() {
		return true
	}
	for _, p := range lanNets {
		if p.Masked().Contains(remote) {
			return true
		}
	}
	return false
}

// LANNets extracts the br-lan subnets from a parsed interface dump (interface named lan).
func LANNets(ifaces []facts.Interface) []netip.Prefix {
	var out []netip.Prefix
	for _, i := range ifaces {
		if i.Name != "lan" {
			continue
		}
		for _, p := range i.IPv4 {
			out = append(out, p.Masked())
		}
	}
	return out
}

// LANAddrs extracts the router's own host addresses on the lan interface (not masked).
func LANAddrs(ifaces []facts.Interface) []netip.Addr {
	var out []netip.Addr
	for _, i := range ifaces {
		if i.Name != "lan" {
			continue
		}
		for _, p := range i.IPv4 {
			out = append(out, p.Addr())
		}
	}
	return out
}

// Provider holds the last good LAN subnet set and the router's own lan addresses.
type Provider struct {
	mu    sync.RWMutex
	nets  []netip.Prefix
	addrs []netip.Addr
}

// RouterIPs are the router's lan addresses from the last good read; empty before the first one.
func (p *Provider) RouterIPs() []netip.Addr {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]netip.Addr(nil), p.addrs...)
}

// Allowed uses the last good set; before the first successful read only loopback passes.
func (p *Provider) Allowed(remote netip.Addr) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return Allowed(remote, p.nets)
}

// Refresh reads network.interface dump; on failure the previous set stays.
func (p *Provider) Refresh(ctx context.Context, u device.Ubus) error {
	raw, err := u.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return err
	}
	ifaces, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return err
	}
	nets, addrs := LANNets(ifaces), LANAddrs(ifaces)
	p.mu.Lock()
	p.nets, p.addrs = nets, addrs
	p.mu.Unlock()
	return nil
}
