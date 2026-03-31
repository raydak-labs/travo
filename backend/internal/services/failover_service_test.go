package services

import (
	"encoding/json"
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
	svc := NewFailoverServiceWithRunner(mockUCI, networkSvc, &MockCommandRunner{}, configPath)

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
	svc := NewFailoverServiceWithRunner(mockUCI, networkSvc, &MockCommandRunner{}, configPath)

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
			Interval:         5,
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
	svc := NewFailoverServiceWithRunner(mockUCI, networkSvc, &MockCommandRunner{}, filepath.Join(t.TempDir(), "failover.json"))

	err := svc.SetConfig(models.FailoverConfig{
		Enabled:    true,
		Candidates: []models.FailoverCandidate{},
		Health:     defaultFailoverHealth(),
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
