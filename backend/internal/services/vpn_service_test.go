package services

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetVpnStatus(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	statuses, err := svc.GetVpnStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) < 2 {
		t.Errorf("expected at least 2 statuses, got %d", len(statuses))
	}

	found := false
	for _, s := range statuses {
		if s.Type == "wireguard" {
			found = true
			// wg0 is disabled=1 in mock, so should not be enabled
			if s.Enabled {
				t.Error("expected wireguard not enabled (disabled=1 in mock)")
			}
		}
	}
	if !found {
		t.Error("expected wireguard status")
	}
}

func TestGetWireguardConfig(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	config, err := svc.GetWireguardConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.PrivateKey == "" {
		t.Error("expected non-empty private key")
	}
	if config.Address == "" {
		t.Error("expected non-empty address")
	}
	if len(config.Peers) == 0 {
		t.Error("expected at least one peer")
	}
}

func TestToggleWireguard(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	err := svc.ToggleWireguard(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "0" {
		t.Errorf("expected disabled=0, got %q", val)
	}
}

func TestGetTailscaleStatus(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	status, err := svc.GetTailscaleStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Installed {
		t.Error("expected tailscale not installed")
	}
}
