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
		// Reporting "no radios" on a failed read would tell the operator the
		// hardware vanished instead of surfacing the read failure.
		return nil, fmt.Errorf("reading wireless sections: %w", err)
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
	defer w.lockUCIWrite()()
	// abort drops the staged wireless delta before surfacing a failure: the uci
	// CLI delta is process-global, so an abandoned write would be committed by a
	// later, unrelated `uci commit wireless`.
	abort := func(err error) (*WirelessApplyResult, error) {
		revertUCIConfig(w.uci, "wireless", "network")
		return nil, err
	}
	switch role {
	case "ap", "sta", "both", "none":
	default:
		return nil, fmt.Errorf("invalid role %q: must be ap, sta, both, or none", role)
	}
	enableAP := role == "ap" || role == "both"
	enableSTA := role == "sta" || role == "both"
	// Set when this call had to invent a WPA key for a default AP.
	var generatedKey string

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

	// A radio that is about to gain an STA section must not leave another saved
	// STA enabled: two active wifi-ifaces bound to network=wwan is the
	// inconsistent config that rpcd rollback cannot repair.
	newSTASection := ""
	if enableSTA && len(staSections) == 0 {
		newSTASection = "sta_" + radioName
	}
	// Only when a new STA section is created: this radio gains a second active
	// wwan client otherwise. Other STA profiles are disabled, not deleted, so
	// saved networks survive.
	if newSTASection != "" {
		if err := w.disableOtherSTASections(newSTASection); err != nil {
			return abort(err)
		}
	}

	// Handle AP sections.
	if enableAP && len(apSections) == 0 {
		apName := "ap_" + radioName
		if err := w.uci.AddSection("wireless", apName, "wifi-iface"); err != nil {
			return abort(fmt.Errorf("creating AP section: %w", err))
		}
		// A shipped default passphrase is a public credential: every unit
		// running this firmware would expose the same open-ish network. Generate
		// a random WPA2 key and report it back so the operator can use it.
		key, err := generateRandomWPAKey()
		if err != nil {
			return abort(fmt.Errorf("generating default AP key: %w", err))
		}
		generatedKey = key
		if err := w.uci.Set("wireless", apName, "device", radioName); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", apName, "mode", "ap"); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", apName, "ssid", "OpenWRT"); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", apName, "encryption", "psk2"); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", apName, "key", key); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", apName, "network", "lan"); err != nil {
			return abort(err)
		}
		apSections = append(apSections, apName)
	}
	for _, section := range apSections {
		if err := w.setIfaceDisabled(section, !enableAP); err != nil {
			return abort(err)
		}
		if enableAP {
			if err := w.ensureSectionRadioEnabled(section); err != nil {
				return abort(err)
			}
		}
	}

	// Handle STA sections.
	if enableSTA && len(staSections) == 0 {
		if err := w.ensureWwanNetwork(); err != nil {
			return abort(fmt.Errorf("ensuring wwan network: %w", err))
		}
		staName := newSTASection
		if err := w.uci.AddSection("wireless", staName, "wifi-iface"); err != nil {
			return abort(fmt.Errorf("creating STA section: %w", err))
		}
		if err := w.uci.Set("wireless", staName, "device", radioName); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", staName, "mode", "sta"); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", staName, "network", "wwan"); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", staName, "ssid", ""); err != nil {
			return abort(err)
		}
		if err := w.uci.Set("wireless", staName, "disabled", "0"); err != nil {
			return abort(err)
		}
		staSections = append(staSections, staName)
	}
	for _, section := range staSections {
		if err := w.setIfaceDisabled(section, !enableSTA); err != nil {
			return abort(err)
		}
		if enableSTA {
			if err := w.ensureSectionRadioEnabled(section); err != nil {
				return abort(err)
			}
		}
	}

	// Validate BEFORE Commit: stageWirelessApply would otherwise run the same
	// check after the bad config is already committed, and the rpcd rollback it
	// triggers would restore the very same broken config.
	if err := w.validateWirelessConsistency(); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return abort(err)
	}
	apply, err := w.stageWirelessApply()
	if err != nil {
		return nil, err
	}
	if apply != nil {
		apply.GeneratedKey = generatedKey
	}
	return apply, nil
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
	defer w.lockUCIWrite()()
	// abort drops every staged delta touched so far: the uci CLI delta is
	// process-global, so an abandoned write would be committed later by an
	// unrelated `uci commit <config>`.
	abort := func(err error) (*WirelessApplyResult, error) {
		revertUCIConfig(w.uci, "wireless", "network", "dhcp", "firewall")
		return nil, err
	}
	if !cfg.Enabled {
		return w.teardownGuestWifi()
	}

	// Network interface for guest subnet
	if err := w.ensureNamedSection("network", "guest", "interface"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("network", "guest", "proto", "static"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("network", "guest", "ipaddr", "192.168.2.1"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("network", "guest", "netmask", "255.255.255.0"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("network"); err != nil {
		return abort(err)
	}

	// DHCP for guest network
	if err := w.ensureNamedSection("dhcp", "guest", "dhcp"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("dhcp", "guest", "interface", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("dhcp", "guest", "start", "100"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("dhcp", "guest", "limit", "50"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("dhcp", "guest", "leasetime", "2h"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("dhcp"); err != nil {
		return abort(err)
	}

	// Wireless interface for guest AP
	guestRadio, err := w.preferredGuestRadio()
	if err != nil {
		return abort(err)
	}
	if err := w.ensureNamedSection("wireless", "guest", "wifi-iface"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("wireless", "guest", "device", guestRadio); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("wireless", "guest", "mode", "ap"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("wireless", "guest", "network", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("wireless", "guest", "ssid", cfg.SSID); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("wireless", "guest", "encryption", cfg.Encryption); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("wireless", "guest", "key", cfg.Key); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("wireless", "guest", "isolate", "1"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("wireless", "guest", "disabled", "0"); err != nil {
		return abort(err)
	}
	if err := w.ensureSectionRadioEnabled("guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return abort(err)
	}

	// Firewall zone for guest
	if err := w.ensureNamedSection("firewall", "guest_zone", "zone"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_zone", "name", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_zone", "network", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_zone", "input", "REJECT"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_zone", "output", "ACCEPT"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_zone", "forward", "REJECT"); err != nil {
		return abort(err)
	}

	// Forwarding: guest -> wan
	if err := w.ensureNamedSection("firewall", "guest_fwd", "forwarding"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_fwd", "src", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_fwd", "dest", "wan"); err != nil {
		return abort(err)
	}

	// Allow DNS from guest
	if err := w.ensureNamedSection("firewall", "guest_dns", "rule"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_dns", "name", "Allow-Guest-DNS"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_dns", "src", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_dns", "dest_port", "53"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_dns", "target", "ACCEPT"); err != nil {
		return abort(err)
	}

	// Allow DHCP from guest
	if err := w.ensureNamedSection("firewall", "guest_dhcp", "rule"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_dhcp", "name", "Allow-Guest-DHCP"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_dhcp", "src", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_dhcp", "dest_port", "67-68"); err != nil {
		return abort(err)
	}
	if err := w.uci.Set("firewall", "guest_dhcp", "target", "ACCEPT"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("firewall"); err != nil {
		return abort(err)
	}

	return w.stageWirelessApply()
}

// teardownGuestWifi disables the guest AP and removes the guest subnet it
// owned. Setting wireless.guest.disabled=1 alone leaves network.guest,
// dhcp.guest and the four firewall.guest_* sections in place, so the
// 192.168.2.0/24 interface, its DHCP scope and the guest zone stay up and
// continue to route and answer on a network the operator switched off.
func (w *WifiService) teardownGuestWifi() (*WirelessApplyResult, error) {
	abort := func(err error) (*WirelessApplyResult, error) {
		revertUCIConfig(w.uci, "wireless", "network", "dhcp", "firewall")
		return nil, err
	}
	if _, err := w.uci.GetAll("wireless", "guest"); err != nil {
		// Guest WiFi was never configured: nothing to tear down.
		return nil, nil
	}

	// 1. Take the AP down first so no client is left on a subnet that is about
	//    to disappear.
	if err := w.uci.Set("wireless", "guest", "disabled", "1"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return abort(err)
	}

	// 2. Remove the DHCP scope before the interface it is bound to.
	if err := w.deleteSectionIfPresent("dhcp", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("dhcp"); err != nil {
		return abort(err)
	}

	// 3. Remove the guest network interface (192.168.2.0/24).
	if err := w.deleteSectionIfPresent("network", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("network"); err != nil {
		return abort(err)
	}

	// 4. Remove the guest firewall zone and its rules/forwarding.
	for _, section := range []string{"guest_dhcp", "guest_dns", "guest_fwd", "guest_zone"} {
		if err := w.deleteSectionIfPresent("firewall", section); err != nil {
			return abort(err)
		}
	}
	if err := w.uci.Commit("firewall"); err != nil {
		return abort(err)
	}

	return w.stageWirelessApply()
}

// deleteSectionIfPresent removes config/section when it exists. A section that
// was never created is not an error: teardown must be idempotent.
func (w *WifiService) deleteSectionIfPresent(config, section string) error {
	if _, err := w.uci.GetAll(config, section); err != nil {
		return nil
	}
	if err := w.uci.DeleteSection(config, section); err != nil {
		return fmt.Errorf("removing %s.%s: %w", config, section, err)
	}
	return nil
}
