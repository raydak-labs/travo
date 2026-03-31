package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

const (
	failoverConfigPath     = "/etc/travo/failover.json"
	failoverGuardPath      = "/etc/travo/failover-in-progress"
	failoverBackupPath     = "/etc/travo/failover-mwan3-backup.json"
	mwan3InitScriptPath    = "/etc/init.d/mwan3"
	mwan3ConfigName        = "mwan3"
	failoverPolicySection  = "travo_failover"
	failoverRuleSection    = "travo_default_v4"
	failoverTickerInterval = 10 * time.Second
)

type failoverConfigFile struct {
	Enabled    bool                        `json:"enabled"`
	Candidates []models.FailoverCandidate  `json:"candidates"`
	Health     models.FailoverHealthConfig `json:"health"`
}

// FailoverService manages app-owned mwan3 failover configuration.
type FailoverService struct {
	uci        uci.UCI
	networkSvc *NetworkService
	cmd        CommandRunner

	configPath string
	guardPath  string
	backupPath string
	initScript string
	alertSvc   *AlertService
	mu         sync.RWMutex
	events     []models.FailoverEvent
	lastActive string
	stopCh     chan struct{}
	stopOnce   sync.Once
}

func NewFailoverService(u uci.UCI, networkSvc *NetworkService) *FailoverService {
	return &FailoverService{
		uci:        u,
		networkSvc: networkSvc,
		cmd:        &RealCommandRunner{},
		configPath: failoverConfigPath,
		guardPath:  failoverGuardPath,
		backupPath: failoverBackupPath,
		initScript: mwan3InitScriptPath,
		events:     make([]models.FailoverEvent, 0, 10),
		stopCh:     make(chan struct{}),
	}
}

func NewFailoverServiceWithRunner(u uci.UCI, networkSvc *NetworkService, cmd CommandRunner, configPath string) *FailoverService {
	return &FailoverService{
		uci:        u,
		networkSvc: networkSvc,
		cmd:        cmd,
		configPath: configPath,
		guardPath:  filepath.Join(filepath.Dir(configPath), "failover-in-progress"),
		backupPath: filepath.Join(filepath.Dir(configPath), "failover-mwan3-backup.json"),
		initScript: mwan3InitScriptPath,
		events:     make([]models.FailoverEvent, 0, 10),
		stopCh:     make(chan struct{}),
	}
}

func (s *FailoverService) SetAlertService(alertSvc *AlertService) {
	s.alertSvc = alertSvc
}

func (s *FailoverService) Start() {
	ticker := time.NewTicker(failoverTickerInterval)
	defer ticker.Stop()

	for {
		if _, err := os.Stat(s.guardPath); err == nil {
			select {
			case <-ticker.C:
				continue
			case <-s.stopCh:
				return
			}
		}
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

func (s *FailoverService) SetConfig(cfg models.FailoverConfig) error {
	if err := s.validateConfig(cfg); err != nil {
		return err
	}
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
		_ = s.restoreManagedSections()
		return err
	}
	if err := s.verifyApply(cfgFile); err != nil {
		_ = s.restoreManagedSections()
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

	candidates := make([]models.FailoverCandidate, 0, len(known))
	for _, candidate := range known {
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

func (s *FailoverService) computeActiveInterface(candidates []models.FailoverCandidate) string {
	for _, candidate := range candidates {
		if !candidate.Enabled || !candidate.Available {
			continue
		}
		if candidate.IsUp && candidate.TrackingState == models.FailoverTrackingStateOnline {
			return candidate.InterfaceName
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
	cfg.Health = normalizeHealth(cfg.Health)
	return cfg, nil
}

func (s *FailoverService) saveConfigFile(cfg failoverConfigFile) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal failover config: %w", err)
	}
	return os.WriteFile(s.configPath, data, 0600)
}

func (s *FailoverService) backupManagedSections(cfg models.FailoverConfig) error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		sections = map[string]map[string]string{}
	}
	managed := map[string]map[string]string{}
	names := managedSectionNames(cfg.Candidates)
	for name, opts := range sections {
		if names[name] || strings.HasPrefix(name, "travo_") {
			managed[name] = opts
		}
	}
	data, err := json.Marshal(managed)
	if err != nil {
		return fmt.Errorf("marshal failover backup: %w", err)
	}
	return os.WriteFile(s.backupPath, data, 0600)
}

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
		_ = s.uci.AddSection(mwan3ConfigName, name, stype)
		for option, value := range opts {
			if strings.HasPrefix(option, ".") {
				continue
			}
			if err := s.uci.Set(mwan3ConfigName, name, option, value); err != nil {
				return err
			}
		}
	}
	return s.uci.Commit(mwan3ConfigName)
}

func (s *FailoverService) deleteManagedSections() error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		return nil
	}
	for name := range sections {
		if strings.HasPrefix(name, "travo_") || name == "wan" || name == "wwan" || name == usbTetherUCIName {
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
		return s.reloadMwan3()
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
	_ = s.uci.Set(mwan3ConfigName, failoverRuleSection, "dest_ip", "0.0.0.0/0")
	_ = s.uci.Set(mwan3ConfigName, failoverRuleSection, "family", "ipv4")
	_ = s.uci.Set(mwan3ConfigName, failoverRuleSection, "use_policy", failoverPolicySection)
	if err := s.uci.Commit(mwan3ConfigName); err != nil {
		return err
	}
	return s.reloadMwan3()
}

func (s *FailoverService) writeInterfaceSection(candidate models.FailoverCandidate, health models.FailoverHealthConfig) error {
	sectionName := candidate.InterfaceName
	if err := s.uci.AddSection(mwan3ConfigName, sectionName, "interface"); err != nil {
		return err
	}
	_ = s.uci.Set(mwan3ConfigName, sectionName, "enabled", boolToUCI(candidate.Enabled))
	_ = s.uci.Set(mwan3ConfigName, sectionName, "family", "ipv4")
	_ = s.uci.Set(mwan3ConfigName, sectionName, "reliability", fmt.Sprintf("%d", health.Reliability))
	_ = s.uci.Set(mwan3ConfigName, sectionName, "count", fmt.Sprintf("%d", health.Count))
	_ = s.uci.Set(mwan3ConfigName, sectionName, "timeout", fmt.Sprintf("%d", health.Timeout))
	_ = s.uci.Set(mwan3ConfigName, sectionName, "interval", fmt.Sprintf("%d", health.Interval))
	_ = s.uci.Set(mwan3ConfigName, sectionName, "failure_interval", fmt.Sprintf("%d", health.FailureInterval))
	_ = s.uci.Set(mwan3ConfigName, sectionName, "recovery_interval", fmt.Sprintf("%d", health.RecoveryInterval))
	_ = s.uci.Set(mwan3ConfigName, sectionName, "down", fmt.Sprintf("%d", health.Down))
	_ = s.uci.Set(mwan3ConfigName, sectionName, "up", fmt.Sprintf("%d", health.Up))
	for _, ip := range health.TrackIPs {
		if err := s.uci.AddList(mwan3ConfigName, sectionName, "track_ip", ip); err != nil {
			_ = s.uci.Set(mwan3ConfigName, sectionName, "track_ip", ip)
		}
	}
	return nil
}

func (s *FailoverService) verifyApply(cfg failoverConfigFile) error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		return fmt.Errorf("verify mwan3 config: %w", err)
	}
	if cfg.Enabled {
		if _, ok := sections[failoverPolicySection]; !ok {
			return errors.New("expected failover policy missing after apply")
		}
		if _, ok := sections[failoverRuleSection]; !ok {
			return errors.New("expected failover rule missing after apply")
		}
	}
	networkStatus, err := s.networkSvc.GetNetworkStatus()
	if err != nil {
		return err
	}
	for _, candidate := range cfg.Candidates {
		if candidate.Enabled && interfacePresent(networkStatus.Interfaces, candidate.InterfaceName) {
			return nil
		}
	}
	if cfg.Enabled {
		return errors.New("no enabled failover candidate is readable at runtime")
	}
	return nil
}

func (s *FailoverService) reloadMwan3() error {
	if !s.serviceInstalled() {
		return nil
	}
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
	if health.FailureInterval > 0 {
		def.FailureInterval = health.FailureInterval
	}
	if health.RecoveryInterval > 0 {
		def.RecoveryInterval = health.RecoveryInterval
	}
	if health.Down > 0 {
		def.Down = health.Down
	}
	if health.Up > 0 {
		def.Up = health.Up
	}
	return def
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
	for _, line := range strings.Split(string(out), "\n") {
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

func managedSectionNames(candidates []models.FailoverCandidate) map[string]bool {
	names := map[string]bool{
		failoverPolicySection: true,
		failoverRuleSection:   true,
	}
	for _, candidate := range candidates {
		names[candidate.InterfaceName] = true
		names[fmt.Sprintf("travo_%s_p%d", failoverSectionName(candidate.InterfaceName), candidate.Priority)] = true
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
