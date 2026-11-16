package vpndns

import (
	"net/netip"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

func TestDecide(t *testing.T) {
	resolvers := []netip.Addr{netip.MustParseAddr("10.64.0.1")}
	applied := Layer(resolvers)
	other := dnslayer.Layer{Name: dnslayer.VPN, Servers: []string{"10.8.0.1"}, NoResolv: true}
	leaky := dnslayer.Layer{Name: dnslayer.VPN, Servers: []string{"10.64.0.1"}}
	st := func(s vpnstatus.State) vpnstatus.TunnelStatus { return vpnstatus.TunnelStatus{State: s} }
	for _, tc := range []struct {
		name      string
		status    vpnstatus.TunnelStatus
		resolvers []netip.Addr
		current   *dnslayer.Layer
		txnOpen   bool
		want      Action
	}{
		{"connected pushes", st(vpnstatus.Connected), resolvers, nil, false, Push},
		{"connected and applied keeps", st(vpnstatus.Connected), resolvers, &applied, false, Keep},
		{"connected with new resolvers pushes", st(vpnstatus.Connected), resolvers, &other, false, Push},
		{"connected without noresolv pushes", st(vpnstatus.Connected), resolvers, &leaky, false, Push},
		{"connected waits for confirmation", st(vpnstatus.Connected), resolvers, nil, true, Keep},
		{"connected without resolvers keeps base", st(vpnstatus.Connected), nil, nil, false, Keep},
		{"connected without resolvers pops", st(vpnstatus.Connected), nil, &applied, false, Pop},
		{"disabled pops", st(vpnstatus.Disabled), resolvers, &applied, false, Pop},
		{"configured pops during a transaction", st(vpnstatus.Configured), resolvers, &applied, true, Pop},
		{"disabled without layer keeps", st(vpnstatus.Disabled), resolvers, nil, false, Keep},
		{"not up debounces", st(vpnstatus.EnabledNotUp), resolvers, &applied, false, Keep},
		{"not up does not push", st(vpnstatus.EnabledNotUp), resolvers, nil, false, Keep},
		{"no handshake keeps", st(vpnstatus.NoHandshake), resolvers, &applied, false, Keep},
		{"stale disabled keeps", vpnstatus.TunnelStatus{State: vpnstatus.Disabled, Stale: true}, resolvers, &applied, false, Keep},
		{"stale connected keeps", vpnstatus.TunnelStatus{State: vpnstatus.Connected, Stale: true}, resolvers, nil, false, Keep},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.status, tc.resolvers, tc.current, tc.txnOpen); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLayerSetsNoResolv(t *testing.T) {
	l := Layer([]netip.Addr{netip.MustParseAddr("10.64.0.1"), netip.MustParseAddr("fd00::1")})
	if l.Name != dnslayer.VPN || !l.NoResolv || len(l.Servers) != 2 || l.Servers[1] != "fd00::1" {
		t.Fatalf("%+v", l)
	}
}
