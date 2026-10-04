package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

const (
	failoverConfigPath       = "/etc/travo/failover.json"
	failoverGuardPath        = crashGuardDir + "/failover-in-progress"
	failoverBackupPath       = "/etc/travo/failover-mwan3-backup.json"
	mwan3InitScriptPath      = "/etc/init.d/mwan3"
	mwan3ConfigName          = "mwan3"
	failoverPolicySection    = "travo_failover"
	failoverRuleSection      = "travo_default_v4"
	failoverTickerInterval   = 10 * time.Second
	failbackHoldDownDuration = 30 * time.Second
)

type failoverConfigFile struct {
	Enabled    bool                        `json:"enabled"`
	Candidates []models.FailoverCandidate  `json:"candidates"`
	Health     models.FailoverHealthConfig `json:"health"`
}

// errApplyRollingBack is returned when a save lands inside an rpcd rollback
// window that a previous apply left open. The change is NOT live, and rpcd is
// still armed to restore the pre-apply config underneath it, so the caller must
// report the failure rather than acknowledging a save that will be reverted.
var errApplyRollingBack = errors.New("an mwan3 apply from a previous save is still rolling back; retry in a few seconds")

// rollbackOrKeepGuard restores the previous mwan3/network sections after a
// failed apply, and clears the crash guard — but only when doing so leaves the
// device in a state we can vouch for.
//
// The guard is kept in two cases:
//
//  1. The restore itself failed. Then the running config is unknown. Same
//     contract as vpn_service.go.
//
//  2. rpcd still has an armed rollback timer from the apply that just failed.
//     Confirming the restore's own session does NOT cancel that timer, so
//     ~30s later rpcd would drop the service's own post-restore snapshot back
//     in — a config this service already judged bad.
//
// Either way the guard stays until the window expires and a later save
// succeeds. That is the documented contract (ADR 0003): an unresolved rollback
// is what a human or a redeploy should clear, and deploy-local.sh and
// install.sh both clear it.
func (s *FailoverService) rollbackOrKeepGuard() {
	// Capture the pending state BEFORE the restore runs. The restore issues its
	// own rpcd apply and clears pendingApplySession on success, so evaluating it
	// afterwards can never observe the window the failed apply left open — and
	// the guard would be dropped on the one path it exists for.
	rollbackPending := s.rollbackStillPending()
	pendingSession := s.pendingApplySession
	if err := s.restoreManagedSections(); err != nil {
		log.Printf("failover: rollback failed, crash guard kept at %s: %v", s.guardPath, err)
		return
	}
	if rollbackPending {
		log.Printf("failover: restored, but rpcd rollback %s was still armed; crash guard kept at %s",
			pendingSession, s.guardPath)
		return
	}
	if err := os.Remove(s.guardPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("failover: remove crash guard %s: %v", s.guardPath, err)
	}
}

// rollbackStillPending reports whether an rpcd session is still armed to roll
// the config back on its own timer.
func (s *FailoverService) rollbackStillPending() bool {
	return s.pendingApplySession != "" && time.Now().Before(s.pendingApplyDeadline)
}

// rollbackGracePeriod is how long after the rpcd rollback timeout a pending
// apply is still considered in flight.
const rollbackGracePeriod = time.Duration(uciApplyRollbackTimeout)*time.Second + 2*time.Second

type FailoverService struct {
	uci        uci.UCI
	ubus       ubus.Ubus
	networkSvc *NetworkService
	cmd        CommandRunner
	applier    UCIApplyConfirm

	configPath string
	guardPath  string
	backupPath string
	initScript string
	alertSvc   *AlertService
	mu         sync.RWMutex
	// applyMu serializes the whole live-state change (guard write, backup,
	// mwan3 apply, verify, guard removal) so a concurrent SetConfig or a
	// concurrent Start() monitor can never interleave with it. It is separate
	// from mu (events/lastActive) so the monitor never deadlocks against it.
	applyMu sync.Mutex
	// pendingApplySession is the rpcd session of an apply that was started but
	// not confirmed, so its rollback window is still open. rpcd allows only one
	// pending rollback at a time and rejects a second rollback-enabled apply
	// with "permission denied" until the first resolves, so the session has to be
	// tracked: while it is set, the open window is already restoring the
	// pre-apply config, and starting another apply would fail (and would fail
	// confusingly, in whichever feature happened to ask next).
	//
	// pendingApplyDeadline is when that window has expired. rpcd rolls the
	// session back on its own timer and nothing calls us when it does, so the
	// deadline is what clears the record; without it the flag would survive the
	// rollback and every later apply would be skipped for the life of the
	// process. Both fields are guarded by applyMu, which every caller of
	// stagedApplyMwan3 already holds.
	pendingApplySession  string
	pendingApplyDeadline time.Time
	events               []models.FailoverEvent
	lastActive           string
	stopCh               chan struct{}
	stopOnce             sync.Once
	onlineSince          map[string]time.Time
}

func NewFailoverService(u uci.UCI, ub ubus.Ubus, networkSvc *NetworkService, pw *auth.RootPassword) *FailoverService {
	return &FailoverService{
		uci:         u,
		ubus:        ub,
		networkSvc:  networkSvc,
		cmd:         &RealCommandRunner{},
		applier:     NewRealUCIApplyConfirm(ub, pw),
		configPath:  failoverConfigPath,
		guardPath:   failoverGuardPath,
		backupPath:  failoverBackupPath,
		initScript:  mwan3InitScriptPath,
		events:      make([]models.FailoverEvent, 0, 10),
		stopCh:      make(chan struct{}),
		onlineSince: make(map[string]time.Time),
	}
}

func NewFailoverServiceWithRunner(u uci.UCI, ub ubus.Ubus, networkSvc *NetworkService, cmd CommandRunner, applier UCIApplyConfirm, configPath string) *FailoverService {
	return &FailoverService{
		uci:         u,
		ubus:        ub,
		networkSvc:  networkSvc,
		cmd:         cmd,
		applier:     applier,
		configPath:  configPath,
		guardPath:   filepath.Join(filepath.Dir(configPath), "failover-in-progress"),
		backupPath:  filepath.Join(filepath.Dir(configPath), "failover-mwan3-backup.json"),
		initScript:  mwan3InitScriptPath,
		events:      make([]models.FailoverEvent, 0, 10),
		stopCh:      make(chan struct{}),
		onlineSince: make(map[string]time.Time),
	}
}

func (s *FailoverService) SetAlertService(alertSvc *AlertService) {
	s.alertSvc = alertSvc
}

func (s *FailoverService) Start() {
	ticker := time.NewTicker(failoverTickerInterval)
	defer ticker.Stop()

	// A stuck guard disables failover with no other symptom, so say so once
	// instead of silently skipping forever (band-switching logs the same case).
	// ADR 0003 §1.2.
	if _, err := os.Stat(s.guardPath); err == nil {
		log.Printf("failover: crash guard found at %s — automatic switching is disabled; remove it or redeploy to re-enable", s.guardPath)
	}

	for {
		if _, err := os.Stat(s.guardPath); err == nil {
			select {
			case <-ticker.C:
				continue
			case <-s.stopCh:
				return
			}
		}
		s.updateOnlineTimestamps()
		s.observeActiveChange()
		select {
		case <-ticker.C:
		case <-s.stopCh:
			return
		}
	}
}

func (s *FailoverService) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *FailoverService) GetConfig() (models.FailoverConfig, error) {
	cfgFile, err := s.loadConfigFile()
	if err != nil {
		return models.FailoverConfig{}, err
	}
	cfg, err := s.buildConfig(cfgFile)
	if err != nil {
		return models.FailoverConfig{}, err
	}
	return cfg, nil
}

func (s *FailoverService) GetEvents() []models.FailoverEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.FailoverEvent, len(s.events))
	copy(out, s.events)
	slices.Reverse(out)
	return out
}

// SetConfig validates, backs up and applies a new failover configuration. The
// whole live-state sequence is serialized: two concurrent saves would otherwise
// interleave their guard file, backup and mwan3 apply steps.
//
// The whole sequence runs inside mutateUCI over mwan3UCIConfigs, so the
// `network` and `mwan3` config locks are held across the blocking work —
// including the rpcd apply, whose snapshot/reload races every other writer of
// `network` — and the staged /tmp/.uci delta is dropped if anything fails
// (ADR 0010). Reads inside (verifyApply) take no lock of their own.
func (s *FailoverService) SetConfig(cfg models.FailoverConfig) error {
	if err := s.validateConfig(cfg); err != nil {
		return err
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	return mutateUCI(s.uci, mwan3UCIConfigs, func() error {
		return s.setConfigLocked(cfg)
	})
}

// setConfigLocked is SetConfig's body; the caller holds applyMu and the
// `network` + `mwan3` config locks.
func (s *FailoverService) setConfigLocked(cfg models.FailoverConfig) error {
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0750); err != nil {
		return fmt.Errorf("create failover config dir: %w", err)
	}
	if err := s.backupManagedSections(); err != nil {
		return err
	}

	cfgFile := failoverConfigFile{
		Enabled:    cfg.Enabled,
		Candidates: make([]models.FailoverCandidate, len(cfg.Candidates)),
		Health:     normalizeHealth(cfg.Health),
	}
	copy(cfgFile.Candidates, cfg.Candidates)
	if err := s.saveConfigFile(cfgFile); err != nil {
		// Nothing has been mutated yet — no backup restore is owed, and running
		// one here meant an ENOSPC on /etc/trafo produced a full mwan3 delete +
		// restore + rpcd apply with no crash guard at all, for a change that
		// never started.
		return err
	}
	if err := os.WriteFile(s.guardPath, []byte(time.Now().Format(time.RFC3339Nano)), 0600); err != nil {
		return fmt.Errorf("write failover guard: %w", err)
	}
	if err := s.applyManagedConfig(cfgFile); err != nil {
		s.rollbackOrKeepGuard()
		return err
	}
	if err := s.verifyApply(cfgFile); err != nil {
		s.rollbackOrKeepGuard()
		return err
	}
	if err := os.Remove(s.guardPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove failover guard: %w", err)
	}
	return nil
}

func (s *FailoverService) observeActiveChange() {
	cfg, err := s.GetConfig()
	if err != nil {
		return
	}
	active := cfg.ActiveInterface
	if active == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastActive == "" {
		s.lastActive = active
		return
	}
	if s.lastActive == active {
		return
	}
	event := models.FailoverEvent{
		FromInterface: s.lastActive,
		ToInterface:   active,
		Timestamp:     time.Now().UnixMilli(),
		Reason:        "active_uplink_changed",
	}
	s.lastActive = active
	s.events = append(s.events, event)
	if len(s.events) > 20 {
		s.events = s.events[len(s.events)-20:]
	}
	if s.alertSvc != nil {
		s.alertSvc.Publish(
			"connection_failover",
			fmt.Sprintf("Connection failover switched from %s to %s", event.FromInterface, event.ToInterface),
			"warning",
		)
	}
}

func (s *FailoverService) buildConfig(cfgFile failoverConfigFile) (models.FailoverConfig, error) {
	serviceInstalled := s.serviceInstalled()
	networkStatus, err := s.networkSvc.GetNetworkStatus()
	if err != nil {
		return models.FailoverConfig{}, err
	}
	candidates := s.discoverCandidates(networkStatus, cfgFile)
	active := ""
	if serviceInstalled && cfgFile.Enabled {
		active = s.computeActiveInterface(candidates)
	}

	var lastEvent *models.FailoverEvent
	events := s.GetEvents()
	if len(events) > 0 {
		lastEvent = &events[0]
	}

	return models.FailoverConfig{
		Available:         true,
		ServiceInstalled:  serviceInstalled,
		Enabled:           cfgFile.Enabled,
		ActiveInterface:   active,
		Candidates:        candidates,
		Health:            normalizeHealth(cfgFile.Health),
		LastFailoverEvent: lastEvent,
	}, nil
}

func (s *FailoverService) discoverCandidates(networkStatus models.NetworkStatus, cfgFile failoverConfigFile) []models.FailoverCandidate {
	known := map[string]models.FailoverCandidate{}
	trackerStates := s.readTrackerStates()
	for _, iface := range networkStatus.Interfaces {
		switch iface.Type {
		case "wan":
			known[iface.Name] = newCandidate(iface.Name, "Ethernet WAN", models.FailoverCandidateKindEthernet, iface.IsUp)
		case "wifi":
			known[iface.Name] = newCandidate(iface.Name, "WiFi uplink", models.FailoverCandidateKindWiFi, iface.IsUp)
		case "usb":
			known[iface.Name] = newCandidate(iface.Name, "USB tether", models.FailoverCandidateKindUSB, iface.IsUp)
		}
	}
	if _, ok := known["wwan"]; !ok && s.hasWirelessStation() {
		known["wwan"] = newCandidate("wwan", "WiFi uplink", models.FailoverCandidateKindWiFi, false)
		known["wwan"] = markUnavailable(known["wwan"])
	}
	s.addDiscoveredUSBNetworkCandidates(known)
	for _, saved := range cfgFile.Candidates {
		if _, ok := known[saved.InterfaceName]; !ok {
			saved.Available = false
			saved.IsUp = false
			saved.TrackingState = models.FailoverTrackingStateNotAvailable
			known[saved.InterfaceName] = saved
		}
	}

	// Iterate the candidates in a stable order. Ranging over the map directly
	// assigned fallback priorities in Go's randomised map order, so every
	// candidate that is not in the saved config came back with a different
	// priority on each call. GetConfig() is polled every 10s and by the UI, and
	// computeActiveInterface picks the active link in priority order, so the
	// reported active interface could flip between equally-ranked links for no
	// reason.
	names := make([]string, 0, len(known))
	for name := range known {
		names = append(names, name)
	}
	slices.Sort(names)

	candidates := make([]models.FailoverCandidate, 0, len(known))
	for _, name := range names {
		candidate := known[name]
		for _, saved := range cfgFile.Candidates {
			if saved.InterfaceName == candidate.InterfaceName {
				candidate.Enabled = saved.Enabled
				candidate.Priority = saved.Priority
			}
		}
		if candidate.Priority == 0 {
			candidate.Priority = len(candidates) + 1
			candidate.Enabled = true
		}
		trackerState, ok := trackerStates[candidate.InterfaceName]
		if !s.serviceInstalled() {
			candidate.TrackingState = models.FailoverTrackingStateNotInstalled
		} else if !candidate.Available {
			candidate.TrackingState = models.FailoverTrackingStateNotAvailable
		} else if !candidate.Enabled {
			candidate.TrackingState = models.FailoverTrackingStateDisabled
		} else if ok {
			candidate.TrackingState = trackerState
		} else if candidate.IsUp {
			candidate.TrackingState = models.FailoverTrackingStateOnline
		} else {
			candidate.TrackingState = models.FailoverTrackingStateOffline
		}
		candidates = append(candidates, candidate)
	}
	slices.SortFunc(candidates, func(a, b models.FailoverCandidate) int {
		if a.Priority != b.Priority {
			return a.Priority - b.Priority
		}
		return strings.Compare(a.InterfaceName, b.InterfaceName)
	})
	return candidates
}

func (s *FailoverService) updateOnlineTimestamps() {
	cfg, err := s.GetConfig()
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, candidate := range cfg.Candidates {
		if candidate.Enabled && candidate.Available && candidate.IsUp && candidate.TrackingState == models.FailoverTrackingStateOnline {
			if _, exists := s.onlineSince[candidate.InterfaceName]; !exists {
				s.onlineSince[candidate.InterfaceName] = time.Now()
			}
		} else {
			delete(s.onlineSince, candidate.InterfaceName)
		}
	}
}

// computeActiveInterface reports which uplink is carrying the connection, or ""
// when none is.
//
// Candidates arrive in priority order, so the first qualifying one wins. Two
// things qualify it: a candidate that has been online long enough to clear the
// hold-down, and — only as a fallback — the uplink that is active right now.
//
// The fallback used to come FIRST and it returned the active uplink whenever
// that uplink was merely online. A lower-priority link that came back first was
// therefore never given back to a higher-priority link that later recovered:
// the sticky branch matched and returned before the hold-down branch ever ran.
// Failback could not happen at all.
func (s *FailoverService) computeActiveInterface(candidates []models.FailoverCandidate) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sticky := ""
	for _, candidate := range candidates {
		if !candidate.Enabled || !candidate.Available {
			continue
		}
		if !candidate.IsUp || candidate.TrackingState != models.FailoverTrackingStateOnline {
			continue
		}
		onlineTime, online := s.onlineSince[candidate.InterfaceName]
		if online && time.Since(onlineTime) >= failbackHoldDownDuration {
			return candidate.InterfaceName
		}
		if sticky == "" && candidate.InterfaceName == s.lastActive {
			sticky = candidate.InterfaceName
		}
	}
	return sticky
}

func (s *FailoverService) hasWirelessStation() bool {
	sections, err := s.uci.GetSections("wireless")
	if err != nil {
		return false
	}
	for _, opts := range sections {
		if opts["mode"] == "sta" {
			return true
		}
	}
	return false
}

func (s *FailoverService) validateConfig(cfg models.FailoverConfig) error {
	if len(cfg.Candidates) == 0 {
		return errors.New("at least one failover candidate is required")
	}
	seen := map[string]bool{}
	enabled := 0
	for _, candidate := range cfg.Candidates {
		if candidate.InterfaceName == "" {
			return errors.New("candidate interface_name is required")
		}
		if seen[candidate.InterfaceName] {
			return fmt.Errorf("duplicate candidate interface: %s", candidate.InterfaceName)
		}
		seen[candidate.InterfaceName] = true
		if candidate.Priority < 1 {
			return fmt.Errorf("candidate %s must have priority >= 1", candidate.InterfaceName)
		}
		if candidate.Enabled {
			enabled++
		}
	}
	if cfg.Enabled && enabled == 0 {
		return errors.New("at least one candidate must be enabled when failover is enabled")
	}
	for _, ip := range cfg.Health.TrackIPs {
		if parsed := net.ParseIP(strings.TrimSpace(ip)); parsed == nil {
			return fmt.Errorf("invalid track IP: %s", ip)
		}
	}
	if err := validateHealthConfig(cfg.Health); err != nil {
		return err
	}
	return nil
}

func validateHealthConfig(health models.FailoverHealthConfig) error {
	if health.Timeout <= 0 {
		return errors.New("health timeout must be greater than 0")
	}
	if health.Interval <= 0 {
		return errors.New("health interval must be greater than 0")
	}
	if health.FailureInterval < 0 {
		return errors.New("health failure_interval must be non-negative")
	}
	if health.RecoveryInterval < 0 {
		return errors.New("health recovery_interval must be non-negative")
	}
	if health.Down <= 0 {
		return errors.New("health down must be greater than 0")
	}
	if health.Up <= 0 {
		return errors.New("health up must be greater than 0")
	}
	if health.Reliability <= 0 {
		return errors.New("health reliability must be greater than 0")
	}
	if health.Count <= 0 {
		return errors.New("health count must be greater than 0")
	}
	if health.Interval <= health.FailureInterval {
		return fmt.Errorf("health interval (%d) must be greater than failure_interval (%d)", health.Interval, health.FailureInterval)
	}
	return nil
}

func (s *FailoverService) loadConfigFile() (failoverConfigFile, error) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return failoverConfigFile{Enabled: false, Health: defaultFailoverHealth()}, nil
		}
		return failoverConfigFile{}, fmt.Errorf("read failover config: %w", err)
	}
	var cfg failoverConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return failoverConfigFile{}, fmt.Errorf("parse failover config: %w", err)
	}
	cfg.Health = normalizeStoredHealth(cfg.Health, healthKeyPresenceOf(data))
	return cfg, nil
}

// healthKeyPresence records which optional health keys the stored config
// actually contained, so a missing key can be defaulted while an explicit 0
// ("do not use interval-based failure detection") is preserved.
type healthKeyPresence struct {
	failureInterval  bool
	recoveryInterval bool
}

// healthKeyPresenceOf inspects the raw stored config for the optional health keys.
func healthKeyPresenceOf(data []byte) healthKeyPresence {
	var envelope struct {
		Health json.RawMessage `json:"health"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Health) == 0 {
		return healthKeyPresence{}
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Health, &keys); err != nil {
		return healthKeyPresence{}
	}
	_, failure := keys["failure_interval"]
	_, recovery := keys["recovery_interval"]
	return healthKeyPresence{failureInterval: failure, recoveryInterval: recovery}
}

func (s *FailoverService) saveConfigFile(cfg failoverConfigFile) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal failover config: %w", err)
	}
	// Atomic: the 10s monitor reads this file, and os.WriteFile truncates before
	// writing, so a save that overlapped a read gave the monitor a 500 and a
	// power cut left a truncated file that fails to parse forever after.
	return writeFileAtomic(s.configPath, data, 0600)
}

// mayDeleteSection reports whether a save may DELETE an mwan3 section. This is
// the only question the ownership fingerprint answers, and it is asked only here.
//
// Ownership is PROVEN BY PROVENANCE, never inferred from content. A section is
// Travo's if and only if it carries the `travo_owner` fingerprint THIS build
// wrote and that fingerprint still matches what it wrote. Deleting is the one
// thing this service cannot undo: the backup the section would have gone into is
// only read when a save FAILS, and nothing puts a section back after a
// successful one — a dropped config section is lost, and mwan3 stops tracking
// that uplink permanently.
//
// Content is therefore never a basis for deletion, and the reason is concrete:
// the stock mwan3 example an operator copies is byte-identical to what this
// service writes for a default-health candidate, so an exact option-set match
// cannot tell the operator's copy from Travo's own output. A non-namespaced
// section is never deleted, whatever it contains (ADR 0005 §1).
func mayDeleteSection(name string, opts map[string]string) bool {
	return strings.HasPrefix(name, failoverSectionPrefix) && ownedByTravo(opts)
}

// needsBackupSection reports whether a save may WRITE to an mwan3 section, and
// so must be able to restore its pre-save state. It is deliberately WIDER than
// mayDeleteSection, because the two answer different questions:
//
//   - "may I delete this?"  → provenance. Provenance is the right answer, and
//     the only safe one: an operator-edited section is kept.
//   - "can I put this back if the apply fails?" → reach. A save regenerates
//     EVERY namespaced section it owns a name for, and regenerating means
//     `uci set` over all of its options, so the pre-save state of an
//     operator-edited travo_ section is destroyed too. Sharing one predicate
//     here left that state in no backup at all, on a path that is only ever read
//     when the save FAILED — the one moment the operator's tuning mattered. The
//     section survived the delete pass and was overwritten anyway.
//
// Adopting a section is therefore not "leave it alone": it means "keep it, and
// be able to roll it back".
func needsBackupSection(name string, opts map[string]string) bool {
	return strings.HasPrefix(name, failoverSectionPrefix)
}

// ownedByTravo reports whether a namespaced section still carries exactly what
// this service wrote to it.
func ownedByTravo(opts map[string]string) bool {
	marker := strings.TrimSpace(opts[failoverOwnerOption])
	return marker != "" && marker == sectionFingerprint(opts)
}

// normaliseOptionValue reduces a UCI value to comparable tokens. GetAll and
// GetSections do not agree on list formatting — one reports "a b", the other
// "a,b" — so every comparison and fingerprint goes through this, or a section
// this service just wrote would read back as user-modified.
func normaliseOptionValue(value string) string {
	tokens := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	slices.Sort(tokens)
	tokens = slices.Compact(tokens)
	return strings.Join(tokens, " ")
}

// failoverOwnerOption marks a section as this service's, and records what it
// wrote so an operator edit is detectable. Section metadata is excluded: GetAll
// drops it and GetSections keeps it, so including it would make every section
// look modified the moment the two are compared.
const failoverOwnerOption = "travo_owner"

// sectionFingerprint is a short digest of a section's options, ignoring the
// marker itself.
func sectionFingerprint(opts map[string]string) string {
	keys := make([]string, 0, len(opts))
	for key := range opts {
		if key == failoverOwnerOption {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	digest := sha256.New()
	for _, key := range keys {
		fmt.Fprintf(digest, "%s=%s\n", key, normaliseOptionValue(opts[key]))
	}
	return hex.EncodeToString(digest.Sum(nil))[:16]
}

// markSectionOwned stamps the ownership marker. It runs after every other
// option of the section is written, because the fingerprint covers them all.
func (s *FailoverService) markSectionOwned(section string) error {
	opts, err := s.uci.GetAll(mwan3ConfigName, section)
	if err != nil {
		return fmt.Errorf("read generated mwan3 section %s: %w", section, err)
	}
	if err := s.uci.Set(mwan3ConfigName, section, failoverOwnerOption, sectionFingerprint(opts)); err != nil {
		return fmt.Errorf("marking mwan3 section %s as owned: %w", section, err)
	}
	return nil
}

// interfaceSectionOptions is the option map writeInterfaceSection produces.
// track_ip is deliberately absent: it is a list this service appends to
// separately, and emitting it here as a joined (or empty) string is how an
// option that does not exist and one that exists compare as different.
func interfaceSectionOptions(candidate models.FailoverCandidate,
	health models.FailoverHealthConfig,
) map[string]string {
	return map[string]string{
		"enabled":           boolToUCI(candidate.Enabled),
		"family":            "ipv4",
		"reliability":       fmt.Sprintf("%d", health.Reliability),
		"count":             fmt.Sprintf("%d", health.Count),
		"timeout":           fmt.Sprintf("%d", health.Timeout),
		"interval":          fmt.Sprintf("%d", health.Interval),
		"failure_interval":  fmt.Sprintf("%d", health.FailureInterval),
		"recovery_interval": fmt.Sprintf("%d", health.RecoveryInterval),
		"down":              fmt.Sprintf("%d", health.Down),
		"up":                fmt.Sprintf("%d", health.Up),
	}
}

// operatorEditsOverwrittenAlert is the alert type raised when a save replaces
// options an operator had hand-tuned inside a travo_ section.
const operatorEditsOverwrittenAlert = "failover_operator_edits_overwritten"

// failoverSectionPrefix namespaces every mwan3 section this service writes.
const failoverSectionPrefix = "travo_"

// failoverInterfaceSectionPrefix prefixes a generated mwan3 interface section.
// mwan3 tracks by the mwan3 section name, so members reference THIS name, not
// the network interface name.
const failoverInterfaceSectionPrefix = failoverSectionPrefix + "if_"

// failoverInterfaceSection is the mwan3 interface section name for a candidate.
func failoverInterfaceSection(interfaceName string) string {
	return failoverInterfaceSectionPrefix + failoverSectionName(interfaceName)
}

func (s *FailoverService) backupManagedSections() error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		sections = map[string]map[string]string{}
	}
	managed := map[string]map[string]string{}
	for name, opts := range sections {
		// Backup by section shape, not by the new candidate list: a section left
		// over from a removed candidate must still be restorable — and so must
		// an operator-edited one the save is about to overwrite (needsBackupSection,
		// which is wider than mayDeleteSection on purpose).
		if needsBackupSection(name, opts) {
			managed[name] = opts
		}
	}
	data, err := json.Marshal(managed)
	if err != nil {
		return fmt.Errorf("marshal failover backup: %w", err)
	}
	// Atomic write: restoreManagedSections reads this on the rollback path, and
	// a truncated backup makes the restore fail after the failure it was
	// supposed to repair.
	return writeFileAtomic(s.backupPath, data, 0600)
}

// restoreManagedSections puts the backed-up mwan3/network sections back and
// re-applies them, so the running mwan3 does not keep the broken partial policy
// while the user is told the save failed.
//
// The re-apply deliberately BYPASSES the pending-rollback-window check that
// stagedApplyMwan3 normally enforces. This function is the recovery path for a
// failed apply, so it runs precisely when a session is still pending; letting it
// short-circuit on errApplyRollingBack would make every failed save leave the
// crash guard behind and permanently disable failover, which is the opposite of
// what the restore is for.
// mwan3ListOptions are the options mwan3 reads as UCI lists. Restoring one with
// Set writes the whole joined value as a SINGLE element, which mwan3 then reads
// as one invalid address — so they are rebuilt with AddList instead.
//
// The list is explicit rather than inferred from the value, because the backup
// cannot tell a list from a scalar that happens to contain a comma: RealUCI
// normalises a printed list ('a' 'b') to a,b so callers can split on it, and the
// backup stores exactly that. An option outside this set that looks joined is
// logged rather than guessed at, so a future list option is noticed instead of
// silently written as one element.
var mwan3ListOptions = map[string]bool{
	"track_ip":   true, // interface
	"use_member": true, // member and policy
}

// RestoreManagedSections is the rollback path: it must put the operator's
// pre-save config back, not a lossy version of it.
func (s *FailoverService) restoreManagedSections() error {
	data, err := os.ReadFile(s.backupPath)
	if err != nil {
		return err
	}
	var sections map[string]map[string]string
	if err := json.Unmarshal(data, &sections); err != nil {
		return err
	}
	// Delete under the same provenance rule the save used, so a failed save
	// unwinds exactly what it changed and nothing else.
	if err := s.deleteManagedSections(); err != nil {
		return err
	}
	for name, opts := range sections {
		stype := opts[".type"]
		if stype == "" {
			continue
		}
		if err := s.uci.AddSection(mwan3ConfigName, name, stype); err != nil {
			return fmt.Errorf("restore add section %s: %w", name, err)
		}
		for option, value := range opts {
			if strings.HasPrefix(option, ".") {
				continue
			}
			if mwan3ListOptions[option] {
				if err := s.restoreListOption(name, option, value); err != nil {
					return err
				}
				continue
			}
			if strings.Contains(value, ",") && !mwan3ScalarMayContainComma[option] {
				log.Printf("WARNING: restoring mwan3.%s.%s as a scalar, but it holds a "+
					"joined value %q — if it is a list option, add it to "+
					"mwan3ListOptions or it will be restored as one invalid element",
					name, option, value)
			}
			if err := s.uci.Set(mwan3ConfigName, name, option, value); err != nil {
				return err
			}
		}
	}
	if err := s.uci.Commit(mwan3ConfigName); err != nil {
		return err
	}
	return s.restoreApplyMwan3(func() error { return s.verifyManagedSections(sections) })
}

// mwan3ScalarMayContainComma are scalar options whose value legitimately holds
// a comma, so they are not warned about. Kept small and explicit: a false
// negative here is a spurious log line, a false positive is a real list option
// silently restored as one element.
var mwan3ScalarMayContainComma = map[string]bool{}

func (s *FailoverService) restoreListOption(section, option, value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if err := s.uci.AddList(mwan3ConfigName, section, option, item); err != nil {
			return fmt.Errorf("restore list %s.%s: %w", section, option, err)
		}
	}
	return nil
}

func (s *FailoverService) deleteManagedSections() error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		return nil
	}
	for name, opts := range sections {
		if mayDeleteSection(name, opts) {
			_ = s.uci.DeleteSection(mwan3ConfigName, name)
		}
	}
	return s.uci.Commit(mwan3ConfigName)
}

func (s *FailoverService) applyManagedConfig(cfg failoverConfigFile) error {
	if err := s.deleteManagedSections(); err != nil {
		return err
	}
	if !cfg.Enabled || !s.serviceInstalled() {
		return s.stagedApplyMwan3(nil)
	}

	for _, candidate := range cfg.Candidates {
		if err := s.writeInterfaceSection(candidate, cfg.Health); err != nil {
			return err
		}
		if candidate.Enabled {
			memberName := fmt.Sprintf("travo_%s_p%d", failoverSectionName(candidate.InterfaceName), candidate.Priority)
			if err := s.adoptSection(memberName, "member"); err != nil {
				return err
			}
			// mwan3 members reference the mwan3 INTERFACE SECTION name, which
			// is the namespaced one, not the network interface.
			iface := failoverInterfaceSection(candidate.InterfaceName)
			if err := s.uci.Set(mwan3ConfigName, memberName, "interface", iface); err != nil {
				return err
			}
			if err := s.uci.Set(mwan3ConfigName, memberName, "metric", fmt.Sprintf("%d", candidate.Priority)); err != nil {
				return err
			}
			if err := s.uci.Set(mwan3ConfigName, memberName, "weight", "1"); err != nil {
				return err
			}
			if err := s.markSectionOwned(memberName); err != nil {
				return err
			}
		}
	}
	if err := s.adoptSection(failoverPolicySection, "policy"); err != nil {
		return err
	}
	for _, candidate := range cfg.Candidates {
		if !candidate.Enabled {
			continue
		}
		memberName := fmt.Sprintf("travo_%s_p%d", failoverSectionName(candidate.InterfaceName), candidate.Priority)
		if err := s.uci.AddList(mwan3ConfigName, failoverPolicySection, "use_member", memberName); err != nil {
			return err
		}
	}
	if err := s.adoptSection(failoverRuleSection, "rule"); err != nil {
		return err
	}
	if err := s.uci.Set(mwan3ConfigName, failoverRuleSection, "dest_ip", "0.0.0.0/0"); err != nil {
		return fmt.Errorf("set rule dest_ip: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, failoverRuleSection, "family", "ipv4"); err != nil {
		return fmt.Errorf("set rule family: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, failoverRuleSection, "use_policy", failoverPolicySection); err != nil {
		return fmt.Errorf("set rule use_policy: %w", err)
	}
	if err := s.markSectionOwned(failoverPolicySection); err != nil {
		return err
	}
	if err := s.markSectionOwned(failoverRuleSection); err != nil {
		return err
	}
	if err := s.uci.Commit(mwan3ConfigName); err != nil {
		return err
	}
	return s.stagedApplyMwan3(func() error { return s.verifyManagedSections(managedSectionNames(cfg.Candidates)) })
}

// adoptSection creates or reuses an mwan3 section for writing. Real
// `uci set config.section=stype` is create-OR-update, so a section that survived
// the delete pass is reused here and every option below is written over it.
//
// The one case worth telling the user about is reuse of an operator-edited
// section: needsBackupSection says the save may write it, mayDeleteSection says
// the save may not delete it, so the operator's extra options and their section
// type survive while the options this service owns are replaced by the saved
// failover settings. That is the deliberate outcome (ADR 0005 §1.2) — Travo's
// saved health config is authoritative for the options it writes, and
// verifyManagedSections requires the live config to match it exactly — but
// reporting it is the difference between a documented take-over and a silent
// loss of the operator's reliability/timeout/up/down tuning.
func (s *FailoverService) adoptSection(section, stype string) error {
	opts, err := s.uci.GetAll(mwan3ConfigName, section)
	if err == nil && needsBackupSection(section, opts) && !ownedByTravo(opts) {
		log.Printf("failover: mwan3 section %s was edited outside Travo; this save "+
			"replaces the options it owns with the saved failover settings", section)
		if s.alertSvc != nil {
			s.alertSvc.Publish(operatorEditsOverwrittenAlert, fmt.Sprintf(
				"Failover adopted mwan3 section %s: options Travo owns were replaced by the saved "+
					"failover settings, other options were kept. Re-apply your tuning there if it was lost.",
				section), "warning")
		}
	}
	return s.uci.AddSection(mwan3ConfigName, section, stype)
}

func (s *FailoverService) writeInterfaceSection(candidate models.FailoverCandidate, health models.FailoverHealthConfig) error {
	// Namespaced (ADR 0005 §1): a generated section that shares its name with a
	// hand-written one is indistinguishable from it on the next save, and the
	// next save deletes it.
	sectionName := failoverInterfaceSection(candidate.InterfaceName)
	if err := s.adoptSection(sectionName, "interface"); err != nil {
		return err
	}
	for option, value := range interfaceSectionOptions(candidate, health) {
		if option == "track_ip" {
			continue
		}
		if err := s.uci.Set(mwan3ConfigName, sectionName, option, value); err != nil {
			return fmt.Errorf("set interface %s: %w", option, err)
		}
	}
	for _, ip := range health.TrackIPs {
		if err := s.uci.AddList(mwan3ConfigName, sectionName, "track_ip", ip); err != nil {
			if setErr := s.uci.Set(mwan3ConfigName, sectionName, "track_ip", ip); setErr != nil {
				return fmt.Errorf("set track_ip %s: %w", ip, setErr)
			}
		}
	}
	return s.markSectionOwned(sectionName)
}

func (s *FailoverService) verifyApply(cfg failoverConfigFile) error {
	var expect map[string]map[string]string
	if cfg.Enabled {
		// Every generated section, with the options that bind them together —
		// not just the policy and the rule.
		expect = managedSectionNames(cfg.Candidates)
	}
	if err := s.verifyManagedSections(expect); err != nil {
		return err
	}
	networkStatus, err := s.networkSvc.GetNetworkStatus()
	if err != nil {
		return err
	}
	// Every enabled, available candidate must be readable at runtime. Requiring
	// only one of them would let a misconfigured uplink verify clean while the
	// others work, and the user would only find out when that link is needed.
	var expected []string
	for _, candidate := range cfg.Candidates {
		if candidate.Enabled && candidate.Available {
			expected = append(expected, candidate.InterfaceName)
		}
	}
	for _, name := range expected {
		if !interfacePresent(networkStatus.Interfaces, name) {
			return fmt.Errorf("enabled failover candidate %s is not readable at runtime", name)
		}
	}
	return nil
}

// verifyManagedSections checks that the mwan3 config the service wrote is
// readable. When expect is non-nil, the given sections must be present and
// carry the option values in the map (an empty value is not checked).
func (s *FailoverService) verifyManagedSections(expect map[string]map[string]string) error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		return fmt.Errorf("verify mwan3 config: %w", err)
	}
	for name, opts := range expect {
		actual, ok := sections[name]
		if !ok {
			return fmt.Errorf("mwan3 section %s missing after apply", name)
		}
		for option, want := range opts {
			// Section metadata (".type") is not an option.
			if want == "" || strings.HasPrefix(option, ".") {
				continue
			}
			// Compare against the same read the expectation was built from:
			// GetSections and Get normalise list values differently, so mixing
			// them would fail the rollback restore of any section with a list.
			if actual[option] != want {
				return fmt.Errorf("mwan3 section %s: %s = %q, want %q", name, option, actual[option], want)
			}
		}
	}
	return nil
}

var mwan3UCIConfigs = []string{"network", "mwan3"}

// stagedApplyMwan3 applies the mwan3 config for a user save, refusing to start
// while an earlier apply is still rolling back.
func (s *FailoverService) stagedApplyMwan3(verify func() error) error {
	if s.rollbackStillPending() {
		// An earlier apply in this service is unconfirmed, so its rollback window
		// is open and rpcd is restoring the pre-apply mwan3 config right now. A
		// second apply would be refused, and reporting success would be worse: the
		// caller deleted the guard and answered 200 while rpcd's timer was about
		// to restore the config from *before* the earlier save — silently
		// discarding both the failed config and this acknowledged one.
		log.Printf("failover: an mwan3 apply is already rolling back (session %s); skipping this apply", s.pendingApplySession)
		return errApplyRollingBack
	}
	// The window expired and rpcd rolled the session back; nothing confirms
	// that, so the deadline is the signal. Forget it and apply.
	s.pendingApplySession = ""
	return s.applyMwan3(verify)
}

// restoreApplyMwan3 is the recovery-path variant: it runs even while an earlier
// apply is still rolling back, because restoring is exactly what must be able to
// happen then. The stale session id is dropped first — rpcd has a single apply
// slot, so carrying it into the restore's own apply would record the restore's
// session under the old deadline if the restore then failed.
func (s *FailoverService) restoreApplyMwan3(verify func() error) error {
	s.pendingApplySession = ""
	return s.applyMwan3(verify)
}

// applyMwan3 runs the rpcd apply+confirm flow with a real rollback window: the
// apply is started first, the config is verified while the previous config is
// still restorable, and only then is the change confirmed. An immediate
// apply+confirm on a policy that re-routes the WAN would close the rollback
// window before anything was checked.
func (s *FailoverService) applyMwan3(verify func() error) error {
	if !s.serviceInstalled() {
		return nil
	}
	if s.applier == nil {
		return s.reloadMwan3Script()
	}
	sid, err := s.applier.StartApply(mwan3UCIConfigs)
	if err != nil {
		return fmt.Errorf("uci apply mwan3: %w", err)
	}
	if sid == "" {
		// Applier had nothing to do (Noop / empty session).
		return nil
	}
	s.pendingApplySession = sid
	// rpcd rolls back after its own timeout; add a margin so the record is only
	// dropped once the rollback has certainly happened.
	s.pendingApplyDeadline = time.Now().Add(rollbackGracePeriod)
	if verify != nil {
		if err := verify(); err != nil {
			// Rollback window is still open: rpcd reverts to the previous config
			// on its own timer. Keep the session recorded until that deadline so
			// the next apply waits for the rollback instead of being rejected.
			return fmt.Errorf("verify mwan3 apply: %w", err)
		}
	}
	if err := s.applier.Confirm(sid); err != nil {
		return fmt.Errorf("uci confirm mwan3: %w", err)
	}
	s.pendingApplySession = ""
	return nil
}

// reloadMwan3Script is the fallback for services without an rpcd applier.
func (s *FailoverService) reloadMwan3Script() error {
	if _, err := s.cmd.Run(s.initScript, "reload"); err != nil {
		if _, restartErr := s.cmd.Run(s.initScript, "restart"); restartErr != nil {
			return fmt.Errorf("reload mwan3: %w", err)
		}
	}
	return nil
}

func (s *FailoverService) serviceInstalled() bool {
	_, err := os.Stat(s.initScript)
	return err == nil
}

func defaultFailoverHealth() models.FailoverHealthConfig {
	return models.FailoverHealthConfig{
		TrackIPs:         []string{"1.1.1.1", "8.8.8.8"},
		Reliability:      1,
		Count:            1,
		Timeout:          2,
		Interval:         5,
		FailureInterval:  5,
		RecoveryInterval: 5,
		Down:             3,
		Up:               3,
	}
}

// normalizeHealth fills in defaults for unset values. An explicit zero
// FailureInterval / RecoveryInterval is meaningful (mwan3 then does not use
// interval-based failure/recovery detection), so it is preserved.
func normalizeHealth(health models.FailoverHealthConfig) models.FailoverHealthConfig {
	def := defaultFailoverHealth()
	if len(health.TrackIPs) > 0 {
		def.TrackIPs = health.TrackIPs
	}
	if health.Reliability > 0 {
		def.Reliability = health.Reliability
	}
	if health.Count > 0 {
		def.Count = health.Count
	}
	if health.Timeout > 0 {
		def.Timeout = health.Timeout
	}
	if health.Interval > 0 {
		def.Interval = health.Interval
	}
	def.FailureInterval = health.FailureInterval
	def.RecoveryInterval = health.RecoveryInterval
	if health.Down > 0 {
		def.Down = health.Down
	}
	if health.Up > 0 {
		def.Up = health.Up
	}
	return def
}

// normalizeStoredHealth normalizes a config read from disk. Only keys that were
// actually present may carry a meaningful zero; a missing key gets the default.
func normalizeStoredHealth(health models.FailoverHealthConfig, present healthKeyPresence) models.FailoverHealthConfig {
	if !present.failureInterval {
		health.FailureInterval = defaultFailoverHealth().FailureInterval
	}
	if !present.recoveryInterval {
		health.RecoveryInterval = defaultFailoverHealth().RecoveryInterval
	}
	return normalizeHealth(health)
}

func (s *FailoverService) readTrackerStates() map[string]models.FailoverTrackingState {
	states := map[string]models.FailoverTrackingState{}
	generated := map[string]bool{}
	if !s.serviceInstalled() {
		return states
	}
	out, err := s.cmd.Run("mwan3", "interfaces")
	if err != nil {
		return states
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "interface ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := fields[1]
		// mwan3 reports state per mwan3 INTERFACE SECTION name; candidates are
		// keyed by network interface name (ADR 0005 §1 namespacing). A
		// hand-written section that shares a name with a network interface must
		// not overwrite the generated one's state.
		key := name
		isGenerated := strings.HasPrefix(name, failoverInterfaceSectionPrefix)
		if isGenerated {
			key = strings.TrimPrefix(name, failoverInterfaceSectionPrefix)
		} else if generated[key] {
			continue
		}
		state := models.FailoverTrackingState("")
		switch {
		case strings.Contains(line, "is online"):
			state = models.FailoverTrackingStateOnline
		case strings.Contains(line, "is offline"):
			state = models.FailoverTrackingStateOffline
		case strings.Contains(line, "is disabled"):
			state = models.FailoverTrackingStateDisabled
		}
		if state == "" {
			continue
		}
		if isGenerated {
			generated[key] = true
		}
		states[key] = state
	}
	return states
}

func (s *FailoverService) addDiscoveredUSBNetworkCandidates(known map[string]models.FailoverCandidate) {
	sections, err := s.uci.GetSections("network")
	if err != nil {
		return
	}
	for section, opts := range sections {
		device := opts["device"]
		if section == usbTetherUCIName || isUSBDeviceName(device) {
			if _, exists := known[section]; !exists {
				known[section] = newCandidate(section, "USB tether", models.FailoverCandidateKindUSB, false)
			}
		}
	}
}

func newCandidate(iface, label string, kind models.FailoverCandidateKind, isUp bool) models.FailoverCandidate {
	return models.FailoverCandidate{
		ID:            iface,
		Label:         label,
		InterfaceName: iface,
		Kind:          kind,
		Available:     true,
		Enabled:       true,
		IsUp:          isUp,
	}
}

func markUnavailable(candidate models.FailoverCandidate) models.FailoverCandidate {
	candidate.Available = false
	candidate.IsUp = false
	return candidate
}

func failoverSectionName(value string) string {
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, ".", "_")
	return value
}

// managedSectionNames lists the mwan3 sections the apply is expected to create
// for a candidate list, with the option values that must match what the apply
// wrote. Members are only written for enabled candidates.
//
// The values matter as much as the names: mwan3 members reference the mwan3
// interface SECTION, so a member left pointing at a pre-namespacing section name
// ("wan" instead of "travo_if_wan") keeps the policy bound to a section the
// service no longer writes or updates. Checking presence alone would let that
// through in production.
func managedSectionNames(candidates []models.FailoverCandidate) map[string]map[string]string {
	names := map[string]map[string]string{
		failoverPolicySection: nil,
		failoverRuleSection:   nil,
	}
	for _, candidate := range candidates {
		iface := failoverInterfaceSection(candidate.InterfaceName)
		names[iface] = map[string]string{"family": "ipv4"}
		if candidate.Enabled {
			memberName := fmt.Sprintf("travo_%s_p%d",
				failoverSectionName(candidate.InterfaceName), candidate.Priority)
			names[memberName] = map[string]string{"interface": iface}
		}
	}
	return names
}

func isUSBDeviceName(name string) bool {
	return name == "usb0" || name == "usb1" || name == "eth1" || name == "eth2"
}

func boolToUCI(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func interfacePresent(interfaces []models.NetworkInterface, name string) bool {
	for _, iface := range interfaces {
		if iface.Name == name {
			return true
		}
	}
	return false
}
