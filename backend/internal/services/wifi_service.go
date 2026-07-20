package services

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// ErrMultipleActiveSTA is returned when more than one STA wifi-iface is enabled
// and bound to network=wwan. netifd can bind only one device to the wwan interface,
// so this state leaves the actually-connected STA without a DHCP lease.
var ErrMultipleActiveSTA = errors.New("wireless config invalid: multiple enabled STA interfaces on network=wwan")

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
	return execx.Run(execx.Slow, "wifi", "up")
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
	uci                 uci.UCI
	ubus                ubus.Ubus
	reloader            WifiReloader
	applier             UCIApplyConfirm // optional; when set, use apply+confirm instead of wifi up
	cmd                 CommandRunner
	priorityFile        string
	autoReconnectFile   string
	reconnectScript     string
	modeFile            string
	repeaterOptionsFile string
}

// uciApplyConfigs is the list of configs copied for staged apply+confirm.
// Include the related network services that WiFi mutations can touch.
var uciApplyConfigs = []string{"wireless", "network", "system", "firewall", "dhcp"}

const defaultPriorityFile = "/etc/travo/wifi-priorities.json"
const defaultAutoReconnectFile = "/etc/travo/autoreconnect.json"
const defaultReconnectScript = "/etc/travo/wifi-reconnect.sh"
const defaultWifiModeFile = "/etc/travo/wifi-mode"
const defaultRepeaterOptionsFile = "/etc/travo/repeater-options.json"

// NewWifiService creates a new WifiService. Uses apply+confirm when applier is set (production),
// otherwise falls back to reloader (e.g. tests or when rpcd session is unavailable).
func NewWifiService(u uci.UCI, ub ubus.Ubus, pw *auth.RootPassword) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: &ShellWifiReloader{}, applier: NewRealUCIApplyConfirm(ub, pw),
		cmd: &RealCommandRunner{}, priorityFile: defaultPriorityFile,
		autoReconnectFile: defaultAutoReconnectFile, reconnectScript: defaultReconnectScript,
		modeFile: defaultWifiModeFile, repeaterOptionsFile: defaultRepeaterOptionsFile,
	}
}

// NewWifiServiceWithReloader creates a WifiService with a custom reloader (for tests).
// Applier is left nil so Reload() is used.
func NewWifiServiceWithReloader(u uci.UCI, ub ubus.Ubus, r WifiReloader) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: defaultPriorityFile, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile,
	}
}

// NewWifiServiceWithPriorityFile creates a WifiService with a custom priority file (for tests).
func NewWifiServiceWithPriorityFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, pf string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: pf, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile,
	}
}

// NewWifiServiceForTesting creates a WifiService with all fields customizable (for tests).
func NewWifiServiceForTesting(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile,
	}
}

// NewWifiServiceForTestingWithModeFile creates a WifiService with a custom mode file (for tests).
func NewWifiServiceForTestingWithModeFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile, modeFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: modeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile,
	}
}

// validateWirelessConsistency enforces invariants that, if violated, leave the router
// in a broken state that rpcd's rollback timer cannot fix (rollback restores the *previous*
// config, which may itself be broken if the bug is in our own writer). Currently checks:
//   - At most one enabled STA wifi-iface may be bound to network=wwan.
func (w *WifiService) validateWirelessConsistency() error {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return fmt.Errorf("failed to get wireless sections: %w", err)
	}
	var activeWwanSTAs []string
	for name, opts := range sections {
		if opts["mode"] != "sta" {
			continue
		}
		if opts["disabled"] == "1" {
			continue
		}
		if opts["network"] != "wwan" {
			continue
		}
		activeWwanSTAs = append(activeWwanSTAs, name)
	}
	if len(activeWwanSTAs) > 1 {
		sort.Strings(activeWwanSTAs)
		return fmt.Errorf("%w: sections=%s", ErrMultipleActiveSTA, strings.Join(activeWwanSTAs, ","))
	}
	return nil
}

func (w *WifiService) stageWirelessApply() (*WirelessApplyResult, error) {
	if err := w.validateWirelessConsistency(); err != nil {
		return nil, err
	}
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
