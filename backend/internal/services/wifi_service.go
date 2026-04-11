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

	"github.com/openwrt-travel-gui/backend/internal/auth"
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

// WirelessApplyResult describes a staged rollback apply that still needs
// browser-driven confirmation.
type WirelessApplyResult struct {
	Token                  string
	RollbackTimeoutSeconds int
}

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
	modeFile          string
}

// uciApplyConfigs is the list of configs copied for staged apply+confirm.
// Include the related network services that WiFi mutations can touch.
var uciApplyConfigs = []string{"wireless", "network", "system", "firewall", "dhcp"}

const defaultPriorityFile = "/etc/travo/wifi-priorities.json"
const defaultAutoReconnectFile = "/etc/travo/autoreconnect.json"
const defaultReconnectScript = "/etc/travo/wifi-reconnect.sh"
const defaultWifiModeFile = "/etc/travo/wifi-mode"

// NewWifiService creates a new WifiService. Uses apply+confirm when applier is set (production),
// otherwise falls back to reloader (e.g. tests or when rpcd session is unavailable).
func NewWifiService(u uci.UCI, ub ubus.Ubus, pw *auth.RootPassword) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: &ShellWifiReloader{}, applier: NewRealUCIApplyConfirm(ub, pw),
		cmd: &RealCommandRunner{}, priorityFile: defaultPriorityFile,
		autoReconnectFile: defaultAutoReconnectFile, reconnectScript: defaultReconnectScript,
		modeFile: defaultWifiModeFile,
	}
}

// NewWifiServiceWithReloader creates a WifiService with a custom reloader (for tests).
// Applier is left nil so Reload() is used.
func NewWifiServiceWithReloader(u uci.UCI, ub ubus.Ubus, r WifiReloader) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: defaultPriorityFile, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
	}
}

// NewWifiServiceWithPriorityFile creates a WifiService with a custom priority file (for tests).
func NewWifiServiceWithPriorityFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, pf string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: pf, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
	}
}

// NewWifiServiceForTesting creates a WifiService with all fields customizable (for tests).
func NewWifiServiceForTesting(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: defaultWifiModeFile,
	}
}

// NewWifiServiceForTestingWithModeFile creates a WifiService with a custom mode file (for tests).
func NewWifiServiceForTestingWithModeFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile, modeFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: modeFile,
	}
}

func (w *WifiService) stageWirelessApply() (*WirelessApplyResult, error) {
	if w.applier != nil {
		token, err := w.applier.StartApply(uciApplyConfigs)
		if err != nil {
			return nil, err
		}
		return &WirelessApplyResult{
			Token:                  token,
			RollbackTimeoutSeconds: uciApplyRollbackTimeout,
		}, nil
	}
	if err := w.reloader.Reload(); err != nil {
		return nil, err
	}
	return nil, nil
}

// ConfirmApply finalizes a staged wireless apply once the browser has proven
// the router is still reachable after the config change.
func (w *WifiService) ConfirmApply(token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("apply token is required")
	}
	if w.applier == nil {
		return nil
	}
	if err := w.applier.Confirm(token); err != nil {
		return err
	}
	// After successful confirm, no reload is needed as apply+confirm already applied changes
	return nil
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

// findSTASectionBySSID returns the UCI section name of a saved STA profile matching ssid, or error if not found.
func (w *WifiService) findSTASectionBySSID(ssid string) (string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("failed to get wireless sections: %w", err)
	}
	for name, opts := range sections {
		if opts["mode"] == "sta" && opts["ssid"] == ssid {
			return name, nil
		}
	}
	return "", fmt.Errorf("no STA section found for SSID %q", ssid)
}

// nextSTASectionName returns a unique UCI section name for a new STA profile (sta0, sta1, …).
func (w *WifiService) nextSTASectionName() string {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "sta0"
	}
	for i := 0; ; i++ {
		candidate := fmt.Sprintf("sta%d", i)
		if _, exists := sections[candidate]; !exists {
			return candidate
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
func (w *WifiService) selectActiveSTA() (string, error) {
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
func (w *WifiService) disableOtherSTASections(activeSection string) error {
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

func (w *WifiService) getWifiSectionsByMode(mode string) ([]string, error) {
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

func (w *WifiService) setIfaceDisabled(section string, disabled bool) error {
	value := "0"
	if disabled {
		value = "1"
	}
	return w.uci.Set("wireless", section, "disabled", value)
}

func (w *WifiService) ensureSectionRadioEnabled(section string) error {
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

func (w *WifiService) deriveWifiMode() string {
	// Persisted mode is authoritative: "client" and "repeater" both have STA+AP
	// enabled in UCI, so UCI-only detection can't distinguish them.
	if w.modeFile != "" {
		if data, err := os.ReadFile(w.modeFile); err == nil {
			saved := strings.TrimSpace(string(data))
			if saved == "client" || saved == "ap" || saved == "repeater" {
				return saved
			}
		}
	}
	// Fall back to UCI detection (used before any explicit SetMode() call).
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

func (w *WifiService) ensureNamedSection(config, section, sectionType string) error {
	if _, err := w.uci.GetAll(config, section); err == nil {
		return nil
	}
	return w.uci.AddSection(config, section, sectionType)
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
		for _, item := range strings.Fields(net) {
			if item == "wwan" {
				return nil
			}
		}
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

// parseIwinfoEncryption parses the structured encryption map returned by iwinfo
// (both from "scan" and "info" responses) into a normalized encryption string
// (e.g. "psk2", "sae", "wep", "none").
func parseIwinfoEncryption(encField interface{}) string {
	encMap, ok := encField.(map[string]interface{})
	if !ok {
		// Fallback: plain string (e.g. UCI value stored directly).
		if s, ok := encField.(string); ok && s != "" {
			return s
		}
		return "none"
	}
	enabled, _ := encMap["enabled"].(bool)
	if !enabled {
		return "none"
	}
	if wpa, ok := encMap["wpa"].([]interface{}); ok {
		authStr := ""
		if auth, ok := encMap["authentication"].([]interface{}); ok && len(auth) > 0 {
			authStr, _ = auth[0].(string)
		}
		wpaVer := 0
		if len(wpa) > 0 {
			wpaVer = int(wpa[len(wpa)-1].(float64))
		}
		switch {
		case authStr == "sae":
			return "sae"
		case authStr == "psk" && wpaVer == 2:
			return "psk2"
		case authStr == "psk":
			return "psk"
		default:
			return "psk2"
		}
	}
	return "wep"
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

	enc := parseIwinfoEncryption(rm["encryption"])

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

// requestBandToUCI maps frontend/API band string (2.4ghz, 5ghz, 6ghz) to UCI band (2g, 5g, 6g).
func requestBandToUCI(band string) string {
	switch strings.ToLower(strings.TrimSpace(band)) {
	case "2.4ghz", "2.4g":
		return "2g"
	case "5ghz", "5g":
		return "5g"
	case "6ghz", "6g":
		return "6g"
	default:
		return ""
	}
}

// getRadioForBand returns the UCI radio section name (e.g. "radio0") that has the given band.
// band is the request band (2.4ghz, 5ghz, 6ghz) or UCI band (2g, 5g, 6g).
func (w *WifiService) getRadioForBand(band string) (string, error) {
	uciBand := requestBandToUCI(band)
	if uciBand == "" {
		uciBand = strings.ToLower(strings.TrimSpace(band))
		if uciBand != "2g" && uciBand != "5g" && uciBand != "6g" {
			return "", fmt.Errorf("unsupported band: %q", band)
		}
	}
	radios, err := w.getWifiRadioNames()
	if err != nil || len(radios) == 0 {
		return "", fmt.Errorf("no radios found")
	}
	for _, name := range radios {
		opts, _ := w.uci.GetAll("wireless", name)
		if opts["band"] == uciBand {
			return name, nil
		}
	}
	return "", fmt.Errorf("no radio found for band %s", uciBand)
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
// Each distinct SSID gets its own UCI section so saved profiles persist across connections.
// All other STA sections are disabled (not deleted) when connecting to a new network.
func (w *WifiService) Connect(config models.WifiConfig) (*WirelessApplyResult, error) {
	// WiFi client must use wwan (not wan) so netifd runs DHCP and routing uses it as WAN
	if err := w.ensureWwanNetwork(); err != nil {
		return nil, err
	}

	// Find or create a dedicated UCI section for this SSID.
	section, err := w.findSTASectionBySSID(config.SSID)
	if err != nil {
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
	if err := w.uci.Set("wireless", section, "key", config.Password); err != nil {
		return nil, fmt.Errorf("setting STA key: %w", err)
	}
	if config.Encryption != "" {
		if err := w.uci.Set("wireless", section, "encryption", config.Encryption); err != nil {
			return nil, fmt.Errorf("setting STA encryption: %w", err)
		}
	}
	if config.Hidden {
		if err := w.uci.Set("wireless", section, "hidden", "1"); err != nil {
			return nil, fmt.Errorf("setting STA hidden flag: %w", err)
		}
	} else {
		if err := w.uci.Set("wireless", section, "hidden", "0"); err != nil {
			return nil, fmt.Errorf("setting STA hidden flag: %w", err)
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

// SetMode sets the app-level WiFi operating mode by enabling/disabling STA and AP sections.
// Uses OpenWRT's apply+confirm flow for safety: if the device crashes or becomes unreachable,
// the rollback timer (30 seconds) will automatically revert to the previous configuration.
// The user's browser polls to confirm the router is still reachable; if confirm succeeds,
// the rollback is cancelled. This prevents soft-brick scenarios without needing a separate
// guard file (which is only required for background tasks that run without user oversight).
func (w *WifiService) SetMode(mode string) (*WirelessApplyResult, error) {
	validModes := map[string]bool{"ap": true, "client": true, "repeater": true}
	if !validModes[mode] {
		return nil, fmt.Errorf("unsupported wifi mode %q", mode)
	}

	apSections, err := w.getWifiSectionsByMode("ap")
	if err != nil {
		return nil, err
	}
	staSections, err := w.getWifiSectionsByMode("sta")
	if err != nil {
		return nil, err
	}

	enableAP := false
	enableSTA := false
	switch mode {
	case "ap":
		enableAP = true
	case "client":
		enableSTA = true
	case "repeater":
		enableAP = true
		enableSTA = true
	}

	if enableSTA && len(staSections) == 0 {
		section, err := w.ensureSTASectionForScan()
		if err != nil {
			return nil, err
		}
		staSections = append(staSections, section)
	}

	// At most one STA may be enabled at a time: two STA wifi-iface sections pointing at
	// network=wwan race for the interface in netifd, and the losing binding leaves the
	// actually-connected STA without DHCP. Pick the single preferred profile and disable the rest.
	var activeSTA string
	if enableSTA {
		var err error
		activeSTA, err = w.selectActiveSTA()
		if err != nil {
			return nil, err
		}
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
	for _, section := range staSections {
		enable := enableSTA && section == activeSTA
		if err := w.setIfaceDisabled(section, !enable); err != nil {
			return nil, err
		}
		if enable {
			if err := w.ensureSectionRadioEnabled(section); err != nil {
				return nil, err
			}
		}
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, err
	}
	if w.modeFile != "" {
		if err := os.MkdirAll(filepath.Dir(w.modeFile), 0750); err == nil {
			_ = os.WriteFile(w.modeFile, []byte(mode), 0600)
		}
	}
	return w.stageWirelessApply()
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
func (w *WifiService) SetAPConfig(section string, config models.APConfig) (*WirelessApplyResult, error) {
	opts, err := w.uci.GetAll("wireless", section)
	if err != nil {
		return nil, fmt.Errorf("AP section %s not found", section)
	}
	if opts["mode"] != "ap" {
		return nil, fmt.Errorf("section %s is not an AP interface", section)
	}
	if config.SSID != "" {
		if err := w.uci.Set("wireless", section, "ssid", config.SSID); err != nil {
			return nil, fmt.Errorf("setting SSID: %w", err)
		}
	}
	if config.Encryption != "" {
		if err := w.uci.Set("wireless", section, "encryption", config.Encryption); err != nil {
			return nil, fmt.Errorf("setting encryption: %w", err)
		}
	}
	if config.Encryption != "none" && config.Key != "" {
		if err := w.uci.Set("wireless", section, "key", config.Key); err != nil {
			return nil, fmt.Errorf("setting key: %w", err)
		}
	}
	disabled := boolToEnabled(!config.Enabled)
	if err := w.uci.Set("wireless", section, "disabled", disabled); err != nil {
		return nil, fmt.Errorf("setting disabled: %w", err)
	}
	if config.Enabled {
		if err := w.ensureSectionRadioEnabled(section); err != nil {
			return nil, fmt.Errorf("enabling AP radio: %w", err)
		}
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, fmt.Errorf("committing wireless: %w", err)
	}
	return w.stageWirelessApply()
}

// GetMACAddresses returns the MAC address info for WiFi interfaces.
func (w *WifiService) GetMACAddresses() ([]models.MACConfig, error) {
	var configs []models.MACConfig

	staSection, err := w.findSTASection()
	if err != nil {
		return configs, nil // No STA section; return empty (not an error)
	}
	staOpts, err := w.uci.GetAll("wireless", staSection)
	if err != nil {
		return configs, nil
	}
	currentMAC := ""
	// Try reading from sysfs (ifname pattern: phy<N>-sta<N>)
	if ifname, _, sysErr := w.findSTADevice(); sysErr == nil && ifname != "" {
		if data, readErr := os.ReadFile("/sys/class/net/" + ifname + "/address"); readErr == nil {
			currentMAC = strings.TrimSpace(string(data))
		}
	}
	configs = append(configs, models.MACConfig{
		Interface:  "sta",
		CurrentMAC: currentMAC,
		CustomMAC:  staOpts["macaddr"],
	})

	return configs, nil
}

// SetMACAddress sets a custom MAC address on the STA WiFi interface.
func (w *WifiService) SetMACAddress(mac string) (*WirelessApplyResult, error) {
	staSection, err := w.findSTASection()
	if err != nil {
		return nil, fmt.Errorf("STA interface not found")
	}
	if mac == "" {
		// Reset: clear the macaddr option
		if err := w.uci.Set("wireless", staSection, "macaddr", ""); err != nil {
			return nil, fmt.Errorf("clearing MAC: %w", err)
		}
	} else {
		if err := w.uci.Set("wireless", staSection, "macaddr", mac); err != nil {
			return nil, fmt.Errorf("setting MAC: %w", err)
		}
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, fmt.Errorf("committing wireless: %w", err)
	}
	return w.stageWirelessApply()
}

// RandomizeMAC generates a random locally-administered unicast MAC address
// and applies it to the STA WiFi interface. It returns the new MAC.
func (w *WifiService) RandomizeMAC() (string, *WirelessApplyResult, error) {
	mac, err := generateRandomMAC()
	if err != nil {
		return "", nil, fmt.Errorf("generating random MAC: %w", err)
	}
	apply, err := w.SetMACAddress(mac)
	if err != nil {
		return "", nil, err
	}
	return mac, apply, nil
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
	"GUARD=\"/etc/travo/autoreconnect-crash-guard\"\n" +
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
	_ = os.Remove(w.reconnectScript)
	return nil
}

// GetSTASignalInfo returns the active STA connection signal, SSID, and radio device.
// Returns (ssid="", signalDBM=0, radio="") when no STA is connected.
func (w *WifiService) GetSTASignalInfo() (ssid string, signalDBM int, radio string, err error) {
	ifname, _, err := w.findSTADevice()
	if err != nil || ifname == "" {
		return "", 0, "", nil // not connected
	}
	resp, err := w.ubus.Call("iwinfo", "info", map[string]interface{}{"device": ifname})
	if err != nil {
		return "", 0, "", err
	}
	ssid, _ = resp["ssid"].(string)
	if sig, ok := resp["signal"].(float64); ok {
		signalDBM = int(sig)
	}
	// Determine current radio from UCI
	section, _ := w.findSTASection()
	if section != "" {
		opts, _ := w.uci.GetAll("wireless", section)
		radio = opts["device"]
	}
	return ssid, signalDBM, radio, nil
}

// ScanRadioForSSID scans the given radio and returns the best signal for the target SSID.
// Returns (0, false, nil) when the SSID is not found on that radio.
func (w *WifiService) ScanRadioForSSID(radioName, ssid string) (int, bool, error) {
	resp, err := w.ubus.Call("iwinfo", "scan", map[string]interface{}{"device": radioName})
	if err != nil {
		return 0, false, err
	}
	results, _ := resp["results"].([]interface{})
	bestSignal := -999
	found := false
	for _, r := range results {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if s, _ := rm["ssid"].(string); s != ssid {
			continue
		}
		found = true
		if sig, ok := rm["signal"].(float64); ok && int(sig) > bestSignal {
			bestSignal = int(sig)
		}
	}
	if !found {
		return 0, false, nil
	}
	return bestSignal, true, nil
}

// SwitchSTAToRadio moves the active STA section to a different radio device and
// applies the change. This is for automated background switches; it uses
// applyWireless (ApplyAndConfirm) rather than the staged browser-confirm flow.
func (w *WifiService) SwitchSTAToRadio(targetRadio string) error {
	section, err := w.findSTASection()
	if err != nil {
		return fmt.Errorf("no STA section to switch: %w", err)
	}
	if err := w.uci.Set("wireless", section, "device", targetRadio); err != nil {
		return fmt.Errorf("uci set device: %w", err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return fmt.Errorf("uci commit: %w", err)
	}
	return w.applyWireless()
}

const wifiSchedulePath = "/etc/travo/wifi-schedule.json"
const wifiScheduleCronPath = "/etc/cron.d/openwrt-gui-wifi-schedule"

// GetWiFiSchedule returns the current cron-based WiFi on/off schedule.
func (w *WifiService) GetWiFiSchedule() (models.WiFiSchedule, error) {
	data, err := os.ReadFile(wifiSchedulePath)
	if err != nil {
		return models.WiFiSchedule{Enabled: false}, nil
	}
	var s models.WiFiSchedule
	if err := json.Unmarshal(data, &s); err != nil {
		return models.WiFiSchedule{Enabled: false}, nil
	}
	return s, nil
}

// SetWiFiSchedule saves the WiFi schedule and updates the cron file.
func (w *WifiService) SetWiFiSchedule(schedule models.WiFiSchedule) error {
	data, err := json.Marshal(schedule)
	if err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/travo", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(wifiSchedulePath, data, 0o644); err != nil {
		return err
	}

	if !schedule.Enabled || schedule.OnTime == "" || schedule.OffTime == "" {
		_ = os.Remove(wifiScheduleCronPath)
		return nil
	}

	onParts := strings.Split(schedule.OnTime, ":")
	offParts := strings.Split(schedule.OffTime, ":")
	if len(onParts) != 2 || len(offParts) != 2 {
		return fmt.Errorf("invalid time format, expected HH:MM")
	}

	// cron format: MM HH * * * user command
	cronContent := fmt.Sprintf("%s %s * * * root /sbin/wifi up\n%s %s * * * root /sbin/wifi down\n",
		onParts[1], onParts[0], offParts[1], offParts[0])
	return os.WriteFile(wifiScheduleCronPath, []byte(cronContent), 0o644)
}

const macPoliciesPath = "/etc/travo/mac-policies.json"

// GetMACPolicies returns the saved per-network MAC policies.
func (w *WifiService) GetMACPolicies() (models.MACPolicies, error) {
	data, err := os.ReadFile(macPoliciesPath)
	if err != nil {
		return models.MACPolicies{Policies: []models.MACPolicy{}}, nil
	}
	var p models.MACPolicies
	if err := json.Unmarshal(data, &p); err != nil {
		return models.MACPolicies{Policies: []models.MACPolicy{}}, nil
	}
	return p, nil
}

// SetMACPolicies saves the per-network MAC policies.
func (w *WifiService) SetMACPolicies(policies models.MACPolicies) error {
	data, err := json.Marshal(policies)
	if err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/travo", 0o755); err != nil {
		return err
	}
	return os.WriteFile(macPoliciesPath, data, 0o644)
}
