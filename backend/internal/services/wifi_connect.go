package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// STA connect/disconnect, saved networks, and priorities.

// ErrPasswordRequiredForNewSTA is returned when Connect would create a new secured STA profile without a password.
var ErrPasswordRequiredForNewSTA = errors.New("password is required when adding a new secured network")

// ErrEncryptionRequiredForNewSTA is returned when Connect creates a new STA profile without an encryption mode.
var ErrEncryptionRequiredForNewSTA = errors.New("encryption is required when adding a new wireless client profile")

// Connect connects to a WiFi network.
// Each distinct SSID gets its own UCI section so saved profiles persist across connections.
// All other STA sections are disabled (not deleted) when connecting to a new network.
//
// For an existing saved profile, an empty Password leaves the stored UCI key unchanged
// (one-tap reconnect from the saved list).
// radioForNewSTA picks which radio a newly created uplink STA should use.
//
// Two properties matter, and the second one used to be missing entirely.
//
// It must be DETERMINISTIC. The previous implementation took the first radio
// found by ranging over a map, so the same request landed on radio0 or radio1
// depending on Go's randomised iteration order. Downstream that is not cosmetic:
// the chosen radio decides whether the uplink ends up sharing a PHY with an access
// point.
//
// So it must also PREFER a radio that is not currently running an enabled access
// point, the same way preferredGuestRadio does for a guest AP. A stock config has
// an access point on every radio, so whichever one is chosen,
// splitAPOffUplinkRadio disables that radio's access point to keep the uplink
// alone. On the NEXT connect, preferring the now-bare radio means the previously
// disabled access point is simply left off and no second one has to be torn down.
// Without the preference, a second connect could pick the radio that now holds
// the only remaining access point and be refused for it, leaving the operator
// unable to move their uplink without first re-enabling an access point by hand.
func (w *WifiService) radioForNewSTA(sections map[string]map[string]string) (string, error) {
	radios := make([]string, 0, len(sections))
	for name, opts := range sections {
		if opts["type"] != "" {
			radios = append(radios, name)
		}
	}
	if len(radios) == 0 {
		return "", fmt.Errorf("no radio found in wireless config")
	}
	sort.Strings(radios) // stable choice: Go map order must not decide this

	hasEnabledAP := func(radio string) bool {
		for _, opts := range sections {
			if opts["mode"] == "ap" && opts["disabled"] != "1" && opts["device"] == radio {
				return true
			}
		}
		return false
	}
	for _, radio := range radios {
		if !hasEnabledAP(radio) {
			return radio, nil
		}
	}
	return radios[0], nil
}

func (w *WifiService) Connect(config models.WifiConfig) (*WirelessApplyResult, error) {
	return w.mutateWireless([]string{"wireless", "network", "firewall"}, func() (*WirelessApplyResult, error) {
		// WiFi client must use wwan (not wan) so netifd runs DHCP and routing uses it as WAN
		if err := w.ensureWwanNetwork(); err != nil {
			return nil, err
		}

		// Find or create a dedicated UCI section for this SSID.
		section, err := w.findSTASectionBySSID(config.SSID)
		if err != nil && !errors.Is(err, ErrNoSTASection) {
			// A real failure (e.g. the wireless config could not be read) must not
			// be treated as "not found": doing so creates a duplicate profile and
			// hides the real error behind a confusing UCI conflict.
			return nil, err
		}
		isNewSection := errors.Is(err, ErrNoSTASection)
		if isNewSection {
			enc := strings.TrimSpace(config.Encryption)
			if enc == "" {
				return nil, ErrEncryptionRequiredForNewSTA
			}
			if enc != "none" && strings.TrimSpace(config.Password) == "" {
				return nil, ErrPasswordRequiredForNewSTA
			}
			// No saved profile for this SSID yet — allocate a new section.
			section, err = w.nextSTASectionName()
			if err != nil {
				return nil, err
			}
			sections, err := w.uci.GetSections("wireless")
			if err != nil {
				return nil, fmt.Errorf("failed to get wireless sections: %w", err)
			}
			firstRadio, err := w.radioForNewSTA(sections)
			if err != nil {
				return nil, err
			}
			if err := w.uci.AddSection("wireless", section, "wifi-iface"); err != nil {
				return nil, fmt.Errorf("creating STA section %s: %w", section, err)
			}
			// Every write is checked: a silently ignored `uci set` (read-only
			// overlay, ENOSPC on the delta) left an empty wifi-iface that was then
			// committed and applied, and the API reported a successful connection to
			// an SSID that was never actually configured.
			if err := w.uci.Set("wireless", section, "device", firstRadio); err != nil {
				return nil, fmt.Errorf("setting STA radio: %w", err)
			}
			if err := w.uci.Set("wireless", section, "mode", "sta"); err != nil {
				return nil, fmt.Errorf("setting STA mode: %w", err)
			}
			if err := w.uci.Set("wireless", section, "network", "wwan"); err != nil {
				return nil, fmt.Errorf("setting STA network: %w", err)
			}
		}

		// Ensure wwan binding is correct.
		if net, err := w.uci.Get("wireless", section, "network"); err != nil || net != "wwan" {
			if err := w.uci.Set("wireless", section, "network", "wwan"); err != nil {
				return nil, fmt.Errorf("setting STA network: %w", err)
			}
		}
		// When band is specified (dual-band connect), attach STA to the radio that has that band
		if config.Band != "" {
			radio, err := w.getRadioForBand(config.Band)
			if err != nil {
				return nil, err
			}
			if err := w.uci.Set("wireless", section, "device", radio); err != nil {
				return nil, fmt.Errorf("setting STA radio: %w", err)
			}
		}
		// The radio the uplink STA will land on, resolved now: it is either the
		// band-matched radio above or whatever the existing/new section already
		// names. splitAPOffUplinkRadio keeps that radio from also running an
		// access point (see its comment for why this cannot be left to
		// reconcileRepeaterAPRadioLayout).
		staOpts, err := w.uci.GetAll("wireless", section)
		if err != nil {
			return nil, fmt.Errorf("reading STA section %s: %w", section, err)
		}
		staRadio := staOpts["device"]
		if err := w.uci.Set("wireless", section, "ssid", config.SSID); err != nil {
			return nil, fmt.Errorf("setting STA ssid: %w", err)
		}
		reuseCredentials := !isNewSection && strings.TrimSpace(config.Password) == ""
		if strings.TrimSpace(config.Password) != "" {
			if err := w.uci.Set("wireless", section, "key", config.Password); err != nil {
				return nil, fmt.Errorf("setting STA key: %w", err)
			}
		}
		if config.Encryption != "" && !reuseCredentials {
			if err := w.uci.Set("wireless", section, "encryption", config.Encryption); err != nil {
				return nil, fmt.Errorf("setting STA encryption: %w", err)
			}
		}
		if !reuseCredentials {
			if config.Hidden {
				if err := w.uci.Set("wireless", section, "hidden", "1"); err != nil {
					return nil, fmt.Errorf("setting STA hidden flag: %w", err)
				}
			} else {
				if err := w.uci.Set("wireless", section, "hidden", "0"); err != nil {
					return nil, fmt.Errorf("setting STA hidden flag: %w", err)
				}
			}
		}
		if err := w.uci.Set("wireless", section, "disabled", "0"); err != nil {
			return nil, fmt.Errorf("enabling STA section: %w", err)
		}
		if err := w.ensureSectionRadioEnabled(section); err != nil {
			return nil, fmt.Errorf("enabling STA radio: %w", err)
		}
		// Disable all other saved STA profiles so only this one connects at runtime.
		if err := w.disableOtherSTASections(section); err != nil {
			return nil, err
		}
		// Keep the radio the uplink is about to take free of access points before
		// the STA is enabled. Outside repeater mode nothing else did this: every
		// stock config has an access point on each radio, so Connect could commit
		// AP+STA on one PHY — the state ADR 0002 §2 says is enough to crash
		// ath11k/IPQ6018 — with no check at all. Runs after
		// disableOtherSTASections so the only enabled uplink is the one being
		// connected, and before Commit so a refusal reaches no running config.
		if err := w.splitAPOffUplinkRadio(staRadio); err != nil {
			return nil, err
		}
		// Reconcile AP radio layout atomically with the STA activation: in repeater mode
		// with ≥2 radios, disable any AP that shares the STA's radio before applying.
		// Skipping this step would commit AP+STA on the same PHY, which crashes the
		// ath11k/IPQ6018 driver and requires a second user-triggered "Fix" apply to recover.
		if err := w.reconcileRepeaterAPRadioLayout(); err != nil {
			return nil, fmt.Errorf("reconciling AP radio layout: %w", err)
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, err
		}
		return w.stageWirelessApply()
	})
}

// Disconnect disconnects from the current WiFi network.
func (w *WifiService) Disconnect() (*WirelessApplyResult, error) {
	return w.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		_, section, err := w.findSTADevice()
		if err != nil {
			// STA interface may already be disabled; fall back to UCI-based lookup
			section, err = w.findSTASection()
			if err != nil {
				return nil, fmt.Errorf("no STA interface found: %w", err)
			}
		}
		if err := w.uci.Set("wireless", section, "disabled", "1"); err != nil {
			return nil, fmt.Errorf("disabling STA section: %w", err)
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, err
		}
		return w.stageWirelessApply()
	})
}

// GetConnection returns the current WiFi connection info.
func (w *WifiService) GetConnection() (models.WifiConnection, error) {
	ifname, _, err := w.findSTADevice()
	if err != nil {
		return models.WifiConnection{Mode: w.deriveWifiMode()}, nil
	}

	resp, err := w.ubus.Call("iwinfo", "info", map[string]any{"device": ifname})
	if err != nil {
		return models.WifiConnection{}, err
	}

	ssid, _ := resp["ssid"].(string)
	bssid, _ := resp["bssid"].(string)
	ch, _ := resp["channel"].(float64)
	sig, _ := resp["signal"].(float64)
	qual, _ := resp["quality"].(float64)
	enc := parseIwinfoEncryption(resp["encryption"])
	band, _ := resp["band"].(string)

	conn := models.WifiConnection{
		SSID: ssid, BSSID: bssid,
		Mode: w.deriveWifiMode(), Channel: int(ch),
		SignalDBM: int(sig), SignalPercent: int(qual),
		Encryption: enc, Band: band,
		Connected: ssid != "",
	}

	// Get IP from wwan interface
	if conn.Connected {
		if wwanData, err := w.ubus.Call("network.interface.wwan", "status", nil); err == nil {
			if addrs, ok := wwanData["ipv4-address"].([]any); ok && len(addrs) > 0 {
				if a, ok := addrs[0].(map[string]any); ok {
					conn.IPAddress, _ = a["address"].(string)
				}
			}
		}
	}

	return conn, nil
}

// loadPriorities reads the priority file and returns an ssid->priority map.
func (w *WifiService) loadPriorities() map[string]int {
	data, err := os.ReadFile(w.priorityFile)
	if err != nil {
		return map[string]int{}
	}
	priorities := map[string]int{}
	if err := json.Unmarshal(data, &priorities); err != nil {
		return map[string]int{}
	}
	return priorities
}

// savePriorities writes the priority map to the priority file.
func (w *WifiService) savePriorities(priorities map[string]int) error {
	dir := filepath.Dir(w.priorityFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating priority directory: %w", err)
	}
	data, err := json.Marshal(priorities)
	if err != nil {
		return fmt.Errorf("marshaling priorities: %w", err)
	}
	return os.WriteFile(w.priorityFile, data, 0600)
}

// ReorderNetworks sets priority order for saved networks based on an ordered list of SSIDs.
// The first SSID in the list gets priority 1 (highest), second gets 2, etc.
func (w *WifiService) ReorderNetworks(ssids []string) error {
	priorities := make(map[string]int, len(ssids))
	for i, ssid := range ssids {
		priorities[ssid] = i + 1
	}
	return w.savePriorities(priorities)
}

// GetSavedNetworks returns saved WiFi networks.
func (w *WifiService) GetSavedNetworks() ([]models.SavedNetwork, error) {
	priorities := w.loadPriorities()

	var networks []models.SavedNetwork
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		// An empty saved-networks list on a failed read would let the UI offer a
		// "forget network" flow for profiles it cannot see, and hides the
		// failure that actually needs attention.
		return nil, fmt.Errorf("reading wireless sections: %w", err)
	}
	for section, opts := range sections {
		if opts["mode"] != "sta" {
			continue
		}
		ssid := strings.TrimSpace(opts["ssid"])
		if ssid == "" {
			continue
		}
		disabled := strings.TrimSpace(opts["disabled"]) == "1"

		priority := 0
		if p, ok := priorities[ssid]; ok {
			priority = p
		}

		networks = append(networks, models.SavedNetwork{
			SSID:        ssid,
			Section:     section,
			Encryption:  opts["encryption"],
			Mode:        "sta",
			AutoConnect: !disabled,
			Priority:    priority,
		})
	}

	// Sort by priority (lower number = higher priority), 0 means unset (goes last)
	sort.Slice(networks, func(i, j int) bool {
		pi, pj := networks[i].Priority, networks[j].Priority
		if pi == 0 && pj == 0 {
			return networks[i].SSID < networks[j].SSID
		}
		if pi == 0 {
			return false
		}
		if pj == 0 {
			return true
		}
		return pi < pj
	})

	return networks, nil
}

// DeleteNetwork removes a saved WiFi network by its UCI section name.
func (w *WifiService) DeleteNetwork(section string) (*WirelessApplyResult, error) {
	return w.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		if section == "" {
			return nil, fmt.Errorf("section name is required")
		}
		if err := w.uci.DeleteSection("wireless", section); err != nil {
			return nil, fmt.Errorf("failed to delete network: %w", err)
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, err
		}
		return w.stageWirelessApply()
	})
}
