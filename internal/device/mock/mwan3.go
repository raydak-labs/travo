package mock

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// MWAN3Interfaces mimics `mwan3 interfaces` on the committed config: an enabled interface
// section with track_ip is online with active tracking (offline when named in
// MWAN3Offline), one without is "unknown and tracking is disabled". Line shapes are the
// container captures in testdata/container/openwrt-25.12.6/mwan3/.
func (d *Device) MWAN3Interfaces(context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var b strings.Builder
	b.WriteString("Interface status:\n")
	for _, s := range d.mwan3Sections("interface") {
		switch {
		case s.Options["enabled"] != "1":
		case len(s.Lists["track_ip"]) == 0:
			fmt.Fprintf(&b, " interface %s is unknown and tracking is disabled\n", s.Name)
		case slices.Contains(d.MWAN3Offline, s.Name):
			fmt.Fprintf(&b, " interface %s is offline and tracking is active\n", s.Name)
		default:
			fmt.Fprintf(&b, " interface %s is online and tracking is active (online 00h:00m:40s, uptime 00h:00m:48s)\n", s.Name)
		}
	}
	b.WriteString("\n")
	return b.String(), nil
}

// MangleRules mimics `iptables -t mangle -S` while mwan3 runs: its hooks, a jump per
// non-IPv6 rule (moved rules first, as Reorder asked), and each policy marking for its
// lowest-metric online member (the last_resort comment otherwise). MangleErr replaces it
// (the incompatible-table failure).
func (d *Device) MangleRules(ctx context.Context) (string, error) {
	if d.MangleErr != nil {
		return "", d.MangleErr
	}
	if d.MangleOut != "" {
		return d.MangleOut, nil
	}
	states, _ := d.MWAN3Interfaces(ctx)
	d.mu.Lock()
	defer d.mu.Unlock()
	lines := []string{"-P PREROUTING ACCEPT", "-P INPUT ACCEPT", "-P FORWARD ACCEPT", "-P OUTPUT ACCEPT", "-P POSTROUTING ACCEPT"}
	if !d.services["mwan3"].running {
		return strings.Join(lines, "\n") + "\n", nil
	}
	lines = append(lines, "-A PREROUTING -j mwan3_hook", "-A OUTPUT -j mwan3_hook")
	members := map[string]device.Section{}
	for _, m := range d.mwan3Sections("member") {
		members[m.Name] = m
	}
	for _, r := range d.orderedRules() {
		lines = append(lines, "-A mwan3_rules -m mark --mark 0x0/0x3f00 -j mwan3_policy_"+r.Options["use_policy"])
	}
	for _, p := range d.mwan3Sections("policy") {
		comment, best := p.Options["last_resort"], 0
		for _, name := range p.Lists["use_member"] {
			m := members[name]
			metric, _ := strconv.Atoi(m.Options["metric"])
			iface := m.Options["interface"]
			if strings.Contains(states, " interface "+iface+" is online ") && (best == 0 || metric < best) {
				comment, best = fmt.Sprintf(`"%s 1 1"`, iface), metric
			}
		}
		lines = append(lines, fmt.Sprintf("-A mwan3_policy_%s -m mark --mark 0x0/0x3f00 -m comment --comment %s -j MARK --set-xmark 0x100/0x3f00", p.Name, comment))
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// orderedRules: non-IPv6 rules, those placed by Reorder first at their index; callers hold d.mu.
func (d *Device) orderedRules() []device.Section {
	pos := func(name string) int {
		for _, r := range slices.Backward(d.Reorders) {
			if at, ok := strings.CutPrefix(r, "mwan3."+name+"="); ok {
				i, _ := strconv.Atoi(at)
				return i
			}
		}
		return 1 << 20
	}
	var out []device.Section
	for _, r := range d.mwan3Sections("rule") {
		if r.Options["family"] != "ipv6" {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b device.Section) int { return pos(a.Name) - pos(b.Name) })
	return out
}

// mwan3Sections lists committed mwan3 sections of one type in name order; callers hold d.mu.
func (d *Device) mwan3Sections(typ string) []device.Section {
	var out []device.Section
	if c := d.committed["mwan3"]; c != nil {
		for _, s := range clone(c).sections {
			if s.Type == typ {
				out = append(out, *s)
			}
		}
	}
	slices.SortFunc(out, func(a, b device.Section) int { return strings.Compare(a.Name, b.Name) })
	return out
}

var _ device.MWAN3Reader = (*Device)(nil)
