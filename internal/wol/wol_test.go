package wol

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

func TestParseMAC(t *testing.T) {
	for _, ok := range []string{"aa:bb:cc:dd:ee:ff", "AA-BB-CC-DD-EE-FF", "02:00:00:00:00:01"} {
		if _, err := ParseMAC(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "aabbccddeeff", "aabb.ccdd.eeff", "01:00:5e:00:00:01", "ff:ff:ff:ff:ff:ff",
		"00:00:00:00:00:00", "aa:bb:cc:dd:ee", "aa:bb:cc:dd:ee:ff:00:11", "aa:bb:cc-dd:ee:ff", "-a:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:f;"} {
		if _, err := ParseMAC(bad); !errors.Is(err, ErrInvalidMAC) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestMagicPacket(t *testing.T) {
	hw, _ := ParseMAC("aa:bb:cc:dd:ee:ff")
	p := device.MagicPacket(hw)
	if len(p) != 102 || !device.IsMagicPacket(p) || p[0] != 0xff || p[6] != 0xaa || p[101] != 0xff || p[96] != 0xaa {
		t.Fatalf("% x", p)
	}
	p[50] ^= 1
	if device.IsMagicPacket(p) {
		t.Fatal("corrupt packet accepted")
	}
}

func TestBroadcast(t *testing.T) {
	cases := map[string]string{"192.168.1.1/24": "192.168.1.255", "10.0.0.5/8": "10.255.255.255", "172.16.4.1/22": "172.16.7.255"}
	for in, want := range cases {
		if b, ok := Broadcast(netip.MustParsePrefix(in)); !ok || b.String() != want {
			t.Errorf("%s: %s %v", in, b, ok)
		}
	}
	for _, in := range []string{"10.0.0.1/31", "10.0.0.1/32", "fd00::1/64"} {
		if _, ok := Broadcast(netip.MustParsePrefix(in)); ok {
			t.Errorf("%s has a broadcast", in)
		}
	}
}

func capturedDump(t *testing.T, path string) []facts.Interface {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/" + path)
	if err != nil {
		t.Fatal(err)
	}
	dump, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		t.Fatal(err)
	}
	return dump
}

// The AXT1800 dumps: guest on br-guest and lan on br-lan are the targets; wwan (uplink),
// loopback and the down wan interfaces never are.
func TestTargetsFromCapturedDumps(t *testing.T) {
	lan := Target{Interface: "lan", Bridge: "br-lan", Broadcast: netip.MustParseAddr("192.168.1.255")}
	guest := Target{Interface: "guest", Bridge: "br-guest", Broadcast: netip.MustParseAddr("192.168.2.255")}
	if got := Targets(capturedDump(t, "interface-dump.json"), ""); !slices.Equal(got, []Target{lan}) {
		t.Fatalf("lan only: %+v", got)
	}
	dump := capturedDump(t, "network/interface-dump.json")
	if got := Targets(dump, ""); !slices.Equal(got, []Target{guest, lan}) {
		t.Fatalf("with guest: %+v", got)
	}
	if got := Targets(dump, "guest"); !slices.Equal(got, []Target{guest}) {
		t.Fatalf("guest filter: %+v", got)
	}
	for _, refused := range []string{"wwan", "wan", "loopback", "nope"} {
		if got := Targets(dump, refused); len(got) != 0 {
			t.Errorf("%s targeted: %+v", refused, got)
		}
	}
}

// Rules the captures do not show: a bridged uplink, a tunnel and a down bridge never get a
// packet (domain values, not a device payload).
func TestTargetsRefuseBridgedUplinkTunnelAndDownBridge(t *testing.T) {
	dump := []facts.Interface{
		{Name: "wan", Up: true, L3Device: "br-wan", IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.1.2/24")}},
		{Name: "wg0", Up: true, L3Device: "wg0", IPv4: []netip.Prefix{netip.MustParsePrefix("10.8.0.2/24")}},
		{Name: "iot", Up: false, L3Device: "br-iot", IPv4: []netip.Prefix{netip.MustParsePrefix("192.168.3.1/24")}},
	}
	if got := Targets(dump, ""); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func seeded() *mock.Device {
	d := mock.New()
	mock.SeedClients(d)
	return d
}

func TestWakeSendsOnTheLANBridge(t *testing.T) {
	d := seeded()
	s := &Service{Ubus: d, Sender: d}
	res, err := s.Wake(context.Background(), "AA-BB-CC-DD-EE-FF", "")
	if err != nil || res.MAC != "aa:bb:cc:dd:ee:ff" || !slices.Equal(res.Interfaces, []string{"lan"}) {
		t.Fatalf("%+v %v", res, err)
	}
	if len(d.Wakes) != 1 || d.Wakes[0].Bridge != "br-lan" || d.Wakes[0].Broadcast.String() != "192.168.1.255" || !device.IsMagicPacket(d.Wakes[0].Packet) {
		t.Fatalf("%+v", d.Wakes)
	}
	if _, err := s.Wake(context.Background(), "aa:bb:cc:dd:ee:ff", "wwan"); !errors.Is(err, ErrInvalidInterface) {
		t.Fatalf("uplink: %v", err)
	}
	if _, err := s.Wake(context.Background(), "ff:ff:ff:ff:ff:ff", ""); !errors.Is(err, ErrInvalidMAC) {
		t.Fatalf("broadcast mac: %v", err)
	}
	if len(d.Wakes) != 1 {
		t.Fatalf("refused requests sent: %+v", d.Wakes)
	}
}

func TestWakeFailures(t *testing.T) {
	d := seeded()
	d.WakeErr = errors.New("sendto: network is down")
	s := &Service{Ubus: d, Sender: d}
	if _, err := s.Wake(context.Background(), "aa:bb:cc:dd:ee:ff", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("send failure: %v", err)
	}
	s = &Service{Ubus: mock.New(), Sender: d}
	if _, err := s.Wake(context.Background(), "aa:bb:cc:dd:ee:ff", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no dump: %v", err)
	}
	s = &Service{Ubus: seeded(), Sender: d}
	s.mu.Lock()
	if _, err := s.Wake(context.Background(), "aa:bb:cc:dd:ee:ff", ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy: %v", err)
	}
}
