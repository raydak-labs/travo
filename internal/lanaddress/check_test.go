package lanaddress

import (
	"errors"
	"net/netip"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

const captures = "../../testdata/device/axt1800-25.12.6/"

func export(t *testing.T, config, name string) []device.Section {
	t.Helper()
	b, err := os.ReadFile(captures + name)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.HasPrefix(text, "package ") { // section excerpts of `uci export`
		text = "package " + config + "\n\n" + text
	}
	secs, err := cli.ParseExport(config, text)
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

// inputs is the captured router: lan `list ipaddr 192.168.1.1/24`, guest 192.168.2.1/24,
// wg0 `addresses 100.64.41.140/32` (wan/uci-export-network.txt); dhcp.lan pool 100+150;
// live wwan lease 10.0.1.195/24 (network/interface-dump.json).
func inputs(t *testing.T) Inputs {
	t.Helper()
	raw, err := os.ReadFile(captures + "network/interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	live, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		t.Fatal(err)
	}
	return Inputs{
		Network:  export(t, "network", "wan/uci-export-network.txt"),
		DHCP:     export(t, "dhcp", "dhcp/uci-export-dhcp.txt"),
		Firewall: export(t, "firewall", "network/uci-export-firewall.txt"),
		Live:     live,
	}
}

func TestPlanKeepsTheListForm(t *testing.T) {
	ws, err := Plan(inputs(t), netip.MustParsePrefix("192.168.8.1/24"))
	want := []Write{{"delete", "ipaddr", ""}, {"add", "ipaddr", "192.168.8.1/24"}}
	if err != nil || !slices.Equal(ws, want) {
		t.Fatalf("%v %v", ws, err)
	}
}

func TestPlanOptionWithNetmaskBecomesCIDR(t *testing.T) {
	in := inputs(t)
	i := slices.IndexFunc(in.Network, func(s device.Section) bool { return s.Name == Interface })
	in.Network[i].Lists = map[string][]string{}
	in.Network[i].Options["ipaddr"], in.Network[i].Options["netmask"] = "192.168.1.1", "255.255.255.0"
	if _, err := Plan(in, netip.MustParsePrefix("192.168.1.1/24")); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("same address in netmask form: %v", err)
	}
	ws, err := Plan(in, netip.MustParsePrefix("10.20.0.1/24"))
	want := []Write{{"set", "ipaddr", "10.20.0.1/24"}, {"delete", "netmask", ""}}
	if err != nil || !slices.Equal(ws, want) {
		t.Fatalf("%v %v", ws, err)
	}
}

func TestPlanRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		next string
		edit func(*Inputs)
		want error
	}{
		"unchanged":      {"192.168.1.1/24", nil, txn.ErrNoChanges},
		"guest overlap":  {"192.168.2.254/24", nil, ErrOverlap},
		"wwan lease":     {"10.0.1.1/24", nil, ErrOverlap},
		"wireguard":      {"100.64.41.129/25", nil, ErrOverlap},
		"pool too large": {"192.168.8.1/25", nil, ErrPoolOutside},
		"foreign proto":  {"192.168.8.1/24", func(in *Inputs) { lan(in).Options["proto"] = "dhcp" }, ErrForeign},
		"two addresses":  {"192.168.8.1/24", func(in *Inputs) { lan(in).Lists["ipaddr"] = []string{"192.168.1.1/24", "10.9.0.1/24"} }, ErrForeign},
		"reservation": {"192.168.8.1/24", func(in *Inputs) {
			in.DHCP = append(in.DHCP, export(t, "dhcp", "clients/uci-export-dhcp-host-staged.txt")...)
		}, ErrDependents},
		"port forward": {"192.168.8.1/24", func(in *Inputs) { in.Firewall = append(in.Firewall, redirect("192.168.1.20")) }, ErrDependents},
		"adguard expert": {"192.168.8.1/24", func(in *Inputs) {
			in.Travo = []device.Section{{Type: "travo-greenfield", Name: "main", Options: map[string]string{"adguard_topology": "expert"}}}
		}, ErrAdGuardExpert},
		"router collides": {"192.168.1.119/24", func(in *Inputs) {
			in.DHCP = append(in.DHCP, export(t, "dhcp", "clients/uci-export-dhcp-host-staged.txt")...)
		}, ErrDependents},
	} {
		in := inputs(t)
		if tc.edit != nil {
			tc.edit(&in)
		}
		if _, err := Plan(in, netip.MustParsePrefix(tc.next)); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

func TestPlanAllowsWhatStillFits(t *testing.T) {
	in := inputs(t)
	in.DHCP = append(in.DHCP, export(t, "dhcp", "clients/uci-export-dhcp-host-staged.txt")...)
	in.Firewall = append(in.Firewall, redirect("192.168.1.20"))
	// Same subnet, new router host: reservation .119 and forward .20 stay valid.
	if _, err := Plan(in, netip.MustParsePrefix("192.168.1.254/24")); err != nil {
		t.Fatal(err)
	}
	// An off DHCP server does not pin the pool.
	in = inputs(t)
	dhcpLAN(&in).Options["ignore"] = "1"
	if _, err := Plan(in, netip.MustParsePrefix("192.168.8.1/28")); err != nil {
		t.Fatal(err)
	}
}

func TestDependentsErrorNamesSections(t *testing.T) {
	in := inputs(t)
	in.Firewall = append(in.Firewall, redirect("192.168.1.20"))
	_, err := Plan(in, netip.MustParsePrefix("192.168.8.1/24"))
	var de *DependentsError
	if !errors.As(err, &de) || !slices.Equal(de.Sections, []string{"firewall.travo_fwd_0 (192.168.1.20)"}) {
		t.Fatalf("%v", err)
	}
}

func TestCarries(t *testing.T) {
	live := inputs(t).Live
	if !Carries(live, netip.MustParsePrefix("192.168.1.1/24")) || Carries(live, netip.MustParsePrefix("192.168.1.1/25")) {
		t.Fatal("Carries must match address and length exactly")
	}
}

func lan(in *Inputs) *device.Section {
	return &in.Network[slices.IndexFunc(in.Network, func(s device.Section) bool { return s.Name == Interface })]
}

func dhcpLAN(in *Inputs) *device.Section {
	return &in.DHCP[slices.IndexFunc(in.DHCP, func(s device.Section) bool { return s.Name == "lan" })]
}

// redirect is the shape internal/portfwd stages (invariants §7).
func redirect(ip string) device.Section {
	return device.Section{Name: "travo_fwd_0", Type: "redirect", Options: map[string]string{
		"target": "DNAT", "src": "wan", "dest": "lan", "proto": "tcp", "src_dport": "8443", "dest_ip": ip, "enabled": "1",
	}, Lists: map[string][]string{}}
}
