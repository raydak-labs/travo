package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// wanCandidate is the enabled priority-1 ethernet candidate the save tests use.
func wanCandidate() models.FailoverCandidate {
	return models.FailoverCandidate{
		InterfaceName: "wan", Kind: models.FailoverCandidateKindEthernet,
		Available: true, Enabled: true, Priority: 1,
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
	// A legacy generated section: the restore must delete it as Travo-owned and
	// put the backup back in its place.
	seedLegacyGeneratedInterface(t, mockUCI, "wan")
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
	candidates := []models.FailoverCandidate{
		{InterfaceName: "wan", Available: true, Enabled: true, Priority: 1},
		{InterfaceName: "wwan", Available: true, Enabled: true, Priority: 2},
	}
	seedGeneratedManagedSections(t, mockUCI, candidates)

	cfg := failoverConfigFile{
		Enabled:    true,
		Candidates: candidates,
		Health:     defaultFailoverHealth(),
	}
	if err := svc.verifyApply(cfg); err != nil {
		t.Fatalf("verifyApply: %v", err)
	}
}

// seedGeneratedManagedSections writes the mwan3 sections applyManagedConfig
// generates for a candidate list, so verifyApply can be exercised on its own.
func seedGeneratedManagedSections(t *testing.T, u uci.UCI, candidates []models.FailoverCandidate) {
	t.Helper()
	for _, section := range []struct{ name, stype string }{
		{failoverPolicySection, "policy"},
		{failoverRuleSection, "rule"},
	} {
		if err := u.AddSection("mwan3", section.name, section.stype); err != nil {
			t.Fatalf("AddSection(%s): %v", section.name, err)
		}
	}
	for _, candidate := range candidates {
		iface := failoverInterfaceSection(candidate.InterfaceName)
		if err := u.AddSection("mwan3", iface, "interface"); err != nil {
			t.Fatalf("AddSection(%s): %v", iface, err)
		}
		if err := u.Set("mwan3", iface, "family", "ipv4"); err != nil {
			t.Fatalf("Set(%s.family): %v", iface, err)
		}
		if !candidate.Enabled {
			continue
		}
		member := fmt.Sprintf("travo_%s_p%d",
			failoverSectionName(candidate.InterfaceName), candidate.Priority)
		if err := u.AddSection("mwan3", member, "member"); err != nil {
			t.Fatalf("AddSection(%s): %v", member, err)
		}
		if err := u.Set("mwan3", member, "interface", iface); err != nil {
			t.Fatalf("Set(%s.interface): %v", member, err)
		}
	}
}

// mwan3 members reference the mwan3 INTERFACE SECTION, so a member left
// pointing at a pre-namespacing name ("wan") binds the policy to a section this
// service no longer writes. Presence-only verification passed that in
// production; only the unit test noticed.
func TestVerifyApplyRejectsStaleGeneratedSectionValues(t *testing.T) {
	t.Parallel()

	candidates := []models.FailoverCandidate{
		{InterfaceName: "wan", Available: true, Enabled: true, Priority: 1},
	}
	cfg := failoverConfigFile{
		Enabled:    true,
		Candidates: candidates,
		Health:     defaultFailoverHealth(),
	}

	t.Run("member references a non-namespaced section", func(t *testing.T) {
		t.Parallel()
		mockUCI := uci.NewMockUCI()
		svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
		seedGeneratedManagedSections(t, mockUCI, candidates)
		if err := mockUCI.Set("mwan3", "travo_wan_p1", "interface", "wan"); err != nil {
			t.Fatalf("Set member interface: %v", err)
		}

		err := svc.verifyApply(cfg)
		if err == nil {
			t.Fatal("verifyApply accepted a member pointing at a stale section name")
		}
		if !strings.Contains(err.Error(), "travo_wan_p1") {
			t.Errorf("the failure must name the offending member, got %v", err)
		}
	})

	t.Run("generated interface section lost its family", func(t *testing.T) {
		t.Parallel()
		mockUCI := uci.NewMockUCI()
		svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
		seedGeneratedManagedSections(t, mockUCI, candidates)
		if err := mockUCI.DeleteOption("mwan3", failoverIfWan, "family"); err != nil {
			t.Fatalf("DeleteOption: %v", err)
		}

		err := svc.verifyApply(cfg)
		if err == nil {
			t.Fatal("verifyApply accepted a generated interface section without family")
		}
		if !strings.Contains(err.Error(), "family") {
			t.Errorf("the failure must name the offending option, got %v", err)
		}
	})
}

func TestSetConfigRemovesSectionsOfDroppedCandidates(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	// Pre-existing mwan3 state: two generated candidates written by an older
	// build (sections named after the network interface), a hand-written
	// interface section and a hand-written policy.
	seedLegacyGeneratedInterface(t, mockUCI, "wan")
	seedLegacyGeneratedInterface(t, mockUCI, "usb0")
	_ = mockUCI.AddSection("mwan3", "hotel", "interface")
	_ = mockUCI.Set("mwan3", "hotel", "ifname", "eth3")
	_ = mockUCI.Set("mwan3", "hotel", "metric", "10")
	_ = mockUCI.AddSection("mwan3", "my_custom_policy", "policy")
	_ = mockUCI.Set("mwan3", "my_custom_policy", "use_member", "travo_wan_p1")

	svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
	cfg := failoverTestConfig(wanCandidate())
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
	if _, ok := sections["wan"]; ok {
		t.Error("a legacy generated interface section must be cleaned up by the next save")
	}
	if _, ok := sections[failoverIfWan]; !ok {
		t.Errorf("expected the namespaced section %s, got %v",
			failoverIfWan, slices.Sorted(maps.Keys(sections)))
	}
	member, _ := mockUCI.Get("mwan3", "travo_wan_p1", "interface")
	if member != failoverIfWan {
		t.Errorf("member must reference the mwan3 interface section, got %q", member)
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

// A hand-written mwan3 interface section must survive a save. It used to be
// deleted on every save, with no error and no log line, and the backup that
// could have restored it is only read on the failure path — so mwan3 silently
// stopped tracking that uplink.
func TestSetConfigKeepsForeignInterfaceSections(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	_ = mockUCI.AddSection("mwan3", "hotel", "interface")
	_ = mockUCI.Set("mwan3", "hotel", "ifname", "eth3")
	_ = mockUCI.Set("mwan3", "hotel", "metric", "10")
	_ = mockUCI.Set("mwan3", "hotel", "track_ip", "1.1.1.1")

	svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
	cfg := failoverTestConfig(wanCandidate())
	if err := svc.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	sections, err := mockUCI.GetSections("mwan3")
	if err != nil {
		t.Fatalf("GetSections: %v", err)
	}
	opts, ok := sections["hotel"]
	if !ok {
		t.Fatalf("hand-written mwan3 interface section was deleted by a failover save")
	}
	if opts["ifname"] != "eth3" {
		t.Errorf("hand-written section was modified: %v", opts)
	}
}

func TestIsManagedSectionRecognisesOnlyTravoSections(t *testing.T) {
	t.Parallel()

	generated := map[string]string{".type": "interface"}
	for _, option := range generatedInterfaceOptions {
		generated[option] = "1"
	}
	// A hand-written section copied from the stock mwan3 example: it carries
	// most, but not all, of the options this service writes.
	handWrittenExample := map[string]string{
		".type": "interface", "enabled": "1", "family": "ipv4", "count": "1",
		"timeout": "2", "interval": "5", "failure_interval": "5",
		"recovery_interval": "5", "down": "3", "up": "3", "track_ip": "1.1.1.1",
	}
	legacyNames := legacyGeneratedNames([]models.FailoverCandidate{{InterfaceName: "wwan"}})
	cases := []struct {
		name string
		opts map[string]string
		want bool
	}{
		{"travo_if_wan", map[string]string{".type": "interface"}, true},
		{"travo_failover", map[string]string{".type": "policy"}, true},
		{"hotel", map[string]string{".type": "interface", "ifname": "eth3"}, false},
		{"my_custom_policy", map[string]string{".type": "policy"}, false},
		{"wan", generated, true},  // legacy generated section, cleaned up once
		{"wwan", generated, true}, // legacy section named after a candidate
		// A candidate-named section this service never wrote must survive even
		// though it looks like an mwan3 interface section.
		{"wan", handWrittenExample, false},
		{"wwan", handWrittenExample, false},
		// The option signature alone is not enough any more either: the name has
		// to be one a generated section could have had.
		{"eth9", generated, false},
	}
	for _, tc := range cases {
		if got := isManagedSection(tc.name, tc.opts, legacyNames); got != tc.want {
			t.Errorf("isManagedSection(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A hand-written mwan3 interface section must survive a save. Option shape alone
// cannot tell it from a legacy generated one: the stock mwan3 example sets
// exactly the options this service writes, so a user who copied it into a section
// of their own ("office") was treated as generated and deleted — and the backup
// that could restore it is only read on the failure path, so mwan3 silently
// stopped tracking that uplink. Keying the legacy cleanup on the known generated
// names, with the option signature as the secondary condition, fixes both
// directions: a section outside those names survives whatever it contains, and a
// candidate-named section survives unless it carries the signature.
func TestSetConfigKeepsHandWrittenInterfaceSections(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	// wwan is a candidate in this save and also the name of a hand-written mwan3
	// interface section: a stock example copied over and renamed after the
	// uplink it tracks. It is missing one of the options this service writes.
	exampleOptions := []string{"enabled", "family", "count", "timeout",
		"interval", "failure_interval", "recovery_interval", "down", "up"}
	if err := mockUCI.AddSection("mwan3", "wwan", "interface"); err != nil {
		t.Fatalf("AddSection: %v", err)
	}
	for i, option := range exampleOptions {
		if err := mockUCI.Set("mwan3", "wwan", option, fmt.Sprintf("%d", i+1)); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if err := mockUCI.Set("mwan3", "wwan", "track_ip", "1.1.1.1"); err != nil {
		t.Fatalf("Set track_ip: %v", err)
	}
	// office carries the FULL option signature but is not a name this service
	// ever generated, so no build of it could have written that section.
	seedLegacyGeneratedInterface(t, mockUCI, "office")

	svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
	cfg := failoverTestConfig(
		wanCandidate(),
		models.FailoverCandidate{InterfaceName: "wwan", Kind: models.FailoverCandidateKindWiFi,
			Available: true, Enabled: true, Priority: 2},
	)
	if err := svc.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	sections, err := mockUCI.GetSections("mwan3")
	if err != nil {
		t.Fatalf("GetSections: %v", err)
	}
	opts, ok := sections["wwan"]
	if !ok {
		t.Fatalf("hand-written mwan3 section named after a candidate was deleted by a save, got %v",
			slices.Sorted(maps.Keys(sections)))
	}
	if opts["track_ip"] != "1.1.1.1" {
		t.Errorf("hand-written section was modified: %v", opts)
	}
	if _, ok := sections["office"]; !ok {
		t.Errorf("a hand-written section outside the generated names was deleted by a save, got %v",
			slices.Sorted(maps.Keys(sections)))
	}
	if _, ok := sections[failoverInterfaceSection("wwan")]; !ok {
		t.Errorf("the generated section must still be written next to it, got %v",
			slices.Sorted(maps.Keys(sections)))
	}
}

// The counterpart: a section carrying this service's full option signature under
// a candidate's name was written by a pre-namespacing build, and the upgrade
// path still has to clean it up.
func TestSetConfigCleansLegacyGeneratedInterfaceOfACandidate(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	seedLegacyGeneratedInterface(t, mockUCI, "wwan")

	svc, _ := newFailoverTestService(t, mockUCI, &NoopUCIApplyConfirm{})
	cfg := failoverTestConfig(
		wanCandidate(),
		models.FailoverCandidate{InterfaceName: "wwan", Kind: models.FailoverCandidateKindWiFi,
			Available: true, Enabled: true, Priority: 2},
	)
	if err := svc.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	sections, err := mockUCI.GetSections("mwan3")
	if err != nil {
		t.Fatalf("GetSections: %v", err)
	}
	if _, ok := sections["wwan"]; ok {
		t.Error("a legacy generated interface section named after a candidate must be cleaned up")
	}
	if _, ok := sections[failoverInterfaceSection("wwan")]; !ok {
		t.Errorf("expected the namespaced replacement, got %v", slices.Sorted(maps.Keys(sections)))
	}
}

// A config-write failure happens before anything is mutated. It must not apply
// anything and must not leave a crash guard behind: the old code ran a full
// delete + restore + rpcd apply here, unguarded, for a change that never
// started (an ENOSPC on /etc/travo).
func TestSetConfigConfigWriteFailureAppliesNothingAndLeavesNoGuard(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	applier := &recordingApplier{}
	tmpDir := t.TempDir()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})
	// A directory where the config file belongs: every write of it fails, for
	// any user, including root.
	blocker := filepath.Join(tmpDir, "failover.json")
	if err := os.Mkdir(blocker, 0o750); err != nil {
		t.Fatalf("create blocking directory: %v", err)
	}
	svc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc,
		&MockCommandRunner{}, applier, blocker)
	svc.initScript = filepath.Join(tmpDir, "mwan3")
	if err := os.WriteFile(svc.initScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write init script: %v", err)
	}

	cfg := failoverTestConfig(wanCandidate())
	if err := svc.SetConfig(cfg); err == nil {
		t.Fatal("expected SetConfig to fail when the config cannot be written")
	}
	if starts, confirms := applier.calls(); len(starts) != 0 || len(confirms) != 0 {
		t.Errorf("no rpcd apply may happen for a change that never started: %v / %v", starts, confirms)
	}
	if _, err := os.Stat(svc.guardPath); err == nil {
		t.Errorf("no crash guard may be written before the first mutation: %s", svc.guardPath)
	}
	if sections, _ := mockUCI.GetSections("mwan3"); len(sections) != 0 {
		t.Errorf("mwan3 must be untouched when the save never started, got %v",
			slices.Sorted(maps.Keys(sections)))
	}
}

func TestReadTrackerStatesMapsNamespacedSections(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: func(name string, _ ...string) ([]byte, error) {
		if name == "mwan3" {
			return []byte("interface travo_if_wan is online\ninterface hotel is offline\n"), nil
		}
		return nil, nil
	}}
	tmpDir := t.TempDir()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(mockUCI, mockUbus, &MockCommandRunner{})
	configPath := filepath.Join(tmpDir, "failover.json")
	svc := NewFailoverServiceWithRunner(mockUCI, mockUbus, networkSvc, cmd,
		&NoopUCIApplyConfirm{}, configPath)
	svc.initScript = filepath.Join(tmpDir, "mwan3")
	if err := os.WriteFile(svc.initScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write init script: %v", err)
	}

	states := svc.readTrackerStates()
	if states["wan"] != models.FailoverTrackingStateOnline {
		t.Errorf("expected the namespaced section to report as network interface wan, got %v", states)
	}
	if states["hotel"] != models.FailoverTrackingStateOffline {
		t.Errorf("a hand-written mwan3 section must be reported under its own name, got %v", states)
	}
}

// failoverIfWan is the mwan3 interface section name for the wan candidate.
const failoverIfWan = failoverInterfaceSectionPrefix + "wan"

// seedLegacyGeneratedInterface writes an mwan3 interface section the way a
// pre-namespacing build did: named after the network interface, carrying every
// option FailoverService writes.
func seedLegacyGeneratedInterface(t *testing.T, u uci.UCI, name string) {
	t.Helper()
	if err := u.AddSection("mwan3", name, "interface"); err != nil {
		t.Fatalf("AddSection(%s): %v", name, err)
	}
	for _, option := range generatedInterfaceOptions {
		if err := u.Set("mwan3", name, option, "1"); err != nil {
			t.Fatalf("Set(%s.%s): %v", name, option, err)
		}
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
	if _, ok := sections[failoverInterfaceSection("wwan")]; !ok {
		t.Errorf("a disabled candidate still gets an interface section (enabled=0), got %v",
			slices.Sorted(maps.Keys(sections)))
	}
	if _, ok := sections["travo_wwan_p2"]; ok {
		t.Error("a disabled candidate must not get a policy member")
	}
}

// rpcd allows only one pending rollback at a time and rejects a second
// rollback-enabled apply while the first is unconfirmed. After a failed verify
// the window is still open, so the next apply — including a restore attempt
// inside the same save — must not be issued. It must also be recognised by the
// next save rather than surfacing as an opaque "permission denied" from rpcd.
func TestStagedApplyMwan3_SkipsWhileARollbackIsPending(t *testing.T) {
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, uci.NewMockUCI(), applier)

	// First call: StartApply succeeds, verification fails, window stays open.
	if err := svc.stagedApplyMwan3(func() error { return fmt.Errorf("verification failed") }); err == nil {
		t.Fatal("expected the failed verification to be reported")
	}
	if svc.pendingApplySession == "" {
		t.Fatal("an unconfirmed apply must stay recorded while its rollback window is open")
	}
	first := len(applier.startCalls)

	// Second call while that window is open: refused, not re-issued, and
	// reported as a failure. Returning nil here acknowledged a save that rpcd
	// was about to revert, so the caller dropped its crash guard and answered
	// 200 for a change that never became live.
	if err := svc.stagedApplyMwan3(nil); !errors.Is(err, errApplyRollingBack) {
		t.Fatalf("a save inside a pending rollback window must fail with errApplyRollingBack, got %v", err)
	}
	if len(applier.startCalls) != first {
		t.Errorf("a second StartApply was issued while a rollback was pending (%d -> %d)", first, len(applier.startCalls))
	}
}

// Once the pending apply is confirmed the window is closed and applies resume.
func TestStagedApplyMwan3_ResumesAfterConfirm(t *testing.T) {
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, uci.NewMockUCI(), applier)

	if err := svc.stagedApplyMwan3(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.pendingApplySession != "" {
		t.Errorf("a confirmed apply must not stay pending, got %q", svc.pendingApplySession)
	}
	first := len(applier.startCalls)
	if err := svc.stagedApplyMwan3(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applier.startCalls) != first+1 {
		t.Errorf("applies should resume after a confirm (%d -> %d)", first, len(applier.startCalls))
	}
}

// rpcd rolls a failed apply back on its own timer and never tells us, so the
// record of the unconfirmed session has to expire on its own. Otherwise it
// survives the rollback and every later failover apply is skipped for the life
// of the process.
func TestStagedApplyMwan3_ResumesAfterTheRollbackWindowExpires(t *testing.T) {
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, uci.NewMockUCI(), applier)

	if err := svc.stagedApplyMwan3(func() error { return fmt.Errorf("verification failed") }); err == nil {
		t.Fatal("expected the failed verification to be reported")
	}
	if svc.pendingApplySession == "" {
		t.Fatal("an unconfirmed apply must stay recorded while its rollback window is open")
	}
	first := len(applier.startCalls)

	// Still inside the window: refused.
	if err := svc.stagedApplyMwan3(nil); !errors.Is(err, errApplyRollingBack) {
		t.Fatalf("a save inside the rollback window must report errApplyRollingBack, got %v", err)
	}
	if len(applier.startCalls) != first {
		t.Fatalf("an apply was issued while the rollback window was still open")
	}

	// Once the window has passed, applies must resume.
	svc.pendingApplyDeadline = time.Now().Add(-time.Second)
	if err := svc.stagedApplyMwan3(nil); err != nil {
		t.Fatalf("unexpected error after the rollback window: %v", err)
	}
	if len(applier.startCalls) != first+1 {
		t.Errorf("applies did not resume after the rollback window expired (%d -> %d)", first, len(applier.startCalls))
	}
}

// The crash guard is the whole point of a guarded live-state write: it must be
// present while a save is in flight and gone once the save succeeded.
//
// The interesting case is the one that regressed. An apply that STARTS and then
// fails verification leaves a pending rpcd session, and the restore that follows
// runs while that window is still open. The restore is the recovery path, so it
// must not be refused just because a window is open — and because it succeeded,
// the guard must be cleared, or Start() skips every monitoring tick until a
// manual rm or a redeploy and failover silently stops working.
func TestSetConfigGuardLifecycle(t *testing.T) {
	t.Parallel()

	guardExists := func(svc *FailoverService) bool {
		_, err := os.Stat(svc.guardPath)
		return err == nil
	}
	candidate := failoverTestConfig(models.FailoverCandidate{
		InterfaceName: "wan", Priority: 1, Enabled: true,
	})

	t.Run("removed after a successful save", func(t *testing.T) {
		t.Parallel()
		svc, _ := newFailoverTestService(t, uci.NewMockUCI(), &recordingApplier{})
		if err := svc.SetConfig(candidate); err != nil {
			t.Fatalf("SetConfig: %v", err)
		}
		if guardExists(svc) {
			t.Errorf("a successful save must not leave %s behind", svc.guardPath)
		}
	})

	t.Run("restore completes while a rollback window is open", func(t *testing.T) {
		t.Parallel()
		applier := &recordingApplier{}
		svc, _ := newFailoverTestService(t, uci.NewMockUCI(), applier)
		if err := svc.SetConfig(candidate); err != nil {
			t.Fatalf("seed save: %v", err)
		}

		// Arm the window exactly the way a failed in-apply verification does:
		// StartApply succeeds and records the session, the verify callback then
		// fails, so the session stays pending and unconfirmed.
		verifyErr := fmt.Errorf("candidate is not readable at runtime")
		if err := svc.stagedApplyMwan3(func() error { return verifyErr }); err == nil {
			t.Fatal("expected the failed verification to be reported")
		}
		if svc.pendingApplySession == "" {
			t.Fatal("an unconfirmed apply must stay recorded while its rollback window is open")
		}

		// A save now correctly refuses...
		if err := svc.stagedApplyMwan3(nil); !errors.Is(err, errApplyRollingBack) {
			t.Fatalf("a save inside an open window must fail with errApplyRollingBack, got %v", err)
		}
		// ...but the restore, which is the recovery path, must still go through.
		if err := svc.restoreManagedSections(); err != nil {
			t.Errorf("the restore must not be refused by its own rollback window: %v", err)
		}
	})

	// The clear-on-success branch, pinned directly. A real save first, so the
	// mwan3/network backup the restore reads exists on disk; the guard is then
	// re-armed to stand in for "a save is in flight". With a working applier the
	// restore completes, so the guard must go.
	t.Run("guard cleared when the rollback succeeds", func(t *testing.T) {
		t.Parallel()
		svc, _ := newFailoverTestService(t, uci.NewMockUCI(), &recordingApplier{})
		if err := svc.SetConfig(candidate); err != nil {
			t.Fatalf("seed save: %v", err)
		}
		if err := os.WriteFile(svc.guardPath, []byte("marker"), 0o600); err != nil {
			t.Fatalf("seed guard: %v", err)
		}

		svc.rollbackOrKeepGuard()

		if guardExists(svc) {
			t.Errorf("a successful rollback must clear %s: the running config is known-good again", svc.guardPath)
		}
	})

	// The other keep-condition, pinned directly. The guard must not be dropped
	// while rpcd still has a rollback armed for the apply that just failed: ~30s
	// later rpcd would drop the post-restore config back in. The check has to
	// read the pending state BEFORE the restore, because the restore's own apply
	// clears it.
	t.Run("kept while the failed apply's rollback window is armed", func(t *testing.T) {
		t.Parallel()
		svc, _ := newFailoverTestService(t, uci.NewMockUCI(), &recordingApplier{})
		if err := svc.SetConfig(candidate); err != nil {
			t.Fatalf("seed save: %v", err)
		}
		verifyErr := fmt.Errorf("config is not readable")
		if err := svc.stagedApplyMwan3(func() error { return verifyErr }); err == nil {
			t.Fatal("expected the failed verification to be reported")
		}
		armedSession := svc.pendingApplySession
		if armedSession == "" {
			t.Fatal("an unconfirmed apply must stay recorded while its rollback window is open")
		}
		if err := os.WriteFile(svc.guardPath, []byte("marker"), 0o600); err != nil {
			t.Fatalf("seed guard: %v", err)
		}

		svc.rollbackOrKeepGuard()

		if !guardExists(svc) {
			t.Errorf("the guard must be kept while rpcd rollback %s is still armed: "+
				"its timer would drop the restored config back in", armedSession)
		}
	})

	t.Run("kept when the rollback itself fails", func(t *testing.T) {
		t.Parallel()
		// rpcd is unavailable: the apply fails, and so does the restore that
		// ends in the same apply. The running config is then unknown, so the
		// guard has to stay.
		applier := &recordingApplier{startErr: fmt.Errorf("rpcd unavailable")}
		svc, _ := newFailoverTestService(t, uci.NewMockUCI(), applier)

		if err := svc.SetConfig(failoverTestConfig(models.FailoverCandidate{
			InterfaceName: "wan", Priority: 1, Enabled: true,
		})); err == nil {
			t.Fatal("expected the failed apply to be reported")
		}
		if !guardExists(svc) {
			t.Errorf("a failed rollback must keep %s: the running config is unknown", svc.guardPath)
		}
	})
}

// ADR 0010: mwan3UCIConfigs includes `network`, and the rpcd apply snapshots
// and reloads it, so the whole save must hold the network + mwan3 config locks
// across that blocking work instead of racing every other writer of `network`.
func TestSetConfigHoldsTheNetworkAndMwan3ConfigLocks(t *testing.T) {
	t.Parallel()

	svc, _ := newFailoverTestService(t, uci.NewMockUCI(), &NoopUCIApplyConfirm{})
	cfg := failoverTestConfig(wanCandidate())

	unlock := lockUCIConfigs("network")
	done := make(chan error, 1)
	go func() { done <- svc.SetConfig(cfg) }()

	select {
	case err := <-done:
		unlock()
		t.Fatalf("SetConfig must wait for the network config lock, returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	unlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SetConfig after the locks were released: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SetConfig did not resume after the locks were released")
	}
}
