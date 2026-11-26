package trust

import (
	"context"
	"encoding/json"
	"net/netip"
	"os"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

func nets(t *testing.T) []netip.Prefix {
	t.Helper()
	b, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	ifaces, err := facts.ParseInterfaceDump(b)
	if err != nil {
		t.Fatal(err)
	}
	return LANNets(ifaces)
}

func TestAllowedMatrix(t *testing.T) {
	lan := nets(t)
	if len(lan) == 0 {
		t.Fatal("fixture yields no lan subnet")
	}
	cases := []struct {
		name, addr string
		want       bool
	}{
		{"lan client", "192.168.1.50", true},
		{"router itself", "192.168.1.1", true},
		{"v4-mapped lan client", "::ffff:192.168.1.50", true},
		{"guest subnet", "192.168.2.20", false},
		{"uplink / wan side", "10.20.30.40", false},
		{"public", "8.8.8.8", false},
		{"loopback v4", "127.0.0.1", true},
		{"loopback v6", "::1", true},
		{"v4-mapped loopback", "::ffff:127.0.0.1", true},
		{"v6 global", "2001:db8::1", false},
		{"just outside lan", "192.168.0.255", false},
	}
	for _, c := range cases {
		if got := Allowed(netip.MustParseAddr(c.addr), lan); got != c.want {
			t.Errorf("%s (%s): got %v", c.name, c.addr, got)
		}
	}
	if Allowed(netip.Addr{}, lan) {
		t.Error("invalid address allowed")
	}
}

func TestOnlyTheLanInterfaceCounts(t *testing.T) {
	for _, p := range nets(t) {
		if p.Contains(netip.MustParseAddr("192.168.2.20")) {
			t.Fatalf("guest/other interface leaked into LAN set: %v", p)
		}
	}
}

func TestProviderBeforeFirstReadAllowsLoopbackOnly(t *testing.T) {
	var p Provider
	if !p.Allowed(netip.MustParseAddr("127.0.0.1")) || p.Allowed(netip.MustParseAddr("192.168.1.50")) {
		t.Fatal("before the first read only loopback may pass")
	}
}

func TestProviderRefreshKeepsLastGoodSetOnFailure(t *testing.T) {
	b, _ := os.ReadFile("../../testdata/device/axt1800-25.12.6/interface-dump.json")
	m := mock.New()
	m.SetUbusReply("network.interface", "dump", json.RawMessage(b))
	var p Provider
	ctx := context.Background()
	if err := p.Refresh(ctx, m); err != nil {
		t.Fatal(err)
	}
	lanClient := netip.MustParseAddr("192.168.1.50")
	if !p.Allowed(lanClient) {
		t.Fatal("lan client refused after refresh")
	}
	m.SetUbusReply("network.interface", "dump", json.RawMessage(`{"nope":1}`))
	if err := p.Refresh(ctx, m); err == nil {
		t.Fatal("bad payload must be an error")
	}
	if !p.Allowed(lanClient) {
		t.Fatal("failed refresh dropped the last good set")
	}
}
