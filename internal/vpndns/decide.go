// Package vpndns keeps the `vpn` DNS layer in step with the WireGuard tunnel and
// verifies that resolution goes through the tunnel resolvers (spec item 8, invariants §8, §11).
//
// The tunnel transaction never touches DNS (invariants §11). A guarded reconciler pushes
// the layer once a handshake proves the tunnel and pops it when the tunnel is terminally
// off; transient states (`enabled_not_up`, `no_handshake`, stale reads) change nothing.
package vpndns

import (
	"net/netip"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

// Action is what the reconciler does to the `vpn` layer.
type Action int

const (
	Keep Action = iota
	Push
	Pop
)

// Layer builds the `vpn` layer. noresolv=1 stops dnsmasq from also asking the uplink
// resolvers from resolv.conf.auto, which would leak queries outside the tunnel.
func Layer(resolvers []netip.Addr) dnslayer.Layer {
	return dnslayer.Layer{Name: dnslayer.VPN, Servers: servers(resolvers), NoResolv: true}
}

func servers(resolvers []netip.Addr) []string {
	out := make([]string, 0, len(resolvers))
	for _, r := range resolvers {
		out = append(out, r.String())
	}
	return out
}

// Decide is pure: tunnel status, the selected profile's resolvers and the current `vpn`
// layer (nil when absent) in, action out. txnOpen means a tunnel transaction awaits
// confirmation: only the fail-safe Pop (back to the uplink resolvers) may run then.
func Decide(st vpnstatus.TunnelStatus, resolvers []netip.Addr, current *dnslayer.Layer, txnOpen bool) Action {
	if st.Stale {
		return Keep // a broken read is not a broken tunnel
	}
	switch st.State {
	case vpnstatus.Disabled, vpnstatus.Configured:
		if current != nil {
			return Pop
		}
	case vpnstatus.Connected:
		if len(resolvers) == 0 {
			if current != nil {
				return Pop
			}
			return Keep
		}
		want := Layer(resolvers)
		if !txnOpen && (current == nil || !current.NoResolv || !slices.Equal(current.Servers, want.Servers)) {
			return Push
		}
	}
	return Keep
}
