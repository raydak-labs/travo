package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
