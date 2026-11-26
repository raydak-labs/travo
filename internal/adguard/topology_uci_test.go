package adguard

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
)

const fixtures = "../../testdata/device/axt1800-25.12.6/"

func fixtureSections(t *testing.T, config string, export bool, files ...string) []device.Section {
	t.Helper()
	var out []device.Section
	for _, f := range files {
		b, err := os.ReadFile(fixtures + f)
		if err != nil {
			t.Fatal(err)
		}
		parse := cli.ParseShow
		if export {
			parse = cli.ParseExport
		}
		secs, err := parse(config, string(b))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, secs...)
	}
	return out
}

func TestPlanExpertOnTheRouterConfig(t *testing.T) {
	dhcp := fixtureSections(t, "dhcp", true, "uci-export-dhcp.txt")
	network := fixtureSections(t, "network", false, "uci-show-network.txt")
	plan, err := PlanExpert(dhcp, network, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Hosts, []string{"127.0.0.1", "192.168.1.1"}) {
		t.Fatalf("hosts %v", plan.Hosts)
	}
	dm := dhcp[slices.IndexFunc(dhcp, func(s device.Section) bool { return s.Type == "dnsmasq" })].Name
	want := []UCIEdit{
		{Config: "dhcp", Section: dm, Option: "port", Before: []string{"53"}, After: []string{"5354"}},
		{Config: "dhcp", Section: "lan", Option: "dhcp_option", List: true, After: []string{"6,192.168.1.1"}},
		{Config: "dhcp", Section: "lan", Option: "dns_service", After: []string{"0"}},
		{Config: "travo-greenfield", Section: "main", Option: "adguard_topology", After: []string{"expert"}},
	}
	if len(plan.Edits) != len(want) {
		t.Fatalf("edits %+v", plan.Edits)
	}
	for i := range want {
		g, w := plan.Edits[i], want[i]
		if g.Config != w.Config || g.Section != w.Section || g.Option != w.Option || g.List != w.List || !slices.Equal(g.Before, w.Before) || !slices.Equal(g.After, w.After) {
			t.Errorf("edit %d: got %+v want %+v", i, g, w)
		}
	}
	inv := Inverse(plan.Edits)
	if !slices.Equal(inv[0].After, []string{"53"}) || inv[1].After != nil || inv[3].After != nil {
		t.Fatalf("inverse restores what was there: %+v", inv)
	}
}

func TestPlanExpertServesTheGuestNetwork(t *testing.T) {
	dhcp := append(fixtureSections(t, "dhcp", true, "uci-export-dhcp.txt"), fixtureSections(t, "dhcp", false, "guest/uci-dhcp-guest.txt")...)
	network := fixtureSections(t, "network", false, "uci-show-network.txt", "guest/uci-network-guest.txt")
	plan, err := PlanExpert(dhcp, network, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Hosts, []string{"127.0.0.1", "192.168.1.1", "192.168.2.1"}) {
		t.Fatalf("hosts %v", plan.Hosts)
	}
	if !slices.ContainsFunc(plan.Edits, func(e UCIEdit) bool {
		return e.Section == "guest" && e.Option == "dhcp_option" && slices.Equal(e.After, []string{"6,192.168.2.1"})
	}) {
		t.Fatalf("guest pool must advertise its own address: %+v", plan.Edits)
	}
	if Mode(nil) != TopologyDefault || Mode([]device.Section{{Name: "main", Options: map[string]string{"adguard_topology": "expert"}}}) != TopologyExpert {
		t.Fatal("Mode reads travo-greenfield.main.adguard_topology")
	}
}

func TestPlanExpertRefusesForeignShapes(t *testing.T) {
	dhcp := fixtureSections(t, "dhcp", true, "uci-export-dhcp.txt")
	network := fixtureSections(t, "network", false, "uci-show-network.txt")
	two := append(slices.Clone(dhcp), device.Section{Name: "second", Type: "dnsmasq"})
	if _, err := PlanExpert(two, network, nil); !errors.Is(err, ErrTopologyForeign) {
		t.Fatalf("two dnsmasq instances: %v", err)
	}
	dhcpLAN := slices.DeleteFunc(slices.Clone(network), func(s device.Section) bool { return s.Name == "lan" })
	if _, err := PlanExpert(dhcp, dhcpLAN, nil); !errors.Is(err, ErrTopologyForeign) {
		t.Fatalf("no lan interface: %v", err)
	}
}
