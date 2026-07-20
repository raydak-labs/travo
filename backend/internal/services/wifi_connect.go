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
func (w *WifiService) Connect(config models.WifiConfig) (*WirelessApplyResult, error) {
	// WiFi client must use wwan (not wan) so netifd runs DHCP and routing uses it as WAN
	if err := w.ensureWwanNetwork(); err != nil {
		return nil, err
	}

	// Find or create a dedicated UCI section for this SSID.
	section, err := w.findSTASectionBySSID(config.SSID)
	isNewSection := err != nil
	if isNewSection {
		enc := strings.TrimSpace(config.Encryption)
		if enc == "" {
			return nil, ErrEncryptionRequiredForNewSTA
		}
		if enc != "none" && strings.TrimSpace(config.Password) == "" {
			return nil, ErrPasswordRequiredForNewSTA
		}
		// No saved profile for this SSID yet — allocate a new section.
		section = w.nextSTASectionName()
		sections, _ := w.uci.GetSections("wireless")
		var firstRadio string
		for name, opts := range sections {
			if opts["type"] != "" {
				firstRadio = name
				break
			}
		}
		if firstRadio == "" {
			return nil, fmt.Errorf("no radio found in wireless config")
		}
		if err := w.uci.AddSection("wireless", section, "wifi-iface"); err != nil {
			return nil, fmt.Errorf("creating STA section %s: %w", section, err)
		}
		_ = w.uci.Set("wireless", section, "device", firstRadio)
		_ = w.uci.Set("wireless", section, "mode", "sta")
		_ = w.uci.Set("wireless", section, "network", "wwan")
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
}

// Disconnect disconnects from the current WiFi network.
func (w *WifiService) Disconnect() (*WirelessApplyResult, error) {
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
}

// GetConnection returns the current WiFi connection info.
func (w *WifiService) GetConnection() (models.WifiConnection, error) {
	ifname, _, err := w.findSTADevice()
	if err != nil {
		return models.WifiConnection{Mode: w.deriveWifiMode()}, nil
	}

	resp, err := w.ubus.Call("iwinfo", "info", map[string]interface{}{"device": ifname})
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
			if addrs, ok := wwanData["ipv4-address"].([]interface{}); ok && len(addrs) > 0 {
				if a, ok := addrs[0].(map[string]interface{}); ok {
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
		return []models.SavedNetwork{}, nil
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
}
