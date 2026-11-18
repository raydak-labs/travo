package netview

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

const captures = "../../testdata/device/axt1800-25.12.6/network/"

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(captures + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func firewall(t *testing.T) []device.Section {
	t.Helper()
	secs, err := cli.ParseExport("firewall", string(read(t, "uci-export-firewall.txt")))
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

// Anonymous (@zone[0], @zone[1]) and named (guest, travo_vpn) zones resolve by name; the
// network value is a list in some sections and an option in others.
func TestZonesFromCapture(t *testing.T) {
	want := []Zone{
		{Name: "lan", Input: "ACCEPT", Output: "ACCEPT", Forward: "ACCEPT", Networks: []string{"lan"}, ForwardsTo: []string{"wan", "travo_vpn"}},
		{Name: "wan", Input: "REJECT", Output: "ACCEPT", Forward: "DROP", Masq: true, Networks: []string{"wan", "wan6", "wwan"}, ForwardsTo: []string{}},
		{Name: "guest", Input: "REJECT", Output: "ACCEPT", Forward: "REJECT", Networks: []string{"guest"}, ForwardsTo: []string{"wan", "travo_vpn"}},
		{Name: "travo_vpn", Input: "REJECT", Output: "ACCEPT", Forward: "REJECT", Masq: true, Networks: []string{"wg0"}, ForwardsTo: []string{}},
	}
	if got := Zones(firewall(t)); !reflect.DeepEqual(got, want) {
		t.Fatalf("zones\n got %+v\nwant %+v", got, want)
	}
}

func TestZonesInheritDefaultsAndSkipUnnamedOrDisabled(t *testing.T) {
	fw := []device.Section{
		{Name: "@defaults[0]", Type: "defaults", Options: map[string]string{"input": "REJECT", "output": "ACCEPT", "forward": "REJECT"}},
		{Name: "a", Type: "zone", Options: map[string]string{"name": "iot", "input": "drop"}},
		{Name: "b", Type: "zone", Options: map[string]string{"input": "ACCEPT"}},
		{Name: "c", Type: "zone", Options: map[string]string{"name": "off", "enabled": "0"}},
		{Name: "d", Type: "forwarding", Options: map[string]string{"src": "iot", "dest": "wan", "enabled": "0"}},
	}
	want := []Zone{{Name: "iot", Input: "DROP", Output: "ACCEPT", Forward: "REJECT", Networks: []string{}, ForwardsTo: []string{}}}
	if got := Zones(fw); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

// showUCI answers Show in config order (the mock sorts sections by name).
type showUCI struct {
	device.UCI
	secs []device.Section
	err  error
}

func (u *showUCI) Show(context.Context, string) ([]device.Section, error) { return u.secs, u.err }

func service(t *testing.T) (*Service, *showUCI) {
	d := mock.New()
	d.SetUbusReply("network.interface", "dump", read(t, "interface-dump.json"))
	u := &showUCI{secs: firewall(t)}
	return &Service{UCI: u, Ubus: d}, u
}

func TestInterfacesJoinZonesAndMarkUplinks(t *testing.T) {
	s, _ := service(t)
	got, err := s.Interfaces(context.Background())
	if err != nil || got.Partial {
		t.Fatal(err, got.Partial)
	}
	by := map[string]Interface{}
	for _, it := range got.Interfaces {
		by[it.Name] = it
	}
	if len(got.Interfaces) != 6 {
		t.Fatalf("%d interfaces", len(got.Interfaces))
	}
	want := Interface{Name: "wwan", Proto: "dhcp", Up: true, UptimeSeconds: 90237, Device: "phy0-sta0",
		IPv4: []string{"10.0.1.195/24"}, IPv6: []string{}, DNSServers: []string{"10.0.1.1"}, Uplink: true, Zone: "wan"}
	if !reflect.DeepEqual(by["wwan"], want) {
		t.Fatalf("wwan %+v", by["wwan"])
	}
	if l := by["lan"]; l.Uplink || l.Zone != "lan" || !reflect.DeepEqual(l.IPv6, []string{"fd06:b820:5389::1/60"}) {
		t.Fatalf("lan %+v", l)
	}
	if w := by["wan6"]; w.Up || !w.Uplink || w.Zone != "wan" || w.Device != "" || w.UptimeSeconds != 0 {
		t.Fatalf("wan6 %+v", w)
	}
	if g := by["guest"]; g.Zone != "guest" || g.Uplink {
		t.Fatalf("guest %+v", g)
	}
	if lo := by["loopback"]; lo.Zone != "" {
		t.Fatalf("loopback in zone %q", lo.Zone)
	}
}

func TestInterfacesWithoutFirewallArePartial(t *testing.T) {
	s, u := service(t)
	u.err = errors.New("uci down")
	got, err := s.Interfaces(context.Background())
	if err != nil || !got.Partial || got.Interfaces[0].Zone != "" {
		t.Fatal(err, got)
	}
	if _, err := s.Zones(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestInterfacesWithoutDumpFail(t *testing.T) {
	s := &Service{UCI: mock.New(), Ubus: mock.New()}
	if _, err := s.Interfaces(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
