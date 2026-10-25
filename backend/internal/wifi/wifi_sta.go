package wifi

import (
	"errors"
	"fmt"
	"sort"
)

// enabledAPSections returns the wifi-iface sections the given wireless config
// says should be running as access points: mode=ap, not disabled, on a radio
// that is not disabled.
//
// An enabled AP section with no device is an ERROR, not an absence: skipping it
// could empty the list and turn "the AP never came up" into "nothing to prove",
// which would cancel the rollback of a broken config.
func enabledAPSections(sections map[string]map[string]string) ([]string, error) {
	var names []string
	for name, opts := range sections {
		if opts["mode"] != "ap" || opts["disabled"] == "1" {
			continue
		}
		device := opts["device"]
		if device == "" {
			return nil, fmt.Errorf("%w: access point section %s has no device, "+
				"so netifd cannot report it", ErrWirelessNotUp, name)
		}
		if radio, ok := sections[device]; ok && radio["disabled"] == "1" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// enabledWwanSTAs returns the wifi-iface sections the given wireless config says
// should be connected as the WiFi uplink: mode=sta, not disabled, bound to
// network=wwan, on a radio that is not disabled.
//
// network=wwan is what makes a STA the uplink: every path in this service that
// creates one binds it there (wifi_connect.go, wifi_scan.go), so this is the
// section whose association carries the router's internet access.
func enabledWwanSTAs(sections map[string]map[string]string) []string {
	var names []string
	for name, opts := range sections {
		if opts["mode"] != "sta" || opts["disabled"] == "1" {
			continue
		}
		if opts["network"] != "wwan" {
			continue
		}
		device := opts["device"]
		if radio, ok := sections[device]; ok && radio["disabled"] == "1" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// findSTADevice discovers the station (client) WiFi interface name by querying network.wireless status.
// It looks for an interface with mode "sta" and returns its ifname (e.g., "phy0-sta0") and section name (e.g., "wifinet2").
func (w *Service) findSTADevice() (ifname string, section string, err error) {
	resp, err := w.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to get wireless status: %w", err)
	}

	for _, radioData := range resp {
		radioMap, ok := radioData.(map[string]any)
		if !ok {
			continue
		}
		ifaces, ok := radioMap["interfaces"].([]any)
		if !ok {
			continue
		}
		for _, iface := range ifaces {
			ifaceMap, ok := iface.(map[string]any)
			if !ok {
				continue
			}
			config, ok := ifaceMap["config"].(map[string]any)
			if !ok {
				continue
			}
			mode, _ := config["mode"].(string)
			if mode == "sta" {
				ifn, _ := ifaceMap["ifname"].(string)
				sec, _ := ifaceMap["section"].(string)
				if ifn != "" {
					return ifn, sec, nil
				}
			}
		}
	}
	return "", "", fmt.Errorf("no STA interface found")
}

// findSTASection discovers the STA section name from UCI config.
// Unlike findSTADevice, this works even when the STA interface is disabled.
func (w *Service) findSTASection() (string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("failed to get wireless sections: %w", err)
	}
	for name, opts := range sections {
		if opts["mode"] == "sta" {
			return name, nil
		}
	}
	return "", fmt.Errorf("no STA section found in UCI config")
}

// ErrNoSTASection reports that no saved STA profile matches the requested SSID.
// It is deliberately distinguishable from a real failure (e.g. an unreadable
// UCI config): treating a read error as "not found" would silently create a
// duplicate profile instead of surfacing the failure.
var ErrNoSTASection = errors.New("no STA section found")

// findSTASectionBySSID returns the UCI section name of a saved STA profile
// matching ssid. It returns an error wrapping ErrNoSTASection when there is no
// match, and a plain error when the wireless config could not be read.
func (w *Service) findSTASectionBySSID(ssid string) (string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("failed to get wireless sections: %w", err)
	}
	for name, opts := range sections {
		if opts["mode"] == "sta" && opts["ssid"] == ssid {
			return name, nil
		}
	}
	return "", fmt.Errorf("%w for SSID %q", ErrNoSTASection, ssid)
}

// nextSTASectionName returns a unique UCI section name for a new STA profile (sta0, sta1, …).
// It returns an error when the section list cannot be read: falling back to a
// fixed name would let the caller `uci set` over an existing section, which
// silently rewrites that section's type and destroys a saved network.
func (w *Service) nextSTASectionName() (string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("failed to get wireless sections: %w", err)
	}
	for i := 0; ; i++ {
		candidate := fmt.Sprintf("sta%d", i)
		if _, exists := sections[candidate]; !exists {
			return candidate, nil
		}
	}
}

// selectActiveSTA picks the single STA wifi-iface that should be active.
// Priority order:
//  1. If exactly one STA section is currently enabled, keep it (preserve user's last Connect).
//  2. Otherwise, rank remaining STA sections by the persisted priority file
//     (lower number = higher priority; 0/unset ranked last).
//  3. Tiebreak by section name (deterministic).
//
// Returns "" if no STA section exists.
func (w *Service) selectActiveSTA() (string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("failed to get wireless sections: %w", err)
	}
	var staNames []string
	var enabledSTAs []string
	for name, opts := range sections {
		if opts["mode"] != "sta" {
			continue
		}
		staNames = append(staNames, name)
		if opts["disabled"] != "1" {
			enabledSTAs = append(enabledSTAs, name)
		}
	}
	if len(staNames) == 0 {
		return "", nil
	}
	if len(enabledSTAs) == 1 {
		return enabledSTAs[0], nil
	}
	priorities := w.loadPriorities()
	sort.Slice(staNames, func(i, j int) bool {
		ssidI := sections[staNames[i]]["ssid"]
		ssidJ := sections[staNames[j]]["ssid"]
		pi, pj := priorities[ssidI], priorities[ssidJ]
		// Unset (0) ranks last.
		if pi == 0 && pj != 0 {
			return false
		}
		if pj == 0 && pi != 0 {
			return true
		}
		if pi != pj {
			return pi < pj
		}
		return staNames[i] < staNames[j]
	})
	return staNames[0], nil
}

// disableOtherSTASections disables every STA wifi-iface except activeSection.
// This ensures only one profile is connected at runtime while others remain saved.
func (w *Service) disableOtherSTASections(activeSection string) error {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return fmt.Errorf("failed to get wireless sections: %w", err)
	}
	for name, opts := range sections {
		if name == activeSection || opts["mode"] != "sta" {
			continue
		}
		if err := w.uci.Set("wireless", name, "disabled", "1"); err != nil {
			return fmt.Errorf("disabling STA section %s: %w", name, err)
		}
	}
	return nil
}

// getWifiRadioNames returns UCI section names of all wifi-device (radio) sections.
func (w *Service) getWifiRadioNames() ([]string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return nil, err
	}
	var names []string
	for name, opts := range sections {
		if opts["type"] != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (w *Service) getWifiSectionsByMode(mode string) ([]string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return nil, err
	}
	var names []string
	for name, opts := range sections {
		if opts["mode"] == mode {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (w *Service) setIfaceDisabled(section string, disabled bool) error {
	value := "0"
	if disabled {
		value = "1"
	}
	return w.uci.Set("wireless", section, "disabled", value)
}

func (w *Service) ensureSectionRadioEnabled(section string) error {
	opts, err := w.uci.GetAll("wireless", section)
	if err != nil {
		return err
	}
	radio := opts["device"]
	if radio == "" {
		return nil
	}
	return w.uci.Set("wireless", radio, "disabled", "0")
}
