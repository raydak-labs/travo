package captive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

const fixtures = "../../testdata/device/axt1800-25.12.6/"

type routes struct {
	route device.IPv4DefaultRoute
	err   error
}

func (r *routes) DefaultIPv4Route(context.Context) (device.IPv4DefaultRoute, bool, error) {
	return r.route, r.route.Device != "", r.err
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

// commitApplier mirrors txn.Manager.Apply: stage, then commit; revert on failure.
type commitApplier struct {
	d    *mock.Device
	fail bool
}

func (a *commitApplier) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return errors.Join(err, a.d.Revert(ctx, "dhcp"))
	}
	if a.fail {
		return errors.Join(errors.New("apply failed"), a.d.Revert(ctx, "dhcp"))
	}
	for _, c := range configs {
		if err := a.d.Commit(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

type rig struct {
	bypass *Bypass
	dev    *mock.Device
	layers *dnslayer.Service
	apply  *commitApplier
	routes *routes
	clock  *clock
}

// newRig is the captured AXT1800: dnsmasq from `uci export dhcp`, netifd from
// `network.interface dump` (wwan on phy0-sta0 with DNS 192.168.90.2).
func newRig(t *testing.T) rig {
	t.Helper()
	dev := mock.New()
	export, err := os.ReadFile(fixtures + "uci-export-dhcp.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport("dhcp", string(export))
	if err != nil {
		t.Fatal(err)
	}
	if err := dev.Load(t.Context(), "dhcp", secs); err != nil {
		t.Fatal(err)
	}
	dump, err := os.ReadFile(fixtures + "interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	dev.SetUbusReply("network.interface", "dump", dump)
	dir := t.TempDir()
	apply := &commitApplier{d: dev}
	layers := &dnslayer.Service{
		Device: dev, Apply: apply,
		Guards:     guard.Dir{Path: filepath.Join(dir, "guards"), Now: time.Now},
		RecordPath: filepath.Join(dir, "dns-layers.json"),
	}
	r := rig{dev: dev, layers: layers, apply: apply, routes: &routes{route: device.IPv4DefaultRoute{Device: "phy0-sta0"}}, clock: &clock{t: time.Unix(1000, 0)}}
	r.bypass = NewBypass(layers, dev, r.routes, r.clock.now)
	return r
}

func dnsmasq(t *testing.T, d *mock.Device) dnslayer.State {
	t.Helper()
	_, st, err := dnslayer.ReadState(t.Context(), d)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestEnablePushesUplinkResolverWithRebindOff(t *testing.T) {
	r := newRig(t)
	base := dnsmasq(t, r.dev)
	if err := r.bypass.Enable(t.Context()); err != nil {
		t.Fatal(err)
	}
	st := dnsmasq(t, r.dev)
	if !slices.Equal(st.Server.Values, []string{"192.168.90.2"}) || st.RebindProtection != (dnslayer.Opt{Set: true, Value: "0"}) {
		t.Fatalf("bypass state %+v", st)
	}
	got := r.bypass.Status()
	if !got.DNSBypass || got.Pending || got.RestoreInSeconds == nil || *got.RestoreInSeconds != 300 {
		t.Fatalf("status %+v", got)
	}
	if err := r.bypass.Restore(t.Context()); err != nil {
		t.Fatal(err)
	}
	if after := dnsmasq(t, r.dev); after.Server.Set != base.Server.Set || !slices.Equal(after.Server.Values, base.Server.Values) || after.RebindProtection != base.RebindProtection || after.NoResolv != base.NoResolv {
		t.Fatalf("restore: got %+v want %+v", after, base)
	}
	if got := r.bypass.Status(); got.DNSBypass || got.RestoreInSeconds != nil {
		t.Fatalf("after restore %+v", got)
	}
}

func TestEnableRefusesWithoutUplinkDNS(t *testing.T) {
	r := newRig(t)
	r.routes.route = device.IPv4DefaultRoute{}
	if err := r.bypass.Enable(t.Context()); !errors.Is(err, ErrNoUplink) {
		t.Fatalf("no route: %v", err)
	}
	r.routes.route = device.IPv4DefaultRoute{Device: "br-lan"} // lan has no dns-server
	if err := r.bypass.Enable(t.Context()); !errors.Is(err, ErrNoUplinkDNS) {
		t.Fatalf("no dns: %v", err)
	}
	r.routes.route, r.routes.err = device.IPv4DefaultRoute{Device: "phy0-sta0"}, errors.New("ip failed")
	if err := r.bypass.Enable(t.Context()); err == nil || errors.Is(err, ErrNoUplink) {
		t.Fatalf("route read error must not read as no uplink: %v", err)
	}
	if st, _ := r.layers.Status(); len(st.Layers) != 0 || st.Pending {
		t.Fatalf("refusal touched the stack: %+v", st)
	}
}

func TestTickRestoresAfterFiveMinutes(t *testing.T) {
	r := newRig(t)
	base := dnsmasq(t, r.dev)
	if err := r.bypass.Enable(t.Context()); err != nil {
		t.Fatal(err)
	}
	r.clock.t = r.clock.t.Add(RestoreAfter - time.Second)
	r.bypass.Tick(t.Context())
	if got := r.bypass.Status(); !got.DNSBypass || *got.RestoreInSeconds != 1 {
		t.Fatalf("before deadline %+v", got)
	}
	r.clock.t = r.clock.t.Add(time.Second)
	r.bypass.Tick(t.Context())
	if got := r.bypass.Status(); got.DNSBypass {
		t.Fatalf("not restored %+v", got)
	}
	if after := dnsmasq(t, r.dev); after.RebindProtection != base.RebindProtection || !slices.Equal(after.Server.Values, base.Server.Values) {
		t.Fatalf("restore %+v", after)
	}
}

func TestEnableAgainExtendsDeadline(t *testing.T) {
	r := newRig(t)
	if err := r.bypass.Enable(t.Context()); err != nil {
		t.Fatal(err)
	}
	r.clock.t = r.clock.t.Add(4 * time.Minute)
	if err := r.bypass.Enable(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := r.bypass.Status(); *got.RestoreInSeconds != 300 {
		t.Fatalf("not extended %+v", got)
	}
}

func TestTickRearmsAfterRestart(t *testing.T) {
	r := newRig(t)
	if err := r.bypass.Enable(t.Context()); err != nil {
		t.Fatal(err)
	}
	restarted := NewBypass(r.layers, r.dev, r.routes, r.clock.now)
	if got := restarted.Status(); !got.DNSBypass || got.RestoreInSeconds != nil {
		t.Fatalf("fresh process: %+v", got)
	}
	restarted.Tick(t.Context()) // arms, never pops on the first sight
	if got := restarted.Status(); !got.DNSBypass || *got.RestoreInSeconds != 300 {
		t.Fatalf("re-armed: %+v", got)
	}
	r.clock.t = r.clock.t.Add(RestoreAfter)
	restarted.Tick(t.Context())
	if restarted.Status().DNSBypass {
		t.Fatal("re-armed deadline did not restore")
	}
}

func TestFailedRestoreKeepsBypassVisible(t *testing.T) {
	r := newRig(t)
	if err := r.bypass.Enable(t.Context()); err != nil {
		t.Fatal(err)
	}
	r.apply.fail = true
	if err := r.bypass.Restore(t.Context()); err == nil {
		t.Fatal("restore reported success")
	}
	// The last pop marked the record restoring before it failed: the layer is gone from
	// the record, but the guard keeps Pending set until the explicit retry path runs.
	if got := r.bypass.Status(); !got.Pending {
		t.Fatalf("failed restore must stay visible: %+v", got)
	}
	r.clock.t = r.clock.t.Add(RestoreAfter)
	r.apply.fail = false
	r.bypass.Tick(t.Context()) // the worker never resumes: only the explicit retry does
	if got := r.bypass.Status(); !got.Pending {
		t.Fatalf("tick hid the failure: %+v", got)
	}
	if err := r.bypass.Restore(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := r.bypass.Status(); got.Pending || got.DNSBypass {
		t.Fatalf("explicit retry did not finish: %+v", got)
	}
	if after := dnsmasq(t, r.dev); after.RebindProtection.Set && after.RebindProtection.Value == "0" {
		t.Fatalf("bypass still in force: %+v", after)
	}
}

func TestCorruptRecordReadsPendingAndTickDoesNotWrite(t *testing.T) {
	r := newRig(t)
	if err := os.WriteFile(r.layers.RecordPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := r.bypass.Status(); !got.Pending {
		t.Fatalf("corrupt record read as absent: %+v", got)
	}
	r.bypass.Tick(t.Context())
	if b, _ := os.ReadFile(r.layers.RecordPath); string(b) != "{" {
		t.Fatalf("tick rewrote a corrupt record: %q", b)
	}
}

func TestStatusAndTickNeverWriteWithoutBypass(t *testing.T) {
	r := newRig(t)
	r.bypass.Tick(t.Context())
	if got := r.bypass.Status(); got.DNSBypass || got.RestoreInSeconds != nil || got.Pending {
		t.Fatalf("idle %+v", got)
	}
	if pending, err := r.dev.Pending(t.Context(), "dhcp"); err != nil || pending {
		t.Fatalf("idle tick staged dhcp: %v %v", pending, err)
	}
	if _, err := os.Stat(r.layers.RecordPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("idle tick wrote a record: %v", err)
	}
}
