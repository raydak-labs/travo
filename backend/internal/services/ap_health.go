package services

import (
	"fmt"
	"strings"
)

// DefaultAPSSID is the SSID applied when an AP section has no SSID configured.
const DefaultAPSSID = "OpenWrt-Travel"

// DefaultAPKey is the WPA key applied when an encrypted AP section has no key configured.
const DefaultAPKey = "travelrouter"

// EnsureAPRunning checks all enabled WiFi AP sections for broken configuration
// (missing SSID, missing key on an encrypted AP) and resets them to a working
// default state.
//
// Deliberately does NOT re-enable disabled APs or radios — a disabled state is
// an explicit user choice and touching it causes driver crashes on ath11k/IPQ6018
// when a STA interface is running on the same radio.
//
// This call is blocking: when any fix is applied it invokes "wifi reload",
// which may take several seconds on resource-constrained hardware.
//
// Returns true if any fixes were applied.
func (w *WifiService) EnsureAPRunning() (bool, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return false, fmt.Errorf("reading wireless sections: %w", err)
	}

	fixed := false
	for name, opts := range sections {
		if opts["mode"] != "ap" {
			continue
		}
		// Skip disabled APs — disabled is an explicit user choice.
		if opts["disabled"] == "1" {
			continue
		}
		sectionFixed, err := w.fixAPSection(name, opts)
		if err != nil {
			return false, fmt.Errorf("fixing AP section %q: %w", name, err)
		}
		if sectionFixed {
			fixed = true
		}
	}

	if fixed {
		if err := w.uci.Commit("wireless"); err != nil {
			return false, fmt.Errorf("committing wireless after AP fix: %w", err)
		}
		if err := w.reloader.Reload(); err != nil {
			return false, fmt.Errorf("reloading wireless after AP fix: %w", err)
		}
	}

	return fixed, nil
}

// fixAPSection checks and repairs a single enabled AP section.
// Returns true if any UCI changes were made.
func (w *WifiService) fixAPSection(section string, opts map[string]string) (bool, error) {
	fixed := false

	if strings.TrimSpace(opts["ssid"]) == "" {
		if err := w.uci.Set("wireless", section, "ssid", DefaultAPSSID); err != nil {
			return false, fmt.Errorf("setting default SSID: %w", err)
		}
		fixed = true
	}

	enc := opts["encryption"]
	if enc != "" && enc != "none" && strings.TrimSpace(opts["key"]) == "" {
		if err := w.uci.Set("wireless", section, "key", DefaultAPKey); err != nil {
			return false, fmt.Errorf("setting default key: %w", err)
		}
		fixed = true
	}

	return fixed, nil
}
