package services

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// WifiReloader applies wireless configuration changes (e.g. "wifi up").
type WifiReloader interface {
	Reload() error
}

// ShellWifiReloader runs "wifi up" via exec to apply UCI wireless changes without
// a full teardown. "wifi reload" tears everything down first and can trigger
// ath11k/IPQ6018 driver crashes; "wifi up" is the recommended OpenWRT approach.
type ShellWifiReloader struct{}

// Reload executes "wifi up" to apply UCI wireless changes.
func (r *ShellWifiReloader) Reload() error {
	return exec.Command("wifi", "up").Run()
}

// NoopWifiReloader does nothing (for tests).
type NoopWifiReloader struct{}

// Reload is a no-op.
func (r *NoopWifiReloader) Reload() error { return nil }

// WifiService provides WiFi scanning, connection, and configuration.
type WifiService struct {
	uci               uci.UCI
	ubus              ubus.Ubus
	reloader          WifiReloader
	applier           UCIApplyConfirm // optional; when set, use apply+confirm instead of wifi up
	cmd               CommandRunner
	priorityFile      string
	autoReconnectFile string
	reconnectScript   string
}

// uciApplyConfigs is the list of configs copied for apply+confirm (same as setup script).
var uciApplyConfigs = []string{"wireless", "network", "system"}

const defaultPriorityFile = "/etc/openwrt-travel-gui/wifi-priorities.json"
const defaultAutoReconnectFile = "/etc/openwrt-travel-gui/autoreconnect.json"
const defaultReconnectScript = "/etc/openwrt-travel-gui/wifi-reconnect.sh"

// NewWifiService creates a new WifiService. Uses apply+confirm when applier is set (production),
// otherwise falls back to reloader (e.g. tests or when rpcd session is unavailable).
func NewWifiService(u uci.UCI, ub ubus.Ubus) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: &ShellWifiReloader{}, applier: NewRealUCIApplyConfirm(ub),
		cmd: &RealCommandRunner{}, priorityFile: defaultPriorityFile,
		autoReconnectFile: defaultAutoReconnectFile, reconnectScript: defaultReconnectScript,
	}
}

// NewWifiServiceWithReloader creates a WifiService with a custom reloader (for tests).
// Applier is left nil so Reload() is used.
func NewWifiServiceWithReloader(u uci.UCI, ub ubus.Ubus, r WifiReloader) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: defaultPriorityFile, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript,
	}
}

// NewWifiServiceWithPriorityFile creates a WifiService with a custom priority file (for tests).
func NewWifiServiceWithPriorityFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, pf string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: pf, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript,
	}
}

// NewWifiServiceForTesting creates a WifiService with all fields customizable (for tests).
func NewWifiServiceForTesting(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile,
	}
}

// findSTADevice discovers the station (client) WiFi interface name by querying network.wireless status.
// It looks for an interface with mode "sta" and returns its ifname (e.g., "phy0-sta0") and section name (e.g., "wifinet2").
func (w *WifiService) findSTADevice() (ifname string, section string, err error) {
	resp, err := w.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to get wireless status: %w", err)
	}

	for _, radioData := range resp {
		radioMap, ok := radioData.(map[string]interface{})
		if !ok {
			continue
		}
		ifaces, ok := radioMap["interfaces"].([]interface{})
		if !ok {
			continue
		}
		for _, iface := range ifaces {
			ifaceMap, ok := iface.(map[string]interface{})
			if !ok {
				continue
			}
			config, ok := ifaceMap["config"].(map[string]interface{})
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
func (w *WifiService) findSTASection() (string, error) {
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

// getWifiRadioNames returns UCI section names of all wifi-device (radio) sections.
func (w *WifiService) getWifiRadioNames() ([]string, error) {
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

// ensureWwanNetwork creates the wwan network interface in UCI if missing (proto=dhcp).
// WiFi client (STA) must use network=wwan so netifd brings it up and runs DHCP; wan is for Ethernet.
// When creating wwan, also adds it to the firewall wan zone so STA gets NAT and internet.
func (w *WifiService) ensureWwanNetwork() error {
	if _, err := w.uci.GetAll("network", "wwan"); err == nil {
		return nil
	}
	if err := w.uci.AddSection("network", "wwan", "interface"); err != nil {
		return fmt.Errorf("adding network wwan: %w", err)
	}
	_ = w.uci.Set("network", "wwan", "proto", "dhcp")
	if err := w.uci.Commit("network"); err != nil {
		return err
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
		return nil
	}
	// Check if wwan is already in the network list to avoid duplicates.
	if net := sections[wanZone]["network"]; strings.Contains(net, "wwan") {
		return nil
	}
	if err := w.uci.AddList("firewall", wanZone, "network", "wwan"); err != nil {
		return err
	}
	return w.uci.Commit("firewall")
}

// ensureSTASectionForScan creates a STA wifi-iface (sta0) if none exists (for Connect, not for Scan).
// Uses network=wwan so the STA gets DHCP and is used as WAN when connected; ensures wwan exists.
// Returns the STA section name and commits wireless if created.
func (w *WifiService) ensureSTASectionForScan() (string, error) {
	if section, err := w.findSTASection(); err == nil {
		return section, nil
	}
	if err := w.ensureWwanNetwork(); err != nil {
		return "", fmt.Errorf("ensuring wwan network: %w", err)
	}
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("wireless sections: %w", err)
	}
	var firstRadio string
	for name, opts := range sections {
		if opts["type"] != "" {
			firstRadio = name
			break
		}
	}
	if firstRadio == "" {
		return "", fmt.Errorf("no radio found in wireless config")
	}
	if err := w.uci.AddSection("wireless", "sta0", "wifi-iface"); err != nil {
		return "", fmt.Errorf("adding sta0: %w", err)
	}
	_ = w.uci.Set("wireless", "sta0", "device", firstRadio)
	_ = w.uci.Set("wireless", "sta0", "mode", "sta")
	_ = w.uci.Set("wireless", "sta0", "network", "wwan")
	_ = w.uci.Set("wireless", "sta0", "disabled", "0")
	if err := w.uci.Commit("wireless"); err != nil {
		return "", fmt.Errorf("committing sta0: %w", err)
	}
	return "sta0", nil
}

// applyWireless applies committed UCI using apply+confirm when applier is set (same as LuCI),
// otherwise runs reloader (wifi up). Use after Commit("wireless") to avoid soft-brick risk.
func (w *WifiService) applyWireless() error {
	if w.applier != nil {
		return w.applier.ApplyAndConfirm(uciApplyConfigs)
	}
	return w.reloader.Reload()
}

// ApplyWireless applies the current wireless (and related) UCI config via apply+confirm.
// Exported for use after EnsureAPRunning when fixes were applied so they take effect without reboot.
func (w *WifiService) ApplyWireless() error {
	return w.applyWireless()
}

// parseScanResultItem builds a WifiScanResult from one iwinfo scan result map.
// bandOverride (e.g. "2.4GHz", "5GHz") is used when the result has no band; empty means derive from channel/frequency.
func parseScanResultItem(rm map[string]interface{}, bandOverride string) models.WifiScanResult {
	ssid, _ := rm["ssid"].(string)
	bssid, _ := rm["bssid"].(string)
	ch, _ := rm["channel"].(float64)
	sig, _ := rm["signal"].(float64)
	qual, _ := rm["quality"].(float64)
	band, _ := rm["band"].(string)

	enc := "none"
	if encMap, ok := rm["encryption"].(map[string]interface{}); ok {
		if enabled, ok := encMap["enabled"].(bool); ok && enabled {
			if wpa, ok := encMap["wpa"].([]interface{}); ok {
				if auth, ok := encMap["authentication"].([]interface{}); ok {
					authStr := ""
					if len(auth) > 0 {
						authStr, _ = auth[0].(string)
					}
					wpaVer := 0
					if len(wpa) > 0 {
						wpaVer = int(wpa[len(wpa)-1].(float64))
					}
					if authStr == "sae" {
						enc = "sae"
					} else if authStr == "psk" && wpaVer == 2 {
						enc = "psk2"
					} else if authStr == "psk" {
						enc = "psk"
					} else {
						enc = "psk2"
					}
				} else {
					enc = "psk2"
				}
			} else {
				enc = "wep"
			}
		}
	}

	if band == "" && bandOverride != "" {
		band = bandOverride
	}
	if band == "" {
		freq, _ := rm["frequency"].(float64)
		if freq > 0 {
			if freq < 3000 {
				band = "2.4GHz"
			} else if freq < 6000 {
				band = "5GHz"
			} else {
				band = "6GHz"
			}
		} else if ch > 0 {
			if ch <= 14 {
				band = "2.4GHz"
			} else if ch <= 177 {
				band = "5GHz"
			} else {
				band = "6GHz"
			}
		}
	}

	return models.WifiScanResult{
		SSID: ssid, BSSID: bssid,
		Channel: int(ch), SignalDBM: int(sig),
		SignalPercent: int(qual), Encryption: enc, Band: band,
	}
}

// radioBandToDisplay maps UCI band (2g, 5g) to display band string.
func radioBandToDisplay(uciBand string) string {
	switch strings.ToLower(uciBand) {
	case "5g":
		return "5GHz"
	case "2g":
		return "2.4GHz"
	case "6g":
		return "6GHz"
	default:
		return ""
	}
}

// Scan returns available WiFi networks by scanning each radio (LuCI-style). No STA required.
func (w *WifiService) Scan() ([]models.WifiScanResult, error) {
	radios, err := w.getWifiRadioNames()
	if err != nil || len(radios) == 0 {
		return []models.WifiScanResult{}, nil
	}

	var all []models.WifiScanResult
	seen := make(map[string]bool)

	for _, radioName := range radios {
		opts, _ := w.uci.GetAll("wireless", radioName)
		bandOverride := radioBandToDisplay(opts["band"])

		resp, err := w.ubus.Call("iwinfo", "scan", map[string]interface{}{"device": radioName})
		if err != nil {
			continue
		}
		results, ok := resp["results"].([]interface{})
		if !ok {
			continue
		}
		for _, r := range results {
			rm, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			item := parseScanResultItem(rm, bandOverride)
			key := item.BSSID + ":" + strconv.Itoa(item.Channel)
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, item)
		}
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].SignalDBM != all[j].SignalDBM {
			return all[i].SignalDBM > all[j].SignalDBM
		}
		if all[i].Channel != all[j].Channel {
			return all[i].Channel < all[j].Channel
		}
		return all[i].SSID < all[j].SSID
	})
	return all, nil
}

// Connect connects to a WiFi network.
func (w *WifiService) Connect(config models.WifiConfig) error {
	_, section, err := w.findSTADevice()
	if err != nil {
		section, err = w.findSTASection()
		if err != nil {
			// No STA in UCI; create sta0 so user can connect (OpenWrt approach: STA only when connecting)
			section, err = w.ensureSTASectionForScan()
			if err != nil {
				return fmt.Errorf("no STA interface found: %w", err)
			}
		}
	}
	// WiFi client must use wwan (not wan) so netifd runs DHCP and routing uses it as WAN
	_ = w.ensureWwanNetwork()
	if net, _ := w.uci.Get("wireless", section, "network"); net == "wan" {
		_ = w.uci.Set("wireless", section, "network", "wwan")
		_ = w.uci.Commit("wireless")
	}
	_ = w.uci.Set("wireless", section, "ssid", config.SSID)
	_ = w.uci.Set("wireless", section, "key", config.Password)
	if config.Encryption != "" {
		_ = w.uci.Set("wireless", section, "encryption", config.Encryption)
	}
	if config.Hidden {
		_ = w.uci.Set("wireless", section, "hidden", "1")
	} else {
		_ = w.uci.Set("wireless", section, "hidden", "0")
	}
	_ = w.uci.Set("wireless", section, "disabled", "0")
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.applyWireless()
}

// Disconnect disconnects from the current WiFi network.
func (w *WifiService) Disconnect() error {
	_, section, err := w.findSTADevice()
	if err != nil {
		// STA interface may already be disabled; fall back to UCI-based lookup
		section, err = w.findSTASection()
		if err != nil {
			return fmt.Errorf("no STA interface found: %w", err)
		}
	}
	_ = w.uci.Set("wireless", section, "disabled", "1")
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.applyWireless()
}

// GetConnection returns the current WiFi connection info.
func (w *WifiService) GetConnection() (models.WifiConnection, error) {
	ifname, _, err := w.findSTADevice()
	if err != nil {
		return models.WifiConnection{}, nil
	}

	resp, err := w.ubus.Call("iwinfo", "info", map[string]interface{}{"device": ifname})
	if err != nil {
		return models.WifiConnection{}, err
	}

	ssid, _ := resp["ssid"].(string)
	bssid, _ := resp["bssid"].(string)
	mode, _ := resp["mode"].(string)
	ch, _ := resp["channel"].(float64)
	sig, _ := resp["signal"].(float64)
	qual, _ := resp["quality"].(float64)
	enc, _ := resp["encryption"].(string)
	band, _ := resp["band"].(string)

	conn := models.WifiConnection{
		SSID: ssid, BSSID: bssid,
		Mode: mode, Channel: int(ch),
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

// SetMode sets the WiFi operating mode (e.g., "ap", "sta", "repeater").
func (w *WifiService) SetMode(mode string) error {
	_ = w.uci.Set("wireless", "default_radio0", "mode", mode)
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.applyWireless()
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
	resp, err := w.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		return []models.SavedNetwork{}, nil
	}

	priorities := w.loadPriorities()

	var networks []models.SavedNetwork
	for _, radioData := range resp {
		radioMap, ok := radioData.(map[string]interface{})
		if !ok {
			continue
		}
		ifaces, ok := radioMap["interfaces"].([]interface{})
		if !ok {
			continue
		}
		for _, iface := range ifaces {
			ifaceMap, ok := iface.(map[string]interface{})
			if !ok {
				continue
			}
			config, ok := ifaceMap["config"].(map[string]interface{})
			if !ok {
				continue
			}
			mode, _ := config["mode"].(string)
			if mode != "sta" {
				continue
			}
			ssid, _ := config["ssid"].(string)
			encryption, _ := config["encryption"].(string)

			section, _ := ifaceMap["section"].(string)
			disabled := false
			if section != "" {
				if d, err := w.uci.Get("wireless", section, "disabled"); err == nil {
					disabled = d == "1"
				}
			}

			priority := 0
			if p, ok := priorities[ssid]; ok {
				priority = p
			}

			networks = append(networks, models.SavedNetwork{
				SSID:        ssid,
				Section:     section,
				Encryption:  encryption,
				Mode:        mode,
				AutoConnect: !disabled,
				Priority:    priority,
			})
		}
	}

	// Sort by priority (lower number = higher priority), 0 means unset (goes last)
	sort.Slice(networks, func(i, j int) bool {
		pi, pj := networks[i].Priority, networks[j].Priority
		if pi == 0 && pj == 0 {
			return false
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

// GetRadioStatus returns whether any WiFi radio is enabled.
func (w *WifiService) GetRadioStatus() (bool, error) {
	for _, radio := range []string{"radio0", "radio1"} {
		opts, err := w.uci.GetAll("wireless", radio)
		if err != nil {
			continue
		}
		if opts["disabled"] != "1" {
			return true, nil
		}
	}
	return false, nil
}

// SetRadioEnabled enables or disables all WiFi radios.
func (w *WifiService) SetRadioEnabled(enabled bool) error {
	value := "0"
	if !enabled {
		value = "1"
	}
	for _, radio := range []string{"radio0", "radio1"} {
		if _, err := w.uci.GetAll("wireless", radio); err == nil {
			_ = w.uci.Set("wireless", radio, "disabled", value)
		}
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.applyWireless()
}

// DeleteNetwork removes a saved WiFi network by its UCI section name.
func (w *WifiService) DeleteNetwork(section string) error {
	if section == "" {
		return fmt.Errorf("section name is required")
	}
	if err := w.uci.DeleteSection("wireless", section); err != nil {
		return fmt.Errorf("failed to delete network: %w", err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.applyWireless()
}

// GetRadios returns information about all WiFi radio hardware.
func (w *WifiService) GetRadios() ([]models.RadioInfo, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return []models.RadioInfo{}, nil
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
		radios = append(radios, models.RadioInfo{
			Name:     name,
			Band:     opts["band"],
			Channel:  channel,
			HTMode:   opts["htmode"],
			Type:     devType,
			Disabled: opts["disabled"] == "1",
		})
	}
	return radios, nil
}

// GetAPConfigs returns the AP configuration for all radios.
func (w *WifiService) GetAPConfigs() ([]models.APConfig, error) {
	var configs []models.APConfig
	for i := 0; i < 4; i++ {
		section := fmt.Sprintf("default_radio%d", i)
		opts, err := w.uci.GetAll("wireless", section)
		if err != nil {
			continue
		}
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
func (w *WifiService) SetAPConfig(section string, config models.APConfig) error {
	opts, err := w.uci.GetAll("wireless", section)
	if err != nil {
		return fmt.Errorf("AP section %s not found", section)
	}
	if opts["mode"] != "ap" {
		return fmt.Errorf("section %s is not an AP interface", section)
	}
	if config.SSID != "" {
		if err := w.uci.Set("wireless", section, "ssid", config.SSID); err != nil {
			return fmt.Errorf("setting SSID: %w", err)
		}
	}
	if config.Encryption != "" {
		if err := w.uci.Set("wireless", section, "encryption", config.Encryption); err != nil {
			return fmt.Errorf("setting encryption: %w", err)
		}
	}
	if config.Encryption != "none" && config.Key != "" {
		if err := w.uci.Set("wireless", section, "key", config.Key); err != nil {
			return fmt.Errorf("setting key: %w", err)
		}
	}
	disabled := "0"
	if !config.Enabled {
		disabled = "1"
	}
	if err := w.uci.Set("wireless", section, "disabled", disabled); err != nil {
		return fmt.Errorf("setting disabled: %w", err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return fmt.Errorf("committing wireless: %w", err)
	}
	_ = w.applyWireless()
	return nil
}

// GetMACAddresses returns the MAC address info for WiFi interfaces.
func (w *WifiService) GetMACAddresses() ([]models.MACConfig, error) {
	var configs []models.MACConfig

	// Get STA interface MAC
	staSection := "sta0"
	staOpts, err := w.uci.GetAll("wireless", staSection)
	if err != nil {
		// Try wifinet2 as fallback section name
		staSection = "wifinet2"
		staOpts, err = w.uci.GetAll("wireless", staSection)
	}
	if err == nil && staOpts["mode"] == "sta" {
		currentMAC := ""
		// Try reading from sysfs
		data, readErr := os.ReadFile("/sys/class/net/phy0-sta0/address")
		if readErr == nil {
			currentMAC = strings.TrimSpace(string(data))
		}
		configs = append(configs, models.MACConfig{
			Interface:  "sta",
			CurrentMAC: currentMAC,
			CustomMAC:  staOpts["macaddr"],
		})
	}

	return configs, nil
}

// SetMACAddress sets a custom MAC address on the STA WiFi interface.
func (w *WifiService) SetMACAddress(mac string) error {
	// Find STA section
	staSection := "sta0"
	opts, err := w.uci.GetAll("wireless", staSection)
	if err != nil {
		staSection = "wifinet2"
		opts, err = w.uci.GetAll("wireless", staSection)
	}
	if err != nil {
		return fmt.Errorf("STA interface not found")
	}
	if opts["mode"] != "sta" {
		return fmt.Errorf("section %s is not a STA interface", staSection)
	}
	if mac == "" {
		// Reset: clear the macaddr option
		if err := w.uci.Set("wireless", staSection, "macaddr", ""); err != nil {
			return fmt.Errorf("clearing MAC: %w", err)
		}
	} else {
		if err := w.uci.Set("wireless", staSection, "macaddr", mac); err != nil {
			return fmt.Errorf("setting MAC: %w", err)
		}
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return fmt.Errorf("committing wireless: %w", err)
	}
	_ = w.applyWireless()
	return nil
}

// RandomizeMAC generates a random locally-administered unicast MAC address
// and applies it to the STA WiFi interface. It returns the new MAC.
func (w *WifiService) RandomizeMAC() (string, error) {
	mac, err := generateRandomMAC()
	if err != nil {
		return "", fmt.Errorf("generating random MAC: %w", err)
	}
	if err := w.SetMACAddress(mac); err != nil {
		return "", err
	}
	return mac, nil
}

// generateRandomMAC creates a random locally-administered unicast MAC address.
// Locally-administered: bit 1 of first octet set. Unicast: bit 0 of first octet cleared.
func generateRandomMAC() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// Set locally-administered bit (bit 1) and clear unicast/multicast bit (bit 0)
	buf[0] = (buf[0] | 0x02) & 0xFE
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
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
func (w *WifiService) SetGuestWifi(cfg models.GuestWifiConfig) error {
	if !cfg.Enabled {
		_, err := w.uci.GetAll("wireless", "guest")
		if err == nil {
			_ = w.uci.Set("wireless", "guest", "disabled", "1")
			_ = w.uci.Commit("wireless")
			_ = w.applyWireless()
		}
		return nil
	}

	// Network interface for guest subnet
	_ = w.uci.Set("network", "guest", "proto", "static")
	_ = w.uci.Set("network", "guest", "ipaddr", "192.168.2.1")
	_ = w.uci.Set("network", "guest", "netmask", "255.255.255.0")
	_ = w.uci.Commit("network")

	// DHCP for guest network
	_ = w.uci.Set("dhcp", "guest", "interface", "guest")
	_ = w.uci.Set("dhcp", "guest", "start", "100")
	_ = w.uci.Set("dhcp", "guest", "limit", "50")
	_ = w.uci.Set("dhcp", "guest", "leasetime", "2h")
	_ = w.uci.Commit("dhcp")

	// Wireless interface for guest AP
	_ = w.uci.Set("wireless", "guest", "device", "radio0")
	_ = w.uci.Set("wireless", "guest", "mode", "ap")
	_ = w.uci.Set("wireless", "guest", "network", "guest")
	_ = w.uci.Set("wireless", "guest", "ssid", cfg.SSID)
	_ = w.uci.Set("wireless", "guest", "encryption", cfg.Encryption)
	_ = w.uci.Set("wireless", "guest", "key", cfg.Key)
	_ = w.uci.Set("wireless", "guest", "isolate", "1")
	_ = w.uci.Set("wireless", "guest", "disabled", "0")
	_ = w.uci.Commit("wireless")

	// Firewall zone for guest
	_ = w.uci.Set("firewall", "guest_zone", "name", "guest")
	_ = w.uci.Set("firewall", "guest_zone", "network", "guest")
	_ = w.uci.Set("firewall", "guest_zone", "input", "REJECT")
	_ = w.uci.Set("firewall", "guest_zone", "output", "ACCEPT")
	_ = w.uci.Set("firewall", "guest_zone", "forward", "REJECT")

	// Forwarding: guest -> wan
	_ = w.uci.Set("firewall", "guest_fwd", "src", "guest")
	_ = w.uci.Set("firewall", "guest_fwd", "dest", "wan")

	// Allow DNS from guest
	_ = w.uci.Set("firewall", "guest_dns", "name", "Allow-Guest-DNS")
	_ = w.uci.Set("firewall", "guest_dns", "src", "guest")
	_ = w.uci.Set("firewall", "guest_dns", "dest_port", "53")
	_ = w.uci.Set("firewall", "guest_dns", "target", "ACCEPT")

	// Allow DHCP from guest
	_ = w.uci.Set("firewall", "guest_dhcp", "name", "Allow-Guest-DHCP")
	_ = w.uci.Set("firewall", "guest_dhcp", "src", "guest")
	_ = w.uci.Set("firewall", "guest_dhcp", "dest_port", "67-68")
	_ = w.uci.Set("firewall", "guest_dhcp", "target", "ACCEPT")
	_ = w.uci.Commit("firewall")

	_ = w.applyWireless()
	return nil
}

// GetAutoReconnect returns whether auto-reconnect is enabled.
func (w *WifiService) GetAutoReconnect() (bool, error) {
	data, err := os.ReadFile(w.autoReconnectFile)
	if err != nil {
		return false, nil
	}
	var config struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return false, nil
	}
	return config.Enabled, nil
}

// SetAutoReconnect enables or disables auto-reconnect to saved WiFi networks.
// When enabled, it writes a reconnect script and adds a cron entry.
// When disabled, it removes the cron entry and script.
func (w *WifiService) SetAutoReconnect(enabled bool) error {
	dir := filepath.Dir(w.autoReconnectFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	config := struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(w.autoReconnectFile, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	if enabled {
		return w.enableAutoReconnect()
	}
	return w.disableAutoReconnect()
}

// reconnectScriptContent is the safe script body (wifi up, not wifi reload).
// Includes a crash guard: if a previous wifi up caused a crash/reboot, the guard
// file will exist on next run and the script exits without retrying.
const reconnectScriptContent = "#!/bin/sh\n# Auto-reconnect to saved WiFi networks\n# Managed by openwrt-travel-gui — do not edit manually\n\n" +
	"GUARD=\"/etc/openwrt-travel-gui/autoreconnect-crash-guard\"\n" +
	"if [ -f \"$GUARD\" ]; then\n    exit 0\nfi\n\n" +
	"IP=$(ubus call network.interface.wwan status 2>/dev/null | jsonfilter -e '@[\"ipv4-address\"][0].address' 2>/dev/null)\n" +
	"if [ -n \"$IP\" ]; then\n    exit 0\nfi\n\n" +
	"# Connection dropped — write guard, bring up WiFi, clear guard on success\n" +
	"echo wifi-reconnect > \"$GUARD\"\nwifi up && rm -f \"$GUARD\"\n"

func (w *WifiService) enableAutoReconnect() error {
	scriptDir := filepath.Dir(w.reconnectScript)
	if err := os.MkdirAll(scriptDir, 0750); err != nil {
		return fmt.Errorf("creating script directory: %w", err)
	}
	if err := os.WriteFile(w.reconnectScript, []byte(reconnectScriptContent), 0750); err != nil {
		return fmt.Errorf("writing reconnect script: %w", err)
	}

	// Add cron entry (every minute)
	cronCmd := fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v '%s'; echo '* * * * * %s') | crontab -`,
		w.reconnectScript, w.reconnectScript)
	if _, err := w.cmd.Run("sh", "-c", cronCmd); err != nil {
		return fmt.Errorf("adding cron entry: %w", err)
	}
	return nil
}

// WriteReconnectScriptSafe writes the current safe reconnect script to disk if the
// script file already exists. Call this on startup so devices that had auto-reconnect
// enabled before a deploy get the safe "wifi up" script instead of the old "wifi reload".
func (w *WifiService) WriteReconnectScriptSafe() {
	if _, err := os.Stat(w.reconnectScript); err != nil {
		return // script not present, nothing to fix
	}
	_ = os.WriteFile(w.reconnectScript, []byte(reconnectScriptContent), 0750)
}

func (w *WifiService) disableAutoReconnect() error {
	// Remove cron entry
	cronCmd := fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v '%s') | crontab -`, w.reconnectScript)
	_, _ = w.cmd.Run("sh", "-c", cronCmd)

	// Remove script file
	os.Remove(w.reconnectScript)
	return nil
}
