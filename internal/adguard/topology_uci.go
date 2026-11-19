package adguard

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Topology modes (ADR 0021), persisted in UCI travo-greenfield.main.adguard_topology.
const (
	TopologyDefault = "default"
	TopologyExpert  = "expert"
)

// TopologyConfig/Section/Option name where the mode lives; it is committed in the same
// transaction as dhcp, so a watchdog restore puts the mode back with dnsmasq's port.
const (
	TopologyConfig  = "travo-greenfield"
	TopologySection = "main"
	TopologyOption  = "adguard_topology"
)

// TopologyConfigs are the UCI configs one switch stages, in one snapshot.
var TopologyConfigs = []string{"dhcp", TopologyConfig}

// ErrTopologyForeign: dhcp or network is not shaped the way the switch can plan for.
var ErrTopologyForeign = errors.New("the lan or dnsmasq configuration is not the shape the expert topology supports")

// UCIEdit is one option the switch changes. Before/After nil = absent. List options are
// written as lists; scalars hold one value.
type UCIEdit struct {
	Config  string   `json:"config"`
	Section string   `json:"section"`
	Option  string   `json:"option"`
	List    bool     `json:"list,omitempty"`
	Before  []string `json:"before"`
	After   []string `json:"after"`
}

// ExpertPlan is what a switch to expert needs from dhcp and network.
type ExpertPlan struct {
	Hosts []string  // AGH bind_hosts: loopback, lan, guest when served
	Edits []UCIEdit // dhcp and the mode
}

// PlanExpert is pure: dhcp, network and travo-greenfield sections in, the expert plan out.
// dnsmasq moves to DnsmasqPort and stops being the advertised DNS, so every served pool
// gets DHCP option 6 with its own address, and odhcpd stops announcing the lan's IPv6
// address as resolver (nothing answers DNS on it in expert mode).
func PlanExpert(dhcp, network, travo []device.Section) (ExpertPlan, error) {
	dm := slices.IndexFunc(dhcp, func(s device.Section) bool { return s.Type == "dnsmasq" })
	if dm < 0 || slices.IndexFunc(dhcp[dm+1:], func(s device.Section) bool { return s.Type == "dnsmasq" }) >= 0 {
		return ExpertPlan{}, fmt.Errorf("%w: want exactly one dnsmasq section", ErrTopologyForeign)
	}
	lan, ok := poolAddress(dhcp, network, "lan")
	if !ok {
		return ExpertPlan{}, fmt.Errorf("%w: lan has no single static IPv4 address or no DHCP pool", ErrTopologyForeign)
	}
	guest, _ := poolAddress(dhcp, network, "guest")
	var edits []UCIEdit
	edits = append(edits, scalarEdit(dhcp[dm], "dhcp", "port", DnsmasqPort))
	for _, p := range []struct{ pool, addr string }{{"lan", lan}, {"guest", guest}} {
		if p.addr == "" {
			continue
		}
		s := dhcp[slices.IndexFunc(dhcp, isPool(p.pool))]
		before := optionValues(s, "dhcp_option")
		opt := "6," + p.addr
		if !slices.Contains(before, opt) {
			edits = append(edits, UCIEdit{Config: "dhcp", Section: s.Name, Option: "dhcp_option", List: true, Before: before, After: append(slices.Clone(before), opt)})
		}
	}
	lanPool := dhcp[slices.IndexFunc(dhcp, isPool("lan"))]
	edits = append(edits, scalarEdit(lanPool, "dhcp", "dns_service", "0"))
	main := device.Section{Name: TopologySection}
	if i := slices.IndexFunc(travo, func(s device.Section) bool { return s.Name == TopologySection }); i >= 0 {
		main = travo[i]
	}
	edits = append(edits, scalarEdit(main, TopologyConfig, TopologyOption, TopologyExpert))
	return ExpertPlan{Hosts: ExpertHosts(lan, guest), Edits: dropNoops(edits)}, nil
}

// Mode reads the persisted mode; absent is default.
func Mode(travo []device.Section) string {
	for _, s := range travo {
		if s.Name == TopologySection && s.Options[TopologyOption] == TopologyExpert {
			return TopologyExpert
		}
	}
	return TopologyDefault
}

// Inverse swaps Before and After: the edits that undo a plan.
func Inverse(edits []UCIEdit) []UCIEdit {
	out := make([]UCIEdit, len(edits))
	for i, e := range edits {
		e.Before, e.After = e.After, e.Before
		out[i] = e
	}
	return out
}

// StageEdits writes every edit's After without committing.
func StageEdits(ctx context.Context, uci device.UCI, edits []UCIEdit) error {
	for _, e := range edits {
		if err := stageEdit(ctx, uci, e); err != nil {
			return fmt.Errorf("stage %s.%s.%s: %w", e.Config, e.Section, e.Option, err)
		}
	}
	return nil
}

func stageEdit(ctx context.Context, uci device.UCI, e UCIEdit) error {
	if e.After == nil || e.List {
		if err := uci.DeleteOption(ctx, e.Config, e.Section, e.Option); err != nil && !errors.Is(err, device.ErrNotFound) {
			return err
		}
	}
	if !e.List && e.After != nil {
		if e.Config == TopologyConfig {
			if err := uci.SetType(ctx, e.Config, e.Section, TopologyConfig); err != nil {
				return err
			}
		}
		return uci.Set(ctx, e.Config, e.Section, e.Option, e.After[0])
	}
	for _, v := range e.After {
		if err := uci.AddToList(ctx, e.Config, e.Section, e.Option, v); err != nil {
			return err
		}
	}
	return nil
}

func scalarEdit(s device.Section, config, option, after string) UCIEdit {
	var before []string
	if v, ok := s.Options[option]; ok {
		before = []string{v}
	}
	return UCIEdit{Config: config, Section: s.Name, Option: option, Before: before, After: []string{after}}
}

func dropNoops(edits []UCIEdit) []UCIEdit {
	return slices.DeleteFunc(edits, func(e UCIEdit) bool { return e.Before != nil && slices.Equal(e.Before, e.After) })
}

func optionValues(s device.Section, option string) []string {
	if v, ok := s.Lists[option]; ok {
		return slices.Clone(v)
	}
	if v, ok := s.Options[option]; ok {
		return []string{v}
	}
	return nil
}

func isPool(iface string) func(device.Section) bool {
	return func(s device.Section) bool { return s.Type == "dhcp" && s.Options["interface"] == iface }
}

// poolAddress is the interface's single static IPv4 address when dnsmasq serves DHCPv4 on it.
func poolAddress(dhcp, network []device.Section, iface string) (string, bool) {
	i := slices.IndexFunc(dhcp, isPool(iface))
	if i < 0 || dhcp[i].Options["ignore"] == "1" || dhcp[i].Options["dhcpv4"] == "disabled" {
		return "", false
	}
	n := slices.IndexFunc(network, func(s device.Section) bool { return s.Type == "interface" && s.Name == iface })
	if n < 0 || network[n].Options["proto"] != "static" {
		return "", false
	}
	addrs := optionValues(network[n], "ipaddr")
	if len(addrs) != 1 {
		return "", false
	}
	host, _, _ := strings.Cut(addrs[0], "/")
	a, err := netip.ParseAddr(host)
	if err != nil || !a.Is4() {
		return "", false
	}
	return a.String(), true
}
