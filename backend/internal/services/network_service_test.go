package services

import (
	"fmt"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// failingSetUCI wraps MockUCI but returns an error from Set.
type failingSetUCI struct {
	*uci.MockUCI
	setErr error
}

func (f *failingSetUCI) Set(_, _, _, _ string) error {
	return f.setErr
}

func TestGetNetworkStatus(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	status, err := svc.GetNetworkStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.WAN == nil {
		t.Fatal("expected WAN interface")
	}
	if !status.WAN.IsUp {
		t.Error("expected WAN to be up")
	}
	if status.WAN.IPAddress == "" {
		t.Error("expected WAN IP address")
	}
	if !status.InternetReachable {
		t.Error("expected internet reachable")
	}
	if len(status.Clients) == 0 {
		t.Error("expected clients")
	}
}

func TestGetWanConfig(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	config, err := svc.GetWanConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Type != "dhcp" {
		t.Errorf("expected type 'dhcp', got %q", config.Type)
	}
	if config.MTU != 1500 {
		t.Errorf("expected MTU 1500, got %d", config.MTU)
	}
}

func TestGetClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	clients, err := svc.GetClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) < 2 {
		t.Errorf("expected at least 2 clients, got %d", len(clients))
	}
}

func TestGetNetworkStatus_IncludesClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(u, ub)

	status, err := svc.GetNetworkStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Clients) < 2 {
		t.Fatalf("expected at least 2 clients from DHCP, got %d", len(status.Clients))
	}

	// Verify clients come from DHCP leases, not hardcoded
	foundLaptop := false
	foundPhone := false
	for _, c := range status.Clients {
		if c.Hostname == "laptop" && c.IPAddress == "192.168.8.100" && c.MACAddress == "AA:BB:CC:11:22:33" {
			foundLaptop = true
			if c.InterfaceName != "br-lan" {
				t.Errorf("expected laptop on br-lan, got %q", c.InterfaceName)
			}
		}
		if c.Hostname == "phone" && c.IPAddress == "192.168.8.101" {
			foundPhone = true
		}
	}
	if !foundLaptop {
		t.Error("expected to find laptop client from DHCP leases")
	}
	if !foundPhone {
		t.Error("expected to find phone client from DHCP leases")
	}
}

func TestGetNetworkStatus_NoDHCP_EmptyClients(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	// Override DHCP response with empty data
	ub.RegisterResponse("dhcp.ipv4leases", map[string]interface{}{
		"device": map[string]interface{}{},
	})
	svc := NewNetworkService(u, ub)

	status, err := svc.GetNetworkStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Clients) != 0 {
		t.Errorf("expected 0 clients when no DHCP leases, got %d", len(status.Clients))
	}
}

func TestSetWanConfigReturnsErrorOnSetFailure(t *testing.T) {
	fu := &failingSetUCI{
		MockUCI: uci.NewMockUCI(),
		setErr:  fmt.Errorf("mock set error"),
	}
	ub := ubus.NewMockUbus()
	svc := NewNetworkService(fu, ub)

	cfg := models.WanConfig{
		Type: "static",
	}
	err := svc.SetWanConfig(cfg)
	if err == nil {
		t.Error("expected error when uci.Set fails")
	}
}

func TestSetWanConfigPropagatesEachFieldError(t *testing.T) {
	tests := []struct {
		name   string
		config models.WanConfig
	}{
		{"Type", models.WanConfig{Type: "static"}},
		{"IPAddress", models.WanConfig{IPAddress: "10.0.0.1"}},
		{"Netmask", models.WanConfig{Netmask: "255.255.255.0"}},
		{"Gateway", models.WanConfig{Gateway: "10.0.0.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fu := &failingSetUCI{
				MockUCI: uci.NewMockUCI(),
				setErr:  fmt.Errorf("set failed"),
			}
			ub := ubus.NewMockUbus()
			svc := NewNetworkService(fu, ub)

			err := svc.SetWanConfig(tt.config)
			if err == nil {
				t.Errorf("expected error for field %s", tt.name)
			}
		})
	}
}
