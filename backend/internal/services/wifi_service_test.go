package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func newTestWifiService() (*WifiService, *uci.MockUCI) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewWifiServiceWithReloader(u, ub, &NoopWifiReloader{})
	// Never let a test write crash guards to the real /etc/travo.
	svc.guardDir = testGuardDir()
	return svc, u
}

type fakeWirelessApplier struct {
	startToken   string
	startErr     error
	confirmErr   error
	applyErr     error
	started      [][]string
	confirmed    []string
	appliedCalls int
}

func (f *fakeWirelessApplier) StartApply(configs []string) (string, error) {
	f.started = append(f.started, append([]string(nil), configs...))
	if f.startErr != nil {
		return "", f.startErr
	}
	if f.startToken != "" {
		return f.startToken, nil
	}
	return "session-123", nil
}

func (f *fakeWirelessApplier) Confirm(token string) error {
	f.confirmed = append(f.confirmed, token)
	return f.confirmErr
}

func (f *fakeWirelessApplier) ApplyAndConfirm(configs []string) error {
	f.appliedCalls++
	if f.applyErr != nil {
		return f.applyErr
	}
	_, err := f.StartApply(configs)
	if err != nil {
		return err
	}
	return f.Confirm(f.startToken)
}

func TestWifiScan(t *testing.T) {
	svc, _ := newTestWifiService()

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
	svc, u := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Test-Network", Password: "testpass", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// New code creates a per-SSID section; sta0=Hotel-WiFi exists, so sta1 is allocated.
	val, _ := u.Get("wireless", "sta1", "ssid")
	if val != "Test-Network" {
		t.Errorf("expected ssid 'Test-Network' in sta1, got %q", val)
	}
	// The existing Hotel-WiFi profile in sta0 must remain (not deleted), just disabled.
	hotelSsid, _ := u.Get("wireless", "sta0", "ssid")
	if hotelSsid != "Hotel-WiFi" {
		t.Errorf("expected sta0 ssid to remain 'Hotel-WiFi', got %q", hotelSsid)
	}
	disabled, _ := u.Get("wireless", "sta0", "disabled")
	if disabled != "1" {
		t.Errorf("expected sta0 disabled='1' after connecting to different network, got %q", disabled)
	}
}

func TestWifiConnect_ReusesSectionForSameSSID(t *testing.T) {
	svc, u := newTestWifiService()

	// Connecting to the already-saved Hotel-WiFi must reuse sta0, not create sta1.
	_, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", Password: "newpass", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	key, _ := u.Get("wireless", "sta0", "key")
	if key != "newpass" {
		t.Errorf("expected sta0 key updated to 'newpass', got %q", key)
	}
	// sta1 must not have been created.
	if ssid, err := u.Get("wireless", "sta1", "ssid"); err == nil {
		t.Errorf("unexpected sta1 created with ssid=%q", ssid)
	}
}

func TestWifiConnect_ReuseSavedProfile_EmptyPasswordKeepsKey(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", Password: "", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	key, _ := u.Get("wireless", "sta0", "key")
	if key != "hotelpass" {
		t.Errorf("expected sta0 key unchanged, got %q", key)
	}
	disabled, _ := u.Get("wireless", "sta0", "disabled")
	if disabled != "0" {
		t.Errorf("expected sta0 enabled, got disabled=%q", disabled)
	}
}

func TestWifiConnect_NewSecuredSSID_RequiresPassword(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Brand-New-Secured-Net", Password: "", Encryption: "psk2",
	})
	if !errors.Is(err, ErrPasswordRequiredForNewSTA) {
		t.Fatalf("expected ErrPasswordRequiredForNewSTA, got %v", err)
	}
}

func TestWifiConnect_NewSTA_RequiresEncryption(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "No-Enc-New-Net", Password: "abcdefgh", Encryption: "",
	})
	if !errors.Is(err, ErrEncryptionRequiredForNewSTA) {
		t.Fatalf("expected ErrEncryptionRequiredForNewSTA, got %v", err)
	}
}

func TestWifiConnect_ReusePreservesHidden(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "sta0", "hidden", "1"); err != nil {
		t.Fatalf("set hidden: %v", err)
	}

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", Password: "", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h, _ := u.Get("wireless", "sta0", "hidden")
	if h != "1" {
		t.Errorf("expected hidden preserved as 1, got %q", h)
	}
}

func TestWifiConnect_ReusePreservesEncryptionWhenPasswordEmpty(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", Password: "", Encryption: "psk2+ccmp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enc, _ := u.Get("wireless", "sta0", "encryption")
	if enc != "psk2" {
		t.Errorf("expected encryption unchanged as psk2, got %q", enc)
	}
}

func TestWifiGetConnection(t *testing.T) {
	svc, _ := newTestWifiService()

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
	svc, u := newTestWifiService()

	_, err := svc.SetMode("client")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("wireless", "default_radio0", "disabled")
	if val != "1" {
		t.Errorf("expected default_radio0 disabled='1', got %q", val)
	}
}

func TestWifiGetSavedNetworks(t *testing.T) {
	svc, _ := newTestWifiService()

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

func TestWifiDisconnect(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.Disconnect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("wireless", "wifinet2", "disabled")
	if val != "1" {
		t.Errorf("expected disabled='1', got %q", val)
	}
}

func TestWifiDisconnectThenReconnect(t *testing.T) {
	svc, u := newTestWifiService()

	// Disconnect — sta0 (Hotel-WiFi) is the active STA section in ubus/UCI.
	if _, err := svc.Disconnect(); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
	// findSTADevice returns section="wifinet2" from ubus mock; UCI sets disabled on that name.
	val, _ := u.Get("wireless", "wifinet2", "disabled")
	if val != "1" {
		t.Errorf("expected disabled='1' after disconnect, got %q", val)
	}

	// Reconnect to a new SSID — a fresh sta1 section is created.
	_, err := svc.Connect(models.WifiConfig{
		SSID: "New-Network", Password: "newpass123", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	// sta1 should hold the new network.
	val, _ = u.Get("wireless", "sta1", "ssid")
	if val != "New-Network" {
		t.Errorf("expected sta1 ssid 'New-Network', got %q", val)
	}
	activeDisabled, _ := u.Get("wireless", "sta1", "disabled")
	if activeDisabled != "0" {
		t.Errorf("expected sta1 disabled='0', got %q", activeDisabled)
	}
}

func TestWifiDeleteNetwork(t *testing.T) {
	svc, u := newTestWifiService()

	// Verify the section exists first
	_, err := u.Get("wireless", "sta0", "ssid")
	if err != nil {
		t.Fatalf("expected sta0 section to exist: %v", err)
	}

	// Delete the network
	_, err = svc.DeleteNetwork("sta0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify section is gone
	_, err = u.Get("wireless", "sta0", "ssid")
	if err == nil {
		t.Error("expected sta0 section to be deleted")
	}
}

func TestWifiConnect_MultipleProfilesPersist(t *testing.T) {
	svc, u := newTestWifiService()

	// Connect to a first new network (sta0=Hotel-WiFi exists, so sta1 is created).
	if _, err := svc.Connect(models.WifiConfig{SSID: "Coffee-Shop", Password: "coffee123", Encryption: "psk2"}); err != nil {
		t.Fatalf("connect Coffee-Shop: %v", err)
	}
	// Connect to a second new network (sta1=Coffee-Shop now exists, so sta2 is created).
	if _, err := svc.Connect(models.WifiConfig{SSID: "Airport-WiFi", Password: "air456", Encryption: "psk2"}); err != nil {
		t.Fatalf("connect Airport-WiFi: %v", err)
	}

	// All three SSID profiles must be present in UCI.
	if ssid, _ := u.Get("wireless", "sta0", "ssid"); ssid != "Hotel-WiFi" {
		t.Errorf("sta0 ssid=%q, want Hotel-WiFi", ssid)
	}
	if ssid, _ := u.Get("wireless", "sta1", "ssid"); ssid != "Coffee-Shop" {
		t.Errorf("sta1 ssid=%q, want Coffee-Shop", ssid)
	}
	if ssid, _ := u.Get("wireless", "sta2", "ssid"); ssid != "Airport-WiFi" {
		t.Errorf("sta2 ssid=%q, want Airport-WiFi", ssid)
	}
	// Only the last-connected network (sta2) should be enabled.
	if d, _ := u.Get("wireless", "sta0", "disabled"); d != "1" {
		t.Errorf("sta0 should be disabled, got disabled=%q", d)
	}
	if d, _ := u.Get("wireless", "sta1", "disabled"); d != "1" {
		t.Errorf("sta1 should be disabled, got disabled=%q", d)
	}
	if d, _ := u.Get("wireless", "sta2", "disabled"); d != "0" {
		t.Errorf("sta2 should be enabled, got disabled=%q", d)
	}
}

func TestWifiDisconnectFallsBackToUCI(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	// Register wireless status with no STA interfaces (simulating disabled STA)
	ub.RegisterResponse("network.wireless.status", map[string]any{
		"radio0": map[string]any{
			"interfaces": []any{},
		},
	})
	svc := NewWifiServiceWithReloader(u, ub, &NoopWifiReloader{})

	_, err := svc.Disconnect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// findSTASection should find "sta0" in mock UCI
	val, _ := u.Get("wireless", "sta0", "disabled")
	if val != "1" {
		t.Errorf("expected disabled='1', got %q", val)
	}
}

func TestWifiConnectFallsBackToUCI(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	// Register wireless status with no STA interfaces (simulating disabled STA)
	ub.RegisterResponse("network.wireless.status", map[string]any{
		"radio0": map[string]any{
			"interfaces": []any{},
		},
	})
	svc := NewWifiServiceWithReloader(u, ub, &NoopWifiReloader{})

	_, err := svc.Connect(models.WifiConfig{
		SSID: "New-Network", Password: "newpass123", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// sta0 exists with Hotel-WiFi; new code creates sta1 for the new SSID.
	val, _ := u.Get("wireless", "sta1", "ssid")
	if val != "New-Network" {
		t.Errorf("expected sta1 ssid 'New-Network', got %q", val)
	}
	val, _ = u.Get("wireless", "sta1", "disabled")
	if val != "0" {
		t.Errorf("expected sta1 disabled='0', got %q", val)
	}
	// Hotel-WiFi profile must remain saved but disabled.
	hotelDisabled, _ := u.Get("wireless", "sta0", "disabled")
	if hotelDisabled != "1" {
		t.Errorf("expected sta0 disabled='1', got %q", hotelDisabled)
	}
}

func TestWifiDeleteNetwork_EmptySection(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.DeleteNetwork("")
	if err == nil {
		t.Error("expected error for empty section")
	}
}

func TestWifiConnectHiddenNetwork(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Hidden-Net", Password: "secretpass", Encryption: "psk2", Hidden: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// New per-SSID section: sta0=Hotel-WiFi exists, so sta1 is created.
	val, _ := u.Get("wireless", "sta1", "ssid")
	if val != "Hidden-Net" {
		t.Errorf("expected sta1 ssid 'Hidden-Net', got %q", val)
	}
	val, _ = u.Get("wireless", "sta1", "hidden")
	if val != "1" {
		t.Errorf("expected hidden='1', got %q", val)
	}
}

func TestWifiConnectNonHiddenNetwork(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Visible-Net", Password: "secretpass", Encryption: "psk2", Hidden: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("wireless", "sta1", "hidden")
	if val != "0" {
		t.Errorf("expected sta1 hidden='0', got %q", val)
	}
}

func TestWifiConnect_NormalizesMissingNetworkToWwan(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Visible-Net", Password: "secretpass", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sta1 is the new section; it must have network=wwan.
	val, _ := u.Get("wireless", "sta1", "network")
	if val != "wwan" {
		t.Errorf("expected sta1 network='wwan', got %q", val)
	}
}

func TestWifiConnect_ReturnsPendingApplyWhenApplierConfigured(t *testing.T) {
	svc, _ := newTestWifiService()
	fake := &fakeWirelessApplier{startToken: "apply-123"}
	svc.applier = fake

	apply, err := svc.Connect(models.WifiConfig{
		SSID: "Test-Network", Password: "testpass", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apply == nil || apply.Token != "apply-123" {
		t.Fatalf("expected pending apply token apply-123, got %#v", apply)
	}
	if len(fake.started) != 1 {
		t.Fatalf("expected exactly one staged apply, got %d", len(fake.started))
	}
	if !slices.Contains(fake.started[0], "firewall") || !slices.Contains(fake.started[0], "dhcp") {
		t.Errorf("expected staged apply configs to include firewall and dhcp, got %v", fake.started[0])
	}
}

func TestWifiConnect_ReturnsApplyError(t *testing.T) {
	svc, _ := newTestWifiService()
	svc.applier = &fakeWirelessApplier{startErr: errors.New("apply failed")}

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Test-Network", Password: "testpass", Encryption: "psk2",
	})
	if err == nil || !strings.Contains(err.Error(), "apply failed") {
		t.Fatalf("expected apply error, got %v", err)
	}
}

// TestWifiConnect_ReconcilesSameRadioAPInRepeaterMode verifies that Connect() atomically
// disables the AP sharing the STA's radio when in repeater mode with ≥2 radios.
//
// The bug: without this fix, Connect() committed AP+STA on the same PHY, which crashes
// the ath11k/IPQ6018 driver. The health check then showed a notification requiring a
// second user-triggered "Fix radio layout" apply. That second apply could itself disconnect
// the user (if on 2.4GHz) before they could confirm, triggering a rollback and infinite loop.
//
// With the fix: the AP on the STA radio is disabled in the same UCI commit as the STA
// activation, so no intermediate broken state is ever applied.
func TestWifiConnect_ReconcilesSameRadioAPInRepeaterMode(t *testing.T) {
	// Default mock: radio0 (2G) hosts default_radio0 (AP) + sta0 (STA "Hotel-WiFi").
	// radio1 (5G) hosts default_radio1 (AP). Both APs are enabled; mode = "repeater".
	svc, u := newTestWifiService()

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", // reuses existing sta0 section on radio0
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// AP on the STA's radio (radio0) must be disabled to prevent ath11k crash.
	d0, _ := u.Get("wireless", "default_radio0", "disabled")
	if d0 != "1" {
		t.Errorf("default_radio0 (AP on STA radio0): want disabled='1', got %q — AP+STA same-radio not reconciled", d0)
	}

	// AP on the other radio (radio1) must remain enabled: downlink AP stays reachable.
	d1, _ := u.Get("wireless", "default_radio1", "disabled")
	if d1 == "1" {
		t.Errorf("default_radio1 (AP on radio1): should remain enabled for downlink, got disabled='1'")
	}

	// The STA itself must be enabled after connect.
	sta, _ := u.Get("wireless", "sta0", "disabled")
	if sta != "0" {
		t.Errorf("sta0: want disabled='0', got %q", sta)
	}
}

func TestWifiDeleteNetwork_NonexistentSection(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.DeleteNetwork("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent section")
	}
}

func TestGetRadios(t *testing.T) {
	svc, _ := newTestWifiService()

	radios, err := svc.GetRadios()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(radios) < 2 {
		t.Fatalf("expected at least 2 radios, got %d", len(radios))
	}

	// Collect radios by name for deterministic checks
	byName := map[string]models.RadioInfo{}
	for _, r := range radios {
		byName[r.Name] = r
	}

	r0, ok := byName["radio0"]
	if !ok {
		t.Fatal("expected radio0")
	}
	if r0.Band != "2g" {
		t.Errorf("radio0 band: expected '2g', got %q", r0.Band)
	}
	if r0.Channel != 6 {
		t.Errorf("radio0 channel: expected 6, got %d", r0.Channel)
	}
	if r0.HTMode != "HT20" {
		t.Errorf("radio0 htmode: expected 'HT20', got %q", r0.HTMode)
	}
	if r0.Type != "mac80211" {
		t.Errorf("radio0 type: expected 'mac80211', got %q", r0.Type)
	}
	if r0.Disabled {
		t.Error("radio0 should not be disabled")
	}

	r1, ok := byName["radio1"]
	if !ok {
		t.Fatal("expected radio1")
	}
	if r1.Band != "5g" {
		t.Errorf("radio1 band: expected '5g', got %q", r1.Band)
	}
	if r1.Channel != 36 {
		t.Errorf("radio1 channel: expected 36, got %d", r1.Channel)
	}
	if r1.HTMode != "VHT80" {
		t.Errorf("radio1 htmode: expected 'VHT80', got %q", r1.HTMode)
	}
}

func TestGetAPConfigs(t *testing.T) {
	svc, _ := newTestWifiService()

	configs, err := svc.GetAPConfigs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) < 2 {
		t.Fatalf("expected at least 2 AP configs, got %d", len(configs))
	}
	found2g := false
	found5g := false
	for _, c := range configs {
		if c.Band == "2g" {
			found2g = true
			if c.SSID != "OpenWrt-Travel" {
				t.Errorf("expected 2g SSID 'OpenWrt-Travel', got '%s'", c.SSID)
			}
		}
		if c.Band == "5g" {
			found5g = true
			if c.SSID != "OpenWrt-Travel-5G" {
				t.Errorf("expected 5g SSID 'OpenWrt-Travel-5G', got '%s'", c.SSID)
			}
		}
	}
	if !found2g {
		t.Error("expected to find 2g AP config")
	}
	if !found5g {
		t.Error("expected to find 5g AP config")
	}
}

func TestSetAPConfig(t *testing.T) {
	svc, _ := newTestWifiService()

	// STA is on radio0 in mock; repeater reconcile keeps AP on radio0 off. Update the 5 GHz AP.
	_, err := svc.SetAPConfig("default_radio1", models.APConfigUpdate{
		SSID:       "MyTravelRouter",
		Encryption: "psk2",
		Key:        "newpassword123",
		Enabled:    models.BoolPtr(true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configs, err := svc.GetAPConfigs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var found *models.APConfig
	for _, c := range configs {
		if c.Section == "default_radio1" {
			found = &c
			break
		}
	}
	if found == nil {
		t.Fatal("expected to find default_radio1 config")
	}
	if found.SSID != "MyTravelRouter" {
		t.Errorf("expected SSID 'MyTravelRouter', got '%s'", found.SSID)
	}
	if !found.Enabled {
		t.Error("expected AP to be enabled")
	}
}

func TestSetAPConfig_OmitEnabledPreservesDisabled(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "disabled", "1")

	_, err := svc.SetAPConfig("default_radio0", models.APConfigUpdate{
		SSID:       "OnlySSID",
		Encryption: "psk2",
		Key:        "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dis, _ := u.Get("wireless", "default_radio0", "disabled")
	if dis != "1" {
		t.Fatalf("expected AP to stay disabled, got disabled=%q", dis)
	}
	ssid, _ := u.Get("wireless", "default_radio0", "ssid")
	if ssid != "OnlySSID" {
		t.Fatalf("expected SSID updated, got %q", ssid)
	}
}

func TestSetAPConfig_InvalidSection(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.SetAPConfig("nonexistent", models.APConfigUpdate{
		SSID:       "Test",
		Encryption: "none",
		Enabled:    models.BoolPtr(true),
	})
	if err == nil {
		t.Error("expected error for nonexistent section")
	}
}

func TestSetAPConfig_ReturnsApplyError(t *testing.T) {
	svc, _ := newTestWifiService()
	svc.applier = &fakeWirelessApplier{startErr: errors.New("apply failed")}

	_, err := svc.SetAPConfig("default_radio0", models.APConfigUpdate{
		SSID:       "MyTravelRouter",
		Encryption: "psk2",
		Key:        "newpassword123",
		Enabled:    models.BoolPtr(true),
	})
	if err == nil || !strings.Contains(err.Error(), "apply failed") {
		t.Fatalf("expected apply error, got %v", err)
	}
}

func TestSetAPConfig_RepeaterDisablesSTARadioAPAfterEnableAttempt(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "sta0", "device", "radio0")
	if _, err := svc.SetMode("repeater"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	if ap0 != "1" {
		t.Fatalf("precondition: expected AP on STA radio disabled, got %q", ap0)
	}

	if _, err := svc.SetAPConfig("default_radio0", models.APConfigUpdate{
		SSID:       "Unified",
		Encryption: "psk2",
		Key:        "password12",
		Enabled:    models.BoolPtr(true),
	}); err != nil {
		t.Fatalf("SetAPConfig: %v", err)
	}
	ap0, _ = u.Get("wireless", "default_radio0", "disabled")
	if ap0 != "1" {
		t.Errorf("expected AP on STA radio to stay disabled after reconcile, got disabled=%q", ap0)
	}
	ssid, _ := u.Get("wireless", "default_radio0", "ssid")
	if ssid != "Unified" {
		t.Errorf("expected SSID updated on STA-radio section for when user turns allow_ap on later, got %q", ssid)
	}
}

func TestSetAPConfig_APModeSkipsRepeaterReconcile(t *testing.T) {
	svc, u := newTestWifiService()
	svc.modeFile = filepath.Join(t.TempDir(), "wifi-mode")
	if err := os.WriteFile(svc.modeFile, []byte("ap"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = u.Set("wireless", "sta0", "disabled", "1")

	if _, err := svc.SetAPConfig("default_radio0", models.APConfigUpdate{
		SSID:       "APOnly",
		Encryption: "psk2",
		Key:        "password12",
		Enabled:    models.BoolPtr(true),
	}); err != nil {
		t.Fatalf("SetAPConfig: %v", err)
	}
	dis, _ := u.Get("wireless", "default_radio0", "disabled")
	if dis == "1" {
		t.Error("expected AP on radio0 enabled in ap mode")
	}
}

// newMACUnitTestService is newTestWifiService with the live-link commands
// stubbed. applyMACImmediate now REPORTS a failed `ip link` instead of logging
// it, and the shared helper keeps the real command runner: on a dev machine
// "ip link set phy0-sta0 down" fails because the interface does not exist,
// which says nothing about the code under test.
func newMACUnitTestService() (*WifiService, *uci.MockUCI) {
	svc, u := newTestWifiService()
	svc.cmd = &MockCommandRunner{}
	return svc, u
}

func TestSetMACAddress(t *testing.T) {
	svc, u := newMACUnitTestService()

	// Input is canonicalized to lowercase colon notation.
	_, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify MAC was set
	opts, err := u.GetAll("wireless", "sta0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts["macaddr"] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected macaddr 'aa:bb:cc:dd:ee:ff', got '%s'", opts["macaddr"])
	}
}

func TestSetMACAddress_Reset(t *testing.T) {
	svc, u := newMACUnitTestService()

	// Set then reset
	_, _ = svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	_, err := svc.SetMACAddress("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts, err := u.GetAll("wireless", "sta0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts["macaddr"] != "" {
		t.Errorf("expected empty macaddr, got '%s'", opts["macaddr"])
	}
}

func TestGetMACAddresses(t *testing.T) {
	svc, _ := newTestWifiService()

	configs, err := svc.GetMACAddresses()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) == 0 {
		t.Fatal("expected at least one MAC config")
	}
	if configs[0].Interface != "sta" {
		t.Errorf("expected interface 'sta', got '%s'", configs[0].Interface)
	}
}

func TestRandomizeMAC(t *testing.T) {
	svc, u := newMACUnitTestService()

	mac, _, err := svc.RandomizeMAC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify MAC format (XX:XX:XX:XX:XX:XX)
	if len(mac) != 17 {
		t.Fatalf("expected 17 char MAC, got %d: %s", len(mac), mac)
	}

	// Parse first octet to check locally-administered + unicast
	var firstOctet int
	if _, err := fmt.Sscanf(mac[:2], "%x", &firstOctet); err != nil {
		t.Fatalf("failed to parse first octet: %v", err)
	}
	if firstOctet&0x02 == 0 {
		t.Error("expected locally-administered bit set (bit 1 of first octet)")
	}
	if firstOctet&0x01 != 0 {
		t.Error("expected unicast bit cleared (bit 0 of first octet)")
	}

	// Verify MAC was applied in UCI
	opts, err := u.GetAll("wireless", "sta0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts["macaddr"] != mac {
		t.Errorf("expected macaddr '%s', got '%s'", mac, opts["macaddr"])
	}
}

func TestRandomizeMAC_UniquePerCall(t *testing.T) {
	svc, _ := newMACUnitTestService()

	mac1, _, err := svc.RandomizeMAC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mac2, _, err := svc.RandomizeMAC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Extremely unlikely to be the same with 46 bits of randomness
	if mac1 == mac2 {
		t.Errorf("expected different MACs, both were %s", mac1)
	}
}

func TestGetGuestWifi_NotConfigured(t *testing.T) {
	svc, _ := newTestWifiService()

	cfg, err := svc.GetGuestWifi()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected guest wifi to be disabled when not configured")
	}
}

func TestSetGuestWifi_Enable(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled:    true,
		SSID:       "Guest-Travel",
		Encryption: "psk2",
		Key:        "guestpass123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify wireless guest section
	opts, err := u.GetAll("wireless", "guest")
	if err != nil {
		t.Fatalf("expected wireless.guest to exist: %v", err)
	}
	if opts["ssid"] != "Guest-Travel" {
		t.Errorf("expected ssid 'Guest-Travel', got %q", opts["ssid"])
	}
	if opts["isolate"] != "1" {
		t.Errorf("expected isolate '1', got %q", opts["isolate"])
	}
	if opts["disabled"] != "0" {
		t.Errorf("expected disabled '0', got %q", opts["disabled"])
	}
	if opts["network"] != "guest" {
		t.Errorf("expected network 'guest', got %q", opts["network"])
	}

	// Verify network.guest interface
	netOpts, err := u.GetAll("network", "guest")
	if err != nil {
		t.Fatalf("expected network.guest to exist: %v", err)
	}
	if netOpts["ipaddr"] != "192.168.2.1" {
		t.Errorf("expected ipaddr '192.168.2.1', got %q", netOpts["ipaddr"])
	}

	// Verify dhcp.guest
	dhcpOpts, err := u.GetAll("dhcp", "guest")
	if err != nil {
		t.Fatalf("expected dhcp.guest to exist: %v", err)
	}
	if dhcpOpts["interface"] != "guest" {
		t.Errorf("expected dhcp interface 'guest', got %q", dhcpOpts["interface"])
	}

	// Verify firewall guest zone
	fwOpts, err := u.GetAll("firewall", "guest_zone")
	if err != nil {
		t.Fatalf("expected firewall.guest_zone to exist: %v", err)
	}
	if fwOpts["forward"] != "REJECT" {
		t.Errorf("expected forward 'REJECT', got %q", fwOpts["forward"])
	}

	// Verify guest->wan forwarding
	fwdOpts, err := u.GetAll("firewall", "guest_fwd")
	if err != nil {
		t.Fatalf("expected firewall.guest_fwd to exist: %v", err)
	}
	if fwdOpts["dest"] != "wan" {
		t.Errorf("expected forwarding dest 'wan', got %q", fwdOpts["dest"])
	}

	// Verify GetGuestWifi returns correct config
	cfg, err := svc.GetGuestWifi()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected guest wifi to be enabled")
	}
	if cfg.SSID != "Guest-Travel" {
		t.Errorf("expected SSID 'Guest-Travel', got %q", cfg.SSID)
	}
}

func TestSetGuestWifi_Disable(t *testing.T) {
	svc, u := newTestWifiService()

	// Enable first
	_, _ = svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled:    true,
		SSID:       "Guest-Travel",
		Encryption: "psk2",
		Key:        "guestpass123",
	})

	// Disable
	_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, err := u.Get("wireless", "guest", "disabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "1" {
		t.Errorf("expected disabled '1', got %q", val)
	}

	cfg, _ := svc.GetGuestWifi()
	if cfg.Enabled {
		t.Error("expected guest wifi to be disabled")
	}
}

func TestSetGuestWifi_DisableWhenNotConfigured(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetGuestWifi_ReturnsApplyError(t *testing.T) {
	svc, _ := newTestWifiService()
	svc.applier = &fakeWirelessApplier{startErr: errors.New("apply failed")}

	_, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled:    true,
		SSID:       "Guest-Travel",
		Encryption: "psk2",
		Key:        "guestpass123",
	})
	if err == nil || !strings.Contains(err.Error(), "apply failed") {
		t.Fatalf("expected apply error, got %v", err)
	}
}

func TestGetRadioStatus_Enabled(t *testing.T) {
	svc, _ := newTestWifiService()

	enabled, err := svc.GetRadioStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Error("expected radios to be enabled by default")
	}
}

func TestGetRadioStatus_AllDisabled(t *testing.T) {
	svc, u := newTestWifiService()

	_ = u.Set("wireless", "radio0", "disabled", "1")
	_ = u.Set("wireless", "radio1", "disabled", "1")

	enabled, err := svc.GetRadioStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Error("expected radios to be disabled")
	}
}

func TestSetRadioEnabled_Disable(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.SetRadioEnabled(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val0, _ := u.Get("wireless", "radio0", "disabled")
	val1, _ := u.Get("wireless", "radio1", "disabled")
	if val0 != "1" {
		t.Errorf("expected radio0 disabled='1', got %q", val0)
	}
	if val1 != "1" {
		t.Errorf("expected radio1 disabled='1', got %q", val1)
	}
}

func TestSetRadioEnabled_Enable(t *testing.T) {
	svc, u := newTestWifiService()

	// First disable
	_, _ = svc.SetRadioEnabled(false)
	// Then enable
	_, err := svc.SetRadioEnabled(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val0, _ := u.Get("wireless", "radio0", "disabled")
	val1, _ := u.Get("wireless", "radio1", "disabled")
	if val0 != "0" {
		t.Errorf("expected radio0 disabled='0', got %q", val0)
	}
	if val1 != "0" {
		t.Errorf("expected radio1 disabled='0', got %q", val1)
	}
}

func TestSetRadioEnabled_UsesDynamicRadioDiscovery(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "radio2", "type", "mac80211")
	_ = u.Set("wireless", "radio2", "band", "6g")
	_ = u.Set("wireless", "radio2", "disabled", "0")

	_, err := svc.SetRadioEnabled(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val2, _ := u.Get("wireless", "radio2", "disabled")
	if val2 != "1" {
		t.Errorf("expected radio2 disabled='1', got %q", val2)
	}
}

func TestGetAPConfigs_DiscoversDynamicAPSections(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.AddSection("wireless", "travel_ap", "wifi-iface")
	_ = u.Set("wireless", "travel_ap", "device", "radio1")
	_ = u.Set("wireless", "travel_ap", "mode", "ap")
	_ = u.Set("wireless", "travel_ap", "ssid", "Travel-Alt")
	_ = u.Set("wireless", "travel_ap", "encryption", "psk2")
	_ = u.Set("wireless", "travel_ap", "key", "travelrouter")

	configs, err := svc.GetAPConfigs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, cfg := range configs {
		if cfg.Section == "travel_ap" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected dynamic AP section to be discovered, got %#v", configs)
	}
}

func TestWifiSetMode_ClientDisablesAPsAndEnablesSTA(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "disabled", "0")
	_ = u.Set("wireless", "default_radio1", "disabled", "0")
	_ = u.Set("wireless", "sta0", "disabled", "1")

	_, err := svc.SetMode("client")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	ap1, _ := u.Get("wireless", "default_radio1", "disabled")
	sta, _ := u.Get("wireless", "sta0", "disabled")
	if ap0 != "1" || ap1 != "1" || sta != "0" {
		t.Fatalf("expected APs disabled and STA enabled, got ap0=%q ap1=%q sta=%q", ap0, ap1, sta)
	}
}

// In repeater mode with ≥2 radios, the STA (uplink) and AP (downlink) must live on
// different radios. On ath11k/IPQ6018 an AP sharing a radio with a STA is forced to
// follow the STA's channel and cannot start until the STA associates — a failing STA
// takes down the AP on that radio. Separating them keeps the downlink AP reachable
// even if the uplink drops.
func TestWifiSetMode_RepeaterEnablesSTAAndAPs(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "disabled", "1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1")
	_ = u.Set("wireless", "sta0", "disabled", "1")
	// sta0 lives on radio0 per the mock UCI. Ensure default_radio0 AP is on radio0 too.
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")

	_, err := svc.SetMode("repeater")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	ap1, _ := u.Get("wireless", "default_radio1", "disabled")
	sta, _ := u.Get("wireless", "sta0", "disabled")
	// AP on radio0 shares with the STA → must be disabled. AP on radio1 stays enabled.
	if ap0 != "1" {
		t.Errorf("expected AP on STA's radio disabled, got ap0=%q", ap0)
	}
	if ap1 != "0" {
		t.Errorf("expected AP on free radio enabled, got ap1=%q", ap1)
	}
	if sta != "0" {
		t.Errorf("expected STA enabled, got sta=%q", sta)
	}
}

func TestWifiSetMode_RepeaterAllowAPOnSTARadioOverridesSplit(t *testing.T) {
	svc, u := newTestWifiService()
	svc.repeaterOptionsFile = filepath.Join(t.TempDir(), "repeater-options.json")
	if err := os.WriteFile(svc.repeaterOptionsFile, []byte(`{"allow_ap_on_sta_radio":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = u.Set("wireless", "default_radio0", "disabled", "1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1")
	_ = u.Set("wireless", "sta0", "disabled", "1")
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")

	if _, err := svc.SetMode("repeater"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	if ap0 != "0" {
		t.Errorf("with allow_ap_on_sta_radio, expected AP on STA radio enabled, got ap0=%q", ap0)
	}
}

func TestRepeaterOptionsRoundTrip(t *testing.T) {
	svc, _ := newTestWifiService()
	svc.repeaterOptionsFile = filepath.Join(t.TempDir(), "repeater-options.json")
	if _, err := svc.SetRepeaterOptions(models.RepeaterOptions{AllowAPOnSTARadio: true}); err != nil {
		t.Fatal(err)
	}
	o, err := svc.GetRepeaterOptions()
	if err != nil {
		t.Fatal(err)
	}
	if !o.AllowAPOnSTARadio {
		t.Fatal("expected AllowAPOnSTARadio true")
	}
}

func TestSetRepeaterOptions_DisallowAPOnSTARadioRunsReconcileInRepeaterMode(t *testing.T) {
	svc, u := newTestWifiService()
	svc.repeaterOptionsFile = filepath.Join(t.TempDir(), "repeater-options.json")
	if err := os.WriteFile(svc.repeaterOptionsFile, []byte(`{"allow_ap_on_sta_radio":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetMode("repeater"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	// Bad state: AP on STA radio enabled while we are about to disallow that policy.
	_ = u.Set("wireless", "default_radio0", "disabled", "0")
	apply, err := svc.SetRepeaterOptions(models.RepeaterOptions{AllowAPOnSTARadio: false})
	if err != nil {
		t.Fatalf("SetRepeaterOptions: %v", err)
	}
	if apply != nil {
		t.Fatalf("expected nil apply without applier, got %#v", apply)
	}
	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	if ap0 != "1" {
		t.Errorf("expected AP on STA radio disabled after options change, got disabled=%q", ap0)
	}
}

func TestSetRepeaterOptions_DisallowAPOnSTARadioReturnsApplyWhenApplierConfigured(t *testing.T) {
	svc, u := newTestWifiService()
	svc.repeaterOptionsFile = filepath.Join(t.TempDir(), "repeater-options.json")
	if err := os.WriteFile(svc.repeaterOptionsFile, []byte(`{"allow_ap_on_sta_radio":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	svc.applier = &fakeWirelessApplier{startToken: "opts-reconcile"}
	if _, err := svc.SetMode("repeater"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	_ = u.Set("wireless", "default_radio0", "disabled", "0")
	apply, err := svc.SetRepeaterOptions(models.RepeaterOptions{AllowAPOnSTARadio: false})
	if err != nil {
		t.Fatalf("SetRepeaterOptions: %v", err)
	}
	if apply == nil || apply.Token != "opts-reconcile" {
		t.Fatalf("expected pending apply token opts-reconcile, got %#v", apply)
	}
}

// With only one radio available, STA+AP coexistence on the same radio is unavoidable.
// We still bring both up — partial connectivity beats none — and let the user upgrade
// their hardware if they need better isolation.
func TestSetModeRepeater_WithSingleRadio_AllowsCoexistence(t *testing.T) {
	svc, u := newTestWifiService()
	// Delete radio1 and everything attached to it.
	_ = u.DeleteSection("wireless", "default_radio1")
	_ = u.DeleteSection("wireless", "radio1")
	_ = u.Set("wireless", "default_radio0", "disabled", "1")
	_ = u.Set("wireless", "sta0", "disabled", "1")

	if _, err := svc.SetMode("repeater"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}

	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	sta, _ := u.Get("wireless", "sta0", "disabled")
	if ap0 != "0" || sta != "0" {
		t.Errorf("single-radio repeater: expected ap0=0 sta=0, got ap0=%q sta=%q", ap0, sta)
	}
}

func TestWifiSetMode_InvalidMode(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.SetMode("invalid")
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
}

func TestGetConnection_DerivesRepeaterMode(t *testing.T) {
	svc, _ := newTestWifiService()

	conn, err := svc.GetConnection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Mode != "repeater" {
		t.Fatalf("expected repeater mode, got %q", conn.Mode)
	}
}

func TestConfirmApply_DelegatesToApplier(t *testing.T) {
	svc, _ := newTestWifiService()
	// Both mock AP sections are enabled, so confirm only reaches the applier once
	// netifd reports them up (see ConfirmApply).
	registerAPStatus(t, svc, map[string]bool{"default_radio0": true, "default_radio1": true})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-456"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.confirmed) != 1 || fake.confirmed[0] != "session-456" {
		t.Fatalf("expected confirm to be called for session-456, got %#v", fake.confirmed)
	}
}

func TestConfirmApply_RequiresToken(t *testing.T) {
	svc, _ := newTestWifiService()
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("   "); err == nil {
		t.Fatal("expected an error for a blank apply token")
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("a rejected token must not reach the applier, got %#v", fake.confirmed)
	}
}

// registerAPStatus publishes a `network.wireless status` payload where each named
// AP section is reported by netifd with the given up flag.
func registerAPStatus(t *testing.T, svc *WifiService, up map[string]bool) {
	t.Helper()
	sections, err := svc.uci.GetSections("wireless")
	if err != nil {
		t.Fatalf("reading wireless sections: %v", err)
	}
	byRadio := map[string][]any{}
	names := slices.Sorted(maps.Keys(up))
	for _, name := range names {
		radio := sections[name]["device"]
		byRadio[radio] = append(byRadio[radio], map[string]any{
			"section": name,
			"ifname":  "phy0-ap0",
			"up":      up[name],
			"config":  map[string]any{"mode": "ap", "ssid": sections[name]["ssid"]},
		})
	}
	resp := map[string]any{}
	for radio, ifaces := range byRadio {
		resp[radio] = map[string]any{"interfaces": ifaces}
	}
	ub, ok := svc.ubus.(*ubus.MockUbus)
	if !ok {
		t.Fatal("expected a MockUbus")
	}
	ub.RegisterResponse("network.wireless.status", resp)
}

// A radio that stays down must NOT have its rollback cancelled: that is the
// "proof of reachability" docs/architecture.md §3 and ADR 0002 §5 require, and
// it is what a client-confirms-instantly bug silently skipped.
func TestConfirmApply_KeepsRollbackArmedWhenAPIsDown(t *testing.T) {
	svc, _ := newTestWifiService()
	registerAPStatus(t, svc, map[string]bool{"default_radio0": true, "default_radio1": false})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	err := svc.ConfirmApply("session-down")
	if !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("expected ErrWirelessNotUp, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("the apply session must stay open when the AP is down, got confirm %#v", fake.confirmed)
	}
}

func TestConfirmApply_FailsClosedWhenWirelessStatusIsUnreadable(t *testing.T) {
	svc, _ := newTestWifiService()
	ub := svc.ubus.(*ubus.MockUbus)
	ub.RegisterResponse("network.wireless.status", nil)
	// A nil registered response makes the mock answer with no interfaces, i.e. no
	// AP is observable at all.
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-unknown"); !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("expected ErrWirelessNotUp when the interfaces cannot be observed, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("unknown state must not cancel the rollback, got confirm %#v", fake.confirmed)
	}
}

// "Turn WiFi off" and client mode leave no access point running, so there is
// nothing to prove and the operator's change must be allowed to stick.
func TestConfirmApply_SkipsProbeWhenNoAPIsEnabled(t *testing.T) {
	svc, u := newTestWifiService()
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "disabled", "1"); err != nil {
			t.Fatal(err)
		}
	}
	// An AP section on a disabled radio is deliberately down as well.
	if err := u.Set("wireless", "radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	_ = u.Set("wireless", "radio1", "disabled", "1")
	registerAPStatus(t, svc, map[string]bool{"default_radio0": false, "default_radio1": false})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-off"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

func TestWifiReorderNetworks(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpFile := t.TempDir() + "/priorities.json"
	svc := NewWifiServiceWithPriorityFile(u, ub, &NoopWifiReloader{}, tmpFile)

	err := svc.ReorderNetworks([]string{"Network-A", "Network-B", "Network-C"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify priorities were saved
	priorities := svc.loadPriorities()
	if priorities["Network-A"] != 1 {
		t.Errorf("expected Network-A priority 1, got %d", priorities["Network-A"])
	}
	if priorities["Network-B"] != 2 {
		t.Errorf("expected Network-B priority 2, got %d", priorities["Network-B"])
	}
	if priorities["Network-C"] != 3 {
		t.Errorf("expected Network-C priority 3, got %d", priorities["Network-C"])
	}
}

func TestWifiGetSavedNetworksWithPriority(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpFile := t.TempDir() + "/priorities.json"
	svc := NewWifiServiceWithPriorityFile(u, ub, &NoopWifiReloader{}, tmpFile)

	// Set priority for Hotel-WiFi (the mock SSID)
	err := svc.ReorderNetworks([]string{"Hotel-WiFi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	networks, err := svc.GetSavedNetworks()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(networks) == 0 {
		t.Fatal("expected at least one saved network")
	}
	if networks[0].Priority != 1 {
		t.Errorf("expected priority 1, got %d", networks[0].Priority)
	}
}

func TestWifiReorderNetworks_Overwrite(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpFile := t.TempDir() + "/priorities.json"
	svc := NewWifiServiceWithPriorityFile(u, ub, &NoopWifiReloader{}, tmpFile)

	// First ordering
	_ = svc.ReorderNetworks([]string{"A", "B", "C"})
	// Second ordering overwrites
	err := svc.ReorderNetworks([]string{"C", "A", "B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	priorities := svc.loadPriorities()
	if priorities["C"] != 1 {
		t.Errorf("expected C priority 1, got %d", priorities["C"])
	}
	if priorities["A"] != 2 {
		t.Errorf("expected A priority 2, got %d", priorities["A"])
	}
	if priorities["B"] != 3 {
		t.Errorf("expected B priority 3, got %d", priorities["B"])
	}
}

func TestGetAutoReconnect_Default(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpDir := t.TempDir()
	svc := NewWifiServiceForTesting(u, ub, &NoopWifiReloader{}, &MockCommandRunner{},
		tmpDir+"/priorities.json", tmpDir+"/autoreconnect.json", tmpDir+"/wifi-reconnect.sh")

	enabled, err := svc.GetAutoReconnect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Error("expected auto-reconnect to be disabled by default")
	}
}

func TestSetAutoReconnect_Enable(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpDir := t.TempDir()
	svc := NewWifiServiceForTesting(u, ub, &NoopWifiReloader{}, &MockCommandRunner{},
		tmpDir+"/priorities.json", tmpDir+"/autoreconnect.json", tmpDir+"/wifi-reconnect.sh")

	err := svc.SetAutoReconnect(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	enabled, err := svc.GetAutoReconnect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Error("expected auto-reconnect to be enabled")
	}

	// Verify script was written and uses "wifi up" (not "wifi reload") for ath11k safety
	data, err := os.ReadFile(tmpDir + "/wifi-reconnect.sh")
	if err != nil {
		t.Fatalf("expected script to exist: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty script")
	}
	if !strings.Contains(string(data), "wifi up") {
		t.Error("expected script to use 'wifi up' for reassociation (avoids ath11k crash from wifi reload)")
	}
	if !strings.Contains(string(data), "crash-guard") {
		t.Error("expected script to include crash guard check")
	}
}

// TestReconnectScript_HasFailureCountGuard verifies the auto-reconnect script caps
// consecutive failures so a persistently broken wireless config cannot be replayed
// indefinitely by cron (e.g. after a rollback recovers the old bad config).
func TestReconnectScript_HasFailureCountGuard(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpDir := t.TempDir()
	svc := NewWifiServiceForTesting(u, ub, &NoopWifiReloader{}, &MockCommandRunner{},
		tmpDir+"/priorities.json", tmpDir+"/autoreconnect.json", tmpDir+"/wifi-reconnect.sh")

	if err := svc.SetAutoReconnect(true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(tmpDir + "/wifi-reconnect.sh")
	if err != nil {
		t.Fatalf("expected script to exist: %v", err)
	}
	script := string(data)

	if !strings.Contains(script, "FAILCOUNT") {
		t.Error("expected script to track a FAILCOUNT so persistent failures stop retrying")
	}
	if !strings.Contains(script, "MAX_FAIL") {
		t.Error("expected script to define a MAX_FAIL ceiling on retries")
	}
	if !strings.Contains(script, "failcount") {
		t.Error("expected script to persist the counter in /etc/travo/autoreconnect-failcount")
	}
}

func TestSetAutoReconnect_Disable(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpDir := t.TempDir()
	svc := NewWifiServiceForTesting(u, ub, &NoopWifiReloader{}, &MockCommandRunner{},
		tmpDir+"/priorities.json", tmpDir+"/autoreconnect.json", tmpDir+"/wifi-reconnect.sh")

	// Enable first, then disable
	_ = svc.SetAutoReconnect(true)
	err := svc.SetAutoReconnect(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	enabled, err := svc.GetAutoReconnect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Error("expected auto-reconnect to be disabled")
	}

	// Verify script was removed
	if _, err := os.Stat(tmpDir + "/wifi-reconnect.sh"); err == nil {
		t.Error("expected script to be removed")
	}
}

func TestEnsureAPRunning_AlreadyHealthy(t *testing.T) {
	svc, _ := newTestWifiService()

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fixed {
		t.Error("expected no fixes when all APs are already healthy")
	}
	if needWifiUp {
		t.Error("expected needWifiUp=false when no fixes")
	}
}

// TestEnsureAPRunning_DisabledAPNoSTA verifies that a disabled AP whose radio
// has no competing STA interface is re-enabled by the health check.
// (radio1 has no STA in the default mock state.)
func TestEnsureAPRunning_DisabledAPNoSTA(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio1", "disabled", "1")

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fix: disabled AP on radio with no STA should be re-enabled")
	}
	if !needWifiUp {
		t.Error("expected needWifiUp=true when AP was re-enabled")
	}
	val, _ := u.Get("wireless", "default_radio1", "disabled")
	if val != "0" {
		t.Errorf("expected disabled='0' after re-enable, got %q", val)
	}
}

// TestEnsureAPRunning_DisabledAPWithActiveSTA verifies that a disabled AP is
// left alone when the same radio already has an active STA interface.
// Re-enabling the AP while a STA is running causes ath11k/IPQ6018 driver crashes.
// (radio0 has sta0 active in the default mock state.)
func TestEnsureAPRunning_DisabledAPWithActiveSTA(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "disabled", "1")

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fixed {
		t.Error("expected no fix: disabled AP must not be re-enabled when STA is active on same radio")
	}
	if needWifiUp {
		t.Error("expected needWifiUp=false when no AP was re-enabled")
	}
	val, _ := u.Get("wireless", "default_radio0", "disabled")
	if val != "1" {
		t.Errorf("expected disabled to remain '1', got %q", val)
	}
}

func TestEnsureAPRunning_EmptySSID(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "ssid", "")

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fix for empty SSID")
	}
	if needWifiUp {
		t.Error("expected needWifiUp=false when only SSID/key fix (no re-enable)")
	}
	val, _ := u.Get("wireless", "default_radio0", "ssid")
	if val != DefaultAPSSID {
		t.Errorf("expected ssid=%q after fix, got %q", DefaultAPSSID, val)
	}
}

func TestEnsureAPRunning_MissingKeyOnEncryptedAP(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "key", "")

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fix for missing key on encrypted AP")
	}
	if needWifiUp {
		t.Error("expected needWifiUp=false when only SSID/key fix (no re-enable)")
	}
	val, _ := u.Get("wireless", "default_radio0", "key")
	if val != DefaultAPKey {
		t.Errorf("expected key=%q after fix, got %q", DefaultAPKey, val)
	}
}

// TestEnsureAPRunning_EnablesRadioWhenAPEnabled verifies that when a radio is disabled
// but has an enabled AP iface, we enable the radio so WiFi is visible.
func TestEnsureAPRunning_EnablesRadioWhenAPEnabled(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "radio1", "disabled", "1") // radio off, but default_radio1 (AP) is on

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fix: enable radio when it has an enabled AP")
	}
	if !needWifiUp {
		t.Error("expected needWifiUp=true when radio was enabled")
	}
	val, _ := u.Get("wireless", "radio1", "disabled")
	if val != "0" {
		t.Errorf("expected radio1 disabled='0' so WiFi is visible, got %q", val)
	}
}

// TestEnsureAPRunning_LeavesRadioDisabledWhenNoEnabledAP verifies that when both
// the radio and its AP are disabled, fixAPSection re-enables the AP (no STA conflict),
// and the "enable radios" loop then enables the radio too (because the snapshot was updated).
func TestEnsureAPRunning_LeavesRadioDisabledWhenNoEnabledAP(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "radio1", "disabled", "1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1") // AP also off

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fix: both AP and radio should be re-enabled (no STA conflict on radio1)")
	}
	if !needWifiUp {
		t.Error("expected needWifiUp=true when AP and radio were re-enabled")
	}
	// AP should be re-enabled (no STA on radio1)
	valAP, _ := u.Get("wireless", "default_radio1", "disabled")
	if valAP != "0" {
		t.Errorf("expected default_radio1 disabled='0', got %q", valAP)
	}
	// Radio should also be enabled since the AP was re-enabled (snapshot updated)
	valRadio, _ := u.Get("wireless", "radio1", "disabled")
	if valRadio != "0" {
		t.Errorf("expected radio1 disabled='0' (AP was re-enabled), got %q", valRadio)
	}
}

// TestEnsureAPRunning_FixesAllBrokenAPs verifies fixes applied across multiple APs.
// default_radio0 disabled=1, radio0 has active sta0 → stays disabled (crash guard).
// default_radio1 ssid="" → SSID fixed.
func TestEnsureAPRunning_FixesAllBrokenAPs(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "disabled", "1") // radio0 has STA → stays disabled
	_ = u.Set("wireless", "default_radio1", "ssid", "")      // enabled, empty SSID → fix

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fix for default_radio1 empty SSID")
	}
	if needWifiUp {
		t.Error("expected needWifiUp=false when only SSID fix, no AP re-enabled")
	}
	// default_radio0 must remain disabled (not touched)
	val0, _ := u.Get("wireless", "default_radio0", "disabled")
	if val0 != "1" {
		t.Errorf("expected default_radio0 disabled to remain '1', got %q", val0)
	}
	// default_radio1 (5G) SSID must be restored to band-specific default
	val1, _ := u.Get("wireless", "default_radio1", "ssid")
	if val1 != DefaultAPSSID5G {
		t.Errorf("expected default_radio1 ssid=%q (5G), got %q", DefaultAPSSID5G, val1)
	}
}

func TestEnsureAPRunning_OpenAPNoKeyFix(t *testing.T) {
	svc, u := newTestWifiService()
	// Open AP: no encryption, no key — this is a valid intentional configuration.
	_ = u.Set("wireless", "default_radio0", "encryption", "")
	_ = u.Set("wireless", "default_radio0", "key", "")

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fixed {
		t.Error("expected no fix for open AP with empty key")
	}
	if needWifiUp {
		t.Error("expected needWifiUp=false when no fix")
	}
	val, _ := u.Get("wireless", "default_radio0", "key")
	if val != "" {
		t.Errorf("expected key to remain empty for open AP, got %q", val)
	}
}

// TestEnsureAPRunning_RadioDefaults verifies that wifi-device sections get country and channel defaults.
func TestEnsureAPRunning_RadioDefaults(t *testing.T) {
	u := uci.NewMockUCI()
	// Remove country from radio0 so health check will set it
	_ = u.Set("wireless", "radio0", "country", "")
	// Set channel to empty so health check sets "auto"
	_ = u.Set("wireless", "radio0", "channel", "")
	ub := ubus.NewMockUbus()
	svc := NewWifiServiceWithReloader(u, ub, &NoopWifiReloader{})

	fixed, _, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fix for radio missing country/channel")
	}
	country, _ := u.Get("wireless", "radio0", "country")
	if country != DefaultCountry {
		t.Errorf("expected radio0 country=%q, got %q", DefaultCountry, country)
	}
	channel, _ := u.Get("wireless", "radio0", "channel")
	if channel != DefaultChannel {
		t.Errorf("expected radio0 channel=%q, got %q", DefaultChannel, channel)
	}
}

// errGetSectionsUCI wraps a real UCI but makes GetSections return an error.
type errGetSectionsUCI struct {
	uci.UCI
}

func (e *errGetSectionsUCI) GetSections(_ string) (map[string]map[string]string, error) {
	return nil, errors.New("simulated GetSections failure")
}

func TestEnsureAPRunning_GetSectionsError(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewWifiServiceWithReloader(&errGetSectionsUCI{uci.NewMockUCI()}, ub, &NoopWifiReloader{})

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err == nil {
		t.Fatal("expected error when GetSections fails")
	}
	if fixed {
		t.Error("expected fixed=false when GetSections fails")
	}
	if needWifiUp {
		t.Error("expected needWifiUp=false when GetSections fails")
	}
}

// errCommitUCI wraps MockUCI but makes Commit return an error.
type errCommitUCI struct {
	*uci.MockUCI
}

func (e *errCommitUCI) Commit(_ string) error {
	return errors.New("simulated Commit failure")
}

func TestEnsureAPRunning_CommitError(t *testing.T) {
	base := uci.NewMockUCI()
	// Empty SSID on an enabled AP triggers a fix → Commit → error.
	_ = base.Set("wireless", "default_radio0", "ssid", "")

	ub := ubus.NewMockUbus()
	svc := NewWifiServiceWithReloader(&errCommitUCI{base}, ub, &NoopWifiReloader{})

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err == nil {
		t.Fatal("expected error when Commit fails")
	}
	if fixed {
		t.Error("expected fixed=false when Commit fails")
	}
	if needWifiUp {
		t.Error("expected needWifiUp=false when Commit fails")
	}
}

func TestParseIwinfoEncryption(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  string
	}{
		{"nil", nil, "none"},
		{"empty map disabled", map[string]any{"enabled": false}, "none"},
		{"wpa2 psk", map[string]any{
			"enabled":        true,
			"wpa":            []any{float64(2)},
			"authentication": []any{"psk"},
		}, "psk2"},
		{"wpa psk", map[string]any{
			"enabled":        true,
			"wpa":            []any{float64(1)},
			"authentication": []any{"psk"},
		}, "psk"},
		{"wpa3 sae", map[string]any{
			"enabled":        true,
			"wpa":            []any{float64(3)},
			"authentication": []any{"sae"},
		}, "sae"},
		{"wep (no wpa key)", map[string]any{
			"enabled": true,
		}, "wep"},
		{"plain string fallback", "psk2", "psk2"},
		{"empty string fallback", "", "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseIwinfoEncryption(tc.input)
			if got != tc.want {
				t.Errorf("parseIwinfoEncryption(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func newTestWifiServiceWithModeFile(modeFile string) (*WifiService, *uci.MockUCI) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewWifiServiceForTestingWithModeFile(u, ub, &NoopWifiReloader{}, &RealCommandRunner{}, defaultPriorityFile, defaultAutoReconnectFile, defaultReconnectScript, modeFile)
	return svc, u
}

func TestSetMode_PersistsModeFile(t *testing.T) {
	tmpFile := t.TempDir() + "/wifi-mode"
	svc, _ := newTestWifiServiceWithModeFile(tmpFile)

	_, err := svc.SetMode("client")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("mode file not written: %v", err)
	}
	if string(data) != "client" {
		t.Errorf("expected mode file content 'client', got %q", string(data))
	}
}

func TestDeriveWifiMode_ReadsFromModeFile(t *testing.T) {
	tmpFile := t.TempDir() + "/wifi-mode"
	svc, _ := newTestWifiServiceWithModeFile(tmpFile)

	// Both STA and AP are enabled in mock UCI, so UCI-only detection would return "repeater".
	// After SetMode("client"), the mode file should make deriveWifiMode return "client".
	_, err := svc.SetMode("client")
	if err != nil {
		t.Fatalf("SetMode error: %v", err)
	}

	mode := svc.deriveWifiMode()
	if mode != "client" {
		t.Errorf("expected deriveWifiMode to return 'client' from mode file, got %q", mode)
	}
}

func TestDeriveWifiMode_FallsBackToUCIWhenNoModeFile(t *testing.T) {
	// No mode file — uses UCI detection.
	svc, _ := newTestWifiServiceWithModeFile("")

	mode := svc.deriveWifiMode()
	// Mock UCI has both ap and sta sections enabled, so UCI detection returns "repeater".
	if mode != "repeater" && mode != "ap" && mode != "client" {
		t.Errorf("unexpected mode %q from UCI fallback", mode)
	}
}

func TestGetHealth_NoSTASection_ReturnsOK(t *testing.T) {
	svc, u := newTestWifiService()
	// Mock has sta0 — delete it so we're in pure AP mode.
	_ = u.DeleteSection("wireless", "sta0")

	h, err := svc.GetHealth()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Status != "ok" {
		t.Errorf("expected ok, got %q issues=%v", h.Status, h.Issues)
	}
}

func TestGetHealth_STAAssociatedWithIP_ReturnsOK(t *testing.T) {
	svc, u := newTestWifiService()
	// Avoid repeater same-radio warning: STA is on radio0 in mock; keep downlink AP only on radio1.
	_ = u.Set("wireless", "default_radio0", "disabled", "1")
	// Mock default state: sta0 enabled, iwinfo.info.ssid=Hotel-WiFi, wwan up with IP 10.0.0.50.
	h, err := svc.GetHealth()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Status != "ok" {
		t.Errorf("expected ok, got %q issues=%v", h.Status, h.Issues)
	}
	if h.STA == nil || h.STA.SSID != "Hotel-WiFi" {
		t.Errorf("expected STA.SSID=Hotel-WiFi, got %#v", h.STA)
	}
	if h.Wwan == nil || h.Wwan.IPAddress != "10.0.0.50" {
		t.Errorf("expected wwan IP 10.0.0.50, got %#v", h.Wwan)
	}
}

// This is the exact failure mode from the device investigation: STA is associated
// on phy0-sta0 but netifd bound wwan to phy1-sta0, leaving the real connection
// without DHCP. Health must surface this as an error, not as "connected".
func TestGetHealth_WwanBoundToDifferentDevice_ReturnsError(t *testing.T) {
	svc, _ := newTestWifiService()
	ub := svc.ubus.(*ubus.MockUbus)

	// STA phy0-sta0 is associated to Hotel-WiFi (mock default iwinfo.info).
	// Override wwan to claim it lives on phy1-sta0 and has no lease.
	ub.RegisterResponse("network.interface.wwan.status", map[string]any{
		"up":           false,
		"device":       "phy1-sta0",
		"ipv4-address": []any{},
	})

	h, err := svc.GetHealth()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Status != "error" {
		t.Errorf("expected error, got %q issues=%v", h.Status, h.Issues)
	}
	if len(h.Issues) == 0 {
		t.Error("expected at least one issue string")
	}
}

func TestGetHealth_STAAssociatedButNoIP_ReturnsWarning(t *testing.T) {
	svc, u := newTestWifiService()
	ub := svc.ubus.(*ubus.MockUbus)
	_ = u.Set("wireless", "default_radio0", "disabled", "1")

	// wwan points at the same iface as STA but has no lease yet (still negotiating).
	ub.RegisterResponse("network.interface.wwan.status", map[string]any{
		"up":           false,
		"device":       "phy0-sta0",
		"ipv4-address": []any{},
	})

	h, err := svc.GetHealth()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Status != "warning" {
		t.Errorf("expected warning, got %q issues=%v", h.Status, h.Issues)
	}
}

func TestGetHealth_RepeaterSameRadioAPSTA(t *testing.T) {
	svc, _ := newTestWifiService()
	h, err := svc.GetHealth()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !h.RepeaterSameRadioAPSTA {
		t.Fatal("expected RepeaterSameRadioAPSTA (STA and AP both on radio0 in mock)")
	}
	if h.Status != "warning" {
		t.Errorf("expected warning status, got %q", h.Status)
	}
	found := false
	for _, i := range h.Issues {
		if strings.Contains(i, "same radio") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected same-radio issue in %#v", h.Issues)
	}
}

func TestReconcileRepeaterAPLayout_NotRepeaterReturnsError(t *testing.T) {
	svc, u := newTestWifiService()
	svc.modeFile = filepath.Join(t.TempDir(), "wifi-mode")
	if err := os.WriteFile(svc.modeFile, []byte("ap"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = u.Set("wireless", "sta0", "disabled", "1")
	_, err := svc.ReconcileRepeaterAPLayout()
	if err == nil || !strings.Contains(err.Error(), "repeater") {
		t.Fatalf("expected repeater-only error, got %v", err)
	}
}

func TestReconcileRepeaterAPLayout_DisablesSTARadioAP(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "disabled", "0")
	_ = u.Set("wireless", "default_radio1", "disabled", "0")
	if _, err := svc.SetMode("repeater"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	if ap0 != "1" {
		t.Fatalf("precondition: ap0 disabled, got %q", ap0)
	}
	// Simulate bad state: AP on STA radio enabled again
	_ = u.Set("wireless", "default_radio0", "disabled", "0")
	if _, err := svc.ReconcileRepeaterAPLayout(); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	ap0, _ = u.Get("wireless", "default_radio0", "disabled")
	if ap0 != "1" {
		t.Errorf("expected AP on STA radio disabled after reconcile, got %q", ap0)
	}
}

func TestValidateWirelessConsistency_SingleActiveSTA_OK(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "sta0", "disabled", "0")
	_ = u.Set("wireless", "sta0", "network", "wwan")

	if err := svc.validateWirelessConsistency(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWirelessConsistency_NoActiveSTA_OK(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "sta0", "disabled", "1")

	if err := svc.validateWirelessConsistency(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWirelessConsistency_MultipleActiveSTAsOnWwan_Error(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "sta0", "disabled", "0")
	_ = u.Set("wireless", "sta0", "network", "wwan")
	_ = u.AddSection("wireless", "sta1", "wifi-iface")
	_ = u.Set("wireless", "sta1", "device", "radio1")
	_ = u.Set("wireless", "sta1", "mode", "sta")
	_ = u.Set("wireless", "sta1", "network", "wwan")
	_ = u.Set("wireless", "sta1", "disabled", "0")

	err := svc.validateWirelessConsistency()
	if err == nil {
		t.Fatal("expected error for two enabled STAs on wwan")
	}
	if !errors.Is(err, ErrMultipleActiveSTA) {
		t.Errorf("expected ErrMultipleActiveSTA, got %v", err)
	}
}

// Belt-and-suspenders: even if something writes a broken UCI directly, the apply
// pipeline must refuse to stage it rather than letting rpcd's rollback timer catch it.
func TestStageWirelessApply_RejectsBrokenConfig(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "sta0", "disabled", "0")
	_ = u.Set("wireless", "sta0", "network", "wwan")
	_ = u.AddSection("wireless", "sta1", "wifi-iface")
	_ = u.Set("wireless", "sta1", "device", "radio1")
	_ = u.Set("wireless", "sta1", "mode", "sta")
	_ = u.Set("wireless", "sta1", "network", "wwan")
	_ = u.Set("wireless", "sta1", "disabled", "0")

	_, err := svc.stageWirelessApply()
	if err == nil {
		t.Fatal("expected stageWirelessApply to refuse broken config")
	}
	if !errors.Is(err, ErrMultipleActiveSTA) {
		t.Errorf("expected ErrMultipleActiveSTA, got %v", err)
	}
}

// Regression: SetMode("repeater") must never enable more than one STA section on network=wwan.
// When the user toggles mode with multiple saved STA profiles, the wrong STA may win the wwan
// binding in netifd, leaving the actually-connected STA without DHCP — "connected, no IP".
func TestSetModeRepeater_WithMultipleSavedSTAs_EnablesOnlyHighestPriority(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpFile := t.TempDir() + "/priorities.json"
	svc := NewWifiServiceWithPriorityFile(u, ub, &NoopWifiReloader{}, tmpFile)

	// Two saved STA profiles, on different radios, both network=wwan, both initially disabled.
	_ = u.Set("wireless", "sta0", "ssid", "LowPriority")
	_ = u.Set("wireless", "sta0", "disabled", "1")
	_ = u.Set("wireless", "sta0", "network", "wwan")
	_ = u.AddSection("wireless", "sta1", "wifi-iface")
	_ = u.Set("wireless", "sta1", "device", "radio1")
	_ = u.Set("wireless", "sta1", "mode", "sta")
	_ = u.Set("wireless", "sta1", "ssid", "HighPriority")
	_ = u.Set("wireless", "sta1", "network", "wwan")
	_ = u.Set("wireless", "sta1", "disabled", "1")

	// HighPriority=1 (higher priority), LowPriority=2.
	if err := svc.ReorderNetworks([]string{"HighPriority", "LowPriority"}); err != nil {
		t.Fatalf("ReorderNetworks: %v", err)
	}

	if _, err := svc.SetMode("repeater"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}

	sta0Disabled, _ := u.Get("wireless", "sta0", "disabled")
	sta1Disabled, _ := u.Get("wireless", "sta1", "disabled")
	if sta0Disabled != "1" {
		t.Errorf("expected LowPriority (sta0) disabled=1, got %q", sta0Disabled)
	}
	if sta1Disabled != "0" {
		t.Errorf("expected HighPriority (sta1) disabled=0, got %q", sta1Disabled)
	}
}

func TestSetModeClient_WithMultipleSavedSTAs_EnablesOnlyHighestPriority(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpFile := t.TempDir() + "/priorities.json"
	svc := NewWifiServiceWithPriorityFile(u, ub, &NoopWifiReloader{}, tmpFile)

	_ = u.Set("wireless", "sta0", "ssid", "Backup")
	_ = u.Set("wireless", "sta0", "disabled", "1")
	_ = u.Set("wireless", "sta0", "network", "wwan")
	_ = u.AddSection("wireless", "sta1", "wifi-iface")
	_ = u.Set("wireless", "sta1", "device", "radio1")
	_ = u.Set("wireless", "sta1", "mode", "sta")
	_ = u.Set("wireless", "sta1", "ssid", "Primary")
	_ = u.Set("wireless", "sta1", "network", "wwan")
	_ = u.Set("wireless", "sta1", "disabled", "1")

	if err := svc.ReorderNetworks([]string{"Primary", "Backup"}); err != nil {
		t.Fatalf("ReorderNetworks: %v", err)
	}

	if _, err := svc.SetMode("client"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}

	sta0Disabled, _ := u.Get("wireless", "sta0", "disabled")
	sta1Disabled, _ := u.Get("wireless", "sta1", "disabled")
	if sta0Disabled != "1" {
		t.Errorf("expected Backup (sta0) disabled=1, got %q", sta0Disabled)
	}
	if sta1Disabled != "0" {
		t.Errorf("expected Primary (sta1) disabled=0, got %q", sta1Disabled)
	}
}

// Regression for the "connected but no IP" bug: Connect() chose sta_B, disabling sta_A.
// Later SetMode("repeater") must not blindly re-enable sta_A; the currently-active profile
// (sta_B) stays the sole STA on wwan.
func TestConnectThenSetModeRepeater_PreservesSingleActiveSTA(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpFile := t.TempDir() + "/priorities.json"
	svc := NewWifiServiceWithPriorityFile(u, ub, &NoopWifiReloader{}, tmpFile)

	// Existing sta0 (Hotel-WiFi) is the mock default.
	// Connect to a second network — Connect() creates sta1 and disables sta0.
	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Password: "latte", Encryption: "psk2",
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	sta0Before, _ := u.Get("wireless", "sta0", "disabled")
	sta1Before, _ := u.Get("wireless", "sta1", "disabled")
	if sta0Before != "1" || sta1Before != "0" {
		t.Fatalf("setup: expected sta0=1 sta1=0 after Connect, got %q %q", sta0Before, sta1Before)
	}

	if _, err := svc.SetMode("repeater"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}

	sta0After, _ := u.Get("wireless", "sta0", "disabled")
	sta1After, _ := u.Get("wireless", "sta1", "disabled")
	if sta0After != "1" {
		t.Errorf("SetMode re-enabled stale STA: expected sta0 disabled=1, got %q", sta0After)
	}
	if sta1After != "0" {
		t.Errorf("SetMode disabled active STA: expected sta1 disabled=0, got %q", sta1After)
	}
}

// Regression: SwitchSTAToRadio must reconcile AP layout so the target radio's AP is
// disabled before commit (ath11k/IPQ6018 crashes if AP and STA share the same PHY).
func TestSwitchSTAToRadio_ReconcilesSameRadioAP(t *testing.T) {
	svc, u := newTestWifiService()

	// Default mock: sta0 on radio0, default_radio0 (AP) on radio0, default_radio1 (AP) on radio1.
	// Switch STA to radio1 — default_radio1 must be disabled, default_radio0 must be enabled.
	if err := svc.SwitchSTAToRadio("radio1"); err != nil {
		t.Fatalf("SwitchSTAToRadio: %v", err)
	}

	staDevice, _ := u.Get("wireless", "sta0", "device")
	if staDevice != "radio1" {
		t.Errorf("expected sta0 device=radio1, got %q", staDevice)
	}

	ap0Disabled, _ := u.Get("wireless", "default_radio0", "disabled")
	ap1Disabled, _ := u.Get("wireless", "default_radio1", "disabled")
	if ap0Disabled == "1" {
		t.Errorf("default_radio0 (AP on radio0, STA moved away) should be enabled, got disabled=1")
	}
	if ap1Disabled != "1" {
		t.Errorf("default_radio1 (AP on radio1, STA now on radio1) should be disabled=1, got %q", ap1Disabled)
	}
}

// ---------------------------------------------------------------------------
// L1: UCI write-sequence safety helpers used by the tests below.
// ---------------------------------------------------------------------------

// revertingUCI wraps MockUCI and implements the optional Revert(config) seam,
// recording every staged-delta rollback. It can also fail one specific Set so
// write sequences can be observed failing half-way.
type revertingUCI struct {
	*uci.MockUCI
	mu        sync.Mutex
	reverted  []string
	commits   []string
	failSet   func(config, section, option string) error
	revertErr error
}

func (r *revertingUCI) Set(config, section, option, value string) error {
	if r.failSet != nil {
		if err := r.failSet(config, section, option); err != nil {
			return err
		}
	}
	return r.MockUCI.Set(config, section, option, value)
}

func (r *revertingUCI) Commit(config string) error {
	r.mu.Lock()
	r.commits = append(r.commits, config)
	r.mu.Unlock()
	return r.MockUCI.Commit(config)
}

func (r *revertingUCI) Revert(config string) error {
	r.mu.Lock()
	r.reverted = append(r.reverted, config)
	r.mu.Unlock()
	return r.revertErr
}

func (r *revertingUCI) revertCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.reverted...)
}

func (r *revertingUCI) commitCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.commits...)
}

// dirtySTAUCI reports a second, permanently-enabled STA bound to wwan. It
// stands in for a device where the wireless config is already inconsistent, so
// the post-mutation validation must fail no matter what the mutation writes.
type dirtySTAUCI struct {
	*uci.MockUCI
}

func (d *dirtySTAUCI) GetSections(config string) (map[string]map[string]string, error) {
	sections, err := d.MockUCI.GetSections(config)
	if err != nil {
		return nil, err
	}
	if config == "wireless" {
		sections["rogue_sta"] = map[string]string{"mode": "sta", "network": "wwan"}
	}
	return sections, nil
}

// ---------------------------------------------------------------------------
// Finding 2: SetRadioRole must validate before Commit, disable other STA
// sections, and never ship a hard-coded key.
// ---------------------------------------------------------------------------

func TestSetRadioRole_ValidatesBeforeCommit(t *testing.T) {
	u := &revertingUCI{MockUCI: uci.NewMockUCI()}
	svc := NewWifiServiceWithReloader(&dirtySTAUCI{u.MockUCI}, ubus.NewMockUbus(), &NoopWifiReloader{})

	if _, err := svc.SetRadioRole("radio1", "sta"); !errors.Is(err, ErrMultipleActiveSTA) {
		t.Fatalf("expected ErrMultipleActiveSTA, got %v", err)
	}
	// The invalid config must never reach the committed state: an rpcd rollback
	// after the fact would restore the very same broken config.
	for _, c := range u.commitCalls() {
		if c == "wireless" {
			t.Error("wireless was committed before validation ran")
		}
	}
}

func TestSetRadioRole_DisablesOtherSTASections(t *testing.T) {
	svc, u := newTestWifiService()
	// sta0 is an enabled STA on radio0 bound to wwan; asking radio1 for the STA
	// role would create a second active wwan STA without disabling the first.
	if _, err := svc.SetRadioRole("radio1", "sta"); err != nil {
		t.Fatalf("SetRadioRole: %v", err)
	}
	dis, _ := u.Get("wireless", "sta0", "disabled")
	if dis != "1" {
		t.Errorf("expected the other STA section to be disabled, got disabled=%q", dis)
	}
	newDis, _ := u.Get("wireless", "sta_radio1", "disabled")
	if newDis != "0" {
		t.Errorf("expected new STA section enabled, got disabled=%q", newDis)
	}
}

func TestSetRadioRole_NewAPGetsRandomKey(t *testing.T) {
	svc, u := newTestWifiService()
	// Remove the AP sections so a default AP has to be created.
	if err := u.DeleteSection("wireless", "default_radio0"); err != nil {
		t.Fatal(err)
	}
	if err := u.DeleteSection("wireless", "default_radio1"); err != nil {
		t.Fatal(err)
	}

	// The apply result only exists on the rpcd apply path used in production.
	svc.applier = &fakeWirelessApplier{startToken: "token-1"}
	apply, err := svc.SetRadioRole("radio0", "ap")
	if err != nil {
		t.Fatalf("SetRadioRole: %v", err)
	}
	key, _ := u.Get("wireless", "ap_radio0", "key")
	if key == "changeme123" {
		t.Error("default AP must not use the hard-coded changeme123 key")
	}
	if len(key) < 8 {
		t.Errorf("generated key %q is too short for WPA (min 8)", key)
	}
	if apply == nil || apply.GeneratedKey != key {
		t.Errorf("expected the generated key to be reported in the apply result, got %+v (key %q)", apply, key)
	}
}

// ---------------------------------------------------------------------------
// Finding 1: role "both" must not commit AP+STA on one radio.
// ---------------------------------------------------------------------------

// repeaterModeService returns a service whose mock config is in repeater mode
// (enabled STA plus enabled APs on both radios) with the given
// allow_ap_on_sta_radio setting persisted.
func repeaterModeService(t *testing.T, allowAPOnSTA bool) (*WifiService, *revertingUCI) {
	t.Helper()
	u := &revertingUCI{MockUCI: uci.NewMockUCI()}
	svc := NewWifiServiceWithReloader(u, ubus.NewMockUbus(), &NoopWifiReloader{})
	svc.guardDir = testGuardDir()
	svc.repeaterOptionsFile = filepath.Join(t.TempDir(), "repeater-options.json")
	opts, err := json.Marshal(models.RepeaterOptions{AllowAPOnSTARadio: allowAPOnSTA})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(svc.repeaterOptionsFile, opts, 0o600); err != nil {
		t.Fatal(err)
	}
	return svc, u
}

// assertNoSameRadioAPSTA fails when radioName has both an enabled AP and an
// enabled STA: the state ADR 0002 §2 says is enough to crash ath11k/IPQ6018.
func assertNoSameRadioAPSTA(t *testing.T, u uci.UCI, radioName string) {
	t.Helper()
	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("reading wireless sections: %v", err)
	}
	ap, sta := false, false
	for name, opts := range sections {
		if opts["device"] != radioName || opts["disabled"] == "1" {
			continue
		}
		switch opts["mode"] {
		case "ap":
			ap = true
			t.Logf("enabled AP on %s: %s", radioName, name)
		case "sta":
			sta = true
			t.Logf("enabled STA on %s: %s", radioName, name)
		}
	}
	if ap && sta {
		t.Errorf("%s has both an enabled AP and an enabled STA", radioName)
	}
}

func TestSetRadioRole_BothIsRefusedWithoutAllowAPOnSTARadio(t *testing.T) {
	svc, u := repeaterModeService(t, false)
	applier := &fakeWirelessApplier{startToken: "must-not-be-used"}
	svc.applier = applier
	// Start from a split layout: the STA owns radio0, the only access point is on
	// radio1. Asking for "both" on radio0 is what would put both on one PHY.
	if err := u.DeleteSection("wireless", "default_radio0"); err != nil {
		t.Fatal(err)
	}
	assertNoSameRadioAPSTA(t, u, "radio0")

	if _, err := svc.SetRadioRole("radio0", "both"); !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected ErrAPAndSTASameRadio, got %v", err)
	}
	// Nothing may reach the running config: the write is refused before Commit,
	// the staged delta is dropped, and no rpcd apply is started (an rpcd rollback
	// could not undo it — the rollback would restore the same broken state).
	for _, c := range u.commitCalls() {
		if c == "wireless" {
			t.Error("wireless was committed after the role was refused")
		}
	}
	for _, c := range []string{"wireless", "network", "firewall"} {
		if !slices.Contains(u.revertCalls(), c) {
			t.Errorf("expected the staged %s delta to be reverted, got %v", c, u.revertCalls())
		}
	}
	if len(applier.started) != 0 {
		t.Errorf("no apply may be started for a refused role, got %v", applier.started)
	}
}

// allow_ap_on_sta_radio is the documented escape hatch: with it set, the same
// request is honoured as asked.
func TestSetRadioRole_BothIsAllowedWhenAllowAPOnSTARadioIsSet(t *testing.T) {
	svc, u := repeaterModeService(t, true)

	if _, err := svc.SetRadioRole("radio0", "both"); err != nil {
		t.Fatalf("SetRadioRole: %v", err)
	}
	apDis, _ := u.Get("wireless", "default_radio0", "disabled")
	staDis, _ := u.Get("wireless", "sta0", "disabled")
	if apDis == "1" || staDis == "1" {
		t.Errorf("expected AP and STA enabled on radio0, got ap=%q sta=%q", apDis, staDis)
	}
}

// Single-radio hardware has no split to make, so coexistence stays allowed — the
// same trade-off SetMode("repeater") already accepts.
func TestSetRadioRole_BothIsAllowedOnSingleRadio(t *testing.T) {
	svc, u := repeaterModeService(t, false)
	if err := u.DeleteSection("wireless", "radio1"); err != nil {
		t.Fatal(err)
	}
	if err := u.DeleteSection("wireless", "default_radio1"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetRadioRole("radio0", "both"); err != nil {
		t.Fatalf("SetRadioRole on single-radio hardware: %v", err)
	}
	apDis, _ := u.Get("wireless", "default_radio0", "disabled")
	staDis, _ := u.Get("wireless", "sta0", "disabled")
	if apDis == "1" || staDis == "1" {
		t.Errorf("expected AP and STA enabled on the only radio, got ap=%q sta=%q", apDis, staDis)
	}
}

// The refused write must be recoverable: a per-radio role that does not put both
// modes on one radio still goes through on the same device.
func TestSetRadioRole_SplitRolesStillWorkOnMultiRadio(t *testing.T) {
	svc, u := repeaterModeService(t, false)

	if _, err := svc.SetRadioRole("radio0", "sta"); err != nil {
		t.Fatalf("STA-only role should be accepted: %v", err)
	}
	apDis, _ := u.Get("wireless", "default_radio0", "disabled")
	staDis, _ := u.Get("wireless", "sta0", "disabled")
	if apDis != "1" {
		t.Errorf("expected the AP on radio0 to be disabled for an STA-only role, got %q", apDis)
	}
	if staDis == "1" {
		t.Errorf("expected the STA on radio0 to stay enabled, got disabled=%q", staDis)
	}
}

// ---------------------------------------------------------------------------
// Finding 9: disabling guest WiFi must tear the whole guest network down.
// ---------------------------------------------------------------------------

func TestSetGuestWifi_DisableTearsDownGuestNetwork(t *testing.T) {
	svc, u := newTestWifiService()

	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-Travel", Encryption: "psk2", Key: "guestpass123",
	}); err != nil {
		t.Fatalf("enable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}); err != nil {
		t.Fatalf("disable guest: %v", err)
	}

	if dis, _ := u.Get("wireless", "guest", "disabled"); dis != "1" {
		t.Errorf("expected wireless.guest disabled=1, got %q", dis)
	}
	removed := []struct{ config, section string }{
		{"network", "guest"},
		{"dhcp", "guest"},
		{"firewall", "guest_zone"},
		{"firewall", "guest_fwd"},
		{"firewall", "guest_dns"},
		{"firewall", "guest_dhcp"},
	}
	for _, r := range removed {
		if _, err := u.GetAll(r.config, r.section); err == nil {
			t.Errorf("expected %s.%s to be removed when guest WiFi is disabled", r.config, r.section)
		}
	}
}

func TestSetGuestWifi_DisableThenEnableRestoresNetwork(t *testing.T) {
	svc, u := newTestWifiService()

	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-Travel", Encryption: "psk2", Key: "guestpass123",
	}); err != nil {
		t.Fatalf("enable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}); err != nil {
		t.Fatalf("disable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-2", Encryption: "psk2", Key: "guestpass123",
	}); err != nil {
		t.Fatalf("re-enable guest: %v", err)
	}
	net, err := u.GetAll("network", "guest")
	if err != nil {
		t.Fatalf("expected network.guest to be restored: %v", err)
	}
	if net["ipaddr"] != "192.168.2.1" {
		t.Errorf("expected guest network to be rebuilt with ipaddr 192.168.2.1, got %q", net["ipaddr"])
	}
	fw, err := u.GetAll("firewall", "guest_zone")
	if err != nil {
		t.Fatalf("expected firewall.guest_zone to be restored: %v", err)
	}
	if fw["name"] != "guest" {
		t.Errorf("expected guest zone to be restored, got %v", fw)
	}
}

// ---------------------------------------------------------------------------
// Finding 6: Connect must drop its staged UCI delta on every error return.
// ---------------------------------------------------------------------------

func TestConnect_RevertsStagedWritesOnFailure(t *testing.T) {
	u := &revertingUCI{MockUCI: uci.NewMockUCI()}
	u.failSet = func(config, section, option string) error {
		if config == "wireless" && option == "ssid" {
			return fmt.Errorf("disk full")
		}
		return nil
	}
	svc := NewWifiServiceWithReloader(u, ubus.NewMockUbus(), &NoopWifiReloader{})

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", Password: "newpass123", Encryption: "psk2",
	}); err == nil {
		t.Fatal("expected Connect to fail")
	}
	// The staged delta must be dropped: the uci CLI keeps it in the
	// process-global /tmp/.uci/wireless/changes, where a later unrelated
	// `uci commit wireless` would persist it.
	if !slices.Contains(u.revertCalls(), "wireless") {
		t.Errorf("expected the staged wireless delta to be reverted, got %v", u.revertCalls())
	}
}

func TestConnect_DoesNotRevertOnSuccess(t *testing.T) {
	u := &revertingUCI{MockUCI: uci.NewMockUCI()}
	svc := NewWifiServiceWithReloader(u, ubus.NewMockUbus(), &NoopWifiReloader{})

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", Password: "newpass123", Encryption: "psk2",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls := u.revertCalls(); len(calls) != 0 {
		t.Errorf("successful Connect must not revert the config it just committed, got %v", calls)
	}
}

// ---------------------------------------------------------------------------
// Finding 8: MAC address handling.
// ---------------------------------------------------------------------------

func TestSetMACAddress_RejectsInvalidMAC(t *testing.T) {
	svc, u := newTestWifiService()
	for _, mac := range []string{"not-a-mac", "AA:BB:CC:DD:EE", "ZZ:BB:CC:DD:EE:FF", "AA:BB:CC:DD:EE:FF:00", "AABBCCDDEEFF"} {
		if _, err := svc.SetMACAddress(mac); err == nil {
			t.Errorf("expected %q to be rejected as a MAC address", mac)
		}
		if opts, _ := u.GetAll("wireless", "sta0"); opts["macaddr"] != "" {
			t.Errorf("invalid MAC %q must not be written to wireless.sta0.macaddr", mac)
		}
	}
}

func TestSetMACAddress_AcceptsValidVariants(t *testing.T) {
	svc, u := newMACUnitTestService()
	// Accepted spellings are canonicalized to lowercase colon notation, which
	// is what netifd and mac80211.sh expect.
	for mac, want := range map[string]string{
		"AA:BB:CC:DD:EE:FF":     "aa:bb:cc:dd:ee:ff",
		"aa:bb:cc:dd:ee:ff":     "aa:bb:cc:dd:ee:ff",
		"AA-BB-CC-DD-EE-FF":     "aa:bb:cc:dd:ee:ff",
		"  AA:BB:CC:DD:EE:FF  ": "aa:bb:cc:dd:ee:ff",
	} {
		if _, err := svc.SetMACAddress(mac); err != nil {
			t.Errorf("expected %q to be accepted: %v", mac, err)
		}
		opts, _ := u.GetAll("wireless", "sta0")
		if opts["macaddr"] != want {
			t.Errorf("input %q: expected macaddr %q, got %q", mac, want, opts["macaddr"])
		}
	}
}

// newMACTestService wires a service whose STA device resolves to phy0-sta0 and
// records every command run, so apply order and crash-guard use can be asserted.
func newMACTestService(t *testing.T, guardDir string, applier UCIApplyConfirm, cmds *[]string) (*WifiService, *uci.MockUCI) {
	t.Helper()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.wireless.status", map[string]any{
		"radio0": map[string]any{
			"interfaces": []any{
				map[string]any{
					"ifname":  "phy0-sta0",
					"section": "sta0",
					"config":  map[string]any{"mode": "sta"},
				},
			},
		},
	})
	runner := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		*cmds = append(*cmds, strings.Join(append([]string{name}, args...), " "))
		return nil, nil
	}}
	svc := NewWifiServiceWithReloader(u, ub, &NoopWifiReloader{})
	svc.cmd = runner
	svc.guardDir = guardDir
	svc.applier = applier
	return svc, u
}

func TestSetMACAddress_GuardRemovedAndLinkTouchedAfterApply(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, u := newMACTestService(t, guardDir, &fakeWirelessApplier{startToken: "t1"}, &cmds)

	guard := filepath.Join(guardDir, "mac-in-progress")
	applied, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applied == nil {
		t.Fatal("expected an apply result")
	}
	if _, statErr := os.Stat(guard); !os.IsNotExist(statErr) {
		t.Errorf("expected the crash guard %s to be removed after success", guard)
	}
	if opts, _ := u.GetAll("wireless", "sta0"); opts["macaddr"] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected macaddr to be committed, got %v", opts)
	}
	if len(cmds) == 0 {
		t.Fatal("expected the MAC to be applied to the live link")
	}
	// The link must only be touched once the config is staged and verified, so
	// it can be the first and only thing that runs.
	for i, c := range cmds {
		if !strings.HasPrefix(c, "ip link set") {
			t.Errorf("command %d (%q) is not link manipulation: %v", i, c, cmds)
		}
	}
	if !strings.Contains(strings.Join(cmds, " "), "address aa:bb:cc:dd:ee:ff") {
		t.Errorf("expected the new MAC to be set on the link, commands: %v", cmds)
	}
}

// The `ip link set <if> address` step is the only one that can leave the router
// worse off: the interface was just taken DOWN, and if the address is rejected
// the function returned without bringing it back up and the caller swallowed the
// error — so the STA silently disappeared behind a 200 response, taking the
// operator's upstream link with it.
func TestSetMACAddress_LinkFailureIsReturnedAndLinkBroughtBackUp(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, _ := newMACTestService(t, guardDir, &fakeWirelessApplier{startToken: "t1"}, &cmds)
	svc.cmd = &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		cmds = append(cmds, strings.Join(append([]string{name}, args...), " "))
		// The address change is what a busy driver rejects.
		if slices.Contains(args, "address") {
			return nil, errors.New("RTNETLINK answers: Operation not supported")
		}
		return nil, nil
	}}

	apply, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Fatal("a failed `ip link set address` must surface as an error, not a success")
	}
	if apply != nil {
		t.Errorf("no apply result may be reported when the live link failed: %+v", apply)
	}
	joined := strings.Join(cmds, " | ")
	if !strings.Contains(joined, "ip link set phy0-sta0 down") {
		t.Fatalf("the link was never taken down, so this test proves nothing: %v", cmds)
	}
	// After the failed address change the link must be brought back up.
	if strings.Count(joined, "ip link set phy0-sta0 up") != 1 {
		t.Errorf("the interface was left down after a failed address change: %v", cmds)
	}
}

// The guard is the only durable record that the MAC sequence did not finish
// (ADR 0003 §1.3: remove it only after the operation completes end to end). It
// was cleared unconditionally as soon as the apply was staged, before the live
// link had been touched at all.
func TestSetMACAddress_GuardSurvivesAFailedLinkChange(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, _ := newMACTestService(t, guardDir, &fakeWirelessApplier{startToken: "t1"}, &cmds)
	svc.cmd = &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if slices.Contains(args, "address") {
			return nil, errors.New("RTNETLINK answers: Operation not supported")
		}
		return nil, nil
	}}

	if _, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected the link failure to surface")
	}
	if _, err := os.Stat(filepath.Join(guardDir, "mac-in-progress")); err != nil {
		t.Errorf("the crash guard must survive an incomplete MAC change: %v", err)
	}
}

// A `ip link down` that fails means nothing was changed, but the sequence still
// did not complete; the guard must stay so the operator can see the device is in
// an unresolved state.
func TestSetMACAddress_GuardSurvivesAFailedLinkDown(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, _ := newMACTestService(t, guardDir, &fakeWirelessApplier{startToken: "t1"}, &cmds)
	svc.cmd = &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		cmds = append(cmds, strings.Join(append([]string{name}, args...), " "))
		if slices.Contains(args, "down") {
			return nil, errors.New("ip: cannot find device phy0-sta0")
		}
		return nil, nil
	}}

	if _, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected the link failure to surface")
	}
	if _, err := os.Stat(filepath.Join(guardDir, "mac-in-progress")); err != nil {
		t.Errorf("the crash guard must survive an incomplete MAC change: %v", err)
	}
	if slices.ContainsFunc(cmds, func(c string) bool { return strings.Contains(c, "address") }) {
		t.Errorf("the address must not be set after the interface could not be taken down: %v", cmds)
	}
}

func TestSetMACAddress_ApplyFailureKeepsGuardAndLeavesLinkAlone(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, _ := newMACTestService(t, guardDir, &fakeWirelessApplier{startErr: errors.New("apply failed")}, &cmds)

	if _, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected the apply failure to surface")
	}
	if len(cmds) != 0 {
		t.Errorf("the live link must not be touched when the apply failed, commands: %v", cmds)
	}
	guard := filepath.Join(guardDir, "mac-in-progress")
	if _, err := os.Stat(guard); err != nil {
		t.Errorf("expected crash guard %s to remain so recovery can act on it: %v", guard, err)
	}
}

func TestSetMACAddress_GuardWrittenBeforeUCIWrites(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	// An applier that inspects the guard while the apply is being staged.
	guardPath := filepath.Join(guardDir, "mac-in-progress")
	applier := &guardCheckingApplier{guardPath: guardPath, guardMissing: func() bool {
		_, err := os.Stat(guardPath)
		return os.IsNotExist(err)
	}}
	svc, _ := newMACTestService(t, guardDir, applier, &cmds)

	if _, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applier.guardWasMissing {
		t.Error("the crash guard must be written before the wireless config is applied")
	}
}

// guardCheckingApplier records whether the crash guard existed when the apply
// was staged.
type guardCheckingApplier struct {
	UCIApplyConfirm
	guardPath       string
	guardMissing    func() bool
	guardWasMissing bool
}

func (g *guardCheckingApplier) StartApply(configs []string) (string, error) {
	g.guardWasMissing = g.guardMissing()
	return "token-guard", nil
}

// Finding 1: a failed `uci show` must never look like an empty config in the
// wireless read paths either.
func TestGetRadios_SurfacesGetSectionsError(t *testing.T) {
	svc := NewWifiServiceWithReloader(&failingGetSectionsUCI{uci.NewMockUCI()}, ubus.NewMockUbus(), &NoopWifiReloader{})

	radios, err := svc.GetRadios()
	if err == nil {
		t.Fatal("expected an error instead of an empty radio list when the UCI read fails")
	}
	if radios != nil {
		t.Errorf("expected no list on error, got %v", radios)
	}
}

func TestGetSavedNetworks_SurfacesGetSectionsError(t *testing.T) {
	svc := NewWifiServiceWithReloader(&failingGetSectionsUCI{uci.NewMockUCI()}, ubus.NewMockUbus(), &NoopWifiReloader{})

	networks, err := svc.GetSavedNetworks()
	if err == nil {
		t.Fatal("expected an error instead of an empty saved-network list when the UCI read fails")
	}
	if networks != nil {
		t.Errorf("expected no list on error, got %v", networks)
	}
}

// Finding 3: nextSTASectionName must not fall back to a name that may exist.
func TestNextSTASectionName_PropagatesReadFailure(t *testing.T) {
	svc := NewWifiServiceWithReloader(&failingGetSectionsUCI{uci.NewMockUCI()}, ubus.NewMockUbus(), &NoopWifiReloader{})

	name, err := svc.nextSTASectionName()
	if err == nil {
		t.Fatal("expected an error when the wireless sections cannot be read")
	}
	if name != "" {
		t.Errorf("expected no fallback name, got %q (writing it would hijack an existing section)", name)
	}
}

// Finding 4: ensureNamedSection must correct a wrong section type.
func TestEnsureNamedSection_FixesWrongType(t *testing.T) {
	svc, u := newTestWifiService()

	// A section that already exists with the wrong type.
	if err := u.AddSection("wireless", "guest", "wifi-device"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ensureNamedSection("wireless", "guest", "wifi-iface"); err != nil {
		t.Fatalf("ensureNamedSection: %v", err)
	}
	opts, err := u.GetAll("wireless", "guest")
	if err != nil {
		t.Fatal(err)
	}
	if opts[".type"] != "wifi-iface" {
		t.Errorf("expected the section type to be corrected to wifi-iface, got %q", opts[".type"])
	}
}

func TestEnsureNamedSection_CreatesMissingSection(t *testing.T) {
	svc, u := newTestWifiService()

	if err := svc.ensureNamedSection("dhcp", "guest", "dhcp"); err != nil {
		t.Fatalf("ensureNamedSection: %v", err)
	}
	opts, err := u.GetAll("dhcp", "guest")
	if err != nil {
		t.Fatalf("expected the section to be created: %v", err)
	}
	if opts[".type"] != "dhcp" {
		t.Errorf("expected type dhcp, got %q", opts[".type"])
	}
}

// A read failure must not be mistaken for a missing section.
func TestEnsureNamedSection_PropagatesReadFailure(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewWifiServiceWithReloader(&failingGetSectionsUCI{u}, ubus.NewMockUbus(), &NoopWifiReloader{})

	if err := svc.ensureNamedSection("wireless", "guest", "wifi-iface"); err == nil {
		t.Fatal("expected the read failure to surface instead of silently creating a section")
	}
}

// blockingUCI pauses the first Commit of a given config until release is
// closed, so a test can hold a mutator mid-sequence at a deterministic point.
type blockingUCI struct {
	uci.UCI
	config  string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingUCI) Commit(config string) error {
	if config == b.config {
		b.once.Do(func() { close(b.entered) })
		<-b.release
	}
	return b.UCI.Commit(config)
}

// The uci CLI keeps uncommitted changes in the process-global
// /tmp/.uci/<config>/changes file, so two writers of the same config corrupt
// each other: one commits the other's half-written section, and one reverts the
// other's staged work on its way out.
//
// `firewall` is written by two different services — WifiService (guest
// isolation) and NetworkService (client block rules) — so a per-service mutex
// cannot serialise them. Each mutator must hold the shared per-config lock for
// its whole read-modify-write sequence.
func TestFirewallWritersAreMutuallyExclusive(t *testing.T) {
	t.Parallel()

	svc, _ := newTestWifiService()
	blocker := &blockingUCI{
		UCI:     uci.NewMockUCI(),
		config:  "firewall",
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	svc.uci = blocker
	svc.reloader = &NoopWifiReloader{}
	svc.applier = &fakeWirelessApplier{}
	net := NewNetworkServiceWithRunner(blocker, ubus.NewMockUbus(), &MockCommandRunner{})

	// The guest-WiFi write is parked at its firewall commit, still holding the
	// config lock and with a staged delta it may yet revert.
	guestDone := make(chan error, 1)
	go func() {
		_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: true, SSID: "guest", Key: "guestpass1"})
		guestDone <- err
	}()
	<-blocker.entered

	// A concurrent client block must not get anywhere near the delta.
	blockDone := make(chan error, 1)
	go func() { blockDone <- net.BlockClient("AA:BB:CC:DD:EE:FF") }()

	select {
	case err := <-blockDone:
		t.Fatalf("BlockClient completed while a guest-WiFi write held the firewall delta (err=%v)", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(blocker.release)
	if err := <-guestDone; err != nil {
		t.Fatalf("guest wifi: %v", err)
	}
	if err := <-blockDone; err != nil {
		t.Fatalf("block client: %v", err)
	}
}

// Every mutator that takes a per-UCI-config lock must do so exactly once, and
// the lock order is always uciWriteMu first, then the config locks. sync.Mutex
// is not reentrant, so a nested acquire deadlocks the whole process rather than
// failing a test — in production that is a router whose wireless save never
// returns.
//
// Every mutator below is therefore invoked together, and the test fails fast
// on its own short deadline instead of waiting out the package -timeout. A
// deadlock introduced by adding a new locked mutator, or by making one of these
// call another, shows up here immediately.
func TestWirelessMutatorsDoNotDeadlock(t *testing.T) {
	t.Parallel()

	svc, mockUCI := newTestWifiService()
	svc.reloader = &NoopWifiReloader{}
	svc.applier = &fakeWirelessApplier{}
	if err := mockUCI.Set("wireless", "default_radio0", "mode", "ap"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	net := NewNetworkServiceWithRunner(mockUCI, ubus.NewMockUbus(), &MockCommandRunner{})

	enabled := true
	mutators := map[string]func(){
		"SetAPConfig": func() {
			_, _ = svc.SetAPConfig("default_radio0", models.APConfigUpdate{SSID: "travo", Enabled: &enabled})
		},
		"SetRadioRole":  func() { _, _ = svc.SetRadioRole("radio0", "ap") },
		"SetMode":       func() { _, _ = svc.SetMode("ap") },
		"SetMACAddress": func() { _, _ = svc.SetMACAddress("AA:BB:CC:DD:EE:01") },
		"RandomizeMAC":  func() { _, _, _ = svc.RandomizeMAC() },
		"Connect":       func() { _, _ = svc.Connect(models.WifiConfig{SSID: "upstream", Password: "secret1234"}) },
		"Disconnect":    func() { _, _ = svc.Disconnect() },
		"DeleteNetwork": func() { _, _ = svc.DeleteNetwork("nonexistent") },
		"SetGuestWifi": func() {
			_, _ = svc.SetGuestWifi(models.GuestWifiConfig{Enabled: true, SSID: "guest", Key: "guestpass1"})
		},
		// The disable branch is the only way to reach teardownGuestWifi, which
		// is deliberately NOT wrapped: it is called from inside SetGuestWifi's
		// mutateWireless closure, so wrapping it would self-deadlock. It is
		// listed here precisely so that adding that wrapper — the obvious next
		// "simplification", since its body is shaped like every other mutator —
		// fails this test instead of wedging the guest-WiFi endpoint in
		// production while CI stays green.
		"teardownGuestWifi": func() {
			_, _ = svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false})
		},
		"BlockClient":   func() { _ = net.BlockClient("AA:BB:CC:DD:EE:02") },
		"UnblockClient": func() { _ = net.UnblockClient("AA:BB:CC:DD:EE:02") },
		// The other services that share the same UCI deltas. They hold the same
		// per-config locks for the same reason, and any of them acquiring a lock
		// it already holds would wedge the device.
		"SetWanConfig": func() { _ = net.SetWanConfig(models.WanConfig{Type: "dhcp"}) },
		"AddDHCPReservation": func() {
			_ = net.AddDHCPReservation(models.DHCPReservation{Name: "laptop", MAC: "AA:BB:CC:DD:EE:03", IP: "192.168.1.50"})
		},
		"DeleteDHCPReservation": func() { _ = net.DeleteDHCPReservation("host_laptop") },
		"ReconcileRepeaterAPLayout": func() {
			_, _ = svc.ReconcileRepeaterAPLayout()
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for _, fn := range mutators {
			wg.Add(1)
			go func(fn func()) {
				defer wg.Done()
				fn()
			}(fn)
		}
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("%d wireless mutators did not finish in 30s: a per-config lock is almost certainly held twice (sync.Mutex is not reentrant)", len(mutators))
	}
}

// mutateWireless derives the lock set and the revert set from ONE list, which
// is what makes "locked it but forgot to revert it" unrepresentable. That only
// holds while it is the single place the config locks are taken: a mutator that
// called lockUCIConfigs directly could still lock without reverting, and the
// bug would be invisible to the deadlock test.
//
// A source grep is the right shape for this — a few lines, no parser. What
// matters is that the string `lockUCIConfigs(` appears in exactly one non-test
// file.
func TestOnlyMutateWirelessTakesUCIConfigLocks(t *testing.T) {
	t.Parallel()

	matches := 0
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if !bytes.Contains(src, []byte("lockUCIConfigs(")) {
			continue
		}
		// mutateWireless is the only WifiService mutator; the other services go
		// through mutateUCI, which lives here too.
		if path != "wifi_service.go" {
			t.Errorf("%s calls lockUCIConfigs directly; only mutateWireless/mutateUCI may, "+
				"so the lock set and the revert set stay the same list", path)
		}
		matches++
	}
	if matches != 1 {
		t.Errorf("expected exactly one file to take UCI config locks (wifi_service.go), found %d", matches)
	}
}

// mutateWireless / mutateUCI derive the lock set and the revert set from ONE
// list, so they cannot disagree with each other. What they CAN get wrong is the
// list itself: a mutator that reaches `uci.Commit("dhcp")` while listing only
// {"network"} reverts the wrong config, and a failed save leaves the dhcp delta
// staged for the next unrelated dhcp writer to commit.
//
// This is the check that catches that. It scans source rather than the AST
// because the shape being asserted is simple, and it follows one level of
// helper calls so that a config reached through `c.commitDhcp()` counts just
// as much as a literal `uci.Commit("dhcp")` in the mutator's own body.
func TestMutateCallsCoverEveryConfigTheyTouch(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	// bodyOf: function name -> source, for every non-test file in the package.
	funcRe := regexp.MustCompile(`^func (?:\([^)]*\) )?(\w+)\(`)
	bodyOf := map[string]string{}
	var current string
	var b strings.Builder
	flush := func() {
		if current != "" {
			bodyOf[current] = b.String()
		}
		b.Reset()
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		for _, ln := range strings.Split(string(raw), "\n") {
			if m := funcRe.FindStringSubmatch(ln); m != nil {
				flush()
				current = m[1]
			}
			b.WriteString(ln)
			b.WriteByte('\n')
		}
	}
	flush()

	wrapCall := regexp.MustCompile(`(?:mutateWireless|mutateUCI)\((?:\w+\.uci, )?\[\]string\{([^}]*)\}`)
	// Two ways a config gets written, and both must be caught:
	//   - through the uci.UCI interface: uci.Commit("dhcp"), n.uciSet("firewall", ...)
	//   - shelled out, which writes the same process-global delta: cmd.Run("uci", "commit", "dhcp")
	uciIface := regexp.MustCompile(`(?:Commit|AddSection|AddList|DeleteSection|DeleteOption|uciSet)\("([a-z]+)"`)
	uciShellCommit := regexp.MustCompile(`"uci",\s*"(?:commit|add|delete)",\s*"([a-z]+)"`)
	uciShellSet := regexp.MustCompile(`"uci",\s*"set",\s*"([a-z]+)[.@]`)
	callRe := regexp.MustCompile(`\.(\w+)\(`)
	strLit := regexp.MustCompile(`"([a-z]+)"`)

	// Every function that calls a mutate* helper, and the configs it listed.
	listed := map[string]map[string]bool{}
	for name, src := range bodyOf {
		m := wrapCall.FindStringSubmatch(src)
		if m == nil {
			continue
		}
		set := map[string]bool{}
		for _, q := range strLit.FindAllStringSubmatch(m[1], -1) {
			set[q[1]] = true
		}
		listed[name] = set
	}
	if len(listed) == 0 {
		t.Fatal("no mutateWireless/mutateUCI calls found — this test is not checking anything")
	}

	for fn, configs := range listed {
		// The mutator's own body, plus the bodies of the helpers it calls.
		reached := bodyOf[fn]
		for _, call := range callRe.FindAllStringSubmatch(reached, -1) {
			if helper, ok := bodyOf[call[1]]; ok && call[1] != fn {
				reached += helper
			}
		}
		seen := map[string]bool{}
		for _, re := range []*regexp.Regexp{uciIface, uciShellCommit, uciShellSet} {
			for _, m := range re.FindAllStringSubmatch(reached, -1) {
				seen[m[1]] = true
			}
		}
		for cfg := range seen {
			if !configs[cfg] {
				t.Errorf("%s reaches uci config %q but its mutate* call lists only %v: "+
					"a failed save would leave a staged %q delta for the next "+
					"unrelated writer to commit", fn, cfg, keys(configs), cfg)
			}
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Band switching: a persisted config must never be able to take the backend
// down with it.
// ---------------------------------------------------------------------------

// writeBandSwitchConfigFile persists raw JSON as the band switcher's config.
func writeBandSwitchConfigFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "band-switch.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write band switch config: %v", err)
	}
	return path
}

// check_interval_sec 0 reached time.NewTicker(0), which PANICS. Start runs in the
// backend process, so that panic took the API, the WebSocket and every other
// service down with it. loadConfig must reject the file instead.
func TestBandSwitchingStartDoesNotPanicOnZeroCheckInterval(t *testing.T) {
	path := writeBandSwitchConfigFile(t, `{"enabled":true,"check_interval_sec":0,"preferred_band":"5g"}`)

	svc := NewBandSwitchingService(nil, path)
	if got := svc.GetConfig().CheckIntervalSec; got != defaultBandSwitchCheckInterval {
		t.Errorf("an invalid config file must fall back to the default interval, got %d", got)
	}
	if got := svc.checkInterval(); got != time.Duration(defaultBandSwitchCheckInterval)*time.Second {
		t.Errorf("ticker interval = %s, want %ds", got, defaultBandSwitchCheckInterval)
	}

	// Start must return normally; a panic here fails the whole test binary.
	svc.Start()
	defer svc.Stop()
}

// The clamp in checkInterval is what stands between a bad value and the panic,
// so it is asserted directly too: a non-positive interval never reaches
// time.NewTicker.
func TestBandSwitchingCheckIntervalClampsNonPositiveValues(t *testing.T) {
	for _, sec := range []int{0, -5, -3600} {
		svc := NewBandSwitchingService(nil, filepath.Join(t.TempDir(), "missing.json"))
		svc.config.CheckIntervalSec = sec
		if got := svc.checkInterval(); got <= 0 {
			t.Errorf("check_interval_sec %d produced a non-positive ticker interval %s", sec, got)
		}
	}
}

// A config file that is not even JSON must not replace the defaults either.
func TestBandSwitchingLoadConfigKeepsDefaultsOnGarbage(t *testing.T) {
	path := writeBandSwitchConfigFile(t, `{"enabled":true,`)

	svc := NewBandSwitchingService(nil, path)
	if got := svc.GetConfig(); got.CheckIntervalSec != defaultBandSwitchCheckInterval || got.Enabled {
		t.Errorf("a truncated config file must be ignored, got %+v", got)
	}
}

// A VALID file must still be honoured — loadConfig is not allowed to silently
// drop every config and always run on defaults.
func TestBandSwitchingLoadConfigAppliesValidFile(t *testing.T) {
	path := writeBandSwitchConfigFile(t,
		`{"enabled":true,"preferred_band":"2g","check_interval_sec":42,"down_switch_threshold_dbm":-70,`+
			`"up_switch_threshold_dbm":-60,"down_switch_delay_sec":30,"up_switch_delay_sec":15,`+
			`"min_viable_signal_dbm":-80}`)

	svc := NewBandSwitchingService(nil, path)
	got := svc.GetConfig()
	if got.CheckIntervalSec != 42 || !got.Enabled || got.PreferredBand != "2g" {
		t.Errorf("a valid config file was not applied: %+v", got)
	}
	if got.UpSwitchDelaySec != 15 {
		t.Errorf("up_switch_delay_sec = %d, want 15", got.UpSwitchDelaySec)
	}
}
