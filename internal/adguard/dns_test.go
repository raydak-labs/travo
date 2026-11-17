package adguard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

type dnsRig struct {
	d      *DNS
	now    time.Time
	probe  error
	route  bool
	layers *dnslayer.Service
	dev    *mock.Device
}

// commitApplier mirrors txn.Manager.Apply: stage, commit, revert on failure.
type commitApplier struct{ d *mock.Device }

func (a commitApplier) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return errors.Join(err, a.d.Revert(ctx, "dhcp"))
	}
	for _, c := range configs {
		if err := a.d.Commit(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// newDNSRig runs the real dnslayer stack on the router's captured dhcp config, whose base
// is old Travo's forwarding to 127.0.0.1#5353 (testdata/device/axt1800-25.12.6/uci-export-dhcp.txt).
func newDNSRig(t *testing.T) *dnsRig {
	t.Helper()
	b, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/uci-export-dhcp.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport("dhcp", string(b))
	if err != nil {
		t.Fatal(err)
	}
	dev := mock.New()
	if err := dev.Load(context.Background(), "dhcp", secs); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r := &dnsRig{now: time.Date(2026, 11, 16, 20, 0, 0, 0, time.UTC), route: true, dev: dev}
	r.layers = &dnslayer.Service{Device: dev, Guards: guard.Dir{Path: filepath.Join(dir, "guards"), Now: time.Now},
		RecordPath: filepath.Join(dir, "dns-layers.json"), Apply: commitApplier{dev}}
	r.d = &DNS{
		Layers: r.layers, StatePath: filepath.Join(dir, "adguard-dns.json"),
		Probe:    func(context.Context) error { return r.probe },
		HasRoute: func(context.Context) (bool, error) { return r.route, nil },
		Now:      func() time.Time { return r.now },
	}
	return r
}

func (r *dnsRig) tick(t *testing.T) DNSStatus {
	t.Helper()
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.now = r.now.Add(15 * time.Second)
	st, err := r.d.Status()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func (r *dnsRig) servers(t *testing.T) []string {
	t.Helper()
	_, st, err := dnslayer.ReadState(context.Background(), r.dev)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Server.Set {
		return nil
	}
	return st.Server.Values
}

func TestFailOpenAfterThresholdThenHoldDown(t *testing.T) {
	r := newDNSRig(t)
	if st := r.tick(t); !st.DefaultDNS || st.FailOpenSince != nil {
		t.Fatalf("old Travo forwarding must be adopted as the layer: %+v", st)
	}
	r.probe = ErrProbe // AGH stopped
	for i := 1; i < FailThreshold; i++ {
		if st := r.tick(t); !st.DefaultDNS {
			t.Fatalf("failed open after %d failures", i)
		}
	}
	st := r.tick(t)
	if st.DefaultDNS || st.FailOpenSince == nil {
		t.Fatalf("want fail open after %d failures: %+v", FailThreshold, st)
	}
	if s := r.servers(t); s != nil {
		t.Fatalf("dnsmasq still forwards to AGH: %v", s)
	}
	// AGH back: probes must pass for the whole hold-down before the layer returns.
	r.probe = nil
	for elapsed := time.Duration(0); elapsed < HoldDown; elapsed += 15 * time.Second {
		if st := r.tick(t); st.DefaultDNS {
			t.Fatalf("re-enabled after %v", elapsed)
		}
	}
	r.probe = ErrProbe // a single failure restarts the hold-down
	r.tick(t)
	r.probe = nil
	r.tick(t)
	if st := r.tick(t); st.DefaultDNS {
		t.Fatal("hold-down did not restart after a failure")
	}
	r.now = r.now.Add(HoldDown)
	if st := r.tick(t); !st.DefaultDNS || st.FailOpenSince != nil {
		t.Fatalf("want re-enabled after hold-down: %+v", st)
	}
	if s := r.servers(t); !slices.Equal(s, DNSLayer.Servers) {
		t.Fatalf("servers = %v", s)
	}
}

func TestUserConfirmReenablesImmediatelyAndUserOffSticks(t *testing.T) {
	r := newDNSRig(t)
	r.tick(t)
	r.probe = ErrProbe
	for range FailThreshold {
		r.tick(t)
	}
	if err := r.d.SetDefault(context.Background(), true); !errors.Is(err, ErrNotAnswering) {
		t.Fatalf("on while AGH is down must be refused: %v", err)
	}
	r.probe = nil
	if err := r.d.SetDefault(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if st, _ := r.d.Status(); !st.DefaultDNS || st.FailOpenSince != nil {
		t.Fatalf("%+v", st)
	}
	if err := r.d.SetDefault(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if st := r.tick(t); st.DefaultDNS {
			t.Fatal("the probe must not undo the user's off")
		}
	}
	if err := r.d.Allow(); err != nil {
		t.Fatal(err)
	}
	if st := r.tick(t); !st.DefaultDNS {
		t.Fatal("after Allow the next passing probe turns default DNS on")
	}
}

func TestTickSkipsWithoutRouteAndUnderCaptive(t *testing.T) {
	r := newDNSRig(t)
	r.tick(t)
	r.probe = ErrProbe
	r.route = false
	for range FailThreshold + 1 {
		if st := r.tick(t); !st.DefaultDNS {
			t.Fatal("no uplink is not an AGH failure")
		}
	}
	r.route = true
	if err := r.layers.Push(context.Background(), dnslayer.Layer{Name: dnslayer.Captive, Servers: []string{"10.0.0.1"}, DisableRebind: true}); err != nil {
		t.Fatal(err)
	}
	for range FailThreshold + 1 {
		if st := r.tick(t); !st.DefaultDNS {
			t.Fatal("a portal hides the Internet; no fail-open under captive")
		}
	}
}

func TestFreshInstallTurnsDefaultDNSOnOnlyOnceAGHResolves(t *testing.T) {
	r := newDNSRig(t)
	if err := r.d.SetDefault(context.Background(), false); err != nil { // base without AGH
		t.Fatal(err)
	}
	if err := r.d.Allow(); err != nil {
		t.Fatal(err)
	}
	r.probe = ErrProbe // installed but not resolving yet
	if st := r.tick(t); st.DefaultDNS {
		t.Fatal("DNS must stay at base until the probe passes")
	}
	r.probe = nil
	if st := r.tick(t); !st.DefaultDNS || st.FailOpenSince != nil {
		t.Fatalf("%+v", st)
	}
}

func TestServiceOffPopsTheLayerBeforeStoppingAGH(t *testing.T) {
	r := newDNSRig(t)
	g := newRig(t)
	g.installed(t, Seed)
	g.svc.DNS = r.d
	r.tick(t)
	if err := g.svc.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if s := r.servers(t); s != nil {
		t.Fatalf("dnsmasq still forwards to a stopped AGH: %v", s)
	}
	if !slices.Equal(g.dev.ServiceActions, []string{"adguardhome stop", "adguardhome disable"}) {
		t.Fatalf("actions = %v", g.dev.ServiceActions)
	}
	if st := r.tick(t); st.DefaultDNS {
		t.Fatal("off must stick")
	}
	if err := g.svc.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if st := r.tick(t); !st.DefaultDNS {
		t.Fatal("on: the next passing probe restores default DNS")
	}
}
