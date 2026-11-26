package ipv6settings

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

const captures = "../../testdata/device/axt1800-25.12.6/"

// export parses a verbatim `uci export` capture: wan6 enabled, wg0 disabled, no wwan6;
// dhcp.lan with ra/dhcpv6 server.
func export(t *testing.T, config, file string) []device.Section {
	t.Helper()
	b, err := os.ReadFile(captures + file)
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport(config, string(b))
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

func fixtures(t *testing.T) (network, dhcp []device.Section) {
	return export(t, "network", "wan/uci-export-network.txt"), export(t, "dhcp", "dhcp/uci-export-dhcp.txt")
}

func edit(secs []device.Section, name string, opts map[string]string) []device.Section {
	out := slices.Clone(secs)
	i := slices.IndexFunc(out, func(s device.Section) bool { return s.Name == name })
	if i < 0 {
		return append(out, device.Section{Name: name, Type: "interface", Options: opts})
	}
	o := map[string]string{}
	for k, v := range out[i].Options {
		o[k] = v
	}
	for k, v := range opts {
		o[k] = v
	}
	out[i].Options = o
	return out
}

func TestReadCaptured(t *testing.T) {
	network, dhcp := fixtures(t)
	v, err := Read(network, dhcp)
	if err != nil || !v.Enabled {
		t.Fatalf("captured = %+v, %v; want enabled", v, err)
	}
	for name, c := range map[string]struct{ net, dhcp []device.Section }{
		"ra relay":     {network, edit(dhcp, LAN, map[string]string{"ra": "relay"})},
		"ndp relay":    {network, edit(dhcp, LAN, map[string]string{"ndp": "relay"})},
		"no dhcp.lan":  {network, nil},
		"user wwan6":   {edit(network, WWAN6, map[string]string{"proto": "static"}), dhcp},
		"wwan6 on wan": {edit(network, WWAN6, map[string]string{"proto": "dhcpv6", "device": "wan"}), dhcp},
	} {
		if _, err := Read(c.net, c.dhcp); !errors.Is(err, ErrForeign) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestPlan(t *testing.T) {
	network, dhcp := fixtures(t)
	off, err := Plan(network, dhcp, Request{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	want := []Write{
		{"dhcp", LAN, "set", "ra", "disabled"}, {"dhcp", LAN, "set", "dhcpv6", "disabled"},
		{"network", WAN6, "set", "disabled", "1"},
	}
	if !slices.Equal(off, want) {
		t.Errorf("off = %+v", off)
	}
	on, err := Plan(network, dhcp, Request{Enabled: true})
	want = []Write{
		{"network", WWAN6, "type", "", "interface"}, {"network", WWAN6, "set", "proto", "dhcpv6"},
		{"network", WWAN6, "set", "device", WWAN6Dev},
	}
	if err != nil || !slices.Equal(on, want) {
		t.Errorf("on = %+v, %v", on, err)
	}
	vpn := edit(network, Tunnel, map[string]string{"disabled": "0"})
	if _, err := Plan(vpn, dhcp, Request{Enabled: true}); !errors.Is(err, ErrVPNLeak) {
		t.Errorf("tunnel on: err = %v", err)
	}
	if w, err := Plan(vpn, dhcp, Request{Enabled: false}); err != nil || len(w) == 0 {
		t.Errorf("turning off with the tunnel on must be allowed: %+v, %v", w, err)
	}
}

type fakeTxns struct{ d *mock.Device }

func (f *fakeTxns) Begin(ctx context.Context, _ string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	if !slices.Equal(configs, Configs) {
		return txn.Status{}, errors.New("unexpected configs")
	}
	if err := stage(ctx); err != nil {
		return txn.Status{}, err
	}
	for _, c := range configs {
		if err := f.d.Commit(ctx, c); err != nil {
			return txn.Status{}, err
		}
	}
	if err := probe(ctx); err != nil {
		return txn.Status{State: txn.RolledBack}, txn.ErrProbeFailed
	}
	return txn.Status{State: txn.Pending}, nil
}

type routes struct {
	mu  sync.Mutex
	n   int
	seq []device.IPv4DefaultRoute
}

func (r *routes) DefaultIPv4Route(context.Context) (device.IPv4DefaultRoute, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	at := min(r.n, len(r.seq)-1)
	r.n++
	return r.seq[at], r.seq[at].Device != "", nil
}

var sta = device.IPv4DefaultRoute{Device: "phy0-sta0", Gateway: "10.0.1.1"}

func setup(t *testing.T, seq ...device.IPv4DefaultRoute) (*Service, *mock.Device) {
	t.Helper()
	d := mock.New()
	network, dhcp := fixtures(t)
	if err := d.Load(t.Context(), "network", network); err != nil {
		t.Fatal(err)
	}
	if err := d.Load(t.Context(), "dhcp", dhcp); err != nil {
		t.Fatal(err)
	}
	return &Service{UCI: d, Routes: &routes{seq: seq}, Txns: &fakeTxns{d: d},
		ProbeWait: time.Second, ProbeSettle: time.Millisecond, Tick: time.Millisecond}, d
}

func TestSetOffThenOn(t *testing.T) {
	s, _ := setup(t, sta)
	if st, err := s.Set(t.Context(), Request{Enabled: false}); err != nil || st.State != txn.Pending {
		t.Fatalf("off: %+v, %v", st, err)
	}
	if v, _ := s.Settings(t.Context()); v.Enabled {
		t.Error("still enabled after off")
	}
	if _, err := s.Set(t.Context(), Request{Enabled: false}); !errors.Is(err, txn.ErrNoChanges) {
		t.Errorf("repeat off: err = %v", err)
	}
	if _, err := s.Set(t.Context(), Request{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	network, dhcp, _ := s.read(t.Context())
	if w, _ := Plan(network, dhcp, Request{Enabled: true}); len(w) != 0 {
		t.Errorf("on after on still plans %+v", w)
	}
	if w, _ := find(network, WWAN6); !ownWWAN6(w) || w.Options["disabled"] != "" {
		t.Errorf("wwan6 = %+v", w)
	}
}

func TestProbeRefusesChangedUplink(t *testing.T) {
	s, _ := setup(t, sta, device.IPv4DefaultRoute{})
	if _, err := s.Set(t.Context(), Request{Enabled: false}); !errors.Is(err, txn.ErrProbeFailed) {
		t.Errorf("lost route: err = %v", err)
	}
}

func TestUnchanged(t *testing.T) {
	other := device.IPv4DefaultRoute{Device: "wan", Gateway: "10.0.1.1"}
	for _, c := range []struct {
		r, before     device.IPv4DefaultRoute
		found, had, w bool
	}{
		{sta, sta, true, true, true},
		{other, sta, true, true, false},
		{sta, sta, false, true, false},
		{device.IPv4DefaultRoute{}, device.IPv4DefaultRoute{}, false, false, true},
	} {
		if got := Unchanged(c.r, c.found, c.before, c.had); got != c.w {
			t.Errorf("%+v: %v", c, got)
		}
	}
}
