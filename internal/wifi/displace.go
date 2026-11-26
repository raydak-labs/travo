package wifi

import (
	"cmp"
	"slices"

	"github.com/raydak-labs/travo/internal/device"
	"github.com/raydak-labs/travo/internal/device/facts"
	"github.com/raydak-labs/travo/internal/lockout"
)

type apMove struct {
	section, target string
}

type displacePlan struct {
	radio    string
	moves    []apMove
	disabled []string
}

// displaceRadio chooses a band-compatible enabled radio R whose enabled APs can each
// move under reconcile rules or be disabled because a twin AP (same SSID, same role)
// stays enabled on another enabled radio. Among candidates it prefers a radio that
// does not host the caller's AP, then radio name ascending.
func displaceRadio(radios []facts.Radio, bands []string, wireless []device.Section, caller lockout.Caller) displacePlan {
	sorted := slices.Clone(radios)
	slices.SortFunc(sorted, func(a, b facts.Radio) int { return cmp.Compare(a.Name, b.Name) })
	avoid := callerRadio(sorted, caller)
	var best displacePlan
	for _, radio := range sorted {
		if radio.Disabled || !slices.Contains(bands, radio.Band) || radioDisabled(wireless, radio.Name) {
			continue
		}
		plan, ok := planRadioDisplace(sorted, wireless, radio.Name)
		if !ok {
			continue
		}
		if best.radio == "" || (best.radio == avoid && plan.radio != avoid) {
			best = plan
		}
	}
	return best
}

func planRadioDisplace(radios []facts.Radio, wireless []device.Section, radio string) (displacePlan, bool) {
	aps := enabledAPsOn(wireless, radio)
	if len(aps) == 0 {
		return displacePlan{}, false
	}
	plan := displacePlan{radio: radio}
	takenRoles := map[string][2]bool{}
	// APs without a twin can only move, so they claim move targets before APs that
	// could fall back to being disabled.
	slices.SortStableFunc(aps, func(a, b device.Section) int {
		return cmp.Compare(boolRank(hasTwinAP(radios, wireless, radio, a)), boolRank(hasTwinAP(radios, wireless, radio, b)))
	})
	for _, ap := range aps {
		if target := findMoveTarget(radios, wireless, radio, ap, takenRoles); target != "" {
			plan.moves = append(plan.moves, apMove{section: ap.Name, target: target})
			roles := takenRoles[target]
			roles[roleIndex(ap)] = true
			takenRoles[target] = roles
			continue
		}
		if hasTwinAP(radios, wireless, radio, ap) {
			plan.disabled = append(plan.disabled, ap.Name)
			continue
		}
		return displacePlan{}, false
	}
	return plan, true
}

func findMoveTarget(radios []facts.Radio, wireless []device.Section, radio string, ap device.Section, takenRoles map[string][2]bool) string {
	idx := roleIndex(ap)
	for _, other := range radios {
		if other.Name == radio || takenRoles[other.Name][idx] {
			continue
		}
		if canTakeAPs(wireless, other, []device.Section{ap}) {
			return other.Name
		}
	}
	return ""
}

func roleIndex(ap device.Section) int {
	if isGuestRole(ap) {
		return 1
	}
	return 0
}

func hasTwinAP(radios []facts.Radio, wireless []device.Section, radio string, ap device.Section) bool {
	ssid := ap.Options["ssid"]
	if ssid == "" {
		return false
	}
	guest := isGuestRole(ap)
	return slices.ContainsFunc(wireless, func(other device.Section) bool {
		otherRadio := other.Options["device"]
		return other.Name != ap.Name &&
			isAP(other) &&
			other.Options["disabled"] != "1" &&
			otherRadio != "" &&
			otherRadio != radio &&
			radioEnabled(radios, wireless, otherRadio) &&
			other.Options["ssid"] == ssid &&
			isGuestRole(other) == guest
	})
}

func radioEnabled(radios []facts.Radio, wireless []device.Section, name string) bool {
	if radioDisabled(wireless, name) {
		return false
	}
	return slices.ContainsFunc(radios, func(r facts.Radio) bool {
		return r.Name == name && !r.Disabled
	})
}

func callerRadio(radios []facts.Radio, caller lockout.Caller) string {
	if caller.Method != lockout.WifiAP || caller.AP == "" {
		return ""
	}
	for _, r := range radios {
		for _, iface := range r.Ifaces {
			if iface.Mode == "ap" && iface.Ifname == caller.AP {
				return r.Name
			}
		}
	}
	return ""
}

// displaceWrites orders a displace so no staged state has an enabled AP and STA on one
// radio: disable enabled uplink STAs, disable displaced APs, move any movable APs,
// stage the STA writes, and enable the STA last.
func displaceWrites(wireless []device.Section, staWrites []uciWrite, plan displacePlan) []uciWrite {
	var out []uciWrite
	sta := uplinkSections(wireless)
	for _, s := range sta {
		if s.Options["disabled"] != "1" {
			out = append(out, uciWrite{"set", "wireless", s.Name, "disabled", "1"})
		}
	}
	for _, section := range plan.disabled {
		out = append(out, uciWrite{"set", "wireless", section, "disabled", "1"})
	}
	for _, m := range plan.moves {
		out = append(out, planAPMove(m.section, m.target))
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

func joinAPExpectations(wireless []device.Section, disabled []string) []apExpectation {
	var out []apExpectation
	for _, section := range wireless {
		if !isAP(section) {
			continue
		}
		if slices.Contains(disabled, section.Name) {
			out = append(out, apExpectation{section: section.Name, enabled: false})
		} else if section.Options["disabled"] != "1" && !radioDisabled(wireless, section.Options["device"]) {
			out = append(out, apExpectation{section: section.Name, enabled: true})
		}
	}
	return out
}

func markAPsDown(aps []lockout.AP, sections []string) {
	for i := range aps {
		if slices.Contains(sections, aps[i].Section) {
			aps[i].Enabled = false
		}
	}
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}
