package services

import (
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
	if err := s.restoreManagedSections(); err != nil {
		log.Printf("failover: rollback failed, crash guard kept at %s: %v", s.guardPath, err)
		return
	}
	if s.rollbackStillPending() {
		log.Printf("failover: restored, but rpcd rollback %s is still armed; crash guard kept at %s",
			s.pendingApplySession, s.guardPath)
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
func (s *FailoverService) SetConfig(cfg models.FailoverConfig) error {
	if err := s.validateConfig(cfg); err != nil {
		return err
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.configPath), 0750); err != nil {
		return fmt.Errorf("create failover config dir: %w", err)
	}
	if err := s.backupManagedSections(cfg); err != nil {
		return err
	}

	cfgFile := failoverConfigFile{
		Enabled:    cfg.Enabled,
		Candidates: make([]models.FailoverCandidate, len(cfg.Candidates)),
		Health:     normalizeHealth(cfg.Health),
	}
	copy(cfgFile.Candidates, cfg.Candidates)
	if err := s.saveConfigFile(cfgFile); err != nil {
		_ = s.restoreManagedSections()
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

func (s *FailoverService) computeActiveInterface(candidates []models.FailoverCandidate) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	currentActive := s.lastActive

	for _, candidate := range candidates {
		if !candidate.Enabled || !candidate.Available {
			continue
		}

		if candidate.IsUp && candidate.TrackingState == models.FailoverTrackingStateOnline {
			if candidate.InterfaceName == currentActive {
				return candidate.InterfaceName
			}
		}
	}

	for _, candidate := range candidates {
		if !candidate.Enabled || !candidate.Available {
			continue
		}

		if candidate.IsUp && candidate.TrackingState == models.FailoverTrackingStateOnline {
			onlineTime, exists := s.onlineSince[candidate.InterfaceName]
			if exists && time.Since(onlineTime) >= failbackHoldDownDuration {
				return candidate.InterfaceName
			}
		}
	}

	return ""
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
	return os.WriteFile(s.configPath, data, 0600)
}

// isManagedSection reports whether an mwan3 section belongs to the failover
// service. Besides our own travo_* sections this covers every mwan3 interface
// section, so a candidate that was removed or renamed is backed up and deleted
// instead of being health-pinged forever.
func isManagedSection(name string, opts map[string]string) bool {
	if strings.HasPrefix(name, "travo_") {
		return true
	}
	if name == failoverPolicySection || name == failoverRuleSection {
		return true
	}
	if name == "wan" || name == "wwan" || name == usbTetherUCIName {
		return true
	}
	return opts[".type"] == "interface"
}

func (s *FailoverService) backupManagedSections(cfg models.FailoverConfig) error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		sections = map[string]map[string]string{}
	}
	managed := map[string]map[string]string{}
	for name, opts := range sections {
		// Backup by section shape, not by the new candidate list: a section left
		// over from a removed candidate must still be restorable.
		if isManagedSection(name, opts) {
			managed[name] = opts
		}
	}
	data, err := json.Marshal(managed)
	if err != nil {
		return fmt.Errorf("marshal failover backup: %w", err)
	}
	return os.WriteFile(s.backupPath, data, 0600)
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
func (s *FailoverService) restoreManagedSections() error {
	data, err := os.ReadFile(s.backupPath)
	if err != nil {
		return err
	}
	var sections map[string]map[string]string
	if err := json.Unmarshal(data, &sections); err != nil {
		return err
	}
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

func (s *FailoverService) deleteManagedSections() error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		return nil
	}
	for name, opts := range sections {
		if isManagedSection(name, opts) {
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
			if err := s.uci.AddSection(mwan3ConfigName, memberName, "member"); err != nil {
				return err
			}
			if err := s.uci.Set(mwan3ConfigName, memberName, "interface", candidate.InterfaceName); err != nil {
				return err
			}
			if err := s.uci.Set(mwan3ConfigName, memberName, "metric", fmt.Sprintf("%d", candidate.Priority)); err != nil {
				return err
			}
			if err := s.uci.Set(mwan3ConfigName, memberName, "weight", "1"); err != nil {
				return err
			}
		}
	}
	if err := s.uci.AddSection(mwan3ConfigName, failoverPolicySection, "policy"); err != nil {
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
	if err := s.uci.AddSection(mwan3ConfigName, failoverRuleSection, "rule"); err != nil {
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
	if err := s.uci.Commit(mwan3ConfigName); err != nil {
		return err
	}
	return s.stagedApplyMwan3(func() error { return s.verifyManagedSections(managedSectionNames(cfg.Candidates)) })
}

func (s *FailoverService) writeInterfaceSection(candidate models.FailoverCandidate, health models.FailoverHealthConfig) error {
	sectionName := candidate.InterfaceName
	if err := s.uci.AddSection(mwan3ConfigName, sectionName, "interface"); err != nil {
		return err
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "enabled", boolToUCI(candidate.Enabled)); err != nil {
		return fmt.Errorf("set interface enabled: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "family", "ipv4"); err != nil {
		return fmt.Errorf("set interface family: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "reliability", fmt.Sprintf("%d", health.Reliability)); err != nil {
		return fmt.Errorf("set interface reliability: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "count", fmt.Sprintf("%d", health.Count)); err != nil {
		return fmt.Errorf("set interface count: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "timeout", fmt.Sprintf("%d", health.Timeout)); err != nil {
		return fmt.Errorf("set interface timeout: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "interval", fmt.Sprintf("%d", health.Interval)); err != nil {
		return fmt.Errorf("set interface interval: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "failure_interval", fmt.Sprintf("%d", health.FailureInterval)); err != nil {
		return fmt.Errorf("set interface failure_interval: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "recovery_interval", fmt.Sprintf("%d", health.RecoveryInterval)); err != nil {
		return fmt.Errorf("set interface recovery_interval: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "down", fmt.Sprintf("%d", health.Down)); err != nil {
		return fmt.Errorf("set interface down: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, sectionName, "up", fmt.Sprintf("%d", health.Up)); err != nil {
		return fmt.Errorf("set interface up: %w", err)
	}
	for _, ip := range health.TrackIPs {
		if err := s.uci.AddList(mwan3ConfigName, sectionName, "track_ip", ip); err != nil {
			if setErr := s.uci.Set(mwan3ConfigName, sectionName, "track_ip", ip); setErr != nil {
				return fmt.Errorf("set track_ip %s: %w", ip, setErr)
			}
		}
	}
	return nil
}

func (s *FailoverService) verifyApply(cfg failoverConfigFile) error {
	var expect map[string]map[string]string
	if cfg.Enabled {
		expect = map[string]map[string]string{failoverPolicySection: nil, failoverRuleSection: nil}
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
// readable. When expect is non-nil, the given sections must be present.
func (s *FailoverService) verifyManagedSections(expect map[string]map[string]string) error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		return fmt.Errorf("verify mwan3 config: %w", err)
	}
	for name, opts := range expect {
		if _, ok := sections[name]; !ok {
			return fmt.Errorf("mwan3 section %s missing after apply", name)
		}
		if opts["family"] != "" {
			if got, _ := s.uci.Get(mwan3ConfigName, name, "family"); got != opts["family"] {
				return fmt.Errorf("mwan3 section %s: family = %q, want %q", name, got, opts["family"])
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
		switch {
		case strings.Contains(line, "is online"):
			states[name] = models.FailoverTrackingStateOnline
		case strings.Contains(line, "is offline"):
			states[name] = models.FailoverTrackingStateOffline
		case strings.Contains(line, "is disabled"):
			states[name] = models.FailoverTrackingStateDisabled
		}
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
// for a candidate list. Members are only written for enabled candidates.
func managedSectionNames(candidates []models.FailoverCandidate) map[string]map[string]string {
	names := map[string]map[string]string{
		failoverPolicySection: nil,
		failoverRuleSection:   nil,
	}
	for _, candidate := range candidates {
		names[candidate.InterfaceName] = nil
		if candidate.Enabled {
			names[fmt.Sprintf("travo_%s_p%d", failoverSectionName(candidate.InterfaceName), candidate.Priority)] = nil
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
