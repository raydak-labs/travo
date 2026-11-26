// Package vpnkill enforces the VPN kill switch with firewall sections Travo owns by name
// (spec item 8, invariants §7, §11). It never edits a section it did not create.
//
// Policy (pinned in docs/invariants.md §11):
//   - The VPN zone `travo_vpn` (network wg0, masq, mtu_fix, REJECT input/forward) and the
//     forwarding lan -> travo_vpn let LAN clients use the tunnel. They stay when the kill switch is off.
//     When the Travo guest zone exists, the forwarding guest -> travo_vpn does the same for guests.
//   - The kill switch is one REJECT rule `travo_killswitch_<zone>` from lan to every zone that
//     holds an uplink network (wan, wan6, wwan, usbtether), plus `travo_killswitch_guest_<zone>`
//     from the guest zone when it exists. fw4 evaluates rules before zone forwardings, so the
//     user's lan -> wan (and the guest's guest_wan) forwarding stays untouched and comes back on disable.
//   - The rules live in UCI, so they hold across tunnel disconnects and from boot, before the
//     tunnel is up. Router-originated traffic (WireGuard handshakes, dnsmasq) is output, not forward.
package vpnkill

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const (
	ZoneName     = "travo_vpn"
	LANForward   = "travo_vpn_lan"
	GuestForward = "travo_vpn_guest"
	RulePrefix   = "travo_killswitch_"
	// GuestZone is the zone internal/wifi writes for the guest AP: section and zone name both
	// `guest` (invariants §10). A zone named guest in any other section is the user's.
	GuestZone = "guest"
	lanZone   = "lan"
	tunnelNet = "wg0"
)

// uplinkNetworks are the candidate uplinks of invariants §7 plus wan6.
var uplinkNetworks = []string{"wan", "wan6", "wwan", "usbtether"}

// ErrUnsupported means the firewall layout has no lan zone, no uplink zone, a zone name that
// cannot form a UCI section name, or a foreign section on a name Travo owns.
var ErrUnsupported = errors.New("firewall layout is not supported for the kill switch")

var sectionName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Desired is pure: the firewall config in, the owned sections the kill switch wants out.
func Desired(sections []device.Section) ([]device.Section, error) {
	hasLAN := false
	var uplinks []string
	for _, s := range sections {
		if Owned(s.Name) {
			continue
		}
		if s.Type != "zone" {
			continue
		}
		name := s.Options["name"]
		if name == lanZone {
			hasLAN = true
			continue
		}
		if name == ZoneName {
			return nil, ErrUnsupported // a foreign zone using our zone name
		}
		if slices.ContainsFunc(networks(s), func(n string) bool { return slices.Contains(uplinkNetworks, n) }) {
			if !sectionName.MatchString(name) {
				return nil, ErrUnsupported
			}
			uplinks = append(uplinks, name)
		}
	}
	if !hasLAN || len(uplinks) == 0 {
		return nil, ErrUnsupported
	}
	slices.Sort(uplinks)
	uplinks = slices.Compact(uplinks)
	out := VPNZone(sections)
	for _, zone := range uplinks {
		out = append(out, rejectRule(RulePrefix+zone, lanZone, zone))
	}
	if HasGuestZone(sections) {
		for _, zone := range uplinks {
			out = append(out, rejectRule(RulePrefix+GuestZone+"_"+zone, GuestZone, zone))
		}
	}
	// An uplink zone named guest_<x> next to an uplink zone <x> would collide on a rule name.
	names := make(map[string]bool, len(out))
	for _, s := range out {
		if names[s.Name] {
			return nil, ErrUnsupported
		}
		names[s.Name] = true
	}
	return out, nil
}

func rejectRule(name, src, dest string) device.Section {
	label := "Travo VPN kill switch (" + dest + ")"
	if src != lanZone {
		label = "Travo VPN kill switch (" + src + " to " + dest + ")"
	}
	return device.Section{Name: name, Type: "rule", Options: map[string]string{
		"name": label, "src": src, "dest": dest, "proto": "all", "target": "REJECT",
	}, Lists: map[string][]string{}}
}

// VPNZone is the zone and forwardings that let LAN (and guest, when the Travo guest zone
// exists) traffic into the tunnel, wan-pattern surface.
func VPNZone(sections []device.Section) []device.Section {
	out := []device.Section{
		{Name: ZoneName, Type: "zone", Options: map[string]string{
			"name": ZoneName, "input": "REJECT", "output": "ACCEPT", "forward": "REJECT", "masq": "1", "mtu_fix": "1",
		}, Lists: map[string][]string{"network": {tunnelNet}}},
		{Name: LANForward, Type: "forwarding", Options: map[string]string{"src": lanZone, "dest": ZoneName}, Lists: map[string][]string{}},
	}
	if HasGuestZone(sections) {
		out = append(out, guestForward())
	}
	return out
}

func guestForward() device.Section {
	return device.Section{Name: GuestForward, Type: "forwarding", Options: map[string]string{"src": GuestZone, "dest": ZoneName}, Lists: map[string][]string{}}
}

// HasGuestZone reports whether the guest zone internal/wifi writes exists, matched by
// section name and zone name so a user's own zone named guest is never adopted.
func HasGuestZone(sections []device.Section) bool {
	return slices.ContainsFunc(sections, func(s device.Section) bool {
		return s.Name == GuestZone && s.Type == "zone" && s.Options["name"] == GuestZone
	})
}

// GuestSections is pure: the firewall config including the Travo guest zone in, the owned
// sections the current VPN state needs for guest clients out. The forwarding follows the VPN
// zone, the rules follow the kill switch; nil when neither is configured.
func GuestSections(sections []device.Section) ([]device.Section, error) {
	if !HasGuestZone(sections) {
		return nil, nil
	}
	var out []device.Section
	if slices.ContainsFunc(sections, func(s device.Section) bool { return s.Name == ZoneName }) {
		out = append(out, guestForward())
	}
	if Enabled(sections) {
		want, err := Desired(sections)
		if err != nil {
			return nil, err
		}
		for _, s := range want {
			if s.Type == "rule" && s.Options["src"] == GuestZone {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func networks(s device.Section) []string {
	if v, ok := s.Lists["network"]; ok {
		return v
	}
	return strings.Fields(s.Options["network"])
}

// Owned reports whether Travo created the named section.
func Owned(name string) bool {
	return name == ZoneName || name == LANForward || name == GuestForward || strings.HasPrefix(name, RulePrefix)
}

// Enabled reports whether any kill switch rule is configured.
func Enabled(sections []device.Section) bool {
	return slices.ContainsFunc(sections, func(s device.Section) bool {
		return s.Type == "rule" && strings.HasPrefix(s.Name, RulePrefix)
	})
}

// Matches reports whether the owned sections in current are exactly want.
func Matches(current, want []device.Section) bool {
	var owned []device.Section
	for _, s := range current {
		if Owned(s.Name) {
			owned = append(owned, s)
		}
	}
	if len(owned) != len(want) {
		return false
	}
	byName := make(map[string]device.Section, len(want))
	for _, s := range want {
		byName[s.Name] = s
	}
	for _, s := range owned {
		w, ok := byName[s.Name]
		if !ok || s.Type != w.Type || !maps.Equal(s.Options, w.Options) ||
			!maps.EqualFunc(nonEmpty(s.Lists), nonEmpty(w.Lists), slices.Equal[[]string]) {
			return false
		}
	}
	return true
}

func nonEmpty(lists map[string][]string) map[string][]string {
	out := make(map[string][]string, len(lists))
	for k, v := range lists {
		if len(v) > 0 {
			out[k] = v
		}
	}
	return out
}
