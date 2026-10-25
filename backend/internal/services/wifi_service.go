package services

import (
	"github.com/openwrt-travel-gui/backend/internal/guard"

	"errors"
	"fmt"
	"log"
	"math"
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
// It also SNAPSHOTS the named configs first, and restores that snapshot if fn
// fails. Both halves are needed, and they are not the same mechanism: the
// revert only drops the uncommitted delta, so a writer that already committed
// (every wireless mutator does, before stageWirelessApply) is not touched by
// it. The snapshot is the only thing that can put a committed config back.
//
// The snapshot is taken INSIDE the locks, before fn runs, so nothing can
// commit between the copy and the mutation.
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

	if w.applier != nil {
		if err := w.applier.Snapshot(configs); err != nil {
			return nil, err
		}
	}

	var res *WirelessApplyResult
	err := mutateUCI(w.uci, configs, func() error {
		var err error
		res, err = fn()
		return err
	})
	if err != nil {
		// fn failed with its changes committed and no apply session to time
		// out (a failed stageWirelessApply has none at all), so the previous
		// config goes back now.
		return nil, w.restoreAfterFailedMutation(err)
	}
	return res, nil
}

// restoreAfterFailedMutation puts the pre-mutation configs back after fn failed
// and keeps fn's error as the one the caller sees. A rollback that itself failed
// is stated in that error rather than logged and dropped: it is the case where
// the previous config is NOT on disk, and the operator has to hear about it.
//
// A missing pending snapshot is NOT that case, and is the one exception: it means
// the applier inside fn already owned and consumed the snapshot —
// `ApplyAndConfirm` (SwitchSTAToRadio) snapshots, restores on its own failure and
// discards on success — so there is nothing left to put back and turning that into
// an error would report a failure that did not happen.
func (w *WifiService) restoreAfterFailedMutation(cause error) error {
	if w.applier == nil {
		return cause
	}
	err := w.applier.Rollback("")
	if err == nil || errors.Is(err, ErrNoSnapshot) {
		return cause
	}
	return fmt.Errorf("%w (restoring the previous config also failed: %v)", cause, err)
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

	// ProbeBudgetSeconds is the wall-clock time a single ConfirmApply can block
	// in the worst case (see wirelessProbeBudgetSeconds). A client that keeps
	// re-POSTing confirm until the rollback deadline has no other way to know
	// that one of its attempts is still in flight, so a probe started late
	// lands after rpcd has already rolled back — and a good-but-slow config gets
	// reverted. Subtract this from the client's own deadline.
	ProbeBudgetSeconds int
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

	// arpFile is the neighbour table the caller classifier reads a client IP to
	// a MAC through. Overridable for tests, like the other file fields here.
	arpFile string

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
		arpFile: procNetARP,
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
		arpFile: procNetARP,
	}
}

// NewWifiServiceWithReloader creates a WifiService with a custom reloader (for tests).
// Applier is left nil so Reload() is used.
func NewWifiServiceWithReloader(u uci.UCI, ub ubus.Ubus, r WifiReloader) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: defaultPriorityFile, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, arpFile: procNetARP,
	}
}

// NewWifiServiceWithPriorityFile creates a WifiService with a custom priority file (for tests).
func NewWifiServiceWithPriorityFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, pf string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: &RealCommandRunner{},
		priorityFile: pf, autoReconnectFile: defaultAutoReconnectFile,
		reconnectScript: defaultReconnectScript, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, arpFile: procNetARP,
	}
}

// NewWifiServiceForTesting creates a WifiService with all fields customizable (for tests).
func NewWifiServiceForTesting(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: defaultWifiModeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, arpFile: procNetARP,
	}
}

// NewWifiServiceForTestingWithModeFile creates a WifiService with a custom mode file (for tests).
func NewWifiServiceForTestingWithModeFile(u uci.UCI, ub ubus.Ubus, r WifiReloader, cmd CommandRunner, pf, arFile, rsFile, modeFile string) *WifiService {
	return &WifiService{
		uci: u, ubus: ub, reloader: r, applier: nil, cmd: cmd,
		priorityFile: pf, autoReconnectFile: arFile,
		reconnectScript: rsFile, modeFile: modeFile,
		repeaterOptionsFile: defaultRepeaterOptionsFile, arpFile: procNetARP,
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

// stageWirelessApply commits the change (its caller does) and opens the apply
// window. If StartApply fails here the mutation has ALREADY committed and there
// is no session, so the only way back is the snapshot mutateWireless took before
// the mutation: mutateWireless restores it for every fn failure, including this
// one. Nothing extra to do here.
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
			ProbeBudgetSeconds:     wirelessProbeBudgetSeconds(),
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
// The proof is made HERE, on the device, not in the caller: docs/architecture/overview.md
// §3 and ADR 0002 §5 require confirm only after reachability is proven, and a
// client that POSTs confirm a millisecond after the apply (the normal case for an
// operator on Ethernet, where WiFi is exactly what is being reconfigured) would
// otherwise cancel the rollback of a config that never came up. A failed proof
// returns BEFORE applier.Confirm — and now rolls the previous config back by
// hand, because leaving it to rpcd's window restored the ALREADY-COMMITTED
// config instead (ADR 0002 §5).
func (w *WifiService) ConfirmApply(token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("apply token is required")
	}
	if w.applier == nil {
		return nil
	}
	if err := w.verifyAppliedWirelessUp(); err != nil {
		return w.refuseAndRollback(token, err)
	}
	if err := w.applier.Confirm(token); err != nil {
		return err
	}
	// After successful confirm, no reload is needed as apply+confirm already applied changes
	return nil
}

// refuseAndRollback follows a refused proof with the action it implies: the
// pre-mutation config goes back on disk NOW, before the operator is told. The
// probe error stays the one the caller unwraps — its message prefix is what the
// frontend matches on — and a failed restore is appended rather than dropped,
// because that is the case where the previous config is not on disk.
//
// The apply session is deliberately NOT confirmed here: Rollback cancels rpcd's
// window itself, so the window cannot fire later and overwrite the restore.
func (w *WifiService) refuseAndRollback(token string, probeErr error) error {
	if err := w.applier.Rollback(token); err != nil {
		return fmt.Errorf("%w (rolling back to the previous config also failed: %v)", probeErr, err)
	}
	return probeErr
}

// ErrWirelessNotUp reports that the access points the just-applied wireless
// config enables are not running. ConfirmApply surfaces it instead of
// cancelling rpcd's rollback window, so a change that cannot bring WiFi up
// reverts to the previous config.
var ErrWirelessNotUp = errors.New(
	"wireless apply not verified: enabled access point(s) did not come up")

const (
	// wirelessConfirmAttempts / wirelessConfirmDelay bound the wait for the
	// interfaces the applied config enables to come up. They are sized from what
	// the device actually takes, not from a guess: after `uci apply` netifd
	// restarts the radios and hostapd has to finish ACS before phy1-ap0 links
	// up, which logread puts at ~10 s.
	//
	// 3 probes x 5 s waits 10 s — the measured link-up time — and the number of
	// attempts is deliberately small: every attempt spends ubus round-trips
	// (see wirelessProbeMaxUbusCalls), and the budget the client subtracts from
	// its own deadline has to cover those too. 8 probes x 2 s covered the same
	// window with 8 x 4 = 32 extra round-trips that the published budget did not
	// count, which put the client's last probe half a second from rpcd's
	// rollback. Fewer, longer waits cover the same device latency for a fraction
	// of the round-trip cost.
	wirelessConfirmAttempts = 3
	wirelessConfirmDelay    = 5 * time.Second

	// wirelessProbeRoundTrip is what ONE ubus round-trip is budgeted at inside
	// the published probe budget. Measured on the device
	// (192.168.1.1, OpenWrt 25.12.3): `network.wireless status` answers in
	// ~4 ms and `network.device status` in ~5 ms, including spawning the ubus
	// client. 100 ms is a ~20x allowance for netifd answering from a busy event
	// loop while it is restarting the radios.
	wirelessProbeRoundTrip = 100 * time.Millisecond

	// wirelessProbeMaxUbusCalls is how many ubus calls one attempt is budgeted
	// at: the wireless status read, plus one `network.device status` per
	// expected interface (the per-interface fallback). Four expected interfaces
	// is more than any layout this service produces (main AP + guest AP + uplink
	// STA). A config with more can still be proven, it just spends more time
	// than the published budget says — and it fails closed, never open.
	wirelessProbeMaxUbusCalls = 5
)

// wirelessProbeSafetyMarginSeconds is the room the CLIENT reserves on top of the
// probe budget: PROBE_SAFETY_MARGIN_MS / 1000 in
// frontend/src/lib/wifi-apply.ts, which is the only place that reserves it. It
// used to exist here as well (as 1) while the client used 500 and the comment
// said "half a second"; the Go gate in wifi_service_test.go now states it once,
// next to the assertion that uses it, and
// TestClientProbeSafetyMarginMatchesTheGoGate fails when the two sides drift.
const wirelessProbeSafetyMarginSeconds = 2

// wirelessRetrySleep is the wait between two confirm probes. It is a var only so
// tests that deliberately exhaust the retry budget do not have to spend it.
var wirelessRetrySleep = time.Sleep

// wirelessProbeBudgetSeconds is the longest a single ConfirmApply can block: the
// first probe is immediate, so only the sleeps between the retries cost time —
// plus the ubus round-trips every attempt makes, which the old version of this
// function did not count at all (it published the sleep total as if a probe cost
// nothing but sleeping, and the last probe then landed 0.5 s before rpcd rolled
// back).
func wirelessProbeBudgetSeconds() int {
	sleeps := (wirelessConfirmAttempts - 1) * wirelessConfirmDelay
	calls := time.Duration(wirelessProbeMaxUbusCalls) *
		wirelessProbeRoundTrip * wirelessConfirmAttempts
	return int(math.Ceil((sleeps + calls).Seconds()))
}

// verifyAppliedWirelessUp proves that every interface the applied config says
// should be running is actually up. It is the confirmation-time counterpart of
// GetHealth: UCI says which AP and uplink-STA sections are enabled, netifd says
// whether they exist and are up.
//
// The uplink STA is proven too, not just the access points. Client mode disables
// every AP, so a client-mode apply used to have an empty proof and confirm
// unconditionally — a STA on the wrong band or with the wrong key then left the
// operator with no uplink and no way back over WiFi, with rpcd's rollback
// cancelled. Probing the STA also closes the same hole in repeater mode, where
// the downlink AP can come up while the uplink never associates.
//
// A config that leaves neither an AP nor an uplink STA enabled has nothing to
// prove — "WiFi off" is a change the operator is allowed to make and
// reachability is exactly what the confirm request itself demonstrates.
func (w *WifiService) verifyAppliedWirelessUp() error {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return fmt.Errorf("reading wireless sections: %w", err)
	}
	wantAPs, err := enabledAPSections(sections)
	if err != nil {
		return err
	}
	wantSTAs := enabledWwanSTAs(sections)
	if len(wantAPs) == 0 && len(wantSTAs) == 0 {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < wirelessConfirmAttempts; attempt++ {
		// The first probe is immediate and costs a single ubus round-trip, so
		// the common "already up" case answers in well under a second and only
		// the slow case spends the retry budget.
		if attempt > 0 {
			wirelessRetrySleep(wirelessConfirmDelay)
		}
		lastErr = w.appliedWirelessUp(sections, wantAPs, wantSTAs)
		if lastErr == nil {
			return nil
		}
		// A payload this probe cannot read will not become readable by waiting:
		// report it straight away so a netifd schema change is diagnosable as
		// one instead of burning the budget and then reading like a dead AP.
		// A status read that FAILED is the opposite case — the socket was gone,
		// netifd was restarting the object, the call timed out — and is very
		// often readable a moment later, so it keeps the retry budget.
		if isUnreadablePayload(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

// ErrWirelessUnverifiable reports that this probe could not READ the answer:
// `ubus call network.wireless status` (or the `network.device status` fallback)
// either failed at the transport level or answered in a shape this build cannot
// read — a radio without an "up" flag, an interface without an "ifname", an
// entry that is not an object at all. It covers both, because to the operator
// they are one condition: Travo does not know whether the new settings came up.
//
// The transport failure used to be reported as ErrWirelessNotUp, which put
// "wireless apply not verified: ..." in front of the operator while rpcd's
// rollback window was still open and the new config was still live — and the
// frontend, which substring-matches that prefix, turned it into "so it rolled
// back to the previous settings". rpcd has rolled back nothing at that point.
//
// It is deliberately NOT ErrWirelessNotUp, and its message must not start with
// the same words either: the frontend tells the two apart by substring, so a
// shared prefix made "I cannot read the answer" say on screen that the router
// had already rolled back — while rpcd's rollback window was still open.
// "I cannot read the answer" and "the access point did not come up" call for
// different responses from an operator and from whoever is on call, and
// collapsing the first into the second is what let a probe that could never pass
// on real hardware look like operator error. Both still keep the rollback armed:
// the apply is not confirmed either way.
var ErrWirelessUnverifiable = errors.New(
	"wireless apply could not be verified: network.wireless status could not be read")

// errWirelessStatusUnread marks the subset of ErrWirelessUnverifiable where the
// question was never answered — the ubus call itself failed. Only this subset is
// worth retrying: a payload in an unknown shape will not become readable.
var errWirelessStatusUnread = errors.New("the ubus call did not complete")

// isUnreadablePayload reports whether err is an unreadable-ANSWER failure, as
// opposed to an unreadable-QUESTION one.
func isUnreadablePayload(err error) bool {
	return errors.Is(err, ErrWirelessUnverifiable) && !errors.Is(err, errWirelessStatusUnread)
}

// appliedWirelessUp cross-checks netifd's view of the wireless interfaces
// against what the applied config enables. A section counts as up only when BOTH
// hold:
//
//  1. netifd LISTS it (the section is present under a radio's "interfaces", so
//     netifd created it), and
//  2. either the owning radio is up and settled (not pending, not
//     retry_setup_failed) AND it carries no other expected interface, or the
//     interface itself answers
//     `ubus call network.device status {"name":"<ifname>"}` with
//     present && up && carrier.
//
// Why the radio is not proof for a shared radio: netifd reports liveness per
// RADIO, so on a radio hosting two wanted interfaces (guest AP beside the main
// AP — SetGuestWifi only excludes the uplink radio) the first one to come up
// settles the radio and used to prove the second one too. The probe then
// confirmed and cancelled the rollback with a dead access point, which is the
// one direction that silently takes the operator's SSID away. The per-device
// answer is therefore the authority whenever a radio carries more than one
// expected interface.
//
// Why per-radio: on OpenWrt 25.12.3 / netifd 2026.02.26-r1 liveness is reported
// per RADIO ("up", "pending", "retry_setup_failed", "autostart"). A per-interface
// entry carries exactly {section, config, ifname, vlans, stations} — there is no
// per-interface "up" and no "config_path". Reading iface["up"] therefore answers
// false for every interface on every device, which is what made ConfirmApply
// refuse to confirm anything. Why the device fallback: a radio can read up while
// the interface on it is still being set up, and carrier is the only per-device
// liveness netifd exposes.
//
// Deliberately not used as liveness signals: stations[] (empty for a healthy
// zero-client AP, and structurally always empty for mode=sta) and iwinfo
// assoclist. See ADR 0002 §5.
//
// It fails closed: an interface that cannot be observed counts as down, because
// the whole point is to keep the rollback armed when the answer is unknown. One
// wireless status read answers every expectation, so a settled single-interface
// radio costs no extra ubus round-trip; the fallback is consulted for an
// unsettled radio or one carrying several expected interfaces, and its cost is
// budgeted (see wirelessProbeMaxUbusCalls).
// The kinds an expected interface can be, held as values rather than inline
// strings because the uplink kind also decides HOW an interface may be proven:
// see appliedWirelessUp.
const (
	expectedAPKind  = "access point"
	expectedSTAKind = "uplink STA"
)

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

// crashGuard returns the guard for feature in the service's guard dir.
func (w *WifiService) crashGuard(feature string) guard.Guard {
	return guard.In(w.guardDir, feature)
}
