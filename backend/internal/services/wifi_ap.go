package services

import (
	"fmt"
	"strconv"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// Radios, AP configuration, and guest WiFi.

func (w *WifiService) preferredGuestRadio() (string, error) {
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return "", err
	}
	if len(radios) == 0 {
		return "", fmt.Errorf("no radio found for guest wifi")
	}
	for _, radio := range radios {
		opts, _ := w.uci.GetAll("wireless", radio)
		if opts["band"] == "2g" {
			return radio, nil
		}
	}
	return radios[0], nil
}

// GetRadioStatus returns whether any WiFi radio is enabled.
func (w *WifiService) GetRadioStatus() (bool, error) {
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return false, err
	}
	for _, radio := range radios {
		opts, _ := w.uci.GetAll("wireless", radio)
		if opts["disabled"] != "1" {
			return true, nil
		}
	}
	return false, nil
}

// SetRadioEnabled enables or disables all WiFi radios.
func (w *WifiService) SetRadioEnabled(enabled bool) (*WirelessApplyResult, error) {
	value := "0"
	if !enabled {
		value = "1"
	}
	radios, err := w.getWifiRadioNames()
	if err != nil {
		return nil, err
	}
	for _, radio := range radios {
		if err := w.uci.Set("wireless", radio, "disabled", value); err != nil {
			return nil, err
		}
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, err
	}
	return w.stageWirelessApply()
}

// GetRadios returns information about all WiFi radio hardware.
func (w *WifiService) GetRadios() ([]models.RadioInfo, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return []models.RadioInfo{}, nil
	}
	// Build role map: for each radio name, detect active AP/STA ifaces.
	type roleFlags struct{ ap, sta bool }
	roles := map[string]roleFlags{}
	for _, opts := range sections {
		if opts["mode"] == "" || opts["type"] != "" {
			continue // skip radio device sections
		}
		device := opts["device"]
		if device == "" || opts["disabled"] == "1" {
			continue
		}
		rf := roles[device]
		switch opts["mode"] {
		case "ap":
			rf.ap = true
		case "sta":
			rf.sta = true
		}
		roles[device] = rf
	}
	var radios []models.RadioInfo
	for name, opts := range sections {
		// wifi-device sections have a "type" option (e.g. "mac80211")
		devType := opts["type"]
		if devType == "" {
			continue
		}
		channel := 0
		if ch, ok := opts["channel"]; ok {
			if v, err := strconv.Atoi(ch); err == nil {
				channel = v
			}
		}
		rf := roles[name]
		role := "none"
		switch {
		case rf.ap && rf.sta:
			role = "both"
		case rf.ap:
			role = "ap"
		case rf.sta:
			role = "sta"
		}
		radios = append(radios, models.RadioInfo{
			Name:     name,
			Band:     opts["band"],
			Channel:  channel,
			HTMode:   opts["htmode"],
			Type:     devType,
			Disabled: opts["disabled"] == "1",
			Role:     role,
		})
	}
	return radios, nil
}

// SetRadioRole assigns a role (ap/sta/both/none) to a specific radio.
// It enables/disables existing iface sections and creates them if needed.
func (w *WifiService) SetRadioRole(radioName, role string) (*WirelessApplyResult, error) {
	switch role {
	case "ap", "sta", "both", "none":
	default:
		return nil, fmt.Errorf("invalid role %q: must be ap, sta, both, or none", role)
	}
	enableAP := role == "ap" || role == "both"
	enableSTA := role == "sta" || role == "both"

	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return nil, err
	}

	// Collect existing AP and STA sections for this radio.
	var apSections, staSections []string
	for name, opts := range sections {
		if opts["device"] != radioName {
			continue
		}
		switch opts["mode"] {
		case "ap":
			apSections = append(apSections, name)
		case "sta":
			staSections = append(staSections, name)
		}
	}

	// Handle AP sections.
	if enableAP && len(apSections) == 0 {
		apName := "ap_" + radioName
		if err := w.uci.AddSection("wireless", apName, "wifi-iface"); err != nil {
			return nil, fmt.Errorf("creating AP section: %w", err)
		}
		_ = w.uci.Set("wireless", apName, "device", radioName)
		_ = w.uci.Set("wireless", apName, "mode", "ap")
		_ = w.uci.Set("wireless", apName, "ssid", "OpenWRT")
		_ = w.uci.Set("wireless", apName, "encryption", "psk2")
		_ = w.uci.Set("wireless", apName, "key", "changeme123")
		_ = w.uci.Set("wireless", apName, "network", "lan")
		apSections = append(apSections, apName)
	}
	for _, section := range apSections {
		if err := w.setIfaceDisabled(section, !enableAP); err != nil {
			return nil, err
		}
		if enableAP {
			if err := w.ensureSectionRadioEnabled(section); err != nil {
				return nil, err
			}
		}
	}

	// Handle STA sections.
	if enableSTA && len(staSections) == 0 {
		if err := w.ensureWwanNetwork(); err != nil {
			return nil, fmt.Errorf("ensuring wwan network: %w", err)
		}
		staName := "sta_" + radioName
		if err := w.uci.AddSection("wireless", staName, "wifi-iface"); err != nil {
			return nil, fmt.Errorf("creating STA section: %w", err)
		}
		_ = w.uci.Set("wireless", staName, "device", radioName)
		_ = w.uci.Set("wireless", staName, "mode", "sta")
		_ = w.uci.Set("wireless", staName, "network", "wwan")
		_ = w.uci.Set("wireless", staName, "ssid", "")
		_ = w.uci.Set("wireless", staName, "disabled", "0")
		staSections = append(staSections, staName)
	}
	for _, section := range staSections {
		if err := w.setIfaceDisabled(section, !enableSTA); err != nil {
			return nil, err
		}
		if enableSTA {
			if err := w.ensureSectionRadioEnabled(section); err != nil {
				return nil, err
			}
		}
	}

	if err := w.uci.Commit("wireless"); err != nil {
		return nil, err
	}
	return w.stageWirelessApply()
}

// GetAPConfigs returns the AP configuration for all radios.
func (w *WifiService) GetAPConfigs() ([]models.APConfig, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return nil, err
	}
	var configs []models.APConfig
	for section, opts := range sections {
		if opts["mode"] != "ap" {
			continue
		}
		radio := opts["device"]
		if radio == "" {
			continue
		}
		radioOpts, _ := w.uci.GetAll("wireless", radio)
		band := radioOpts["band"]
		channel := 0
		if ch, ok := radioOpts["channel"]; ok {
			if v, err := strconv.Atoi(ch); err == nil {
				channel = v
			}
		}
		enabled := opts["disabled"] != "1"
		configs = append(configs, models.APConfig{
			Radio:      radio,
			Band:       band,
			SSID:       opts["ssid"],
			Encryption: opts["encryption"],
			Key:        opts["key"],
			Enabled:    enabled,
			Channel:    channel,
			Section:    section,
		})
	}
	return configs, nil
}

// SetAPConfig updates AP configuration for a specific section.
// When update.Enabled is nil, UCI disabled is not modified (for repeater credential sync).
func (w *WifiService) SetAPConfig(section string, update models.APConfigUpdate) (*WirelessApplyResult, error) {
	opts, err := w.uci.GetAll("wireless", section)
	if err != nil {
		return nil, fmt.Errorf("AP section %s not found", section)
	}
	if opts["mode"] != "ap" {
		return nil, fmt.Errorf("section %s is not an AP interface", section)
	}
	if update.SSID != "" {
		if err := w.uci.Set("wireless", section, "ssid", update.SSID); err != nil {
			return nil, fmt.Errorf("setting SSID: %w", err)
		}
	}
	if update.Encryption != "" {
		if err := w.uci.Set("wireless", section, "encryption", update.Encryption); err != nil {
			return nil, fmt.Errorf("setting encryption: %w", err)
		}
	}
	if update.Encryption != "none" && update.Key != "" {
		if err := w.uci.Set("wireless", section, "key", update.Key); err != nil {
			return nil, fmt.Errorf("setting key: %w", err)
		}
	}
	if update.Enabled != nil {
		disabled := boolToEnabled(!*update.Enabled)
		if err := w.uci.Set("wireless", section, "disabled", disabled); err != nil {
			return nil, fmt.Errorf("setting disabled: %w", err)
		}
		if *update.Enabled {
			if err := w.ensureSectionRadioEnabled(section); err != nil {
				return nil, fmt.Errorf("enabling AP radio: %w", err)
			}
		}
	}
	if err := w.reconcileRepeaterAPRadioLayout(); err != nil {
		return nil, err
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, fmt.Errorf("committing wireless: %w", err)
	}
	return w.stageWirelessApply()
}

// GetGuestWifi returns the guest WiFi configuration.
func (w *WifiService) GetGuestWifi() (*models.GuestWifiConfig, error) {
	opts, err := w.uci.GetAll("wireless", "guest")
	if err != nil {
		return &models.GuestWifiConfig{Enabled: false}, nil
	}
	return &models.GuestWifiConfig{
		Enabled:    opts["disabled"] != "1",
		SSID:       opts["ssid"],
		Encryption: opts["encryption"],
		Key:        opts["key"],
	}, nil
}

// SetGuestWifi creates or updates the guest WiFi network with full isolation.
func (w *WifiService) SetGuestWifi(cfg models.GuestWifiConfig) (*WirelessApplyResult, error) {
	if !cfg.Enabled {
		_, err := w.uci.GetAll("wireless", "guest")
		if err == nil {
			if err := w.uci.Set("wireless", "guest", "disabled", "1"); err != nil {
				return nil, err
			}
			if err := w.uci.Commit("wireless"); err != nil {
				return nil, err
			}
			return w.stageWirelessApply()
		}
		return nil, nil
	}

	// Network interface for guest subnet
	if err := w.ensureNamedSection("network", "guest", "interface"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("network", "guest", "proto", "static"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("network", "guest", "ipaddr", "192.168.2.1"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("network", "guest", "netmask", "255.255.255.0"); err != nil {
		return nil, err
	}
	if err := w.uci.Commit("network"); err != nil {
		return nil, err
	}

	// DHCP for guest network
	if err := w.ensureNamedSection("dhcp", "guest", "dhcp"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("dhcp", "guest", "interface", "guest"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("dhcp", "guest", "start", "100"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("dhcp", "guest", "limit", "50"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("dhcp", "guest", "leasetime", "2h"); err != nil {
		return nil, err
	}
	if err := w.uci.Commit("dhcp"); err != nil {
		return nil, err
	}

	// Wireless interface for guest AP
	guestRadio, err := w.preferredGuestRadio()
	if err != nil {
		return nil, err
	}
	if err := w.ensureNamedSection("wireless", "guest", "wifi-iface"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("wireless", "guest", "device", guestRadio); err != nil {
		return nil, err
	}
	if err := w.uci.Set("wireless", "guest", "mode", "ap"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("wireless", "guest", "network", "guest"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("wireless", "guest", "ssid", cfg.SSID); err != nil {
		return nil, err
	}
	if err := w.uci.Set("wireless", "guest", "encryption", cfg.Encryption); err != nil {
		return nil, err
	}
	if err := w.uci.Set("wireless", "guest", "key", cfg.Key); err != nil {
		return nil, err
	}
	if err := w.uci.Set("wireless", "guest", "isolate", "1"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("wireless", "guest", "disabled", "0"); err != nil {
		return nil, err
	}
	if err := w.ensureSectionRadioEnabled("guest"); err != nil {
		return nil, err
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, err
	}

	// Firewall zone for guest
	if err := w.ensureNamedSection("firewall", "guest_zone", "zone"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_zone", "name", "guest"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_zone", "network", "guest"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_zone", "input", "REJECT"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_zone", "output", "ACCEPT"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_zone", "forward", "REJECT"); err != nil {
		return nil, err
	}

	// Forwarding: guest -> wan
	if err := w.ensureNamedSection("firewall", "guest_fwd", "forwarding"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_fwd", "src", "guest"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_fwd", "dest", "wan"); err != nil {
		return nil, err
	}

	// Allow DNS from guest
	if err := w.ensureNamedSection("firewall", "guest_dns", "rule"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_dns", "name", "Allow-Guest-DNS"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_dns", "src", "guest"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_dns", "dest_port", "53"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_dns", "target", "ACCEPT"); err != nil {
		return nil, err
	}

	// Allow DHCP from guest
	if err := w.ensureNamedSection("firewall", "guest_dhcp", "rule"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_dhcp", "name", "Allow-Guest-DHCP"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_dhcp", "src", "guest"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_dhcp", "dest_port", "67-68"); err != nil {
		return nil, err
	}
	if err := w.uci.Set("firewall", "guest_dhcp", "target", "ACCEPT"); err != nil {
		return nil, err
	}
	if err := w.uci.Commit("firewall"); err != nil {
		return nil, err
	}

	return w.stageWirelessApply()
}
