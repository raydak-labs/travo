package tailscale

import (
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
)

func TestLANPrefix(t *testing.T) {
	network, err := cli.ParseShow("network", string(fixture(t, "../uci-show-network.txt")))
	if err != nil {
		t.Fatal(err)
	}
	p, err := LANPrefix(network)
	if err != nil || p.String() != "192.168.1.0/24" {
		t.Fatalf("%v %v", p, err)
	}
	lan := func(opts map[string]string, lists map[string][]string) []device.Section {
		return []device.Section{{Name: "lan", Type: "interface", Options: opts, Lists: lists}}
	}
	for name, tc := range map[string]struct {
		in   []device.Section
		want string
	}{
		"netmask":    {lan(map[string]string{"ipaddr": "192.168.8.1", "netmask": "255.255.255.0"}, nil), "192.168.8.0/24"},
		"list cidr":  {lan(map[string]string{}, map[string][]string{"ipaddr": {"10.1.2.3/16"}}), "10.1.0.0/16"},
		"bad mask":   {lan(map[string]string{"ipaddr": "192.168.8.1", "netmask": "255.0.255.0"}, nil), ""},
		"no netmask": {lan(map[string]string{"ipaddr": "192.168.8.1"}, nil), ""},
		"ipv6":       {lan(map[string]string{"ipaddr": "fd00::1/64"}, nil), ""},
		"absent":     {nil, ""},
	} {
		p, err := LANPrefix(tc.in)
		if tc.want == "" {
			if err == nil {
				t.Errorf("%s: got %v", name, p)
			}
			continue
		}
		if err != nil || p.String() != tc.want {
			t.Errorf("%s: %v %v", name, p, err)
		}
	}
}

func TestFirewallSectionsNeverCollideWithKillSwitch(t *testing.T) {
	for _, s := range FirewallSections(true) {
		if vpnkill.Owned(s.Name) {
			t.Fatalf("%s is a kill switch name", s.Name)
		}
	}
	zone := FirewallSections(false)[0]
	if zone.Options["input"] != "REJECT" || zone.Lists["device"][0] != TunDevice || len(zone.Lists["network"]) != 0 {
		t.Fatalf("zone %+v", zone)
	}
}

func TestKillSwitchDoesNotTreatTailscaleAsUplink(t *testing.T) {
	fw, err := cli.ParseShow("firewall", string(fixture(t, "../uci-show-firewall.txt")))
	if err != nil {
		t.Fatal(err)
	}
	before, err := vpnkill.Desired(fw)
	if err != nil {
		t.Fatal(err)
	}
	after, err := vpnkill.Desired(append(fw, FirewallSections(true)...))
	if err != nil || len(after) != len(before) {
		t.Fatalf("kill switch changed with the tailscale zone: %d -> %d (%v)", len(before), len(after), err)
	}
}

func TestCheckFirewall(t *testing.T) {
	fw, err := cli.ParseShow("firewall", string(fixture(t, "../uci-show-firewall.txt")))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckFirewall(fw); err != nil {
		t.Fatal(err)
	}
	squat := device.Section{Name: "cfg99", Type: "zone", Options: map[string]string{"name": ZoneName}}
	if err := CheckFirewall(append(fw, squat)); err == nil {
		t.Fatal("foreign travo_ts zone accepted")
	}
	if err := CheckFirewall(nil); err == nil {
		t.Fatal("no lan zone accepted")
	}
}
