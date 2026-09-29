package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestFailoverServiceGetConfigDefaults(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})
	configPath := filepath.Join(t.TempDir(), "failover.json")
	svc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc, &MockCommandRunner{}, &NoopUCIApplyConfirm{}, configPath)

	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}
	if cfg.Enabled {
		t.Fatalf("expected failover disabled by default")
	}
	if len(cfg.Candidates) == 0 {
		t.Fatalf("expected discovered candidates")
	}
}

func TestFailoverServiceSetConfigWritesManagedSections(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "failover.json")
	svc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc, &MockCommandRunner{}, &NoopUCIApplyConfirm{}, configPath)

	cfg := models.FailoverConfig{
		Enabled: true,
		Candidates: []models.FailoverCandidate{
			{
				ID:            "wan",
				Label:         "Ethernet WAN",
				InterfaceName: "wan",
				Kind:          models.FailoverCandidateKindEthernet,
				Available:     true,
				Enabled:       true,
				Priority:      1,
			},
			{
				ID:            "wwan",
				Label:         "WiFi uplink",
				InterfaceName: "wwan",
				Kind:          models.FailoverCandidateKindWiFi,
				Available:     true,
				Enabled:       true,
				Priority:      2,
			},
		},
		Health: models.FailoverHealthConfig{
			TrackIPs:         []string{"1.1.1.1", "8.8.8.8"},
			Reliability:      1,
			Count:            1,
			Timeout:          2,
			Interval:         10,
			FailureInterval:  5,
			RecoveryInterval: 5,
			Down:             3,
			Up:               3,
		},
	}

	svc.initScript = filepath.Join(tmpDir, "mwan3")
	if err := os.WriteFile(svc.initScript, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write init script: %v", err)
	}

	if err := svc.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig returned error: %v", err)
	}

	sections, err := mockUCI.GetSections("mwan3")
	if err != nil {
		t.Fatalf("GetSections returned error: %v", err)
	}
	if _, ok := sections["travo_failover"]; !ok {
		t.Fatalf("expected travo_failover policy section")
	}
	if _, ok := sections["travo_default_v4"]; !ok {
		t.Fatalf("expected travo_default_v4 rule section")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	var stored failoverConfigFile
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal stored config: %v", err)
	}
	if !stored.Enabled {
		t.Fatalf("expected stored config enabled")
	}
}

func TestFailoverServiceRejectsEnabledConfigWithoutCandidates(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})
	svc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc, &MockCommandRunner{}, &NoopUCIApplyConfirm{}, filepath.Join(t.TempDir(), "failover.json"))

	err := svc.SetConfig(models.FailoverConfig{
		Enabled:    true,
		Candidates: []models.FailoverCandidate{},
		Health:     defaultFailoverHealth(),
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateHealthConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		health  models.FailoverHealthConfig
		wantErr error
	}{
		{
			name: "valid config",
			health: models.FailoverHealthConfig{
				Timeout:          2,
				Interval:         5,
				FailureInterval:  3,
				RecoveryInterval: 3,
				Down:             3,
				Up:               3,
				Reliability:      1,
				Count:            1,
			},
			wantErr: nil,
		},
		{
			name: "zero timeout",
			health: models.FailoverHealthConfig{
				Timeout:          0,
				Interval:         5,
				FailureInterval:  3,
				RecoveryInterval: 3,
				Down:             3,
				Up:               3,
			},
			wantErr: errors.New("health timeout must be greater than 0"),
		},
		{
			name: "negative failure interval",
			health: models.FailoverHealthConfig{
				Timeout:          2,
				Interval:         5,
				FailureInterval:  -1,
				RecoveryInterval: 3,
				Down:             3,
				Up:               3,
			},
			wantErr: errors.New("health failure_interval must be non-negative"),
		},
		{
			name: "interval less than failure interval",
			health: models.FailoverHealthConfig{
				Timeout:          2,
				Interval:         3,
				FailureInterval:  5,
				RecoveryInterval: 3,
				Down:             3,
				Up:               3,
				Reliability:      1,
				Count:            1,
			},
			wantErr: errors.New("health interval (3) must be greater than failure_interval (5)"),
		},
		{
			name: "zero down",
			health: models.FailoverHealthConfig{
				Timeout:          2,
				Interval:         5,
				FailureInterval:  3,
				RecoveryInterval: 3,
				Down:             0,
				Up:               3,
			},
			wantErr: errors.New("health down must be greater than 0"),
		},
		{
			name: "zero reliability",
			health: models.FailoverHealthConfig{
				Timeout:          2,
				Interval:         5,
				FailureInterval:  3,
				RecoveryInterval: 3,
				Down:             3,
				Up:               3,
				Reliability:      0,
			},
			wantErr: errors.New("health reliability must be greater than 0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateHealthConfig(tt.health)
			if tt.wantErr != nil && err == nil {
				t.Errorf("expected error %v", tt.wantErr)
			}
			if tt.wantErr != nil && err != nil && !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestFailbackHoldDown(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})

	now := time.Now()
	oldTime := now.Add(-time.Minute)

	candidates := []models.FailoverCandidate{
		{
			Enabled:       true,
			Available:     true,
			Priority:      1,
			InterfaceName: "wan",
			IsUp:          true,
			TrackingState: models.FailoverTrackingStateOnline,
		},
		{
			Enabled:       true,
			Available:     true,
			Priority:      2,
			InterfaceName: "wwan",
			IsUp:          true,
			TrackingState: models.FailoverTrackingStateOnline,
		},
	}

	tests := []struct {
		name        string
		onlineSince map[string]time.Time
		wantActive  string
		description string
	}{
		{
			name:        "wan has been online for hold-down duration",
			onlineSince: map[string]time.Time{"wan": oldTime, "wwan": oldTime},
			wantActive:  "wan",
			description: "Higher priority interface (wan) has been stable for >30 seconds",
		},
		{
			name:        "wan just came online, wwan has been stable",
			onlineSince: map[string]time.Time{"wan": now, "wwan": oldTime},
			wantActive:  "wwan",
			description: "Lower priority interface (wwan) remains active while higher priority wan is within hold-down period",
		},
		{
			name:        "both interfaces recently online within hold-down",
			onlineSince: map[string]time.Time{"wan": now, "wwan": now},
			wantActive:  "",
			description: "No interface has exceeded hold-down period yet",
		},
		{
			name:        "wan disabled, wwan stable",
			onlineSince: map[string]time.Time{"wan": oldTime, "wwan": oldTime},
			wantActive:  "wwan",
			description: "Disabled candidate (wan) is excluded from consideration",
		},
		{
			name:        "wan not available, wwan stable",
			onlineSince: map[string]time.Time{"wwan": oldTime},
			wantActive:  "wwan",
			description: "Unavailable candidate (wan) is excluded from consideration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testCandidates := make([]models.FailoverCandidate, len(candidates))
			copy(testCandidates, candidates)

			if tt.name == "wan disabled, wwan stable" {
				for i := range testCandidates {
					if testCandidates[i].InterfaceName == "wan" {
						testCandidates[i].Enabled = false
						break
					}
				}
			}

			if tt.name == "wan not available, wwan stable" {
				for i := range testCandidates {
					if testCandidates[i].InterfaceName == "wan" {
						testCandidates[i].Available = false
						break
					}
				}
			}

			testSvc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc, &MockCommandRunner{}, &NoopUCIApplyConfirm{}, filepath.Join(t.TempDir(), "failover.json"))
			testSvc.onlineSince = tt.onlineSince

			gotActive := testSvc.computeActiveInterface(testCandidates)
			if gotActive != tt.wantActive {
				t.Errorf("%s: computeActiveInterface() = %v, want %v", tt.description, gotActive, tt.wantActive)
			}
		})
	}
}

func TestFailbackImmediateFailover(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})
	configPath := filepath.Join(t.TempDir(), "failover.json")
	svc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc, &MockCommandRunner{}, &NoopUCIApplyConfirm{}, configPath)

	now := time.Now()

	candidates := []models.FailoverCandidate{
		{
			Enabled:       true,
			Available:     true,
			Priority:      1,
			InterfaceName: "wan",
			IsUp:          false,
			TrackingState: models.FailoverTrackingStateOffline,
		},
		{
			Enabled:       true,
			Available:     true,
			Priority:      2,
			InterfaceName: "wwan",
			IsUp:          true,
			TrackingState: models.FailoverTrackingStateOnline,
		},
	}

	oldTime := now.Add(-time.Minute)

	svc.onlineSince = map[string]time.Time{
		"wwan": oldTime,
	}

	active := svc.computeActiveInterface(candidates)
	if active != "wwan" {
		t.Errorf("Immediate failover to lower priority: got %v, want wwan", active)
	}
}

func TestFailbackPriorityOrderingWithHoldDown(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})

	now := time.Now()
	oldTime := now.Add(-time.Minute)

	tests := []struct {
		name        string
		candidates  []models.FailoverCandidate
		onlineSince map[string]time.Time
		wantActive  string
	}{
		{
			name: "multiple candidates, lowest priority stable only",
			candidates: []models.FailoverCandidate{
				{Priority: 1, InterfaceName: "wan", Enabled: true, Available: true, IsUp: true, TrackingState: models.FailoverTrackingStateOnline},
				{Priority: 2, InterfaceName: "wwan", Enabled: true, Available: true, IsUp: true, TrackingState: models.FailoverTrackingStateOnline},
				{Priority: 3, InterfaceName: "usb0", Enabled: true, Available: true, IsUp: true, TrackingState: models.FailoverTrackingStateOnline},
			},
			onlineSince: map[string]time.Time{"wan": now, "wwan": now, "usb0": oldTime},
			wantActive:  "usb0",
		},
		{
			name: "middle priority stable, lowest and highest online but too recent",
			candidates: []models.FailoverCandidate{
				{Priority: 1, InterfaceName: "wan", Enabled: true, Available: true, IsUp: true, TrackingState: models.FailoverTrackingStateOnline},
				{Priority: 2, InterfaceName: "wwan", Enabled: true, Available: true, IsUp: true, TrackingState: models.FailoverTrackingStateOnline},
				{Priority: 3, InterfaceName: "usb0", Enabled: true, Available: true, IsUp: true, TrackingState: models.FailoverTrackingStateOnline},
			},
			onlineSince: map[string]time.Time{"wan": now, "wwan": oldTime, "usb0": now},
			wantActive:  "wwan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc, &MockCommandRunner{}, &NoopUCIApplyConfirm{}, filepath.Join(t.TempDir(), "failover.json"))
			svc.onlineSince = tt.onlineSince

			gotActive := svc.computeActiveInterface(tt.candidates)
			if gotActive != tt.wantActive {
				t.Errorf("computeActiveInterface() = %v, want %v", gotActive, tt.wantActive)
			}
		})
	}
}

// recordingApplier captures the rpcd apply/confirm calls and detects overlap.
type recordingApplier struct {
	mu           sync.Mutex
	startCalls   [][]string
	confirmCalls []string
	startErr     error
	confirmErr   error
	inFlight     int
	maxInFlight  int
}

func (r *recordingApplier) StartApply(configs []string) (string, error) {
	r.mu.Lock()
	r.startCalls = append(r.startCalls, slices.Clone(configs))
	r.inFlight++
	if r.inFlight > r.maxInFlight {
		r.maxInFlight = r.inFlight
	}
	r.mu.Unlock()

	if r.startErr != nil {
		r.mu.Lock()
		r.inFlight--
		r.mu.Unlock()
		return "", r.startErr
	}
	time.Sleep(20 * time.Millisecond)
	r.mu.Lock()
	r.inFlight--
	r.mu.Unlock()
	return "sess-1", nil
}

func (r *recordingApplier) Confirm(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.confirmCalls = append(r.confirmCalls, sessionID)
	return r.confirmErr
}

func (r *recordingApplier) ApplyAndConfirm(configs []string) error {
	if _, err := r.StartApply(configs); err != nil {
		return err
	}
	return r.Confirm("sess-1")
}

func (r *recordingApplier) calls() (starts [][]string, confirms []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.startCalls), slices.Clone(r.confirmCalls)
}

// newFailoverTestService builds a FailoverService with temp paths and mwan3 installed.
func newFailoverTestService(t *testing.T, mockUCI uci.UCI, applier UCIApplyConfirm) (*FailoverService, string) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "failover.json")
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})
	svc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc, &MockCommandRunner{}, applier, configPath)
	svc.initScript = filepath.Join(tmpDir, "mwan3")
	if err := os.WriteFile(svc.initScript, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write init script: %v", err)
	}
	return svc, tmpDir
}

// failoverTestHealth is a valid health config (interval must exceed failure_interval).
func failoverTestHealth() models.FailoverHealthConfig {
	return models.FailoverHealthConfig{
		TrackIPs:         []string{"1.1.1.1", "8.8.8.8"},
		Reliability:      1,
		Count:            1,
		Timeout:          2,
		Interval:         10,
		FailureInterval:  5,
		RecoveryInterval: 5,
		Down:             3,
		Up:               3,
	}
}

func failoverTestConfig(candidates ...models.FailoverCandidate) models.FailoverConfig {
	return models.FailoverConfig{
		Enabled:    true,
		Candidates: candidates,
		Health:     failoverTestHealth(),
	}
}

func TestFailoverServiceSetConfigSerializesConcurrentApplies(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, mockUCI, applier)

	cfg := failoverTestConfig(
		models.FailoverCandidate{InterfaceName: "wan", Kind: models.FailoverCandidateKindEthernet, Available: true, Enabled: true, Priority: 1},
		models.FailoverCandidate{InterfaceName: "wwan", Kind: models.FailoverCandidateKindWiFi, Available: true, Enabled: true, Priority: 2},
	)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.SetConfig(cfg)
		}()
	}
	wg.Wait()

	applier.mu.Lock()
	maxInFlight := applier.maxInFlight
	applier.mu.Unlock()
	if maxInFlight > 1 {
		t.Errorf("concurrent SetConfig calls overlapped in the apply stage (max in flight = %d)", maxInFlight)
	}
	starts, confirms := applier.calls()
	if len(starts) == 0 {
		t.Fatal("expected at least one staged apply")
	}
	if len(confirms) != len(starts) {
		t.Errorf("every started apply must be confirmed: starts=%d confirms=%d", len(starts), len(confirms))
	}
}

func TestFailoverServiceSetConfigConfirmsOnlyAfterVerify(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, mockUCI, applier)

	cfg := failoverTestConfig(
		models.FailoverCandidate{InterfaceName: "wan", Kind: models.FailoverCandidateKindEthernet, Available: true, Enabled: true, Priority: 1},
		models.FailoverCandidate{InterfaceName: "wwan", Kind: models.FailoverCandidateKindWiFi, Available: true, Enabled: true, Priority: 2},
	)
	if err := svc.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	starts, confirms := applier.calls()
	if len(starts) == 0 {
		t.Fatal("expected a staged rpcd apply")
	}
	if !slices.Equal(starts[0], mwan3UCIConfigs) {
		t.Errorf("expected staged configs %v, got %v", mwan3UCIConfigs, starts[0])
	}
	if len(confirms) != len(starts) {
		t.Errorf("expected every staged apply to be confirmed after verification: starts=%d confirms=%d", len(starts), len(confirms))
	}
}

func TestFailoverStagedApplyKeepsRollbackWindowOnVerifyFailure(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, mockUCI, applier)

	verifyErr := errors.New("config is not readable")
	err := svc.stagedApplyMwan3(func() error { return verifyErr })
	if err == nil {
		t.Fatal("expected staged apply to fail when verification fails")
	}
	_, confirms := applier.calls()
	if len(confirms) != 0 {
		t.Errorf("must not confirm when verification failed (rollback window must stay open), got %v", confirms)
	}
}

func TestFailoverRestoreManagedSectionsReloadsThroughStagedApply(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	_ = mockUCI.AddSection("mwan3", "wan", "interface")
	_ = mockUCI.Set("mwan3", "wan", "proto", "dhcp")
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, mockUCI, applier)

	backup, err := json.Marshal(map[string]map[string]string{
		"wan": {".type": "interface", "proto": "dhcp", "family": "ipv4"},
	})
	if err != nil {
		t.Fatalf("marshal backup: %v", err)
	}
	if err := os.WriteFile(svc.backupPath, backup, 0600); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	if err := svc.restoreManagedSections(); err != nil {
		t.Fatalf("restoreManagedSections: %v", err)
	}
	if family, _ := mockUCI.Get("mwan3", "wan", "family"); family != "ipv4" {
		t.Errorf("expected restored wan section, family = %q", family)
	}
	starts, confirms := applier.calls()
	if len(starts) == 0 {
		t.Fatal("restore must reload mwan3 through a staged apply, not just commit")
	}
	if len(confirms) == 0 {
		t.Fatal("restore must confirm the staged apply after verification")
	}
}

func TestVerifyApplyRequiresEveryEnabledCandidate(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})

	// usb0 is enabled and available but has no runtime interface, so a config
	// that "works" for wan/wwan must not verify clean.
	cfg := failoverConfigFile{
		Enabled: true,
		Candidates: []models.FailoverCandidate{
			{InterfaceName: "wan", Available: true, Enabled: true, Priority: 1},
			{InterfaceName: "wwan", Available: true, Enabled: true, Priority: 2},
			{InterfaceName: "usb0", Available: true, Enabled: true, Priority: 3},
		},
		Health: defaultFailoverHealth(),
	}
	_ = mockUCI.AddSection("mwan3", "travo_failover", "policy")
	_ = mockUCI.AddSection("mwan3", "travo_default_v4", "rule")

	if err := svc.verifyApply(cfg); err == nil {
		t.Fatal("expected verifyApply to fail while an enabled candidate is missing at runtime")
	}
}

func TestVerifyApplyAcceptsAllEnabledCandidatesPresent(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
	_ = mockUCI.AddSection("mwan3", "travo_failover", "policy")
	_ = mockUCI.AddSection("mwan3", "travo_default_v4", "rule")

	cfg := failoverConfigFile{
		Enabled: true,
		Candidates: []models.FailoverCandidate{
			{InterfaceName: "wan", Available: true, Enabled: true, Priority: 1},
			{InterfaceName: "wwan", Available: true, Enabled: true, Priority: 2},
		},
		Health: defaultFailoverHealth(),
	}
	if err := svc.verifyApply(cfg); err != nil {
		t.Fatalf("verifyApply: %v", err)
	}
}

func TestSetConfigRemovesSectionsOfDroppedCandidates(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	// Pre-existing mwan3 state: two candidates plus a hand-written policy.
	_ = mockUCI.AddSection("mwan3", "wan", "interface")
	_ = mockUCI.Set("mwan3", "wan", "proto", "dhcp")
	_ = mockUCI.AddSection("mwan3", "usb0", "interface")
	_ = mockUCI.Set("mwan3", "usb0", "proto", "dhcp")
	_ = mockUCI.AddSection("mwan3", "my_custom_policy", "policy")
	_ = mockUCI.Set("mwan3", "my_custom_policy", "use_member", "travo_wan_p1")

	svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
	cfg := failoverTestConfig(
		models.FailoverCandidate{InterfaceName: "wan", Kind: models.FailoverCandidateKindEthernet, Available: true, Enabled: true, Priority: 1},
	)
	if err := svc.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	sections, err := mockUCI.GetSections("mwan3")
	if err != nil {
		t.Fatalf("GetSections: %v", err)
	}
	if _, ok := sections["usb0"]; ok {
		t.Error("mwan3 interface section of a dropped candidate must be removed")
	}
	if _, ok := sections["my_custom_policy"]; !ok {
		t.Error("unmanaged mwan3 sections must not be touched")
	}

	backupData, err := os.ReadFile(svc.backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	var backup map[string]map[string]string
	if err := json.Unmarshal(backupData, &backup); err != nil {
		t.Fatalf("unmarshal backup: %v", err)
	}
	if _, ok := backup["usb0"]; !ok {
		t.Error("dropped candidate must be restorable from the backup")
	}
}

func TestNormalizeHealthPreservesExplicitZeroIntervals(t *testing.T) {
	t.Parallel()

	health := normalizeHealth(models.FailoverHealthConfig{
		TrackIPs:         []string{"1.1.1.1"},
		Reliability:      2,
		Count:            2,
		Timeout:          3,
		Interval:         10,
		FailureInterval:  0,
		RecoveryInterval: 0,
		Down:             4,
		Up:               4,
	})
	if health.FailureInterval != 0 {
		t.Errorf("explicit failure_interval 0 must be preserved, got %d", health.FailureInterval)
	}
	if health.RecoveryInterval != 0 {
		t.Errorf("explicit recovery_interval 0 must be preserved, got %d", health.RecoveryInterval)
	}
}

func TestNormalizeStoredHealthDefaultsMissingKeys(t *testing.T) {
	t.Parallel()

	def := defaultFailoverHealth()
	got := normalizeStoredHealth(models.FailoverHealthConfig{Interval: 10}, healthKeyPresence{})
	if got.FailureInterval != def.FailureInterval {
		t.Errorf("missing failure_interval must fall back to the default, got %d", got.FailureInterval)
	}
	if got.RecoveryInterval != def.RecoveryInterval {
		t.Errorf("missing recovery_interval must fall back to the default, got %d", got.RecoveryInterval)
	}

	got = normalizeStoredHealth(models.FailoverHealthConfig{Interval: 10, FailureInterval: 0}, healthKeyPresence{failureInterval: true})
	if got.FailureInterval != 0 {
		t.Errorf("stored failure_interval 0 must be preserved, got %d", got.FailureInterval)
	}
}

func TestLoadConfigFilePreservesStoredZeroFailureInterval(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "failover.json")
	stored := `{"enabled":true,"candidates":[],"health":{"interval":10,"failure_interval":0,"recovery_interval":0}}`
	if err := os.WriteFile(configPath, []byte(stored), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	mockUCI := uci.NewMockUCI()
	svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
	svc.configPath = configPath

	cfg, err := svc.loadConfigFile()
	if err != nil {
		t.Fatalf("loadConfigFile: %v", err)
	}
	if cfg.Health.FailureInterval != 0 {
		t.Errorf("stored failure_interval 0 must survive a reload, got %d", cfg.Health.FailureInterval)
	}
}

func TestSetConfigWithDisabledCandidateStillVerifies(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, mockUCI, applier)

	cfg := failoverTestConfig(
		models.FailoverCandidate{InterfaceName: "wan", Kind: models.FailoverCandidateKindEthernet, Available: true, Enabled: true, Priority: 1},
		models.FailoverCandidate{InterfaceName: "wwan", Kind: models.FailoverCandidateKindWiFi, Available: true, Enabled: false, Priority: 2},
	)
	if err := svc.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig with a disabled candidate: %v", err)
	}
	sections, err := mockUCI.GetSections("mwan3")
	if err != nil {
		t.Fatalf("GetSections: %v", err)
	}
	if _, ok := sections["wwan"]; !ok {
		t.Error("a disabled candidate still gets an mwan3 interface section (enabled=0)")
	}
	if _, ok := sections["travo_wwan_p2"]; ok {
		t.Error("a disabled candidate must not get a policy member")
	}
}
