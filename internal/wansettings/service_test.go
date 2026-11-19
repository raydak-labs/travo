package wansettings

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// fakeTxns stages, commits and probes like Begin without the watchdog.
type fakeTxns struct {
	d        *mock.Device
	begun    int
	probeErr error
}

func (f *fakeTxns) Begin(ctx context.Context, _ string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	f.begun++
	if !slices.Equal(configs, []string{"network"}) {
		return txn.Status{}, errors.New("unexpected configs")
	}
	if err := stage(ctx); err != nil {
		return txn.Status{}, errors.Join(err, f.d.Revert(ctx, "network"))
	}
	if err := f.d.Commit(ctx, "network"); err != nil {
		return txn.Status{}, err
	}
	if f.probeErr = probe(ctx); f.probeErr != nil {
		return txn.Status{Meta: txn.Meta{ID: "0123456789abcdef"}, State: txn.RolledBack}, txn.ErrProbeFailed
	}
	return txn.Status{Meta: txn.Meta{ID: "0123456789abcdef"}, State: txn.Pending}, nil
}

// routes scripts the kernel default route: before is read in stage, after in the probe.
type routes struct {
	mu    sync.Mutex
	reads int
	seq   []device.IPv4DefaultRoute // empty Device = no default route
}

func (r *routes) DefaultIPv4Route(context.Context) (device.IPv4DefaultRoute, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	at := min(r.reads, len(r.seq)-1)
	r.reads++
	return r.seq[at], r.seq[at].Device != "", nil
}

var sta = device.IPv4DefaultRoute{Device: "phy0-sta0", Gateway: "10.0.1.1"}

func setup(t *testing.T, seq ...device.IPv4DefaultRoute) (*Service, *mock.Device, *fakeTxns) {
	t.Helper()
	d := mock.New()
	if err := d.Load(t.Context(), "network", network(t)); err != nil {
		t.Fatal(err)
	}
	d.SetUbusReply("network.device", "status", capture(t, "wan/device-status-wan.json")) // no carrier
	f := &fakeTxns{d: d}
	s := &Service{UCI: d, Keys: d, Ubus: d, Routes: &routes{seq: seq}, Txns: f,
		ProbeWait: time.Second, ProbeSettle: time.Millisecond, Tick: time.Millisecond}
	return s, d, f
}

func wan(t *testing.T, d *mock.Device) device.Section {
	t.Helper()
	secs, err := d.Show(t.Context(), "network")
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(secs, func(s device.Section) bool { return s.Name == Interface })
	return secs[i]
}

func TestStaticWithoutCarrierKeepsTheSTARoute(t *testing.T) {
	s, d, _ := setup(t, sta)
	st, err := s.Set(t.Context(), Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "203.0.113.1", DNS: []string{"1.1.1.1", "9.9.9.9"}})
	if err != nil || st.State != txn.Pending {
		t.Fatalf("set = %+v, %v", st, err)
	}
	w := wan(t, d)
	if w.Options["proto"] != "static" || w.Options["ipaddr"] != "203.0.113.5/24" || w.Options["gateway"] != "203.0.113.1" ||
		!slices.Equal(w.Lists["dns"], []string{"1.1.1.1", "9.9.9.9"}) || w.Options["device"] != "wan" {
		t.Fatalf("wan = %+v", w)
	}
	v, err := s.Settings(t.Context())
	if err != nil || v.IPAddr != "203.0.113.5/24" {
		t.Fatalf("view = %+v, %v", v, err)
	}
}

func TestUnchangedRequestStagesNothing(t *testing.T) {
	s, _, f := setup(t, sta)
	if _, err := s.Set(t.Context(), Request{Proto: ProtoDHCP}); !errors.Is(err, txn.ErrNoChanges) || f.begun != 0 {
		t.Fatalf("err = %v, begun = %d", err, f.begun)
	}
}

func TestPPPoEPasswordIsWriteOnly(t *testing.T) {
	s, d, f := setup(t, sta)
	if _, err := s.Set(t.Context(), Request{Proto: ProtoPPPoE, Username: "u@isp", Password: "top secret"}); err != nil {
		t.Fatal(err)
	}
	w := wan(t, d)
	if w.Options["password"] != "top secret" || w.Options["username"] != "u@isp" {
		t.Fatalf("wan = %+v", w.Options)
	}
	v, err := s.Settings(t.Context())
	if err != nil || !v.PasswordSet || strings.Contains(strings.Join([]string{v.Username, v.IPAddr, v.Gateway}, " "), "secret") {
		t.Fatalf("view = %+v, %v", v, err)
	}
	// Same username, password omitted: the stored one is kept and nothing is staged.
	if _, err := s.Set(t.Context(), Request{Proto: ProtoPPPoE, Username: "u@isp"}); !errors.Is(err, txn.ErrNoChanges) || f.begun != 1 {
		t.Fatalf("kept password: err = %v, begun = %d", err, f.begun)
	}
	// Back to DHCP drops the PPPoE options.
	if _, err := s.Set(t.Context(), Request{Proto: ProtoDHCP}); err != nil {
		t.Fatal(err)
	}
	if w := wan(t, d); w.Options["username"] != "" || w.Options["password"] != "" || w.Options["proto"] != "dhcp" {
		t.Fatalf("wan = %+v", w.Options)
	}
}

func TestStaticToDHCPDropsStaticOptionsButKeepsDHCPDNS(t *testing.T) {
	s, d, _ := setup(t, sta)
	if _, err := s.Set(t.Context(), Request{Proto: ProtoStatic, IPAddr: "203.0.113.5/24", Gateway: "203.0.113.1", DNS: []string{"1.1.1.1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Set(t.Context(), Request{Proto: ProtoDHCP}); err != nil {
		t.Fatal(err)
	}
	w := wan(t, d)
	for _, o := range []string{"ipaddr", "netmask", "gateway", "dns"} {
		if _, ok := w.Options[o]; ok || w.Lists[o] != nil {
			t.Errorf("%s kept: %+v", o, w)
		}
	}
	// A dhcp wan with its own resolvers (peerdns 0) keeps them across a PPPoE switch.
	net := withWAN(t, map[string]string{"device": "wan", "proto": "dhcp", "peerdns": "0", "dns": "127.0.0.1"}, nil)
	p, err := Diff(net, Request{Proto: ProtoPPPoE, Username: "u", Password: "p"})
	if err != nil || slices.ContainsFunc(p.Writes, func(w write) bool { return w.Option == "dns" || w.Option == "password" }) {
		t.Fatalf("plan = %+v, %v", p.Writes, err)
	}
}

func TestStaticOverlappingLANRefusedBeforeAnyWrite(t *testing.T) {
	s, d, f := setup(t, sta)
	_, err := s.Set(t.Context(), Request{Proto: ProtoStatic, IPAddr: "192.168.1.50/24", Gateway: "192.168.1.1"})
	if !errors.Is(err, ErrOverlapsLAN) || f.begun != 0 {
		t.Fatalf("err = %v, begun = %d", err, f.begun)
	}
	if pending, _ := d.Pending(t.Context(), "network"); pending {
		t.Fatal("staged a write")
	}
}

func TestProbeFailsWhenTheActiveUplinkRouteChanges(t *testing.T) {
	s, _, f := setup(t, sta, device.IPv4DefaultRoute{})
	if _, err := s.Set(t.Context(), Request{Proto: ProtoPPPoE, Username: "u", Password: "p"}); !errors.Is(err, txn.ErrProbeFailed) || !errors.Is(f.probeErr, ErrNotProven) {
		t.Fatalf("err = %v, probe = %v", err, f.probeErr)
	}
}

func TestProven(t *testing.T) {
	wanRoute := device.IPv4DefaultRoute{Device: "wan", Gateway: "203.0.113.1"}
	ppp := device.IPv4DefaultRoute{Device: "pppoe-wan", Gateway: "198.51.100.1"}
	for name, c := range map[string]struct {
		r         Reading
		routeDev  string
		wasActive bool
		before    device.IPv4DefaultRoute
		had       bool
		want      bool
	}{
		"route via wan with carrier":       {Reading{true, wanRoute, true}, "wan", true, wanRoute, true, true},
		"pppoe route with carrier":         {Reading{true, ppp, true}, "pppoe-wan", true, wanRoute, true, true},
		"wan was active, route lost":       {Reading{true, device.IPv4DefaultRoute{}, false}, "wan", true, wanRoute, true, false},
		"wan was active, now via sta":      {Reading{true, sta, true}, "wan", true, wanRoute, true, false},
		"no carrier, sta route unchanged":  {Reading{false, sta, true}, "wan", false, sta, true, true},
		"carrier, sta route unchanged":     {Reading{true, sta, true}, "wan", false, sta, true, true},
		"no carrier, sta route gone":       {Reading{false, device.IPv4DefaultRoute{}, false}, "wan", false, sta, true, false},
		"no carrier, sta gateway changed":  {Reading{false, device.IPv4DefaultRoute{Device: "phy0-sta0", Gateway: "10.0.1.9"}, true}, "wan", false, sta, true, false},
		"no route before and after":        {Reading{false, device.IPv4DefaultRoute{}, false}, "wan", false, device.IPv4DefaultRoute{}, false, true},
		"wan route without carrier is not": {Reading{false, wanRoute, true}, "wan", false, device.IPv4DefaultRoute{}, false, false},
	} {
		if got := Proven(c.r, c.routeDev, c.wasActive, c.before, c.had); got != c.want {
			t.Errorf("%s: got %v", name, got)
		}
	}
}
