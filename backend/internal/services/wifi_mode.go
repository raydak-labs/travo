package services

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// persistedWifiMode reads the mode file and answers only the three modes the
// product knows. An unreadable or unrecognised file is not an error: it just
// means the mode has to come from the config.
func (w *WifiService) persistedWifiMode() (string, bool) {
	if w.modeFile == "" {
		return "", false
	}
	data, err := os.ReadFile(w.modeFile)
	if err != nil {
		return "", false
	}
	saved := strings.TrimSpace(string(data))
	switch saved {
	case "client", "ap", "repeater":
		return saved, true
	default:
		return "", false
	}
}

// configMatchesWifiMode reports whether the live config is consistent with the
// persisted mode. "client" requires no enabled access point, "ap" requires no
// enabled STA, and "repeater" requires both — the same shape the UCI detection
// below uses, so the two paths cannot disagree about what a mode looks like.
func (w *WifiService) configMatchesWifiMode(mode string) bool {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		// Cannot prove it matches; let UCI detection answer instead.
		return false
	}
	var hasSTA, hasAP bool
	for _, opts := range sections {
		if opts["disabled"] == "1" {
			continue
		}
		switch opts["mode"] {
		case "sta":
			hasSTA = true
		case "ap":
			hasAP = true
		}
	}
	switch mode {
	case "client":
		return !hasAP
	case "ap":
		return hasAP && !hasSTA
	case "repeater":
		return hasAP && hasSTA
	default:
		return false
	}
}

func (w *WifiService) deriveWifiMode() string {
	// The persisted mode is preferred over UCI detection, but only when the live
	// config is consistent with it. Client mode leaves NO access point enabled
	// and repeater mode leaves one, so the two are distinguishable in UCI after
	// all — which means a persisted value the config disagrees with is stale
	// rather than authoritative.
	//
	// Staleness is reachable: a rollback, a config restore, a hand edit, or a
	// write that did not go through SetMode all leave the file behind. Observed on
	// 192.168.1.1: the file said "client" while an access point was enabled, the
	// UI reported Client as already active, and selecting Client again did nothing
	// at all — the operator could not change mode from the UI, with no error.
	if saved, ok := w.persistedWifiMode(); ok && w.configMatchesWifiMode(saved) {
		return saved
	}
	// Fall back to UCI detection (used before any explicit SetMode() call, and
	// whenever the persisted mode is stale).
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "client"
	}
	hasEnabledSTA := false
	hasEnabledAP := false
	for _, opts := range sections {
		if opts["disabled"] == "1" {
			continue
		}
		switch opts["mode"] {
		case "sta":
			hasEnabledSTA = true
		case "ap":
			hasEnabledAP = true
		}
	}
	switch {
	case hasEnabledSTA && hasEnabledAP:
		return "repeater"
	case hasEnabledAP:
		return "ap"
	default:
		return "client"
	}
}

// ensureNamedSection makes sure config/section exists and carries the expected UCI
// section type. A section that exists with the wrong type would make every later
// Set write meaningless options, so the type is corrected explicitly.
func (w *WifiService) ensureNamedSection(config, section, sectionType string) error {
	// GetSections reports both existence and the section type (".type"), and
	// unlike GetAll it distinguishes "not there" from "could not read" — a read
	// failure must not be turned into a blind section creation.
	sections, err := w.uci.GetSections(config)
	if err != nil {
		return fmt.Errorf("reading %s sections: %w", config, err)
	}
	opts, exists := sections[section]
	if !exists {
		if err := w.uci.AddSection(config, section, sectionType); err != nil {
			return fmt.Errorf("creating %s.%s as %s: %w", config, section, sectionType, err)
		}
		return nil
	}
	// A missing .type means the backend does not report section types
	// (e.g. an in-memory test double seeded with Set); leave it alone.
	current := opts[".type"]
	if current == "" || current == sectionType {
		return nil
	}
	// `uci set <config>.<section>=<type>` rewrites the type in place and keeps
	// the section's options.
	if err := w.uci.AddSection(config, section, sectionType); err == nil {
		return nil
	}
	// Backends that refuse to re-type an existing section: recreate it. The
	// options of a wrong-typed section are meaningless by definition.
	if err := w.uci.DeleteSection(config, section); err != nil {
		return fmt.Errorf("fixing %s.%s section type (want %s, got %s): %w", config, section, sectionType, current, err)
	}
	if err := w.uci.AddSection(config, section, sectionType); err != nil {
		return fmt.Errorf("recreating %s.%s as %s: %w", config, section, sectionType, err)
	}
	return nil
}

// ensureWwanNetwork creates the wwan network interface in UCI if missing (proto=dhcp).
// WiFi client (STA) must use network=wwan so netifd brings it up and runs DHCP; wan is for Ethernet.
// When creating wwan, also adds it to the firewall wan zone so STA gets NAT and internet.
func (w *WifiService) ensureWwanNetwork() error {
	if _, err := w.uci.GetAll("network", "wwan"); err == nil {
		return w.ensureWwanFirewall()
	}
	if err := w.uci.AddSection("network", "wwan", "interface"); err != nil {
		return fmt.Errorf("adding network wwan: %w", err)
	}
	if err := w.uci.Set("network", "wwan", "proto", "dhcp"); err != nil {
		return fmt.Errorf("setting network wwan proto: %w", err)
	}
	if err := w.uci.Commit("network"); err != nil {
		return fmt.Errorf("committing network wwan: %w", err)
	}
	if err := w.ensureWwanFirewall(); err != nil {
		return fmt.Errorf("ensuring wwan in firewall: %w", err)
	}
	return nil
}

// ensureWwanFirewall adds wwan to the firewall wan zone's network list so STA gets internet.
func (w *WifiService) ensureWwanFirewall() error {
	sections, err := w.uci.GetSections("firewall")
	if err != nil {
		return err
	}
	var wanZone string
	for name, opts := range sections {
		// Real UCI: anonymous zones have .type="zone"; mock UCI may not have .type.
		isZone := opts[".type"] == "zone" || opts["input"] != ""
		if isZone && opts["name"] == "wan" {
			wanZone = name
			break
		}
	}
	if wanZone == "" {
		return fmt.Errorf("wan firewall zone not found")
	}
	// Check if wwan is already in the network list to avoid duplicates.
	if net := sections[wanZone]["network"]; net != "" {
		if slices.Contains(strings.Fields(net), "wwan") {
			return nil
		}
	}
	if err := w.uci.AddList("firewall", wanZone, "network", "wwan"); err != nil {
		return err
	}
	return w.uci.Commit("firewall")
}
