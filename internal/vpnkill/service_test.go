package vpnkill

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

var ctx = context.Background()

// Captured `uci show firewall` of the AXT1800 (testdata/device/axt1800-25.12.6/PROVENANCE.md).
const firewallFixture = "../../testdata/device/axt1800-25.12.6/uci-show-firewall.txt"

func fixture(t *testing.T) []device.Section {
	t.Helper()
	raw, err := os.ReadFile(firewallFixture)
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseShow("firewall", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

// fakeTxns mirrors txn.Manager.Begin: stage, refuse an empty change, commit, probe.
type fakeTxns struct {
	d        *mock.Device
	summary  string
	probeErr error
}

func (f *fakeTxns) Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	f.summary = summary
	if err := stage(ctx); err != nil {
		return txn.Status{}, errors.Join(err, f.d.Revert(ctx, "firewall"))
	}
	if pending, _ := f.d.Pending(ctx, "firewall"); !pending {
		return txn.Status{}, txn.ErrNoChanges
	}
	if err := f.d.Commit(ctx, "firewall"); err != nil {
		return txn.Status{}, err
	}
	if probe != nil {
		if f.probeErr = probe(ctx); f.probeErr != nil {
			return txn.Status{State: txn.RolledBack}, nil
		}
	}
	return txn.Status{ID: "0123456789abcdef", State: txn.Pending}, nil
}

type tunnel struct{ st vpnstatus.TunnelStatus }

func (t *tunnel) Status(context.Context) (vpnstatus.TunnelStatus, error) { return t.st, nil }

func setup(t *testing.T) (*Service, *mock.Device, *tunnel, *fakeTxns) {
	t.Helper()
	d := mock.New()
	if err := d.Load(ctx, "firewall", fixture(t)); err != nil {
		t.Fatal(err)
	}
	tun := &tunnel{st: vpnstatus.TunnelStatus{State: vpnstatus.Connected}}
	txns := &fakeTxns{d: d}
	return &Service{UCI: d, Tunnel: tun, Txns: txns, ProbeWait: 50 * time.Millisecond}, d, tun, txns
}

func show(t *testing.T, d *mock.Device) []device.Section {
	t.Helper()
	secs, err := d.Show(ctx, "firewall")
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

func TestDesiredOnDeviceFixture(t *testing.T) {
	want, err := Desired(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range want {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{ZoneName, LANForward, RulePrefix + "wan"}) {
		t.Fatalf("%v", names)
	}
	rule := want[2]
	if rule.Options["src"] != "lan" || rule.Options["dest"] != "wan" || rule.Options["target"] != "REJECT" || rule.Options["proto"] != "all" {
		t.Fatalf("%+v", rule)
	}
	if _, family := rule.Options["family"]; family {
		t.Fatal("the rule must cover IPv4 and IPv6")
	}
}

func TestDesiredRefusesUnsupportedLayouts(t *testing.T) {
	base := fixture(t)
	without := func(zone string) []device.Section {
		return slices.DeleteFunc(slices.Clone(base), func(s device.Section) bool { return s.Type == "zone" && s.Options["name"] == zone })
	}
	for name, secs := range map[string][]device.Section{
		"no lan zone":    without("lan"),
		"no uplink zone": without("wan"),
		"foreign zone name": append(slices.Clone(base), device.Section{Name: "cfg99", Type: "zone",
			Options: map[string]string{"name": ZoneName}}),
	} {
		if _, err := Desired(secs); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestDesiredCoversEveryUplinkZone(t *testing.T) {
	secs := append(fixture(t), device.Section{Name: "cfg77", Type: "zone",
		Options: map[string]string{"name": "tether"}, Lists: map[string][]string{"network": {"usbtether"}}})
	want, err := Desired(secs)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 4 || want[2].Name != RulePrefix+"tether" || want[3].Name != RulePrefix+"wan" {
		t.Fatalf("%+v", want)
	}
}

func TestEnableDisableRoundTrip(t *testing.T) {
	svc, d, _, txns := setup(t)
	before := show(t, d)
	st, err := svc.Set(ctx, true)
	if err != nil || st.State != txn.Pending || txns.summary != "Enable VPN kill switch" {
		t.Fatalf("%+v %v", st, err)
	}
	state, err := svc.Status(ctx)
	if err != nil || !state.Enabled || !state.Zone {
		t.Fatalf("%+v %v", state, err)
	}
	foreign := slices.DeleteFunc(show(t, d), func(x device.Section) bool { return Owned(x.Name) })
	if !reflect.DeepEqual(foreign, before) {
		t.Fatal("a section Travo does not own changed")
	}
	if _, err := svc.Set(ctx, true); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("second enable: %v", err)
	}
	if _, err := svc.Set(ctx, false); err != nil {
		t.Fatal(err)
	}
	state, _ = svc.Status(ctx)
	if state.Enabled || !state.Zone {
		t.Fatalf("disable must keep the VPN zone: %+v", state)
	}
	if _, err := svc.Set(ctx, false); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("second disable: %v", err)
	}
	if len(show(t, d)) != len(before)+2 {
		t.Fatal("only the VPN zone and forwarding may remain")
	}
}

func TestEnableNeedsConnectedTunnel(t *testing.T) {
	for _, st := range []vpnstatus.TunnelStatus{
		{State: vpnstatus.Disabled}, {State: vpnstatus.Configured}, {State: vpnstatus.EnabledNotUp},
		{State: vpnstatus.NoHandshake}, {State: vpnstatus.Connected, Stale: true},
	} {
		svc, d, tun, _ := setup(t)
		tun.st = st
		if _, err := svc.Set(ctx, true); !errors.Is(err, ErrNotConnected) {
			t.Fatalf("%+v: %v", st, err)
		}
		if Enabled(show(t, d)) {
			t.Fatal("rules written")
		}
	}
}

func TestDisableNeedsNoTunnel(t *testing.T) {
	svc, d, tun, _ := setup(t)
	if _, err := svc.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	tun.st = vpnstatus.TunnelStatus{State: vpnstatus.Disabled}
	if _, err := svc.Set(ctx, false); err != nil || Enabled(show(t, d)) {
		t.Fatalf("disable while the tunnel is down: %v", err)
	}
}

func TestEnableProbeFailsWhenTunnelDrops(t *testing.T) {
	svc, _, tun, txns := setup(t)
	svc.Tunnel = &dropping{tun}
	st, err := svc.Set(ctx, true)
	if err != nil || st.State != txn.RolledBack || !errors.Is(txns.probeErr, ErrNotConnected) {
		t.Fatalf("%+v %v %v", st, err, txns.probeErr)
	}
}

// dropping reports connected for the precondition, then no handshake after the reload.
type dropping struct{ t *tunnel }

func (d *dropping) Status(context.Context) (vpnstatus.TunnelStatus, error) {
	st := d.t.st
	d.t.st = vpnstatus.TunnelStatus{State: vpnstatus.NoHandshake}
	return st, nil
}

func TestForeignSectionOnOwnedNameIsRefused(t *testing.T) {
	svc, d, _, _ := setup(t)
	if err := d.SetType(ctx, "firewall", LANForward, "rule"); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, "firewall"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Set(ctx, true); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("%v", err)
	}
}
