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
	"time"

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

// uciConfigLocks maps a config name to its lock, so a writer of `firewall` and
// a writer of `dhcp` never block each other.
var uciConfigLocks sync.Map // config name -> *sync.Mutex

// lockUCIConfigs locks each named config and returns a single unlock func.
//
// The names are sorted before locking, which makes the acquisition order GLOBAL
// and identical on every path. That is what makes nesting safe: with a fixed
// order, no two goroutines can hold overlapping sets in opposite orders, so the
// wait-for graph is acyclic and deadlock is impossible. Without the sort, two
// flows that each need {network, firewall} but list them in opposite order
// would deadlock the moment they met — and on a router that means every other
// endpoint that touches a config also stops responding, because they all queue
// on the same per-config locks.
//
// The locks are still NOT reentrant: a flow that holds a config must not ask for
// it again. Helpers called from inside a transaction therefore have a lock-free
// core (…Locked) that the transaction calls, while the thin exported wrapper
// takes the lock for callers that enter cold. See setupWireGuardFirewallLocked.
func lockUCIConfigs(configs ...string) func() {
	// Sorted + de-duplicated: acquiring the same mutex twice on one path would
	// self-deadlock, and mutateUCI callers legitimately pass a set.
	names := append([]string(nil), configs...)
	slices.Sort(names)
	locks := make([]*sync.Mutex, 0, len(names))
	prev := ""
	for i, c := range names {
		if i > 0 && c == prev {
			continue
		}
		prev = c
		v, _ := uciConfigLocks.LoadOrStore(c, &sync.Mutex{})
		m := v.(*sync.Mutex)
		m.Lock()
		locks = append(locks, m)
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
}

// revertUCIConfig drops the staged UCI delta for each config that has one.
//
// The uci CLI keeps uncommitted changes in the process-global
// /tmp/.uci/<config>/changes file, so a write sequence that fails half-way
// must be reverted: otherwise a later, unrelated `uci commit <config>` (a WAN
// save, a DHCP change, …) silently persists the abandoned delta.
//
// The caller must already hold the config lock for every name it passes, since
// the revert discards whatever *anyone* has staged for that config.
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

// mutateWireless runs a wireless write sequence with the wireless write lock
// and the shared per-config locks held, reverting every named config if fn
// fails.
//
// Passing the config list ONCE is the point. The uci CLI keeps uncommitted
// changes in the process-global /tmp/.uci/<config>/changes file, so a config
// left staged by a failed write is committed by the next unrelated writer of
// that config — putting a change the API reported as failed into the running
// config. Deriving the lock set and the revert set from the same list is what
// keeps them in agreement.
//
// Lock order is always: uciWriteMu, then the config locks. Neither is
// reentrant, so mutateWireless must not be nested and no fn passed to it may
// take either lock itself. Callers that only read UCI should not use this.
func (w *WifiService) mutateWireless(configs []string, fn func() (*WirelessApplyResult, error)) (*WirelessApplyResult, error) {
	defer w.lockUCIWrite()()

	var res *WirelessApplyResult
	err := mutateUCI(w.uci, configs, func() error {
		var err error
		res, err = fn()
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// withConfigLocks holds the named configs' locks for the duration of fn, in the
// same globally ordered way, but does NOT revert on failure.
//
// For the writers that shell out to `uci` instead of going through the UCI
// interface — AdGuardService and USBTetheringService do this — and therefore have
// no uci.UCI to revert through. Taking the locks is still the point: without them
// those writers interleave with the ones that DO revert, and the abandoned delta
// gets committed by whoever commits next. The missing half is stated rather than
// hidden: a failure in fn leaves its staged delta in place, and the next writer of
// that config will commit it. Passing a uci.UCI and using mutateUCI where one is
// available is preferred; this exists for the services that genuinely have none.
func withConfigLocks(configs []string, fn func() error) error {
	defer lockUCIConfigs(configs...)()
	return fn()
}

// mutateUCI is mutateWireless for the other services: it holds the named
// configs' locks for the duration of fn and reverts all of them if fn fails.
// Same one-list rule, same reason — see mutateWireless.
func mutateUCI(u uci.UCI, configs []string, fn func() error) error {
	defer lockUCIConfigs(configs...)()

	if err := fn(); err != nil {
		revertUCIConfig(u, configs...)
		return err
	}
	return nil
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
	// crontabFile and scheduleFile override the WiFi on/off schedule's crontab
	// and state-JSON paths (tests only), like the fields above.
	crontabFile  string
	scheduleFile string
	// toggleScriptPath overrides where the generated toggle helper is written
	// (tests only).
	toggleScriptPath string
	guardDir         string

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

// NewWifiService creates a new WifiService. Uses apply+confirm when applier is set (production),
// otherwise falls back to the (test-only) reloader.
func NewWifiService(u uci.UCI, ub ubus.Ubus, pw *auth.RootPassword) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: &NoopWifiReloader{}, applier: NewRealUCIApplyConfirm(ub, pw),
		cmd: &RealCommandRunner{}, priorityFile: defaultPriorityFile,
		autoReconnectFile: defaultAutoReconnectFile, reconnectScript: defaultReconnectScript,
		modeFile: defaultWifiModeFile, repeaterOptionsFile: defaultRepeaterOptionsFile,
		guardDir: crashGuardDir,
	}
}

// NewWifiServiceWithApplier wires an explicit apply/confirm step into a
// WifiService. Production uses NewWifiService (the real rpcd applier); this
// constructor is the seam other packages' tests use to drive the wireless
// mutation envelope (token, rollback timeout, generated key) that only exists
// when an applier is configured.
func NewWifiServiceWithApplier(u uci.UCI, ub ubus.Ubus, applier UCIApplyConfirm) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: &NoopWifiReloader{}, applier: applier,
		cmd: &RealCommandRunner{}, priorityFile: defaultPriorityFile,
		autoReconnectFile: defaultAutoReconnectFile, reconnectScript: defaultReconnectScript,
		modeFile: defaultWifiModeFile, repeaterOptionsFile: defaultRepeaterOptionsFile,
		guardDir: crashGuardDir,
	}
}

// NewWifiServiceWithReloader creates a WifiService with a custom reloader (for tests).
// Applier is left nil so Reload() is used.
func NewWifiServiceWithReloader(u uci.UCI, ub ubus.Ubus, r WifiReloader) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: defaultPriorityFile, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, guardDir: crashGuardDir,
	}
}

// NewWifiServiceWithPriorityFile creates a WifiService with a custom priority file (for tests).
func NewWifiServiceWithPriorityFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, pf string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: pf, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, guardDir: crashGuardDir,
	}
}

// NewWifiServiceForTesting creates a WifiService with all fields customizable (for tests).
func NewWifiServiceForTesting(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, guardDir: crashGuardDir,
	}
}

// NewWifiServiceForTestingWithModeFile creates a WifiService with a custom mode file (for tests).
func NewWifiServiceForTestingWithModeFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile, modeFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: modeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, guardDir: crashGuardDir,
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

// ConfirmApply finalizes a staged wireless apply once the router has proven it
// is still reachable on the new settings.
//
// The proof is made HERE, on the device, not in the caller: docs/architecture.md
// §3 and ADR 0002 §5 require confirm only after reachability is proven, and a
// client that POSTs confirm a millisecond after the apply (the normal case for an
// operator on Ethernet, where WiFi is exactly what is being reconfigured) would
// otherwise cancel the rollback of a config that never came up. A failed proof
// returns BEFORE applier.Confirm, so the apply session stays open and rpcd rolls
// the wireless config back when the window expires.
func (w *WifiService) ConfirmApply(token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("apply token is required")
	}
	if w.applier == nil {
		return nil
	}
	if err := w.verifyAppliedWirelessUp(); err != nil {
		return err
	}
	if err := w.applier.Confirm(token); err != nil {
		return err
	}
	// After successful confirm, no reload is needed as apply+confirm already applied changes
	return nil
}

// ErrWirelessNotUp reports that the access points the just-applied wireless
// config enables are not running. ConfirmApply surfaces it instead of
// cancelling rpcd's rollback window, so a change that cannot bring WiFi up
// reverts to the previous config.
var ErrWirelessNotUp = errors.New(
	"wireless apply not verified: enabled access point(s) did not come up")

const (
	// wirelessConfirmAttempts / wirelessConfirmDelay bound the wait for the
	// access points to come up. netifd needs a moment to re-associate after
	// `uci apply`, and the whole wait (3 probes, 2 s apart = 4 s) stays far
	// inside rpcd's 30 s rollback window, so a confirm that fails here still
	// leaves the window time to do its job.
	wirelessConfirmAttempts = 3
	wirelessConfirmDelay    = 2 * time.Second
)

// verifyAppliedWirelessUp proves that every access point the applied config says
// should be running is actually up. It is the confirmation-time counterpart of
// GetHealth: UCI says which AP sections are enabled, netifd says whether they
// exist and are up.
//
// A config that leaves no access point enabled has nothing to prove — client mode
// and "WiFi off" are changes the operator is allowed to make and reachability is
// exactly what the confirm request itself demonstrates.
func (w *WifiService) verifyAppliedWirelessUp() error {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return fmt.Errorf("reading wireless sections: %w", err)
	}
	want := enabledAPSections(sections)
	if len(want) == 0 {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < wirelessConfirmAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(wirelessConfirmDelay)
		}
		lastErr = w.apInterfacesUp(sections, want)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

// enabledAPSections returns the wifi-iface sections the given wireless config
// says should be running: mode=ap, not disabled, on a radio that is not
// disabled. An AP on a radio the config switches off is deliberately down.
func enabledAPSections(sections map[string]map[string]string) []string {
	var names []string
	for name, opts := range sections {
		if opts["mode"] != "ap" || opts["disabled"] == "1" {
			continue
		}
		device := opts["device"]
		if device == "" {
			continue
		}
		if radio, ok := sections[device]; ok && radio["disabled"] == "1" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// apInterfacesUp cross-checks netifd's view of the wireless interfaces against
// the AP sections the config enables: each expected section must appear in
// `ubus call network.wireless status` with up=true. This is the access-point
// counterpart of GetHealth's iwinfo-vs-netifd cross-check — and it fails closed:
// an interface that cannot be observed is treated as down, because the whole
// point is to keep the rollback armed when the answer is unknown.
func (w *WifiService) apInterfacesUp(sections map[string]map[string]string, want []string) error {
	resp, err := w.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		return fmt.Errorf("%w: cannot read network.wireless status: %v", ErrWirelessNotUp, err)
	}
	down := make(map[string]bool, len(want))
	for _, name := range want {
		down[name] = true
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
			name, _ := ifaceMap["section"].(string)
			if _, wanted := down[name]; !wanted {
				// netifd did not report the section: fall back to mode+ssid so a
				// build without "section" does not read as every AP being down.
				cfg, _ := ifaceMap["config"].(map[string]any)
				mode, _ := cfg["mode"].(string)
				ssid, _ := cfg["ssid"].(string)
				for _, candidate := range want {
					opts := sections[candidate]
					if ssid != "" && opts["ssid"] == ssid && opts["mode"] == mode {
						name = candidate
						break
					}
				}
			}
			if _, wanted := down[name]; !wanted {
				continue
			}
			if up, _ := ifaceMap["up"].(bool); up {
				delete(down, name)
			}
		}
	}
	if len(down) == 0 {
		return nil
	}
	missing := make([]string, 0, len(down))
	for name := range down {
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return fmt.Errorf("%w: sections %s", ErrWirelessNotUp, strings.Join(missing, ","))
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
		dir = crashGuardDir
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
		dir = crashGuardDir
	}
	mkErr := os.MkdirAll(dir, 0o750)
	if mkErr == nil {
		return dir
	}
	fallback := filepath.Join(os.TempDir(), "travo-guards")
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
		configured = crashGuardDir
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
