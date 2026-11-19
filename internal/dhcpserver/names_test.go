package dhcpserver

import (
	"errors"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func TestNamesEmptyOnTheCapture(t *testing.T) {
	s, _, _ := setup(t)
	got, err := s.DNSNames(t.Context())
	if err != nil || len(got) != 0 || got == nil {
		t.Fatalf("%#v %v", got, err)
	}
}

func TestAddAndRemoveDNSName(t *testing.T) {
	s, d, f := setup(t)
	got, err := s.AddDNSName(t.Context(), " NAS.home. ", "192.168.1.20")
	if err != nil || got.ID != "travo_dns_1" || got.IP != "192.168.1.20" || !got.Owned || !slices.Equal(got.Names, []string{"nas.home"}) {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := s.AddDNSName(t.Context(), "printer", "fd00::20"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDNSName(t.Context(), "nas.HOME", "192.168.1.21"); !errors.Is(err, ErrNameExists) {
		t.Fatal(err)
	}
	list, _ := s.DNSNames(t.Context())
	if len(list) != 2 || list[1].ID != "travo_dns_2" || list[1].IP != "fd00::20" {
		t.Fatalf("%+v", list)
	}
	// The dnsmasq section the DNS layer stack owns is untouched.
	if v, _ := opt(t, d, "@dnsmasq", "noresolv"); v != "1" {
		t.Fatalf("noresolv=%q", v)
	}
	if err := s.RemoveDNSName(t.Context(), "travo_dns_1"); err != nil {
		t.Fatal(err)
	}
	commits, reloads := f.commits, d.Reloads
	if err := s.RemoveDNSName(t.Context(), "travo_dns_1"); !errors.Is(err, ErrNameNotFound) {
		t.Fatal(err)
	}
	if f.commits != commits || d.Reloads != reloads+1 {
		t.Fatal("an absent entry commits nothing but reloads")
	}
	// The freed number is reused.
	if got, _ := s.AddDNSName(t.Context(), "nas", "192.168.1.20"); got.ID != "travo_dns_1" {
		t.Fatalf("%+v", got)
	}
}

// userDomain has the fields dnsmasq's init script reads (testdata
// dhcp/init-dnsmasq-domain-add.txt): name (space-separated or a list) and ip.
func userDomain(name string) device.Section {
	return device.Section{Name: name, Type: "domain",
		Options: map[string]string{"name": "router gw", "ip": "192.168.1.1"}}
}

func TestUserDomainSectionsAreReadOnly(t *testing.T) {
	s, d, _ := setup(t)
	secs, _ := d.Show(t.Context(), "dhcp")
	secs = append(secs, userDomain("cfg0ab"), userDomain("travo_dns_x"),
		device.Section{Name: "cfg0ac", Type: "domain", Options: map[string]string{"name": "noip"}})
	if err := d.Load(t.Context(), "dhcp", secs); err != nil {
		t.Fatal(err)
	}
	list, _ := s.DNSNames(t.Context())
	if len(list) != 2 || list[0].Owned || list[1].Owned || !slices.Equal(list[0].Names, []string{"router", "gw"}) {
		t.Fatalf("%+v", list)
	}
	for _, id := range []string{"cfg0ab", "travo_dns_x", "lan"} {
		if err := s.RemoveDNSName(t.Context(), id); !errors.Is(err, ErrNameForeign) {
			t.Errorf("%s: %v", id, err)
		}
	}
	if _, err := s.AddDNSName(t.Context(), "GW", "192.168.1.2"); !errors.Is(err, ErrNameExists) {
		t.Fatal(err)
	}
}

func TestValidateName(t *testing.T) {
	for _, c := range [][2]string{
		{"", "192.168.1.2"}, {"-nas", "192.168.1.2"}, {"nas..lan", "192.168.1.2"}, {"nas_1", "192.168.1.2"},
		{"host.123", "192.168.1.2"}, {"nas", "fe80::1%br-lan"}, {"nas", "0.0.0.0"}, {"nas", "224.0.0.1"}, {"nas", "nas"},
	} {
		if _, _, err := ValidateName(c[0], c[1]); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q %q: %v", c[0], c[1], err)
		}
	}
	if n, ip, err := ValidateName("my-nas.lan", "::ffff:192.168.1.2"); err != nil || n != "my-nas.lan" || ip.String() != "192.168.1.2" {
		t.Fatalf("%s %s %v", n, ip, err)
	}
}

func TestNamesFull(t *testing.T) {
	s, _, _ := setup(t)
	for i := range MaxNames {
		if _, err := s.AddDNSName(t.Context(), "h"+string(rune('a'+i%26))+string(rune('a'+i/26)), "192.168.1.2"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddDNSName(t.Context(), "onemore", "192.168.1.2"); !errors.Is(err, ErrNamesFull) {
		t.Fatal(err)
	}
}
