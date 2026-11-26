package clients

import (
	"errors"
	"net/netip"
	"os"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

func TestListMergesTheCaptures(t *testing.T) {
	s, _, _ := setup(t)
	got, err := s.List(t.Context())
	if err != nil || got.Partial {
		t.Fatal(got, err)
	}
	if len(got.Clients) != 2 {
		t.Fatalf("%+v", got.Clients)
	}
	wired, station := got.Clients[0], got.Clients[1]
	if wired.MAC != "9c:eb:e8:d3:f8:d1" || wired.IP != "192.168.1.2" || wired.Connection != LAN {
		t.Fatalf("%+v", wired)
	}
	if station.MAC != "22:4e:76:6c:2d:62" || station.IP != "192.168.1.151" || station.Connection != WiFi ||
		station.AP != "phy1-ap0" || station.SSID != "TravelAP" || *station.Signal != -38 ||
		station.Hostname != "REDACTED-HOST" || *station.LeaseExpiresIn != 3600 {
		t.Fatalf("%+v", station)
	}
}

func TestListDropsUnreadableOptionalSources(t *testing.T) {
	s, d, _ := setup(t)
	d.LeasesErr = errors.New("io")
	d.SetUbusReply("hostapd.phy1-ap0", "get_clients", []byte(`{}`))
	got, err := s.List(t.Context())
	if err != nil || !got.Partial || len(got.Clients) != 2 || got.Clients[1].Connection != LAN {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestListFailsClosedOnACorruptAliasStore(t *testing.T) {
	s, _, _ := setup(t)
	if err := os.WriteFile(s.Aliases.Path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(t.Context()); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatal(err)
	}
	if err := s.SetAlias("22:4e:76:6c:2d:62", "Phone"); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(s.Aliases.Path); string(b) != "{" {
		t.Fatal("a corrupt store was overwritten")
	}
}

func TestAliasStoreWithoutAnAliasesObjectIsCorrupt(t *testing.T) {
	s, _, _ := setup(t)
	for _, body := range []string{"null", "{}", `{"aliases":null}`} {
		if err := os.WriteFile(s.Aliases.Path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Aliases.All(); !errors.Is(err, ErrStoreCorrupt) {
			t.Fatal(body, err)
		}
	}
	if err := os.WriteFile(s.Aliases.Path, []byte(`{"aliases":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Aliases.All(); err != nil {
		t.Fatal(err)
	}
}

// A hotel network on the LAN's prefix: its neighbours sit on the STA device and never count,
// neither as clients nor as the caller.
func TestNeighboursOnTheUplinkDeviceAreNeverClients(t *testing.T) {
	s, d, _ := setup(t)
	d.SetNeighbours([]byte(`[{"dst":"192.168.1.2","dev":"phy0-sta0","lladdr":"98:9b:cb:c0:3e:60","state":["REACHABLE"]},` +
		`{"dst":"192.168.1.2","dev":"br-lan","lladdr":"9c:eb:e8:d3:f8:d1","state":["DELAY"]}]`))
	got, _ := s.List(t.Context())
	if slices.ContainsFunc(got.Clients, func(c Client) bool { return c.MAC == "98:9b:cb:c0:3e:60" }) {
		t.Fatalf("%+v", got.Clients)
	}
	if _, err := s.SetBlocked(t.Context(), "9c:eb:e8:d3:f8:d1", true, netip.MustParseAddr("192.168.1.2")); !errors.Is(err, ErrIsCaller) {
		t.Fatal(err)
	}
	d.SetNeighbours([]byte(`[{"dst":"192.168.1.2","dev":"br-lan","lladdr":"9c:eb:e8:d3:f8:d1","state":["DELAY"]},` +
		`{"dst":"192.168.1.2","dev":"br-lan","lladdr":"9c:eb:e8:d3:f8:d2","state":["DELAY"]}]`))
	if _, err := s.SetBlocked(t.Context(), "22:4e:76:6c:2d:62", true, netip.MustParseAddr("192.168.1.2")); !errors.Is(err, ErrUnavailable) {
		t.Fatal("ambiguous caller MAC accepted", err)
	}
}

func TestReserveRefusesAStagedNetworkAndRetriesTheReload(t *testing.T) {
	s, d, f := setup(t)
	if err := d.Set(t.Context(), "network", "lan", "ipaddr", "192.168.2.1/24"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(t.Context(), Reservation{MAC: "22:4e:76:6c:2d:62", IP: "192.168.2.50"}); !errors.Is(err, txn.ErrForeignStaged) {
		t.Fatal(err)
	}
	if err := d.Revert(t.Context(), "network"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(t.Context(), Reservation{MAC: "22:4e:76:6c:2d:62", IP: "192.168.1.50"}); err != nil {
		t.Fatal(err)
	}
	before := d.Reloads
	if _, err := s.Reserve(t.Context(), Reservation{MAC: "22:4e:76:6c:2d:62", IP: "192.168.1.50"}); err != nil || f.applies != 1 || d.Reloads != before+1 {
		t.Fatal("an unchanged retry must reload without committing", err, f.applies, d.Reloads-before)
	}
	if err := s.Unreserve(t.Context(), "aa:bb:cc:00:00:09"); !errors.Is(err, ErrReservationNotFound) || d.Reloads != before+2 {
		t.Fatal(err, d.Reloads-before)
	}
}

func TestAliasRoundTrip(t *testing.T) {
	s, _, _ := setup(t)
	if err := s.SetAlias("22:4E:76:6C:2D:62", " Phone "); err != nil {
		t.Fatal(err)
	}
	got, _ := s.List(t.Context())
	if got.Clients[1].Alias != "Phone" {
		t.Fatalf("%+v", got.Clients[1])
	}
	for _, bad := range []string{"bad\x00", string(make([]rune, 65))} {
		if err := s.SetAlias("22:4e:76:6c:2d:62", bad); !errors.Is(err, ErrAliasInvalid) {
			t.Fatal(err)
		}
	}
	if err := s.SetAlias("01:00:5e:00:00:01", "x"); err == nil {
		t.Fatal("multicast MAC accepted")
	}
	if err := s.SetAlias("22:4e:76:6c:2d:62", ""); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.Aliases.All(); len(all) != 0 {
		t.Fatal(all)
	}
}

func TestReserveCreatesAnOwnedHostAndIsIdempotent(t *testing.T) {
	s, d, f := setup(t)
	r, err := s.Reserve(t.Context(), Reservation{MAC: "22:4E:76:6C:2D:62", IP: "192.168.1.151", Name: "phone"})
	if err != nil || !r.Owned || r.IP != "192.168.1.151" || r.Name != "phone" {
		t.Fatal(r, err)
	}
	if _, err := s.Reserve(t.Context(), Reservation{MAC: "22:4e:76:6c:2d:62", IP: "192.168.1.151", Name: "phone"}); err != nil || f.applies != 1 {
		t.Fatal("unchanged reservation committed again", err, f.applies)
	}
	if pending, _ := d.Pending(t.Context(), "dhcp"); pending {
		t.Fatal("left staged changes")
	}
	if _, err := s.Reserve(t.Context(), Reservation{MAC: "9c:eb:e8:d3:f8:d1", IP: "192.168.1.151"}); !errors.Is(err, ErrIPTaken) {
		t.Fatal(err)
	}
	got, _ := s.List(t.Context())
	if !got.Clients[1].Reserved || got.Clients[0].Reserved {
		t.Fatalf("%+v", got.Clients)
	}
	if err := s.Unreserve(t.Context(), "22:4e:76:6c:2d:62"); err != nil {
		t.Fatal(err)
	}
	if err := s.Unreserve(t.Context(), "22:4e:76:6c:2d:62"); !errors.Is(err, ErrReservationNotFound) {
		t.Fatal(err)
	}
}

func TestBlockRefusesTheCallerAndStagesTheRule(t *testing.T) {
	s, d, f := setup(t)
	caller := netip.MustParseAddr("192.168.1.2")
	if _, err := s.SetBlocked(t.Context(), "9c:eb:e8:d3:f8:d1", true, caller); !errors.Is(err, ErrIsCaller) || len(f.begins) != 0 {
		t.Fatal(err, f.begins)
	}
	st, err := s.SetBlocked(t.Context(), "22:4e:76:6c:2d:62", true, caller)
	if err != nil || st.State != txn.Pending {
		t.Fatal(st, err)
	}
	fw, _ := d.Show(t.Context(), "firewall")
	if !slices.Equal(Blocked(fw), []string{"22:4e:76:6c:2d:62"}) {
		t.Fatal(Blocked(fw))
	}
	if _, err := s.SetBlocked(t.Context(), "22:4e:76:6c:2d:62", true, caller); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatal(err)
	}
	// Unblocking never needs the caller check: it only gives access back.
	if _, err := s.SetBlocked(t.Context(), "22:4e:76:6c:2d:62", false, netip.Addr{}); err != nil {
		t.Fatal(err)
	}
	fw, _ = d.Show(t.Context(), "firewall")
	if len(Blocked(fw)) != 0 {
		t.Fatal(Blocked(fw))
	}
}

func TestBlockFailsClosedWhenTheCallerCannotBeMapped(t *testing.T) {
	s, d, _ := setup(t)
	d.SetNeighbours([]byte("not json"))
	if _, err := s.SetBlocked(t.Context(), "22:4e:76:6c:2d:62", true, netip.MustParseAddr("192.168.1.2")); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestKick(t *testing.T) {
	s, d, _ := setup(t)
	if err := s.Kick(t.Context(), "22:4e:76:6c:2d:62", netip.MustParseAddr("192.168.1.151")); !errors.Is(err, ErrIsCaller) {
		t.Fatal(err)
	}
	if err := s.Kick(t.Context(), "9c:eb:e8:d3:f8:d1", netip.MustParseAddr("192.168.1.151")); !errors.Is(err, ErrNotAssociated) {
		t.Fatal(err)
	}
	if err := s.Guards.Acquire(KickGuard); err != nil { // left by a crash
		t.Fatal(err)
	}
	if err := s.Kick(t.Context(), "22:4e:76:6c:2d:62", netip.MustParseAddr("192.168.1.2")); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.Kicks, []string{"phy1-ap0 22:4e:76:6c:2d:62"}) {
		t.Fatal(d.Kicks)
	}
	if err := s.Guards.Acquire(KickGuard); err != nil {
		t.Fatal("guard not released after the kick", err)
	}
}
