package uplinkview

import (
	"context"
	"errors"
	"net/netip"
	"strconv"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/failover"
)

func str(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

type settings struct {
	set failover.Settings
	err error
}

func (s settings) LoadSettings() (failover.Settings, error) { return s.set, s.err }

// captured is mock mode's AXT1800: verbatim interface dump, wireless status and sysfs links
// (internal/device/mock/testdata/PROVENANCE.md), stock mwan3 config.
func captured(t *testing.T) *mock.Device {
	t.Helper()
	d := mock.New()
	mock.SeedClients(d)
	mock.SeedWiFi(d)
	if err := mock.SeedNetLinks(d); err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport(failover.Config, "package mwan3\n\n"+string(mock.MWAN3Stock))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Load(context.Background(), failover.Config, secs); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestUplinksFromCaptures(t *testing.T) {
	d := captured(t)
	mock.SeedConnectivity(d) // captured default route: via 192.168.90.2 dev phy0-sta0
	s := &Service{Ubus: d, UCI: d, Links: d, Routes: d, Failover: settings{set: failover.Settings{Uplinks: []string{"wan", "wwan"}}}}
	got, err := s.Uplinks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ name, configured, up, device, ipv4, ssid, kind string }{
		{"wan", "true", "false", "<nil>", "<nil>", "<nil>", "<nil>"},
		{"wwan", "true", "true", "phy0-sta0", "192.168.90.241/24", "HomeUplink", "<nil>"},
		{"usbtether", "false", "false", "<nil>", "<nil>", "<nil>", "<nil>"},
	}
	if len(got.Uplinks) != len(want) {
		t.Fatalf("uplinks = %+v", got.Uplinks)
	}
	for i, w := range want {
		u := got.Uplinks[i]
		row := []string{u.Name, bool2(u.Configured), bool2(u.Up), str(u.Device), str(u.IPv4), str(u.SSID), str(u.Kind)}
		if row[0] != w.name || row[1] != w.configured || row[2] != w.up || row[3] != w.device || row[4] != w.ipv4 || row[5] != w.ssid || row[6] != w.kind {
			t.Errorf("uplink %d = %v, want %+v", i, row, w)
		}
	}
	details := []struct{ proto, gateway, band, signal string }{
		{"dhcp", "<nil>", "<nil>", "<nil>"},
		{"dhcp", "192.168.90.2", "5g", "-63"},
		{"<nil>", "<nil>", "<nil>", "<nil>"},
	}
	for i, w := range details {
		u := got.Uplinks[i]
		row := struct{ proto, gateway, band, signal string }{str(u.Proto), str(u.Gateway), str(u.Band), num(u.SignalDBm)}
		if row != w {
			t.Errorf("uplink %s details = %+v, want %+v", u.Name, row, w)
		}
	}
	if f := got.Failover; f == nil || f.Enabled || f.Active || len(f.Order) != 2 {
		t.Errorf("failover on the stock config = %+v, want disabled and not active", f)
	}
}

func num(p *int) string {
	if p == nil {
		return "<nil>"
	}
	return strconv.Itoa(*p)
}

func bool2(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestUplinksOptionalReadsLeaveNull(t *testing.T) {
	d := mock.New()
	mock.SeedClients(d) // interface dump only: no wireless status, no mwan3 config
	s := &Service{Ubus: d, UCI: d, Failover: settings{}}
	got, err := s.Uplinks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Uplinks[1].SSID != nil || got.Failover != nil {
		t.Errorf("ssid = %s, failover = %+v; want both null", str(got.Uplinks[1].SSID), got.Failover)
	}
	d = captured(t)
	mock.SeedConnectivity(d)
	d.SetUbusDeviceReply("iwinfo", "info", "phy0-sta0", []byte("not json"))
	s = &Service{Ubus: d, UCI: d}
	if got, _ = s.Uplinks(context.Background()); got.Uplinks[1].Gateway != nil || got.Uplinks[1].SignalDBm != nil {
		t.Errorf("no route reader and a broken iwinfo reply: gateway %s, signal %s; want null", str(got.Uplinks[1].Gateway), num(got.Uplinks[1].SignalDBm))
	}
	d = captured(t)
	s = &Service{Ubus: d, UCI: d, Failover: settings{err: errors.New("torn failover.json")}}
	if got, _ = s.Uplinks(context.Background()); got.Failover != nil {
		t.Errorf("unreadable settings: failover = %+v, want null", got.Failover)
	}
}

func TestUplinksNeedTheInterfaceDump(t *testing.T) {
	for name, s := range map[string]*Service{"no port": {}, "no dump": {Ubus: mock.New()}} {
		if _, err := s.Uplinks(context.Background()); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestBuildRules(t *testing.T) {
	pfx := func(s string) []netip.Prefix { return []netip.Prefix{netip.MustParsePrefix(s)} }
	got := build(inputs{
		Interfaces: []facts.Interface{
			{Name: "wan", Up: true, L3Device: "eth0"}, // carrier, no address yet
			{Name: "wwan", Up: true, L3Device: "phy0-sta0", IPv4: pfx("192.168.90.241/24")},
			{Name: "usbtether", Up: true, L3Device: "usb0", IPv4: pfx("172.20.10.2/28")},
		},
		Radios: []facts.Radio{
			{Name: "radio0", Ifaces: []facts.WirelessIface{{Mode: "ap", Ifname: "phy0-ap0", SSID: "AP"}, {Mode: "sta", Ifname: "phy0-sta1", SSID: "Other"}}},
			{Name: "radio1", Ifaces: []facts.WirelessIface{{Mode: "sta", Ifname: "phy0-sta0", SSID: "Hotel"}}},
		},
		Links: []device.NetLink{{Name: "usb0", Bus: "usb", Driver: "rndis_host"}},
	})
	if got.Uplinks[0].Up || str(got.Uplinks[0].Device) != "eth0" {
		t.Errorf("wan without IPv4 = %+v", got.Uplinks[0])
	}
	if str(got.Uplinks[1].SSID) != "Hotel" || got.Uplinks[0].SSID != nil {
		t.Errorf("ssid: wwan %s, wan %s", str(got.Uplinks[1].SSID), str(got.Uplinks[0].SSID))
	}
	if str(got.Uplinks[2].Kind) != "rndis" || got.Uplinks[1].Kind != nil {
		t.Errorf("kind: usbtether %s, wwan %s", str(got.Uplinks[2].Kind), str(got.Uplinks[1].Kind))
	}
	if got.Failover != nil {
		t.Errorf("failover = %+v", got.Failover)
	}

	down := build(inputs{
		Interfaces: []facts.Interface{{Name: "wwan"}},
		Radios:     []facts.Radio{{Ifaces: []facts.WirelessIface{{Mode: "sta", SSID: "Hotel"}}}},
		Links:      []device.NetLink{{Name: "eth1", Bus: "usb", Driver: "r8152"}},
	})
	if str(down.Uplinks[1].SSID) != "Hotel" {
		t.Errorf("wwan down: ssid = %s, want the configured STA's", str(down.Uplinks[1].SSID))
	}
	if down.Uplinks[2].Kind != nil {
		t.Errorf("unconfigured usbtether kind = %s", str(down.Uplinks[2].Kind))
	}

	signal := -50
	wwan := facts.Interface{Name: "wwan", Proto: "dhcp", Up: true, L3Device: "phy0-sta0", IPv4: pfx("192.168.90.241/24")}
	radios := []facts.Radio{{Band: "5g", Ifaces: []facts.WirelessIface{{Mode: "sta", Ifname: "phy0-sta0", SSID: "Hotel"}}}}
	for name, tc := range map[string]struct {
		in                        inputs
		proto, gateway, band, sig string
	}{
		"all read":                {inputs{Interfaces: []facts.Interface{wwan}, Radios: radios, Route: &device.IPv4DefaultRoute{Device: "phy0-sta0", Gateway: "192.168.90.2"}, STAInfo: &facts.IwinfoInfo{Signal: &signal}}, "dhcp", "192.168.90.2", "5g", "-50"},
		"no route":                {inputs{Interfaces: []facts.Interface{wwan}, Radios: radios}, "dhcp", "<nil>", "5g", "<nil>"},
		"route on another device": {inputs{Interfaces: []facts.Interface{wwan}, Route: &device.IPv4DefaultRoute{Device: "eth0", Gateway: "10.0.0.1"}}, "dhcp", "<nil>", "<nil>", "<nil>"},
		"route without gateway":   {inputs{Interfaces: []facts.Interface{wwan}, Route: &device.IPv4DefaultRoute{Device: "phy0-sta0"}}, "dhcp", "<nil>", "<nil>", "<nil>"},
		"empty proto":             {inputs{Interfaces: []facts.Interface{{Name: "wwan", Up: true, L3Device: "phy0-sta0", IPv4: pfx("192.168.90.241/24")}}}, "<nil>", "<nil>", "<nil>", "<nil>"},
		"wwan not up":             {inputs{Interfaces: []facts.Interface{{Name: "wwan", Proto: "dhcp", L3Device: "phy0-sta0"}}, Radios: radios, STAInfo: &facts.IwinfoInfo{Signal: &signal}}, "dhcp", "<nil>", "5g", "<nil>"},
		"radios unreadable":       {inputs{Interfaces: []facts.Interface{wwan}, STAInfo: &facts.IwinfoInfo{Signal: &signal}}, "dhcp", "<nil>", "<nil>", "-50"},
	} {
		u := build(tc.in).Uplinks[1]
		if str(u.Proto) != tc.proto || str(u.Gateway) != tc.gateway || str(u.Band) != tc.band || num(u.SignalDBm) != tc.sig {
			t.Errorf("%s: proto %s gateway %s band %s signal %s", name, str(u.Proto), str(u.Gateway), str(u.Band), num(u.SignalDBm))
		}
	}
}
