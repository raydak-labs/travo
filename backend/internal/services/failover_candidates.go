package services

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

func (s *FailoverService) discoverCandidates(networkStatus models.NetworkStatus, cfgFile failoverConfigFile) []models.FailoverCandidate {
	known := map[string]models.FailoverCandidate{}
	trackerStates := s.readTrackerStates()
	for _, iface := range networkStatus.Interfaces {
		switch iface.Type {
		case "wan":
			known[iface.Name] = newCandidate(iface.Name, "Ethernet WAN", models.FailoverCandidateKindEthernet, iface.IsUp)
		case "wifi":
			known[iface.Name] = newCandidate(iface.Name, "WiFi uplink", models.FailoverCandidateKindWiFi, iface.IsUp)
		case "usb":
			known[iface.Name] = newCandidate(iface.Name, "USB tether", models.FailoverCandidateKindUSB, iface.IsUp)
		}
	}
	if _, ok := known["wwan"]; !ok && s.hasWirelessStation() {
		known["wwan"] = newCandidate("wwan", "WiFi uplink", models.FailoverCandidateKindWiFi, false)
		known["wwan"] = markUnavailable(known["wwan"])
	}
	s.addDiscoveredUSBNetworkCandidates(known)
	for _, saved := range cfgFile.Candidates {
		if _, ok := known[saved.InterfaceName]; !ok {
			saved.Available = false
			saved.IsUp = false
			saved.TrackingState = models.FailoverTrackingStateNotAvailable
			known[saved.InterfaceName] = saved
		}
	}

	// Iterate the candidates in a stable order. Ranging over the map directly
	// assigned fallback priorities in Go's randomised map order, so every
	// candidate that is not in the saved config came back with a different
	// priority on each call. GetConfig() is polled every 10s and by the UI, and
	// computeActiveInterface picks the active link in priority order, so the
	// reported active interface could flip between equally-ranked links for no
	// reason.
	names := make([]string, 0, len(known))
	for name := range known {
		names = append(names, name)
	}
	slices.Sort(names)

	candidates := make([]models.FailoverCandidate, 0, len(known))
	for _, name := range names {
		candidate := known[name]
		for _, saved := range cfgFile.Candidates {
			if saved.InterfaceName == candidate.InterfaceName {
				candidate.Enabled = saved.Enabled
				candidate.Priority = saved.Priority
			}
		}
		if candidate.Priority == 0 {
			candidate.Priority = len(candidates) + 1
			candidate.Enabled = true
		}
		trackerState, ok := trackerStates[candidate.InterfaceName]
		switch {
		case !s.serviceInstalled():
			candidate.TrackingState = models.FailoverTrackingStateNotInstalled
		case !candidate.Available:
			candidate.TrackingState = models.FailoverTrackingStateNotAvailable
		case !candidate.Enabled:
			candidate.TrackingState = models.FailoverTrackingStateDisabled
		case ok:
			candidate.TrackingState = trackerState
		case candidate.IsUp:
			candidate.TrackingState = models.FailoverTrackingStateOnline
		default:
			candidate.TrackingState = models.FailoverTrackingStateOffline
		}
		candidates = append(candidates, candidate)
	}
	slices.SortFunc(candidates, func(a, b models.FailoverCandidate) int {
		if a.Priority != b.Priority {
			return a.Priority - b.Priority
		}
		return strings.Compare(a.InterfaceName, b.InterfaceName)
	})
	return candidates
}

func (s *FailoverService) updateOnlineTimestamps() {
	cfg, err := s.GetConfig()
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, candidate := range cfg.Candidates {
		if candidate.Enabled && candidate.Available && candidate.IsUp && candidate.TrackingState == models.FailoverTrackingStateOnline {
			if _, exists := s.onlineSince[candidate.InterfaceName]; !exists {
				s.onlineSince[candidate.InterfaceName] = time.Now()
			}
		} else {
			delete(s.onlineSince, candidate.InterfaceName)
		}
	}
}

// computeActiveInterface reports which uplink is carrying the connection, or ""
// when none is.
//
// Candidates arrive in priority order, so the first qualifying one wins: a
// candidate online long enough to clear the hold-down, and only as a fallback the
// uplink that is active right now. The order is what makes failback possible: a
// higher-priority link that recovers must win over the sticky active one.
func (s *FailoverService) computeActiveInterface(candidates []models.FailoverCandidate) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sticky := ""
	for _, candidate := range candidates {
		if !candidate.Enabled || !candidate.Available {
			continue
		}
		if !candidate.IsUp || candidate.TrackingState != models.FailoverTrackingStateOnline {
			continue
		}
		onlineTime, online := s.onlineSince[candidate.InterfaceName]
		if online && time.Since(onlineTime) >= failbackHoldDownDuration {
			return candidate.InterfaceName
		}
		if sticky == "" && candidate.InterfaceName == s.lastActive {
			sticky = candidate.InterfaceName
		}
	}
	return sticky
}

func (s *FailoverService) hasWirelessStation() bool {
	sections, err := s.uci.GetSections("wireless")
	if err != nil {
		return false
	}
	for _, opts := range sections {
		if opts["mode"] == "sta" {
			return true
		}
	}
	return false
}

func (s *FailoverService) readTrackerStates() map[string]models.FailoverTrackingState {
	states := map[string]models.FailoverTrackingState{}
	generated := map[string]bool{}
	if !s.serviceInstalled() {
		return states
	}
	out, err := s.cmd.Run("mwan3", "interfaces")
	if err != nil {
		return states
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "interface ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := fields[1]
		// mwan3 reports state per mwan3 INTERFACE SECTION name; candidates are
		// keyed by network interface name (ADR 0005 §1 namespacing). A
		// hand-written section that shares a name with a network interface must
		// not overwrite the generated one's state.
		key := name
		isGenerated := strings.HasPrefix(name, failoverInterfaceSectionPrefix)
		if isGenerated {
			key = strings.TrimPrefix(name, failoverInterfaceSectionPrefix)
		} else if generated[key] {
			continue
		}
		state := models.FailoverTrackingState("")
		switch {
		case strings.Contains(line, "is online"):
			state = models.FailoverTrackingStateOnline
		case strings.Contains(line, "is offline"):
			state = models.FailoverTrackingStateOffline
		case strings.Contains(line, "is disabled"):
			state = models.FailoverTrackingStateDisabled
		}
		if state == "" {
			continue
		}
		if isGenerated {
			generated[key] = true
		}
		states[key] = state
	}
	return states
}

func (s *FailoverService) addDiscoveredUSBNetworkCandidates(known map[string]models.FailoverCandidate) {
	sections, err := s.uci.GetSections("network")
	if err != nil {
		return
	}
	for section, opts := range sections {
		device := opts["device"]
		if section == usbTetherUCIName || isUSBDeviceName(device) {
			if _, exists := known[section]; !exists {
				known[section] = newCandidate(section, "USB tether", models.FailoverCandidateKindUSB, false)
			}
		}
	}
}

func newCandidate(iface, label string, kind models.FailoverCandidateKind, isUp bool) models.FailoverCandidate {
	return models.FailoverCandidate{
		ID:            iface,
		Label:         label,
		InterfaceName: iface,
		Kind:          kind,
		Available:     true,
		Enabled:       true,
		IsUp:          isUp,
	}
}

func markUnavailable(candidate models.FailoverCandidate) models.FailoverCandidate {
	candidate.Available = false
	candidate.IsUp = false
	return candidate
}

// managedSectionNames lists the mwan3 sections the apply is expected to create
// for a candidate list, with the option values that must match what the apply
// wrote. Members are only written for enabled candidates. The values matter: a
// member pointing at the wrong interface section keeps the policy bound to a
// section the service does not manage.
func managedSectionNames(candidates []models.FailoverCandidate) map[string]map[string]string {
	names := map[string]map[string]string{
		failoverPolicySection: nil,
		failoverRuleSection:   nil,
	}
	for _, candidate := range candidates {
		iface := failoverInterfaceSection(candidate.InterfaceName)
		names[iface] = map[string]string{"family": "ipv4"}
		if candidate.Enabled {
			memberName := fmt.Sprintf("travo_%s_p%d",
				failoverSectionName(candidate.InterfaceName), candidate.Priority)
			names[memberName] = map[string]string{"interface": iface}
		}
	}
	return names
}

func isUSBDeviceName(name string) bool {
	return name == "usb0" || name == "usb1" || name == "eth1" || name == "eth2"
}

func interfacePresent(interfaces []models.NetworkInterface, name string) bool {
	for _, iface := range interfaces {
		if iface.Name == name {
			return true
		}
	}
	return false
}
