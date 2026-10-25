package failover

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/network"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/system"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

const (
	failoverConfigPath       = "/etc/travo/failover.json"
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
		slog.Error("failover: rollback failed; crash guard kept", "guard", s.crashGuard.Path(), "err", err)
		return
	}
	if rollbackPending {
		slog.Warn("failover: restored, but the rpcd rollback is still armed; crash guard kept", "session", pendingSession, "guard", s.crashGuard.Path())
		return
	}
	if err := s.crashGuard.Clear(); err != nil {
		slog.Warn("failover: clearing crash guard", "err", err)
	}
}

// rollbackStillPending reports whether an rpcd session is still armed to roll
// the config back on its own timer.
func (s *FailoverService) rollbackStillPending() bool {
	return s.pendingApplySession != "" && time.Now().Before(s.pendingApplyDeadline)
}

// rollbackGracePeriod is how long after the rpcd rollback timeout a pending
// apply is still considered in flight.
const rollbackGracePeriod = time.Duration(platform.ApplyRollbackTimeout)*time.Second + 2*time.Second

type FailoverService struct {
	uci        uci.UCI
	ubus       ubus.Ubus
	networkSvc *network.NetworkService
	cmd        platform.CommandRunner
	applier    platform.UCIApplyConfirm

	configPath string
	crashGuard guard.Guard
	backupPath string
	initScript string
	alertSvc   *system.AlertService
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

func NewFailoverService(u uci.UCI, ub ubus.Ubus, networkSvc *network.NetworkService, pw *auth.RootPassword) *FailoverService {
	return &FailoverService{
		uci:         u,
		ubus:        ub,
		networkSvc:  networkSvc,
		cmd:         &platform.RealCommandRunner{},
		applier:     platform.NewRealUCIApplyConfirm(ub, pw),
		configPath:  failoverConfigPath,
		crashGuard:  guard.New("failover"),
		backupPath:  failoverBackupPath,
		initScript:  mwan3InitScriptPath,
		events:      make([]models.FailoverEvent, 0, 10),
		stopCh:      make(chan struct{}),
		onlineSince: make(map[string]time.Time),
	}
}

func NewFailoverServiceWithRunner(u uci.UCI, ub ubus.Ubus, networkSvc *network.NetworkService, cmd platform.CommandRunner, applier platform.UCIApplyConfirm, configPath string) *FailoverService {
	return &FailoverService{
		uci:         u,
		ubus:        ub,
		networkSvc:  networkSvc,
		cmd:         cmd,
		applier:     applier,
		configPath:  configPath,
		crashGuard:  guard.In(filepath.Dir(configPath), "failover"),
		backupPath:  filepath.Join(filepath.Dir(configPath), "failover-mwan3-backup.json"),
		initScript:  mwan3InitScriptPath,
		events:      make([]models.FailoverEvent, 0, 10),
		stopCh:      make(chan struct{}),
		onlineSince: make(map[string]time.Time),
	}
}

func (s *FailoverService) SetAlertService(alertSvc *system.AlertService) {
	s.alertSvc = alertSvc
}

func (s *FailoverService) Start() {
	ticker := time.NewTicker(failoverTickerInterval)
	defer ticker.Stop()

	// A stuck guard disables failover with no other symptom, so say so once
	// instead of silently skipping forever (band-switching logs the same case).
	// ADR 0003 §1.2.
	if s.crashGuard.Present() {
		slog.Warn("failover: crash guard found; automatic switching disabled until it is removed or the app is redeployed", "guard", s.crashGuard.Path())
	}

	for {
		if s.crashGuard.Present() {
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

	return platform.MutateUCI(s.uci, mwan3UCIConfigs, func() error {
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
	if err := s.crashGuard.Mark(); err != nil {
		return err
	}
	if err := s.applyManagedConfig(cfgFile); err != nil {
		s.rollbackOrKeepGuard()
		return err
	}
	if err := s.verifyApply(cfgFile); err != nil {
		s.rollbackOrKeepGuard()
		return err
	}
	if err := s.crashGuard.Clear(); err != nil {
		return err
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

func (s *FailoverService) saveConfigFile(cfg failoverConfigFile) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal failover config: %w", err)
	}
	// Atomic: the 10s monitor reads this file, and os.WriteFile truncates before
	// writing, so a save that overlapped a read gave the monitor a 500 and a
	// power cut left a truncated file that fails to parse forever after.
	return platform.WriteFileAtomic(s.configPath, data)
}

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

// mwan3ScalarMayContainComma are scalar options whose value legitimately holds
// a comma, so they are not warned about. Kept small and explicit: a false
// negative here is a spurious log line, a false positive is a real list option
// silently restored as one element.
var mwan3ScalarMayContainComma = map[string]bool{}

func failoverSectionName(value string) string {
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, ".", "_")
	return value
}

func boolToUCI(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
