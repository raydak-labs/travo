package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
