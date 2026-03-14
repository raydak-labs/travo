package services

import (
	"fmt"
	"strings"
)

// Default AP and radio values applied when UCI has missing or invalid entries.
const (
	DefaultAPSSID   = "OpenWrt-Travel"    // 2.4 GHz default AP SSID
	DefaultAPSSID5G = "OpenWrt-Travel-5G" // 5 GHz default AP SSID (separate so clients can choose band)
	DefaultAPKey    = "travelrouter"      // default WPA key for AP when encryption is set but key missing
	DefaultCountry  = "US"                // default regulatory country for wifi-device
	DefaultChannel  = "auto"              // default channel (auto selection)
)

// EnsureAPRunning checks wireless config and applies safe defaults:
//   - wifi-device (radio): set country to DefaultCountry and channel to DefaultChannel when missing.
//   - AP sections: fix missing SSID (band-specific), encryption, and key; re-enable disabled APs
//     unless the same radio has an active STA (ath11k crash avoidance).
//
// UCI changes are committed when fixes are applied; no wifi command is run by this function.
//
// Returns: fixed (any UCI changes committed), needWifiUp (at least one AP was re-enabled).
func (w *WifiService) EnsureAPRunning() (fixed bool, needWifiUp bool, err error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return false, false, fmt.Errorf("reading wireless sections: %w", err)
	}

	fixed = false
	reEnabled := false

	// Fix wifi-device (radio) sections: country and channel defaults.
	for name, opts := range sections {
		if opts["type"] == "" {
			continue
		}
		// This is a wifi-device section (has type, e.g. mac80211).
		radioFixed, fixErr := w.fixRadioSection(name, opts)
		if fixErr != nil {
			return false, false, fmt.Errorf("fixing radio %q: %w", name, fixErr)
		}
		if radioFixed {
			fixed = true
		}
	}

	// Fix AP (wifi-iface mode=ap) sections.
	for name, opts := range sections {
		if opts["mode"] != "ap" {
			continue
		}
		sectionFixed, sectionReEnabled, fixErr := w.fixAPSection(name, opts, sections)
		if fixErr != nil {
			return false, false, fmt.Errorf("fixing AP section %q: %w", name, fixErr)
		}
		if sectionFixed {
			fixed = true
		}
		if sectionReEnabled {
			reEnabled = true
			// Update in-memory snapshot so the "enable radios" loop below sees the change.
			sections[name]["disabled"] = "0"
		}
	}

	// Enable radios that have at least one enabled AP, so WiFi is actually visible.
	// (UCI can have AP iface disabled=0 but radio disabled=1, which shows no networks.)
	for name, opts := range sections {
		if opts["type"] == "" || opts["disabled"] != "1" {
			continue
		}
		radio := name
		if !radioHasEnabledAP(sections, radio) {
			continue
		}
		if setErr := w.uci.Set("wireless", radio, "disabled", "0"); setErr != nil {
			return false, false, fmt.Errorf("enabling radio %q: %w", radio, setErr)
		}
		fixed = true
		reEnabled = true
	}

	if fixed {
		if err := w.uci.Commit("wireless"); err != nil {
			return false, false, fmt.Errorf("committing wireless after AP fix: %w", err)
		}
	}

	return fixed, reEnabled, nil
}

// fixRadioSection applies defaults to a wifi-device: country=US, channel=auto when missing.
func (w *WifiService) fixRadioSection(section string, opts map[string]string) (bool, error) {
	fixed := false
	if opts["country"] == "" {
		if setErr := w.uci.Set("wireless", section, "country", DefaultCountry); setErr != nil {
			return false, fmt.Errorf("setting country: %w", setErr)
		}
		fixed = true
	}
	ch := strings.TrimSpace(opts["channel"])
	if ch == "" || ch == "0" {
		if setErr := w.uci.Set("wireless", section, "channel", DefaultChannel); setErr != nil {
			return false, fmt.Errorf("setting channel: %w", setErr)
		}
		fixed = true
	}
	return fixed, nil
}

// radioHasSTA reports whether any non-disabled STA interface is configured on
// the given radio. An active STA on the same radio as an AP would cause an
// ath11k/IPQ6018 kernel crash if both are enabled and wifi is reloaded.
func radioHasSTA(sections map[string]map[string]string, radio string) bool {
	for _, opts := range sections {
		if opts["device"] == radio && opts["mode"] == "sta" && opts["disabled"] != "1" {
			return true
		}
	}
	return false
}

// radioHasEnabledAP reports whether the given radio has at least one enabled AP iface.
func radioHasEnabledAP(sections map[string]map[string]string, radio string) bool {
	for _, opts := range sections {
		if opts["device"] == radio && opts["mode"] == "ap" && opts["disabled"] != "1" {
			return true
		}
	}
	return false
}

// fixAPSection checks and repairs a single AP section.
// Uses band-specific default SSID (2.4 vs 5G). Ensures encryption and key are set (psk2 + travelrouter).
// Returns: fixed (any UCI changes made), reEnabled (AP was changed from disabled to enabled).
func (w *WifiService) fixAPSection(section string, opts map[string]string, sections map[string]map[string]string) (fixed bool, reEnabled bool, err error) {
	if opts["disabled"] == "1" {
		// Do not re-enable if the same radio has an active STA — that combination
		// crashes the ath11k driver on IPQ6018 hardware.
		if radioHasSTA(sections, opts["device"]) {
			return false, false, nil
		}
		if setErr := w.uci.Set("wireless", section, "disabled", "0"); setErr != nil {
			return false, false, fmt.Errorf("re-enabling AP section: %w", setErr)
		}
		fixed = true
		reEnabled = true
		// Fall through to also fix SSID/key on the now-enabled AP.
	}

	// Default SSID: band-specific so 2.4 and 5G can be distinguished.
	if strings.TrimSpace(opts["ssid"]) == "" {
		ssid := defaultSSIDForBand(sections, opts["device"])
		if setErr := w.uci.Set("wireless", section, "ssid", ssid); setErr != nil {
			return false, false, fmt.Errorf("setting default SSID: %w", setErr)
		}
		fixed = true
	}

	enc := strings.TrimSpace(opts["encryption"])
	key := strings.TrimSpace(opts["key"])
	// When encryption is set (not open AP), ensure a key is set. Leave open APs (enc empty/none) unchanged.
	if enc != "" && enc != "none" && key == "" {
		if setErr := w.uci.Set("wireless", section, "key", DefaultAPKey); setErr != nil {
			return false, false, fmt.Errorf("setting default key: %w", setErr)
		}
		fixed = true
	}

	return fixed, reEnabled, nil
}

// defaultSSIDForBand returns the default AP SSID for the given radio (2.4 vs 5G).
func defaultSSIDForBand(sections map[string]map[string]string, radio string) string {
	if radio == "" {
		return DefaultAPSSID
	}
	r, ok := sections[radio]
	if !ok {
		return DefaultAPSSID
	}
	if strings.ToLower(r["band"]) == "5g" {
		return DefaultAPSSID5G
	}
	return DefaultAPSSID
}
