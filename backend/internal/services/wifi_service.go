package services

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// ErrMultipleActiveSTA is returned when more than one STA wifi-iface is enabled
// and bound to network=wwan. netifd can bind only one device to the wwan interface,
// so this state leaves the actually-connected STA without a DHCP lease.
var ErrMultipleActiveSTA = errors.New("wireless config invalid: multiple enabled STA interfaces on network=wwan")

// WifiReloader applies wireless configuration changes. Production wiring always
// has the rpcd applier configured; the reloader is the test seam only.
type WifiReloader interface {
	Reload() error
}

// NoopWifiReloader does nothing (for tests).
type NoopWifiReloader struct{}

// Reload is a no-op.
func (r *NoopWifiReloader) Reload() error { return nil }

// uciReverter is implemented by UCI backends that can drop staged (uncommitted)
// changes for a config. RealUCI implements it with `uci revert <config>`; the
// in-memory mock does not, which simply skips the rollback.
type uciReverter interface {
	Revert(config string) error
}

// revertUCIConfig drops the staged UCI delta for each config that has one.
//
// The uci CLI keeps uncommitted changes in the process-global
// /tmp/.uci/<config>/changes file, so a write sequence that fails half-way
// must be reverted: otherwise a later, unrelated `uci commit <config>` (a WAN
// save, a DHCP change, …) silently persists the abandoned delta.
func revertUCIConfig(u uci.UCI, configs ...string) {
	reverter, ok := u.(uciReverter)
	if !ok {
		return
	}
	for _, config := range configs {
		if err := reverter.Revert(config); err != nil {
			log.Printf("WARNING: uci revert %s: %v", config, err)
		}
	}
}

// WirelessApplyResult describes a staged rollback apply that still needs
// browser-driven confirmation.
type WirelessApplyResult struct {
	Token                  string
	RollbackTimeoutSeconds int

	// GeneratedKey carries a WPA passphrase the service had to invent because
	// the caller did not supply one (e.g. SetRadioRole creating a default AP).
	// It is otherwise unrecoverable, so the caller must be able to show it to
	// the operator. Empty when no key was generated.
	GeneratedKey string
}

// WifiService provides WiFi scanning, connection, and configuration.
type WifiService struct {
	uci                 uci.UCI
	ubus                ubus.Ubus
	reloader            WifiReloader
	applier             UCIApplyConfirm // optional; when set, use apply+confirm instead of any reload
	cmd                 CommandRunner
	priorityFile        string
	autoReconnectFile   string
	reconnectScript     string
	modeFile            string
	repeaterOptionsFile string
	guardDir            string
	// guardFallbackDir overrides the directory used when guardDir cannot be
	// created. Empty in production, which falls back to a directory under the
	// system temp dir. It is a field rather than an env lookup so a test can
	// exercise the fallback path hermetically: redirecting TMPDIR with t.Setenv
	// would mutate the whole process environment, and this package runs many
	// t.Parallel() tests that would then inherit (and lose) the temp dir.
	guardFallbackDir string

	// uciWriteMu serializes UCI write sequences (Set/AddSection/Commit/revert
	// against the process-global uci delta). Read-only paths never take it, so
	// status/scan requests stay concurrent.
	uciWriteMu sync.Mutex
}

// uciApplyConfigs is the list of configs copied for staged apply+confirm.
// Include the related network services that WiFi mutations can touch.
var uciApplyConfigs = []string{"wireless", "network", "system", "firewall", "dhcp"}

const defaultPriorityFile = "/etc/travo/wifi-priorities.json"
const defaultAutoReconnectFile = "/etc/travo/autoreconnect.json"
const defaultReconnectScript = "/etc/travo/wifi-reconnect.sh"
const defaultWifiModeFile = "/etc/travo/wifi-mode"
const defaultRepeaterOptionsFile = "/etc/travo/repeater-options.json"

// Crash guards live in /etc/trafo, the single directory every guard check
// and the redeploy recovery path look at (AGENTS.md, ADR 0003 §2). State
// files (aliases, priorities, repeater options) stay in /etc/travo.
const defaultGuardDir = "/etc/trafo"

// NewWifiService creates a new WifiService. Uses apply+confirm when applier is set (production),
// otherwise falls back to the (test-only) reloader.
func NewWifiService(u uci.UCI, ub ubus.Ubus, pw *auth.RootPassword) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: &NoopWifiReloader{}, applier: NewRealUCIApplyConfirm(ub, pw),
		cmd: &RealCommandRunner{}, priorityFile: defaultPriorityFile,
		autoReconnectFile: defaultAutoReconnectFile, reconnectScript: defaultReconnectScript,
		modeFile: defaultWifiModeFile, repeaterOptionsFile: defaultRepeaterOptionsFile,
		guardDir: defaultGuardDir,
	}
}

// NewWifiServiceWithReloader creates a WifiService with a custom reloader (for tests).
// Applier is left nil so Reload() is used.
func NewWifiServiceWithReloader(u uci.UCI, ub ubus.Ubus, r WifiReloader) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: defaultPriorityFile, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, guardDir: defaultGuardDir,
	}
}

// NewWifiServiceWithPriorityFile creates a WifiService with a custom priority file (for tests).
func NewWifiServiceWithPriorityFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, pf string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: pf, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, guardDir: defaultGuardDir,
	}
}

// NewWifiServiceForTesting creates a WifiService with all fields customizable (for tests).
func NewWifiServiceForTesting(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, guardDir: defaultGuardDir,
	}
}

// NewWifiServiceForTestingWithModeFile creates a WifiService with a custom mode file (for tests).
func NewWifiServiceForTestingWithModeFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile, modeFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: modeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, guardDir: defaultGuardDir,
	}
}

// testGuardDir returns a private guard directory so tests never touch /etc/travo.
func testGuardDir() string {
	dir, err := os.MkdirTemp("", "travo-guard-*")
	if err != nil {
		return filepath.Join(os.TempDir(), "travo-guard")
	}
	return dir
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
		radioMap, ok := radioData.(map[string]any)
		if !ok {
			continue
		}
		ifaces, ok := radioMap["interfaces"].([]any)
		if !ok {
			continue
		}
		for _, iface := range ifaces {
			ifaceMap, ok := iface.(map[string]any)
			if !ok {
				continue
			}
			config, ok := ifaceMap["config"].(map[string]any)
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

// ErrNoSTASection reports that no saved STA profile matches the requested SSID.
// It is deliberately distinguishable from a real failure (e.g. an unreadable
// UCI config): treating a read error as "not found" would silently create a
// duplicate profile instead of surfacing the failure.
var ErrNoSTASection = errors.New("no STA section found")

// findSTASectionBySSID returns the UCI section name of a saved STA profile
// matching ssid. It returns an error wrapping ErrNoSTASection when there is no
// match, and a plain error when the wireless config could not be read.
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
	return "", fmt.Errorf("%w for SSID %q", ErrNoSTASection, ssid)
}

// nextSTASectionName returns a unique UCI section name for a new STA profile (sta0, sta1, …).
// It returns an error when the section list cannot be read: falling back to a
// fixed name would let the caller `uci set` over an existing section, which
// silently rewrites that section's type and destroys a saved network.
func (w *WifiService) nextSTASectionName() (string, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("failed to get wireless sections: %w", err)
	}
	for i := 0; ; i++ {
		candidate := fmt.Sprintf("sta%d", i)
		if _, exists := sections[candidate]; !exists {
			return candidate, nil
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

// applyWireless applies committed UCI using apply+confirm when applier is set (same as LuCI),
// otherwise it uses the reloader seam (tests only). Use after Commit("wireless").
func (w *WifiService) applyWireless() error {
	if w.applier != nil {
		return w.applier.ApplyAndConfirm(uciApplyConfigs)
	}
	return w.reloader.Reload()
}

// lockUCIWrite serializes one UCI write sequence (Set/AddSection/Commit/revert)
// against the process-global uci delta in /tmp/.uci/<config>/changes: two
// concurrent mutations would otherwise interleave, commit each other's
// half-written options, and revert each other's rollback. Read-only paths never
// take this lock, so status and scan requests stay concurrent.
//
// The returned function releases the lock and must be deferred by the caller.
func (w *WifiService) lockUCIWrite() func() {
	w.uciWriteMu.Lock()
	return w.uciWriteMu.Unlock
}

// guardPath returns the crash-guard path for a feature inside the guard dir.
func (w *WifiService) guardPath(feature string) string {
	dir := w.guardDir
	if dir == "" {
		dir = defaultGuardDir
	}
	return filepath.Join(dir, feature+"-in-progress")
}

// resolveGuardDir returns the guard directory to use, falling back to a temp
// directory when the configured one is not writable (dev host, unit tests, or a
// broken installation). The guard is never skipped: a missing durable marker is
// exactly the case the operator has to hear about, so it is logged as an error.
func (w *WifiService) resolveGuardDir() string {
	dir := w.guardDir
	if dir == "" {
		dir = defaultGuardDir
	}
	mkErr := os.MkdirAll(dir, 0o750)
	if mkErr == nil {
		return dir
	}
	fallback := w.guardFallbackDir
	if fallback == "" {
		fallback = filepath.Join(os.TempDir(), "travo-guards")
	}
	log.Printf("ERROR: %s is not writable (%v); crash guards are being written to %s instead. A device-side power loss is NOT protected by a durable marker.", dir, mkErr, fallback)
	_ = os.MkdirAll(fallback, 0o750)
	return fallback
}

// writeCrashGuard creates /etc/travo/<feature>-in-progress before a dangerous
// live-state change (ADR 0003). The marker is removed by the caller only after
// the change completed successfully.
func (w *WifiService) writeCrashGuard(feature string) error {
	path := filepath.Join(w.resolveGuardDir(), feature+"-in-progress")
	if err := os.WriteFile(path, []byte("travo: "+feature+" in progress\n"), 0o600); err != nil {
		return fmt.Errorf("writing crash guard %s: %w", path, err)
	}
	return nil
}

// clearCrashGuard removes a crash guard after the operation succeeded.
func (w *WifiService) clearCrashGuard(feature string) {
	// Clear every directory the guard could have been written to. writeCrashGuard
	// uses resolveGuardDir(), which falls back to a temp directory when the
	// configured one is not writable; removing only the configured path left that
	// fallback guard behind forever, and a stale guard means "skip".
	// The configured dir is also removed on its own because the resolution can
	// differ between the write and the clear (e.g. the directory became writable).
	for _, dir := range w.guardDirs() {
		if err := os.Remove(filepath.Join(dir, feature+"-in-progress")); err != nil && !os.IsNotExist(err) {
			log.Printf("WARNING: removing crash guard %s/%s: %v", dir, feature, err)
		}
	}
}

// guardDirs returns the directories a crash guard for this service may live in,
// most likely first: the resolved directory (configured dir, or the temp
// fallback when it is not writable) and the configured directory itself.
func (w *WifiService) guardDirs() []string {
	configured := w.guardDir
	if configured == "" {
		configured = defaultGuardDir
	}
	resolved := w.resolveGuardDir()
	if resolved == configured {
		return []string{configured}
	}
	return []string{resolved, configured}
}

// ApplyWireless applies the current wireless (and related) UCI config via apply+confirm.
// Exported for use after EnsureAPRunning when fixes were applied so they take effect without reboot.
func (w *WifiService) ApplyWireless() error {
	return w.applyWireless()
}
