package services

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestWifiScan(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewWifiService(u, ub)

	results, err := svc.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) < 3 {
		t.Errorf("expected at least 3 results, got %d", len(results))
	}
	if results[0].SSID == "" {
		t.Error("expected non-empty SSID")
	}
}

func TestWifiConnect(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewWifiService(u, ub)

	err := svc.Connect(models.WifiConfig{
		SSID: "Test-Network", Password: "testpass", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("wireless", "sta0", "ssid")
	if val != "Test-Network" {
		t.Errorf("expected ssid 'Test-Network', got %q", val)
	}
}

func TestWifiGetConnection(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewWifiService(u, ub)

	conn, err := svc.GetConnection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !conn.Connected {
		t.Error("expected connected=true")
	}
	if conn.SSID != "Hotel-WiFi" {
		t.Errorf("expected SSID 'Hotel-WiFi', got %q", conn.SSID)
	}
}

func TestWifiSetMode(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewWifiService(u, ub)

	err := svc.SetMode("sta")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("wireless", "default_radio0", "mode")
	if val != "sta" {
		t.Errorf("expected mode 'sta', got %q", val)
	}
}

func TestWifiGetSavedNetworks(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewWifiService(u, ub)

	networks, err := svc.GetSavedNetworks()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(networks) == 0 {
		t.Error("expected at least one saved network")
	}
	if networks[0].SSID != "Hotel-WiFi" {
		t.Errorf("expected SSID 'Hotel-WiFi', got %q", networks[0].SSID)
	}
}
