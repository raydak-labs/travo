package wifi

import (
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// reconcileRadio handles a dual-radio device whose only band-compatible radio carries
// the AP: the STA takes that radio and every enabled AP on it moves to target. Radios
// are tried by name. An empty sta means no layout works (wifi_no_free_radio).
func reconcileRadio(radios []facts.Radio, bands []string, wireless []device.Section) (sta, target string, moved []string) {
	for _, radio := range radios {
		if radio.Disabled || !slices.Contains(bands, radio.Band) || radioDisabled(wireless, radio.Name) {
			continue
		}
		aps := enabledAPsOn(wireless, radio.Name)
		if len(aps) == 0 {
			continue
		}
		for _, other := range radios {
			if other.Name != radio.Name && canTakeAPs(wireless, other, aps) {
				for _, ap := range aps {
					moved = append(moved, ap.Name)
				}
				return radio.Name, other.Name, moved
			}
		}
	}
	return "", "", nil
}

func enabledAPsOn(wireless []device.Section, radio string) []device.Section {
	var out []device.Section
	for _, s := range wireless {
		if isAP(s) && s.Options["device"] == radio && s.Options["disabled"] != "1" {
			out = append(out, s)
		}
	}
	return out
}

// canTakeAPs: the radio is enabled, carries no STA except the uplink (which leaves in
// the same commit), and has no enabled AP of the same role (main or guest) already.
func canTakeAPs(wireless []device.Section, radio facts.Radio, aps []device.Section) bool {
	if radio.Disabled || radioDisabled(wireless, radio.Name) {
		return false
	}
	for _, s := range wireless {
		if s.Type != "wifi-iface" || s.Options["device"] != radio.Name || s.Options["disabled"] == "1" {
			continue
		}
		if s.Options["mode"] == "sta" && s.Options["network"] != "wwan" {
			return false
		}
		if isAP(s) && slices.ContainsFunc(aps, func(ap device.Section) bool { return isGuestRole(ap) == isGuestRole(s) }) {
			return false
		}
	}
	return true
}

func isGuestRole(s device.Section) bool { return !slices.Contains(sectionNetworks(s), "lan") }

// reconcileWrites orders a move so no staged state has an enabled AP and STA on one
// radio: disable the enabled uplink STAs, move the APs, stage the STA, enable it last.
func reconcileWrites(wireless []device.Section, staWrites []uciWrite, moved []string, target string) []uciWrite {
	var out []uciWrite
	sta := uplinkSections(wireless)
	for _, s := range sta {
		if s.Options["disabled"] != "1" {
			out = append(out, uciWrite{"set", "wireless", s.Name, "disabled", "1"})
		}
	}
	for _, section := range moved {
		out = append(out, planAPMove(section, target))
	}
	enable := uciWrite{"set", "wireless", "wwan", "disabled", "0"}
	if len(sta) > 0 {
		enable.section = sta[0].Name
	}
	for _, w := range staWrites {
		if w != enable && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return append(out, enable)
}
