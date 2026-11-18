package clients

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

// The rule Travo writes is the shape staged on the AXT1800 and rendered by fw4 into the
// global forward chain (clients/fw4-print-block.txt).
func TestBlockRuleMatchesTheStagedCapture(t *testing.T) {
	captured := parsed(t, "firewall", "clients/uci-export-firewall-block-staged.txt", true)
	want := BlockRule("46:ba:e6:8d:7d:6b")
	if len(captured) != 1 || !sameRule(captured[0], want) || captured[0].Name != want.Name {
		t.Fatalf("%+v\n%+v", captured, want)
	}
	if got := Blocked(captured); !reflect.DeepEqual(got, []string{"46:ba:e6:8d:7d:6b"}) {
		t.Fatal(got)
	}
	drifted := captured[0]
	drifted.Options = map[string]string{"name": "x", "src": "lan", "dest": "*", "proto": "all", "target": "ACCEPT"}
	if got := Blocked([]device.Section{drifted}); len(got) != 0 {
		t.Fatal("a drifted rule reads as blocked", got)
	}
}

func TestStageBlockRefusesAForeignSection(t *testing.T) {
	d := mock.New()
	foreign := []device.Section{{Name: BlockSection("46:ba:e6:8d:7d:6b"), Type: "zone"}}
	if _, err := StageBlock(t.Context(), d, foreign, "46:ba:e6:8d:7d:6b"); !errors.Is(err, ErrBlockConflict) {
		t.Fatal(err)
	}
	if _, err := StageUnblock(t.Context(), d, foreign, "46:ba:e6:8d:7d:6b"); !errors.Is(err, ErrBlockConflict) {
		t.Fatal(err)
	}
}

func TestReservationsReadTheStagedCapture(t *testing.T) {
	got := Reservations(parsed(t, "dhcp", "clients/uci-export-dhcp-host-staged.txt", true))
	want := []Reservation{{MAC: "46:ba:e6:8d:7d:6b", IP: "192.168.1.119", Name: "Watch", Owned: true, Section: "travo_host_46bae68d7d6b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	// uci show prints a one-element list like an option (device-facts §0): same result.
	if shown := Reservations(parsed(t, "dhcp", "clients/uci-show-dhcp-host-staged.txt", false)); !reflect.DeepEqual(shown, want) {
		t.Fatalf("%+v", shown)
	}
}

func TestLANPrefixReadsCIDRAndNetmask(t *testing.T) {
	p, err := LANPrefix(parsed(t, "network", "clients/uci-show-network-lan.txt", false))
	if err != nil || p.String() != "192.168.1.1/24" {
		t.Fatal(p, err)
	}
	old := []device.Section{{Name: "lan", Type: "interface", Options: map[string]string{"ipaddr": "10.0.0.1", "netmask": "255.255.252.0"}}}
	if p, err := LANPrefix(old); err != nil || p.String() != "10.0.0.1/22" {
		t.Fatal(p, err)
	}
	bad := []device.Section{{Name: "lan", Type: "interface", Options: map[string]string{"ipaddr": "10.0.0.1", "netmask": "255.0.255.0"}}}
	if _, err := LANPrefix(bad); !errors.Is(err, ErrNoLAN) {
		t.Fatal(err)
	}
}

func TestValidateReservation(t *testing.T) {
	lan := netip.MustParsePrefix("192.168.1.1/24")
	existing := []device.Section{
		{Name: "a", Type: "host", Options: map[string]string{"ip": "192.168.1.50"}, Lists: map[string][]string{"mac": {"aa:bb:cc:00:00:01"}}},
		{Name: "nas", Type: "host", Options: map[string]string{"ip": "192.168.1.60", "name": "nas"}}, // MAC-less: dnsmasq still serves it
	}
	ok, err := ValidateReservation(Reservation{MAC: "AA:BB:CC:00:00:02", IP: " 192.168.1.51 ", Name: "nas-1"}, lan, existing)
	if err != nil || ok.MAC != "aa:bb:cc:00:00:02" || ok.IP != "192.168.1.51" {
		t.Fatal(ok, err)
	}
	for _, tc := range []struct {
		r     Reservation
		field string
	}{
		{Reservation{MAC: "ff:ff:ff:ff:ff:ff", IP: "192.168.1.51"}, "mac"},
		{Reservation{MAC: "aa:bb:cc:00:00:02", IP: "192.168.2.51"}, "ip"},
		{Reservation{MAC: "aa:bb:cc:00:00:02", IP: "192.168.1.1"}, "ip"},
		{Reservation{MAC: "aa:bb:cc:00:00:02", IP: "192.168.1.255"}, "ip"},
		{Reservation{MAC: "aa:bb:cc:00:00:02", IP: "192.168.1.0"}, "ip"},
		{Reservation{MAC: "aa:bb:cc:00:00:02", IP: "fd00::1"}, "ip"},
		{Reservation{MAC: "aa:bb:cc:00:00:02", IP: "192.168.1.51", Name: "bad name"}, "name"},
	} {
		var fe *FieldError
		if _, err := ValidateReservation(tc.r, lan, existing); !errors.As(err, &fe) || fe.Field != tc.field {
			t.Errorf("%+v: %v", tc.r, err)
		}
	}
	for _, ip := range []string{"192.168.1.50", "192.168.1.60"} {
		if _, err := ValidateReservation(Reservation{MAC: "aa:bb:cc:00:00:02", IP: ip}, lan, existing); !errors.Is(err, ErrIPTaken) {
			t.Fatal(ip, err)
		}
	}
	// The section the request updates does not conflict with itself.
	if _, err := ValidateReservation(Reservation{MAC: "aa:bb:cc:00:00:01", IP: "192.168.1.50"}, lan, existing); err != nil {
		t.Fatal(err)
	}
}

func TestWildcardSelectorsAreNeverEditedAway(t *testing.T) {
	d := mock.New()
	wild := device.Section{Name: "cfg02", Type: "host", Options: map[string]string{"ip": "192.168.1.9"},
		Lists: map[string][]string{"mac": {"aa:bb:cc:00:00:01", "00:20:e0:3b:13:*"}}}
	if err := d.Load(t.Context(), "dhcp", []device.Section{wild}); err != nil {
		t.Fatal(err)
	}
	if _, err := StageReservation(t.Context(), d, []device.Section{wild}, Reservation{MAC: "aa:bb:cc:00:00:01", IP: "192.168.1.10"}); !errors.Is(err, ErrReservationShared) {
		t.Fatal(err)
	}
	if err := StageUnreserve(t.Context(), d, []device.Section{wild}, "aa:bb:cc:00:00:01"); err != nil {
		t.Fatal(err)
	}
	got, _ := d.Show(t.Context(), "dhcp")
	if len(got) != 1 || !reflect.DeepEqual(hostSelectors(got[0]), []string{"00:20:e0:3b:13:*"}) {
		t.Fatalf("%+v", got)
	}
}

func TestStageReservationRefusesASharedHost(t *testing.T) {
	d := mock.New()
	shared := []device.Section{{Name: "cfg01", Type: "host", Lists: map[string][]string{"mac": {"aa:bb:cc:00:00:01", "aa:bb:cc:00:00:02"}}}}
	if _, err := StageReservation(t.Context(), d, shared, Reservation{MAC: "aa:bb:cc:00:00:01", IP: "192.168.1.9"}); !errors.Is(err, ErrReservationShared) {
		t.Fatal(err)
	}
}
