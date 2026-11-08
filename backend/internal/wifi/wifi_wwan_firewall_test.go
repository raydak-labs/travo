package wifi

import (
	"slices"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// realFormatUCI returns list options comma-joined from GetSections, as
// parseShowOutput does on a device; the mock hands them back space-separated.
type realFormatUCI struct {
	*revertingUCI
	addLists []string
}

func (r *realFormatUCI) AddList(config, section, option, value string) error {
	r.addLists = append(r.addLists, value)
	return r.revertingUCI.AddList(config, section, option, value)
}

func (r *realFormatUCI) GetSections(config string) (map[string]map[string]string, error) {
	sections, err := r.revertingUCI.GetSections(config)
	if err != nil {
		return nil, err
	}
	for _, opts := range sections {
		if v, ok := opts["network"]; ok && config == "firewall" {
			opts["network"] = strings.Join(strings.Fields(v), ",")
		}
	}
	return sections, nil
}

func wwanFirewallFixture(t *testing.T, zoneNetworks string) (*Service, *realFormatUCI) {
	t.Helper()
	u := &realFormatUCI{revertingUCI: &revertingUCI{MockUCI: uci.NewMockUCI()}}
	if err := u.Set("firewall", "zone_wan", "network", zoneNetworks); err != nil {
		t.Fatal(err)
	}
	return &Service{uci: u}, u
}

func zoneNetworks(t *testing.T, u *realFormatUCI) []string {
	t.Helper()
	v, err := u.Get("firewall", "zone_wan", "network")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(v)
}

func TestEnsureWwanFirewall_PresentInCommaJoinedListIsNoOp(t *testing.T) {
	svc, u := wwanFirewallFixture(t, "wan wan6 wwan")

	if err := svc.ensureWwanFirewall(); err != nil {
		t.Fatal(err)
	}
	if len(u.addLists) != 0 {
		t.Errorf("AddList called %v, want none", u.addLists)
	}
	if c := u.commitCalls(); len(c) != 0 {
		t.Errorf("commits = %v, want none", c)
	}
}

func TestEnsureWwanFirewall_AbsentIsAddedOnceAndCommitted(t *testing.T) {
	svc, u := wwanFirewallFixture(t, "wan wan6")

	if err := svc.ensureWwanFirewall(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(u.addLists, []string{"wwan"}) {
		t.Errorf("AddList calls = %v, want exactly [wwan]", u.addLists)
	}
	if c := u.commitCalls(); !slices.Equal(c, []string{"firewall"}) {
		t.Errorf("commits = %v, want [firewall]", c)
	}
	if got := zoneNetworks(t, u); !slices.Equal(got, []string{"wan", "wan6", "wwan"}) {
		t.Errorf("zone network = %v", got)
	}
}

func TestEnsureWwanFirewall_CollapsesDuplicatesKeepingOrder(t *testing.T) {
	svc, u := wwanFirewallFixture(t, "wan wan6"+strings.Repeat(" wwan", 16))

	if err := svc.ensureWwanFirewall(); err != nil {
		t.Fatal(err)
	}
	if got := zoneNetworks(t, u); !slices.Equal(got, []string{"wan", "wan6", "wwan"}) {
		t.Errorf("zone network = %v, want [wan wan6 wwan]", got)
	}
	if c := u.commitCalls(); !slices.Equal(c, []string{"firewall"}) {
		t.Errorf("commits = %v, want exactly one firewall commit", c)
	}

	if err := svc.ensureWwanFirewall(); err != nil {
		t.Fatal(err)
	}
	if c := u.commitCalls(); len(c) != 1 {
		t.Errorf("second call committed again: %v", c)
	}
}
