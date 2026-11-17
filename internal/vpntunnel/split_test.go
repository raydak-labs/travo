package vpntunnel

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

func prefixes4(t *testing.T, in ...string) []netip.Prefix {
	t.Helper()
	out := make([]netip.Prefix, 0, len(in))
	for _, s := range in {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}

func TestValidateSplit(t *testing.T) {
	got, err := ValidateSplit(prefixes4(t, "10.9.0.7/16", "1.1.1.1/32"))
	if err != nil || !slices.Equal(got, prefixes4(t, "1.1.1.1/32", "10.9.0.0/16")) {
		t.Fatalf("%v %v", got, err)
	}
	for name, in := range map[string][]netip.Prefix{
		"default":   prefixes4(t, "0.0.0.0/0"),
		"ipv6":      {netip.MustParsePrefix("2001:db8::/32")},
		"duplicate": prefixes4(t, "10.0.0.0/8", "10.1.2.3/8"),
		"zero":      {{}},
	} {
		if _, err := ValidateSplit(in); !errors.Is(err, ErrInvalidSplit) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := ValidateSplit(make([]netip.Prefix, MaxSplit+1)); !errors.Is(err, ErrInvalidSplit) {
		t.Error("too many")
	}
}

func TestSplitRoutesAddsUncoveredResolvers(t *testing.T) {
	dns := []netip.Addr{netip.MustParseAddr("198.18.0.1"), netip.MustParseAddr("10.9.0.53"), netip.MustParseAddr("fd00::53")}
	got := splitRoutes(prefixes4(t, "10.9.0.0/16"), dns)
	if !slices.Equal(got, prefixes4(t, "10.9.0.0/16", "198.18.0.1/32")) {
		t.Fatalf("%v", got)
	}
	if splitRoutes(nil, dns) != nil {
		t.Fatal("full tunnel needs no routes")
	}
	if back := userSplit(got, dns); !slices.Equal(back, prefixes4(t, "10.9.0.0/16")) {
		t.Fatalf("%v", back)
	}
}

// The captured wg0 configuration (testdata/device/axt1800-25.12.6/wireguard/PROVENANCE.md).
func capturedNetwork(t *testing.T, state string) *mock.Device {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/wireguard/" + state + "/uci-show-network-wg0.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseShow("network", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	d := mock.New()
	if err := d.Load(context.Background(), "network", secs); err != nil {
		t.Fatal(err)
	}
	// The stock firewall of the same device (testdata/device/axt1800-25.12.6/PROVENANCE.md).
	raw, err = os.ReadFile("../../testdata/device/axt1800-25.12.6/uci-show-firewall.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw, err := cli.ParseShow("firewall", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Load(context.Background(), "firewall", fw); err != nil {
		t.Fatal(err)
	}
	return d
}

type stageOnly struct {
	d       *mock.Device
	probe   txn.Probe
	configs []string
}

func (f *stageOnly) BeginWireGuard(ctx context.Context, summary string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	return f.BeginWireGuardWith(ctx, summary, nil, stage, probe)
}

// BeginWireGuardWith mirrors txn: stage under one lock set, refuse an empty change, commit all.
func (f *stageOnly) BeginWireGuardWith(ctx context.Context, _ string, extra []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	configs := append([]string{"network"}, extra...)
	f.configs = configs
	revert := func() error {
		var errs []error
		for _, c := range configs {
			errs = append(errs, f.d.Revert(ctx, c))
		}
		return errors.Join(errs...)
	}
	if err := stage(ctx); err != nil {
		return txn.Status{}, errors.Join(err, revert())
	}
	changed := false
	for _, c := range configs {
		pending, _ := f.d.Pending(ctx, c)
		changed = changed || pending
	}
	if !changed {
		return txn.Status{}, txn.ErrNoChanges
	}
	f.probe = probe
	for _, c := range configs {
		if err := f.d.Commit(ctx, c); err != nil {
			return txn.Status{}, err
		}
	}
	return txn.Status{ID: "0123456789abcdef", State: txn.Pending}, nil
}

type selected struct{ cfg vpnprofiles.Config }

func (s selected) SelectedConfig() (vpnprofiles.Config, error) { return s.cfg, nil }

// uciKeys stages keys as plain options, like the real writer's end state.
type uciKeys struct{ d *mock.Device }

func (k uciKeys) SetWireGuardKey(ctx context.Context, section, option, value string) error {
	return k.d.Set(ctx, "network", section, option, value)
}

func splitService(t *testing.T, state string) (*Service, *mock.Device, *stageOnly) {
	d := capturedNetwork(t, state)
	txns := &stageOnly{d: d}
	p := &proofDevice{}
	cfg := vpnprofiles.Config{DNS: []netip.Addr{netip.MustParseAddr("198.18.0.1")}}
	return New(d, uciKeys{d}, p, p, p, p, selected{cfg}, txns, func() time.Time { return time.Unix(100, 0) }), d, txns
}

func peer(t *testing.T, d *mock.Device) device.Section {
	t.Helper()
	secs, _ := d.Show(context.Background(), "network")
	for _, s := range secs {
		if s.Type == peerType {
			return s
		}
	}
	t.Fatal("no peer")
	return device.Section{}
}

func TestSetSplitOnDisabledTunnelStagesOnly(t *testing.T) {
	s, d, txns := splitService(t, "disabled")
	ctx := context.Background()
	if _, err := s.SetSplit(ctx, prefixes4(t, "10.9.0.0/16")); err != nil {
		t.Fatal(err)
	}
	if peer(t, d).Options["route_allowed_ips"] != "0" {
		t.Fatal("split must stop routing allowed_ips")
	}
	got, err := s.Split(ctx)
	if err != nil || !slices.Equal(got.Routes, prefixes4(t, "10.9.0.0/16")) {
		t.Fatalf("%+v %v", got, err)
	}
	secs, _ := d.Show(ctx, "network")
	routes, _ := currentSplit(secs)
	if !slices.Equal(routes, prefixes4(t, "10.9.0.0/16", "198.18.0.1/32")) {
		t.Fatalf("resolver route missing: %v", routes)
	}
	if err := txns.probe(ctx); err != nil {
		t.Fatalf("disabled tunnel has nothing live to prove: %v", err)
	}
	if _, err := s.SetSplit(ctx, prefixes4(t, "10.9.0.0/16")); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("same split: %v", err)
	}
	if _, err := s.SetSplit(ctx, nil); err != nil {
		t.Fatal(err)
	}
	secs, _ = d.Show(ctx, "network")
	if routes, _ := currentSplit(secs); len(routes) != 0 || peer(t, d).Options["route_allowed_ips"] != "1" {
		t.Fatalf("full tunnel not restored: %v %v", routes, peer(t, d).Options)
	}
}

func TestSetSplitRefusesForeignOwnedName(t *testing.T) {
	s, d, _ := splitService(t, "disabled")
	ctx := context.Background()
	if err := d.SetType(ctx, "network", splitPrefix+"0", "interface"); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, "network"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSplit(ctx, prefixes4(t, "10.9.0.0/16")); !errors.Is(err, errUnavailable) {
		t.Fatalf("%v", err)
	}
}

func TestEnableKeepsSplitSelection(t *testing.T) {
	s, d, _ := splitService(t, "disabled")
	ctx := context.Background()
	if _, err := s.SetSplit(ctx, prefixes4(t, "10.9.0.0/16")); err != nil {
		t.Fatal(err)
	}
	s.profiles = selected{vpnprofiles.Config{PrivateKey: "k", Addresses: prefixes4(t, "100.64.41.140/32"),
		Peers: []vpnprofiles.Peer{{PublicKey: "p", AllowedIPs: prefixes4(t, "0.0.0.0/0")}}}}
	if _, err := s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if peer(t, d).Options["route_allowed_ips"] != "0" {
		t.Fatal("re-enable dropped the split selection")
	}
	if got, _ := s.Split(ctx); len(got.Routes) == 0 {
		t.Fatal("split routes lost")
	}
}

func TestSplitProofWantsUplinkDefault(t *testing.T) {
	d := &proofDevice{routeCalls: 1} // the first read already finds the uplink default
	s := Service{routes: d, ubus: d, control: d}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if ok, _ := s.hasUplinkDefault(ctx); !ok {
		t.Fatal("fixture default via phy0-sta0 must count as the uplink")
	}
}

func enableProfile(t *testing.T) selected {
	return selected{vpnprofiles.Config{PrivateKey: "k", Addresses: prefixes4(t, "100.64.41.140/32"),
		Peers: []vpnprofiles.Peer{{PublicKey: "p", AllowedIPs: prefixes4(t, "0.0.0.0/0")}}}}
}

func firewallHas(t *testing.T, d *mock.Device, name string) bool {
	t.Helper()
	secs, err := d.Show(context.Background(), "firewall")
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(secs, func(s device.Section) bool { return s.Name == name })
}

// LAN clients need the VPN zone whenever the tunnel is on (#87, device-facts §1).
func TestEnableStagesVPNZoneInTheSameTransaction(t *testing.T) {
	s, d, txns := splitService(t, "disabled")
	s.profiles = enableProfile(t)
	ctx := context.Background()
	before, _ := d.Show(ctx, "firewall")
	if _, err := s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(txns.configs, []string{"network", "firewall"}) {
		t.Fatalf("configs %v", txns.configs)
	}
	if !firewallHas(t, d, "travo_vpn") || !firewallHas(t, d, "travo_vpn_lan") || firewallHas(t, d, "travo_killswitch_wan") {
		t.Fatal("want the VPN zone and forwarding, no kill switch rule")
	}
	after, _ := d.Show(ctx, "firewall")
	if len(after) != len(before)+2 {
		t.Fatal("enable touched other firewall sections")
	}
	if _, err := s.Set(ctx, true); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("second enable: %v", err)
	}
	if _, err := s.Set(ctx, false); err != nil {
		t.Fatal(err)
	}
	if !firewallHas(t, d, "travo_vpn") {
		t.Fatal("disable keeps the zone")
	}
}

func TestEnableAddsOnlyMissingZoneToRunningTunnel(t *testing.T) {
	s, d, txns := splitService(t, "disabled")
	s.profiles = enableProfile(t)
	ctx := context.Background()
	if _, err := s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"travo_vpn_lan", "travo_vpn"} {
		if err := d.DeleteSection(ctx, "firewall", name); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Commit(ctx, "firewall"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Set(ctx, true); err != nil {
		t.Fatalf("zone-only repair: %v", err)
	}
	if !firewallHas(t, d, "travo_vpn") {
		t.Fatal("zone not restored")
	}
	if err := txns.probe(ctx); err != nil {
		t.Fatalf("firewall-only change must not restart wg0: %v", err)
	}
}

// Guest clients use the tunnel like LAN when the Travo guest zone exists (#89).
func TestEnableForwardsTheGuestZone(t *testing.T) {
	s, d, _ := splitService(t, "disabled")
	s.profiles = enableProfile(t)
	ctx := context.Background()
	// The zone shape internal/wifi planGuest writes.
	if err := d.SetType(ctx, "firewall", "guest", "zone"); err != nil {
		t.Fatal(err)
	}
	for option, value := range map[string]string{"name": "guest", "network": "guest", "input": "REJECT", "output": "ACCEPT", "forward": "REJECT"} {
		if err := d.Set(ctx, "firewall", "guest", option, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Commit(ctx, "firewall"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !firewallHas(t, d, "travo_vpn_guest") || firewallHas(t, d, "travo_killswitch_guest_wan") {
		t.Fatal("want the guest forwarding, no kill switch rule")
	}
}
