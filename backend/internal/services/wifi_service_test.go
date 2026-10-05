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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/auth"
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
	snapErr      error
	rollbackErr  error
	started      [][]string
	confirmed    []string
	snapshotted  [][]string
	rolledBack   []string
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

// Snapshot and Rollback are recorded only; this fake owns no files.
func (f *fakeWirelessApplier) Snapshot(configs []string) error {
	f.snapshotted = append(f.snapshotted, append([]string(nil), configs...))
	return f.snapErr
}

func (f *fakeWirelessApplier) Rollback(sessionID string) error {
	f.rolledBack = append(f.rolledBack, sessionID)
	return f.rollbackErr
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

	_, err := svc.SetMode("client", LockoutRequest{})
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
	}, LockoutRequest{})
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
	}, LockoutRequest{})
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
	}, LockoutRequest{})
	if err == nil {
		t.Error("expected error for nonexistent section")
	}
}

func TestSetAPConfig_ReturnsApplyError(t *testing.T) {
	svc, _ := newTestWifiService()
	svc.applier = &fakeWirelessApplier{startErr: errors.New("apply failed")}

	// radio1 carries no uplink STA, so enabling its AP is a legal request and
	// the apply failure is what this test is about.
	_, err := svc.SetAPConfig("default_radio1", models.APConfigUpdate{
		SSID:       "MyTravelRouter",
		Encryption: "psk2",
		Key:        "newpassword123",
		Enabled:    models.BoolPtr(true),
	}, LockoutRequest{})
	if err == nil || !strings.Contains(err.Error(), "apply failed") {
		t.Fatalf("expected apply error, got %v", err)
	}
}

// Enabling the access point on the radio that carries the uplink STA is refused
// outright now: any enabled mode=sta on that radio is the uplink, whether or not
// it carries network=wwan, so there is nothing left for the repeater reconcile
// to undo afterwards. The AP on the uplink PHY therefore stays off.
func TestSetAPConfig_RepeaterRefusesEnableOnTheSTARadio(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "sta0", "device", "radio0")
	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	if ap0 != "1" {
		t.Fatalf("precondition: expected AP on STA radio disabled, got %q", ap0)
	}

	_, err := svc.SetAPConfig("default_radio0", models.APConfigUpdate{
		SSID:       "Unified",
		Encryption: "psk2",
		Key:        "password12",
		Enabled:    models.BoolPtr(true),
	}, LockoutRequest{})
	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected ErrAPAndSTASameRadio, got %v", err)
	}
	if ap0, _ = u.Get("wireless", "default_radio0", "disabled"); ap0 != "1" {
		t.Errorf("expected AP on STA radio to stay disabled, got disabled=%q", ap0)
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
	}, LockoutRequest{}); err != nil {
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
	}, LockoutRequest{})
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

	// Verify network.guest interface. The address is not pinned to a literal:
	// what matters is that the guest subnet does not collide with network.lan,
	// which is why SetGuestWifi refuses an overlap (see
	// TestSetGuestWifi_RefusesASubnetThatOverlapsLAN).
	netOpts, err := u.GetAll("network", "guest")
	if err != nil {
		t.Fatalf("expected network.guest to exist: %v", err)
	}
	lanOpts, err := u.GetAll("network", "lan")
	if err != nil {
		t.Fatalf("expected network.lan to exist: %v", err)
	}
	if sameSubnet(t, netOpts["ipaddr"], netOpts["netmask"], lanOpts["ipaddr"], lanOpts["netmask"]) {
		t.Errorf("guest subnet %s/%s overlaps network.lan %s/%s",
			netOpts["ipaddr"], netOpts["netmask"], lanOpts["ipaddr"], lanOpts["netmask"])
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
	}, LockoutRequest{})

	// Disable
	_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{})
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

	_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{})
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
	}, LockoutRequest{})
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

	_, err := svc.SetRadioEnabled(false, LockoutRequest{})
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
	_, _ = svc.SetRadioEnabled(false, LockoutRequest{})
	// Then enable
	_, err := svc.SetRadioEnabled(true, LockoutRequest{})
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

	_, err := svc.SetRadioEnabled(false, LockoutRequest{})
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

	_, err := svc.SetMode("client", LockoutRequest{})
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
	// The downlink AP on the free radio must be ENABLED for repeater mode to have
	// somewhere to put the AP. A config whose only other-radio AP is disabled is
	// refused rather than silently enabling an AP onto the uplink radio — see
	// TestSetMode_RepeaterRefusesWhenNoOtherRadioAPIsEnabled.
	_ = u.Set("wireless", "default_radio1", "disabled", "0")
	_ = u.Set("wireless", "sta0", "disabled", "1")
	// sta0 lives on radio0 per the mock UCI. Ensure default_radio0 AP is on radio0 too.
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")

	_, err := svc.SetMode("repeater", LockoutRequest{})
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

	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
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
	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
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
	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
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

	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
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

	_, err := svc.SetMode("invalid", LockoutRequest{})
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
	registerInterfaceStatus(t, svc, map[string]bool{"default_radio0": true, "default_radio1": true})
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

// registerInterfaceStatus publishes a `network.wireless status` payload for the
// named sections, each reported up or down. Liveness goes where netifd puts it
// on this firmware: on the RADIO (up/pending/retry_setup_failed), not on the
// interface entry, which carries exactly section, config, ifname, vlans and
// stations. A radio is up when any of the sections it owns is up.
//
// `network.device status` is registered DOWN for these fixtures: they vary the
// radio-level signal, and the per-interface fallback has its own tests. (The
// mock cannot answer per ifname, so a mixed answer is not expressible here.)
func registerInterfaceStatus(t *testing.T, svc *WifiService, up map[string]bool) {
	t.Helper()
	sections, err := svc.uci.GetSections("wireless")
	if err != nil {
		t.Fatalf("reading wireless sections: %v", err)
	}
	byRadio := map[string][]any{}
	radioUp := map[string]bool{}
	names := slices.Sorted(maps.Keys(up))
	for _, name := range names {
		opts := sections[name]
		radio := opts["device"]
		ifname := "phy0-ap0"
		if opts["mode"] == "sta" {
			ifname = "phy0-sta0"
		}
		byRadio[radio] = append(byRadio[radio], map[string]any{
			"section":  name,
			"ifname":   ifname,
			"config":   map[string]any{"mode": opts["mode"], "ssid": opts["ssid"]},
			"vlans":    []any{},
			"stations": []any{},
		})
		if up[name] {
			radioUp[radio] = true
		}
	}
	resp := map[string]any{}
	for radio, ifaces := range byRadio {
		resp[radio] = map[string]any{
			"up":                 radioUp[radio],
			"pending":            false,
			"autostart":          true,
			"disabled":           false,
			"retry_setup_failed": false,
			"interfaces":         ifaces,
		}
	}
	registerPayload(t, svc, resp)
	registerDeviceStatus(t, svc, map[string]bool{"fixture radios settle nothing": false})
}

// registerPayload installs a whole `network.wireless status` response.
func registerPayload(t *testing.T, svc *WifiService, resp map[string]any) {
	t.Helper()
	ub, ok := svc.ubus.(*ubus.MockUbus)
	if !ok {
		t.Fatal("expected a MockUbus")
	}
	ub.RegisterResponse("network.wireless.status", resp)
}

// registerDeviceStatus installs a `network.device status` answer. The mock keys
// responses by path and method and ignores the requested device name, so the
// answer applies to every ifname the probe asks about; the map exists so a test
// says what it means.
func registerDeviceStatus(t *testing.T, svc *WifiService, up map[string]bool) {
	t.Helper()
	anyUp := false
	for _, isUp := range up {
		anyUp = anyUp || isUp
	}
	ub, ok := svc.ubus.(*ubus.MockUbus)
	if !ok {
		t.Fatal("expected a MockUbus")
	}
	ub.RegisterResponse("network.device.status", map[string]any{
		"external": true,
		"present":  true,
		"up":       anyUp,
		"carrier":  anyUp,
	})
}

// setRadioPending flips a radio-level liveness flag on the payload a preceding
// registerInterfaceStatus installed.
func setRadioPending(t *testing.T, svc *WifiService, radio string, pending bool) {
	t.Helper()
	resp, err := svc.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		t.Fatalf("reading back the registered payload: %v", err)
	}
	entry, ok := resp[radio].(map[string]any)
	if !ok {
		t.Fatalf("the registered payload has no radio %s", radio)
	}
	entry["pending"] = pending
}

// quietConfirmRetries drops the wait between confirm probes so a test that
// deliberately exhausts the retry budget does not have to spend it.
func quietConfirmRetries(t *testing.T) {
	t.Helper()
	prev := wirelessRetrySleep
	wirelessRetrySleep = func(time.Duration) {}
	t.Cleanup(func() { wirelessRetrySleep = prev })
}

// A radio that stays down must NOT have its rollback cancelled: that is the
// "proof of reachability" docs/architecture.md §3 and ADR 0002 §5 require, and
// it is what a client-confirms-instantly bug silently skipped.
func TestConfirmApply_KeepsRollbackArmedWhenAPIsDown(t *testing.T) {
	svc, _ := newTestWifiService()
	registerInterfaceStatus(t, svc, map[string]bool{"default_radio0": true, "default_radio1": false})
	quietConfirmRetries(t)
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

// A radio is a per-RADIO signal, so on a radio hosting TWO wanted interfaces one
// of them coming up used to prove the other. This layout is real: SetGuestWifi
// only excludes the uplink radio, so on the captured layout the guest AP lands
// beside the main AP on radio1. Confirming there with a dead main AP silently
// removes the operator's main SSID and cancels the rollback that would have
// restored it.
func TestConfirmApply_SettledRadioDoesNotProveASecondInterfaceOnIt(t *testing.T) {
	svc, u := newTestWifiService()
	// Guest AP beside the main AP on radio1, both wanted.
	if err := u.Set("wireless", "default_radio0", "device", "radio1"); err != nil {
		t.Fatal(err)
	}
	// The radio reads up because ONE of its interfaces came up...
	registerInterfaceStatus(t, svc, map[string]bool{"default_radio0": true, "default_radio1": false})
	// ...and both per-device answers say the interfaces are not live.
	registerDeviceStatus(t, svc, map[string]bool{"phy0-ap0": false})
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-two-ap"); !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("a settled radio must not prove a second interface on it, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("the rollback must stay armed when one of two interfaces on a radio is down, "+
			"got confirm %#v", fake.confirmed)
	}
}

// The single-interface case keeps working: a settled radio with nothing else on
// it is still proof, without paying a device round-trip.
func TestConfirmApply_SettledRadioStillProvesItsOnlyInterface(t *testing.T) {
	svc, u := newTestWifiService()
	// Only one wanted interface in total, so the radio has nothing else on it.
	if err := u.Set("wireless", "default_radio1", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	registerInterfaceStatus(t, svc, map[string]bool{"default_radio0": true})
	// The per-device fallback answers down; a settled single-interface radio must
	// not need it.
	registerDeviceStatus(t, svc, map[string]bool{"phy0-ap0": false})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-single-ap"); err != nil {
		t.Fatalf("a settled radio carrying one wanted interface must prove it, got %v", err)
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

func TestConfirmApply_FailsClosedWhenWirelessStatusIsUnreadable(t *testing.T) {
	svc, _ := newTestWifiService()
	ub := svc.ubus.(*ubus.MockUbus)
	ub.RegisterResponse("network.wireless.status", nil)
	// A nil registered response makes the mock answer with no interfaces, i.e. no
	// AP is observable at all.
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-unknown"); !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("expected ErrWirelessNotUp when the interfaces cannot be observed, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("unknown state must not cancel the rollback, got confirm %#v", fake.confirmed)
	}
}

// A malformed radio the apply has nothing to do with must not turn a transient
// netifd hiccup into a proof that is never retried: the interfaces that read
// fine are still proven, and the radio that owns the wanted sections is the one
// that decides.
func TestConfirmApply_UnrelatedUnreadableRadioDoesNotAbortTheProof(t *testing.T) {
	svc, _ := newTestWifiService()
	registerInterfaceStatus(t, svc, map[string]bool{"default_radio0": true, "default_radio1": true})
	resp, err := svc.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		t.Fatalf("reading back the registered payload: %v", err)
	}
	// A third radio netifd answers in a shape this build cannot read. It owns no
	// wanted section.
	resp["radio2"] = "up"
	registerPayload(t, svc, resp)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-foreign-shape"); err != nil {
		t.Fatalf("an unreadable unrelated radio must not abort the proof, got %v", err)
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

// ...but it still fails closed for the interfaces it might own.
func TestConfirmApply_UnreadableRadioStillFailsClosedForItsInterfaces(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "default_radio0", "device", "radio2"); err != nil {
		t.Fatal(err)
	}
	registerInterfaceStatus(t, svc, map[string]bool{"default_radio0": true, "default_radio1": true})
	resp, err := svc.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		t.Fatalf("reading back the registered payload: %v", err)
	}
	resp["radio2"] = "up"
	registerPayload(t, svc, resp)
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	err = svc.ConfirmApply("session-own-shape")
	if !errors.Is(err, ErrWirelessUnverifiable) {
		t.Fatalf("an unreadable radio that owns a wanted section must not confirm, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("an unreadable payload must not cancel the rollback, got confirm %#v", fake.confirmed)
	}
}

// "Turn WiFi off" leaves neither an access point nor an uplink STA, so there is
// nothing to prove and the operator's change must be allowed to stick.
func TestConfirmApply_SkipsProbeWhenWiFiTurnedOff(t *testing.T) {
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
	// The saved STA profile is the only section left that could be "wanted", and
	// a disabled one is not. Bound to wwan and enabled it would be: that is the
	// client-mode probe, not the WiFi-off case.
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	registerInterfaceStatus(t, svc, map[string]bool{
		"default_radio0": false, "default_radio1": false, "sta0": false,
	})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-off"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

// ---------------------------------------------------------------------------
// Confirm-time verification: the uplink STA is proven, not just the access
// points; an AP section netifd cannot report is unprovable rather than absent;
// and the probe cost is reported so the client can keep its deadline honest.
// ---------------------------------------------------------------------------

// putInClientMode turns the mock config into what SetMode("client") leaves
// behind: no enabled access point, one enabled uplink STA bound to network=wwan.
func putInClientMode(t *testing.T, u *uci.MockUCI) {
	t.Helper()
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "disabled", "1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
}

// A client-mode change has no access point to prove, so while the proof covered
// only APs `want` was empty and ConfirmApply cancelled rpcd's rollback no matter
// what: an STA on the wrong band or with the wrong key left the operator with no
// uplink and no way back over WiFi.
func TestConfirmApply_ClientModeWithDownSTADoesNotConfirm(t *testing.T) {
	svc, u := newTestWifiService()
	putInClientMode(t, u)
	registerInterfaceStatus(t, svc, map[string]bool{"sta0": false})
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	err := svc.ConfirmApply("session-client-down")
	if !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("expected ErrWirelessNotUp for a client-mode apply with no uplink, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("the apply session must stay open while the uplink is down, got confirm %#v",
			fake.confirmed)
	}
}

func TestConfirmApply_ClientModeWithUpSTAConfirms(t *testing.T) {
	svc, u := newTestWifiService()
	putInClientMode(t, u)
	registerInterfaceStatus(t, svc, map[string]bool{"sta0": true})
	// The uplink STA is proven by carrier, not by its radio: a configured station
	// that never associated also leaves the radio up. See
	// TestConfirmApply_UplinkSTANeverAssociatedIsNotUp.
	registerDeviceStatus(t, svc, map[string]bool{"sta0": true})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-client-up"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.confirmed) != 1 || fake.confirmed[0] != "session-client-up" {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

// Repeater mode is no escape hatch: the downlink AP can come up while the uplink
// never associates, and that is the same unrecoverable state.
//
// netifd cannot express "AP up, uplink not associated" through the radio-level
// signal — a radio is up or it is not — so what a still-assocsiating uplink
// actually looks like is a radio that has not settled yet. That is the fixture:
// the radio the uplink lives on is pending.
func TestConfirmApply_RepeaterModeAlsoProbesTheSTA(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	registerInterfaceStatus(t, svc, map[string]bool{
		"default_radio0": true, "default_radio1": true, "sta0": false,
	})
	setRadioPending(t, svc, "radio0", true)
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-repeater-down"); !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("expected ErrWirelessNotUp when the uplink STA is down, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("a down uplink STA must not cancel the rollback, got confirm %#v", fake.confirmed)
	}
}

// A section netifd cannot be asked about proves nothing. Skipping it (as this
// used to) could empty the want-list and turn "the AP never came up" into
// "nothing to prove".
func TestEnabledAPSections_APWithoutDeviceIsUnprovable(t *testing.T) {
	sections := map[string]map[string]string{
		"radio0":  {"type": "mac80211", "disabled": "0"},
		"orphan":  {"mode": "ap", "disabled": "0"},
		"radio1":  {"type": "mac80211", "disabled": "0"},
		"default": {"mode": "ap", "device": "radio1", "disabled": "0"},
	}
	if _, err := enabledAPSections(sections); !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("an enabled AP section with no device must be unprovable, got %v", err)
	}
	delete(sections, "orphan")
	names, err := enabledAPSections(sections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "default" {
		t.Fatalf("expected only the provable AP, got %v", names)
	}
}

func TestConfirmApply_KeepsRollbackArmedWhenAPSectionHasNoDevice(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "default_radio0", "device", ""); err != nil {
		t.Fatal(err)
	}
	registerInterfaceStatus(t, svc, map[string]bool{"default_radio0": true, "default_radio1": true})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-orphan"); !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("expected ErrWirelessNotUp for an unprovable AP section, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("an unprovable AP must not cancel the rollback, got confirm %#v", fake.confirmed)
	}
}

// The confirm probe must not spend its retry budget in the common case, and the
// caller has to be able to subtract that budget from its own rollback deadline:
// a probe started near the deadline is answered after rpcd already rolled back,
// which reverts a config that was merely slow.
func TestConfirmApply_ProbesImmediatelyAndReportsProbeBudget(t *testing.T) {
	svc, u := newTestWifiService()
	putInClientMode(t, u)
	registerInterfaceStatus(t, svc, map[string]bool{"sta0": true})
	registerDeviceStatus(t, svc, map[string]bool{"sta0": true})
	svc.applier = &fakeWirelessApplier{startToken: "budget-token"}

	apply, err := svc.stageWirelessApply()
	if err != nil {
		t.Fatalf("stageWirelessApply: %v", err)
	}
	if apply.ProbeBudgetSeconds != wirelessProbeBudgetSeconds() {
		t.Fatalf("expected the apply result to report a probe budget of %ds, got %d",
			wirelessProbeBudgetSeconds(), apply.ProbeBudgetSeconds)
	}
	if apply.ProbeBudgetSeconds <= 0 || apply.ProbeBudgetSeconds >= apply.RollbackTimeoutSeconds {
		t.Fatalf("the probe budget (%ds) must be positive and fit inside the rollback window (%ds)",
			apply.ProbeBudgetSeconds, apply.RollbackTimeoutSeconds)
	}
	// The budget is what the client subtracts from its own deadline, so it has to
	// cover the whole time ONE probe can block: the sleeps between retries AND
	// the ubus round-trips the per-interface fallback makes. Counting only the
	// sleeps (as this used to) understated the blocking time by 2.5 s per
	// attempt and pushed the last probe to within half a second of rpcd's
	// rollback.
	sleepOnly := int((wirelessConfirmAttempts - 1) * wirelessConfirmDelay / time.Second)
	if apply.ProbeBudgetSeconds <= sleepOnly {
		t.Fatalf("the published budget (%ds) does not account for the ubus round-trips "+
			"of the per-interface fallback on top of the %ds of sleeps",
			apply.ProbeBudgetSeconds, sleepOnly)
	}
	// ...without giving up the patience the device needs: after `uci apply`
	// netifd restarts the radios and hostapd finishes ACS before the AP links
	// up, which the device log puts at ~10 s.
	const deviceLinkSeconds = 10
	if sleepOnly < deviceLinkSeconds {
		t.Fatalf("the probe waits %ds, less than the ~%ds the device needs to link an AP up",
			sleepOnly, deviceLinkSeconds)
	}
	// The client reserves the budget plus its own margin and caps that
	// reservation at half the rollback window. A budget that outgrows the cap
	// would be silently truncated there, and the probe would then be issued too
	// late — so the cap is asserted here rather than discovered on a device.
	//
	// The margin is the one number wifi-apply.ts reserves
	// (PROBE_SAFETY_MARGIN_MS), stated here in seconds because the gate speaks
	// in the units of the published budget. Keeping them apart is what let the
	// Go comment say "half a second" while the gate reserved a full one;
	// TestClientProbeSafetyMarginMatchesTheGoGate is what keeps them one number.
	const probeMarginSeconds = wirelessProbeSafetyMarginSeconds
	reserved := apply.ProbeBudgetSeconds + probeMarginSeconds
	if reserved*2 > apply.RollbackTimeoutSeconds {
		t.Fatalf("budget %ds plus %ds of client margin outgrows half the %ds rollback window",
			apply.ProbeBudgetSeconds, probeMarginSeconds, apply.RollbackTimeoutSeconds)
	}

	fake := &fakeWirelessApplier{}
	svc.applier = fake
	start := time.Now()
	if err := svc.ConfirmApply("session-fast"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= wirelessConfirmDelay {
		t.Fatalf("an already-up config answered only after %s; the first probe must be immediate",
			elapsed)
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

	_, err := svc.SetMode("client", LockoutRequest{})
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
	_, err := svc.SetMode("client", LockoutRequest{})
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
	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
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

	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
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

	if _, err := svc.SetMode("client", LockoutRequest{}); err != nil {
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

	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
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

// Regression: a background radio switch must never put the uplink on a PHY that
// runs an access point (ath11k/IPQ6018 crashes if AP and STA share one PHY).
// The switch splits the AP off the target radio when it can, and refuses with
// ErrAPAndSTASameRadio when it cannot. Both branches are pinned in
// wifi_radio_choice_test.go, which supersedes the earlier "always refuse"
// contract this test used to assert.

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

	if _, err := svc.SetRadioRole("radio1", "sta", LockoutRequest{}); !errors.Is(err, ErrMultipleActiveSTA) {
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
	if _, err := svc.SetRadioRole("radio1", "sta", LockoutRequest{}); err != nil {
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
	apply, err := svc.SetRadioRole("radio0", "ap", LockoutRequest{})
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

	if _, err := svc.SetRadioRole("radio0", "both", LockoutRequest{}); !errors.Is(err, ErrAPAndSTASameRadio) {
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

	if _, err := svc.SetRadioRole("radio0", "both", LockoutRequest{}); err != nil {
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

	if _, err := svc.SetRadioRole("radio0", "both", LockoutRequest{}); err != nil {
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

	if _, err := svc.SetRadioRole("radio0", "sta", LockoutRequest{}); err != nil {
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
// The same-radio refusal has to cover every writer of an AP section, not just
// SetRadioRole: SetGuestWifi and SetAPConfig create/enable APs too, and
// reconcileRepeaterAPRadioLayout protects neither outside repeater mode.
// ---------------------------------------------------------------------------

// clientModeService returns a service on the mock config switched to client
// mode: the uplink STA sta0 is enabled and bound to network=wwan on radio0 — the
// 2.4 GHz radio preferredGuestRadio picks, and the one a repeater or client STA
// is normally on — with no access point enabled.
func clientModeService(t *testing.T, allowAPOnSTA bool) (*WifiService, *revertingUCI) {
	t.Helper()
	svc, u := repeaterModeService(t, allowAPOnSTA)
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "disabled", "1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	return svc, u
}

func guestEnable() models.GuestWifiConfig {
	return models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-Travel", Encryption: "psk2", Key: "guestpass123",
	}
}

// The 2.4 GHz preference must not be a hard veto. On the archetypal travel
// router the uplink STA is on the 2.4 GHz radio, so preferring it made the
// guest access point a 409 with no way forward but allow_ap_on_sta_radio — i.e.
// deliberately re-enabling the crash state the guard exists to prevent. The
// guest AP belongs on the radio that does NOT carry the uplink, exactly like
// the downlink AP does.
func TestSetGuestWifi_FallsBackToTheRadioWithoutTheUplink(t *testing.T) {
	svc, u := clientModeService(t, false)

	if _, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{}); err != nil {
		t.Fatalf("SetGuestWifi on the 2.4 GHz uplink radio must fall back, got %v", err)
	}
	device, _ := u.Get("wireless", "guest", "device")
	if device != "radio1" {
		t.Errorf("expected the guest AP on radio1, the radio without the uplink, got %q", device)
	}
	if dis, _ := u.Get("wireless", "guest", "disabled"); dis != "0" {
		t.Errorf("expected the guest AP to be enabled, got disabled=%q", dis)
	}
	for _, radio := range []string{"radio0", "radio1"} {
		assertNoSameRadioAPSTA(t, u, radio)
	}
}

// Single-radio hardware has no split to make, and rejectAPOnUplinkRadio accepts
// the coexistence there — the same trade-off SetMode("repeater") and
// SetRadioRole already accept, and the only case left where preferredGuestRadio
// cannot fall back. Pinned so guest WiFi cannot silently become impossible on
// single-radio travel routers.
func TestSetGuestWifi_SingleRadioKeepsCoexistence(t *testing.T) {
	svc, u := repeaterModeService(t, false)
	for _, section := range []string{"radio1", "default_radio1"} {
		if err := u.DeleteSection("wireless", section); err != nil {
			t.Fatal(err)
		}
	}
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{}); err != nil {
		t.Fatalf("guest WiFi must stay available on single-radio hardware: %v", err)
	}
	device, _ := u.Get("wireless", "guest", "device")
	if device != "radio0" {
		t.Errorf("expected the guest AP on the only radio, got %q", device)
	}
}

func TestSetGuestWifi_AllowedOnTheRadioWithoutTheUplink(t *testing.T) {
	svc, u := clientModeService(t, false)
	// Move the uplink to the 5 GHz radio; the guest AP then lands on radio0,
	// which is what the split layout asks for.
	if err := u.Set("wireless", "sta0", "device", "radio1"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{}); err != nil {
		t.Fatalf("SetGuestWifi: %v", err)
	}
	device, _ := u.Get("wireless", "guest", "device")
	if device != "radio0" {
		t.Errorf("expected the guest AP on radio0, got %q", device)
	}
}

// The guest AP no longer needs the escape hatch to sit on the 2.4 GHz uplink
// radio: preferredGuestRadio falls back to the other one. The hatch is still
// what lets an access point STAY on the uplink radio, which is what the
// SetAPConfig sibling test covers.
func TestSetGuestWifi_AllowedWithAllowAPOnSTARadio(t *testing.T) {
	svc, u := clientModeService(t, true)

	if _, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{}); err != nil {
		t.Fatalf("guest WiFi must be available with allow_ap_on_sta_radio: %v", err)
	}
	if dis, _ := u.Get("wireless", "guest", "disabled"); dis != "0" {
		t.Errorf("expected the guest AP to be enabled, got disabled=%q", dis)
	}
}

// ---------------------------------------------------------------------------
// Connect activates the uplink STA, so it needs the same explicit
// AP-on-the-target-radio check the other writers have: reconcileRepeaterAP-
// RadioLayout returns immediately outside repeater mode, so outside it nothing
// stood between Connect and an enabled STA sharing a PHY with an enabled AP.
// ---------------------------------------------------------------------------

// clientModeWithAPService returns a service forced into client mode by its mode
// file — so reconcileRepeaterAPRadioLayout is a no-op, exactly as it is on a
// device whose operator never selected repeater mode — with the uplink STA on
// radio0 and no access point enabled. Tests enable the access points they need.
func clientModeWithAPService(t *testing.T) (*WifiService, *revertingUCI) {
	t.Helper()
	svc, u := clientModeService(t, false)
	modeFile := filepath.Join(t.TempDir(), "wifi-mode")
	if err := os.WriteFile(modeFile, []byte("client"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc.modeFile = modeFile
	if got := svc.deriveWifiMode(); got != "client" {
		t.Fatalf("fixture is not in client mode, got %q", got)
	}
	return svc, u
}

// A second enabled access point on ANOTHER radio is the layout ADR 0002 §2 asks
// for, so Connect is not refused; with no split available the uplink would put
// an AP and a STA on the only PHY that runs an access point, which is refused
// rather than silently taking the router's WiFi away.
func TestConnect_RefusesWhenTheTargetRadioHasAnEnabledAP(t *testing.T) {
	svc, u := clientModeWithAPService(t)
	// An access point on radio0 and none anywhere else: freeing radio0 would
	// mean disabling the router's only WiFi.
	if err := u.Set("wireless", "default_radio0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	if err := u.DeleteSection("wireless", "default_radio1"); err != nil {
		t.Fatal(err)
	}
	applier := &fakeWirelessApplier{startToken: "must-not-be-used"}
	svc.applier = applier

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Band: "2g", Encryption: "psk2", Password: "cafepass1",
	})
	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected ErrAPAndSTASameRadio, got %v", err)
	}
	if dis, _ := u.Get("wireless", "default_radio0", "disabled"); dis == "1" {
		t.Error("the refused connect disabled the access point anyway")
	}
	for _, c := range u.commitCalls() {
		if c == "wireless" {
			t.Error("wireless was committed after the connect was refused")
		}
	}
	if len(applier.started) != 0 {
		t.Errorf("no apply may be started for a refused connect, got %v", applier.started)
	}
}

// The split layout is not a refusal: with an enabled access point on the other
// radio the uplink is welcome, and the AP sharing the target radio is disabled
// so the PHY is not asked to do both — the same move SetMode("repeater") makes,
// applied here in client mode too.
func TestConnect_MovesTheEnabledAPOffTheUplinkRadio(t *testing.T) {
	svc, u := clientModeWithAPService(t)
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "disabled", "0"); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Band: "2g", Encryption: "psk2", Password: "cafepass1",
	}); err != nil {
		t.Fatalf("Connect must be allowed when the downlink AP has the other radio: %v", err)
	}
	dis, _ := u.Get("wireless", "default_radio0", "disabled")
	if dis != "1" {
		t.Errorf("expected the access point on the uplink radio to be disabled, got %q", dis)
	}
	if dis, _ := u.Get("wireless", "default_radio1", "disabled"); dis != "0" {
		t.Errorf("expected the access point on the other radio to stay enabled, got %q", dis)
	}
	assertNoSameRadioAPSTA(t, u, "radio0")
}

// allow_ap_on_sta_radio is the operator's explicit override for the case
// SetAPConfig cannot resolve itself: the AP sits on the radio the operator named
// and there is no split to move it to.
func TestSetAPConfig_EnableAllowedWithAllowAPOnSTARadio(t *testing.T) {
	svc, u := clientModeService(t, true)

	enabled := true
	if _, err := svc.SetAPConfig("default_radio0", models.APConfigUpdate{
		SSID: "OpenWrt-Travel", Encryption: "psk2", Key: "travelrouter", Enabled: &enabled,
	}, LockoutRequest{}); err != nil {
		t.Fatalf("allow_ap_on_sta_radio must enable an access point on the uplink radio: %v", err)
	}
	if dis, _ := u.Get("wireless", "default_radio0", "disabled"); dis != "0" {
		t.Errorf("expected the access point to be enabled, got disabled=%q", dis)
	}
}

// ---------------------------------------------------------------------------
// Finding 3: role "ap" on the uplink radio is deliberately AP-only, not a
// silent AP+STA commit and not a refusal of the ordinary "make this radio an
// AP" action.
// ---------------------------------------------------------------------------

func TestSetRadioRole_APRoleOnUplinkRadioLeavesAPOnly(t *testing.T) {
	svc, u := clientModeService(t, false)

	if _, err := svc.SetRadioRole("radio0", "ap", LockoutRequest{}); err != nil {
		t.Fatalf("role 'ap' on the uplink radio is a legitimate request: %v", err)
	}
	assertNoSameRadioAPSTA(t, u, "radio0")
	if dis, _ := u.Get("wireless", "sta0", "disabled"); dis != "1" {
		t.Errorf("expected the uplink STA on the AP radio to be disabled, got %q", dis)
	}
	dis, _ := u.Get("wireless", "default_radio0", "disabled")
	if dis == "1" {
		t.Errorf("expected the access point to be enabled, got disabled=%q", dis)
	}
}

// Same guarantee for a radio that has no AP section yet: the created default AP
// must still leave the radio AP-only.
func TestSetRadioRole_APRoleOnUplinkRadioCreatesAPOnly(t *testing.T) {
	svc, u := clientModeService(t, false)
	if err := u.DeleteSection("wireless", "default_radio0"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetRadioRole("radio0", "ap", LockoutRequest{}); err != nil {
		t.Fatalf("role 'ap' on the uplink radio is a legitimate request: %v", err)
	}
	if _, err := u.GetAll("wireless", "ap_radio0"); err != nil {
		t.Fatalf("expected a default AP to be created on radio0: %v", err)
	}
	assertNoSameRadioAPSTA(t, u, "radio0")
}

func TestSetAPConfig_EnableRefusesUplinkRadio(t *testing.T) {
	svc, u := clientModeService(t, false)

	enabled := true
	_, err := svc.SetAPConfig("default_radio0", models.APConfigUpdate{
		SSID: "OpenWrt-Travel", Encryption: "psk2", Key: "travelrouter", Enabled: &enabled,
	}, LockoutRequest{})
	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected ErrAPAndSTASameRadio, got %v", err)
	}
	dis, _ := u.Get("wireless", "default_radio0", "disabled")
	if dis != "1" {
		t.Errorf("the refused request enabled the access point anyway, got disabled=%q", dis)
	}
	if commits := u.commitCalls(); len(commits) != 0 {
		t.Errorf("a refused request must commit nothing, got %v", commits)
	}
}

func TestSetAPConfig_EnableAllowedOnTheRadioWithoutTheUplink(t *testing.T) {
	svc, u := clientModeService(t, false)
	if err := u.Set("wireless", "sta0", "device", "radio1"); err != nil {
		t.Fatal(err)
	}

	enabled := true
	if _, err := svc.SetAPConfig("default_radio0", models.APConfigUpdate{
		SSID: "OpenWrt-Travel", Encryption: "psk2", Key: "travelrouter", Enabled: &enabled,
	}, LockoutRequest{}); err != nil {
		t.Fatalf("SetAPConfig: %v", err)
	}
	dis, _ := u.Get("wireless", "default_radio0", "disabled")
	if dis != "0" {
		t.Errorf("expected the access point to be enabled, got disabled=%q", dis)
	}
}

// A refused role must not leave network.wwan and a wan-zone firewall entry
// behind: ensureWwanNetwork COMMITS both, and a commit cannot be undone by the
// staged-delta revert, so the answer "refused" would be a lie.
func TestSetRadioRole_RefusalDoesNotCommitWwanOrFirewall(t *testing.T) {
	svc, u := repeaterModeService(t, false)
	// No STA section on radio0 left, so the request would have to create one —
	// and creating one is what reaches ensureWwanNetwork.
	if err := u.DeleteSection("wireless", "sta0"); err != nil {
		t.Fatal(err)
	}
	if _, err := u.GetAll("network", "wwan"); err == nil {
		t.Fatal("expected the mock to start without network.wwan")
	}

	if _, err := svc.SetRadioRole("radio0", "both", LockoutRequest{}); !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected ErrAPAndSTASameRadio, got %v", err)
	}
	if _, err := u.GetAll("network", "wwan"); err == nil {
		t.Error("the refused role created and committed network.wwan")
	}
	if net, _ := u.Get("firewall", "zone_wan", "network"); strings.Contains(net, "wwan") {
		t.Errorf("the refused role added wwan to the wan firewall zone, got %q", net)
	}
	for _, c := range []string{"network", "firewall"} {
		if slices.Contains(u.commitCalls(), c) {
			t.Errorf("%s was committed for a refused role", c)
		}
	}
}

// ---------------------------------------------------------------------------
// Finding 9: disabling guest WiFi must tear the whole guest network down.
// ---------------------------------------------------------------------------

func TestSetGuestWifi_DisableTearsDownGuestNetwork(t *testing.T) {
	svc, u := newTestWifiService()

	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-Travel", Encryption: "psk2", Key: "guestpass123",
	}, LockoutRequest{}); err != nil {
		t.Fatalf("enable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{}); err != nil {
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
	}, LockoutRequest{}); err != nil {
		t.Fatalf("enable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{}); err != nil {
		t.Fatalf("disable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-2", Encryption: "psk2", Key: "guestpass123",
	}, LockoutRequest{}); err != nil {
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

// Snapshot and Rollback own no files here; the guard test is about ordering.
func (g *guardCheckingApplier) Snapshot([]string) error { return nil }

func (g *guardCheckingApplier) Rollback(string) error { return nil }

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
		_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: true, SSID: "guest", Key: "guestpass1"}, LockoutRequest{})
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
			_, _ = svc.SetAPConfig("default_radio0", models.APConfigUpdate{SSID: "travo", Enabled: &enabled}, LockoutRequest{})
		},
		"SetRadioRole":  func() { _, _ = svc.SetRadioRole("radio0", "ap", LockoutRequest{}) },
		"SetMode":       func() { _, _ = svc.SetMode("ap", LockoutRequest{}) },
		"SetMACAddress": func() { _, _ = svc.SetMACAddress("AA:BB:CC:DD:EE:01") },
		"RandomizeMAC":  func() { _, _, _ = svc.RandomizeMAC() },
		"Connect":       func() { _, _ = svc.Connect(models.WifiConfig{SSID: "upstream", Password: "secret1234"}) },
		"Disconnect":    func() { _, _ = svc.Disconnect() },
		"DeleteNetwork": func() { _, _ = svc.DeleteNetwork("nonexistent") },
		"SetGuestWifi": func() {
			_, _ = svc.SetGuestWifi(models.GuestWifiConfig{Enabled: true, SSID: "guest", Key: "guestpass1"}, LockoutRequest{})
		},
		// The disable branch is the only way to reach teardownGuestWifi, which
		// is deliberately NOT wrapped: it is called from inside SetGuestWifi's
		// mutateWireless closure, so wrapping it would self-deadlock. It is
		// listed here precisely so that adding that wrapper — the obvious next
		// "simplification", since its body is shaped like every other mutator —
		// fails this test instead of wedging the guest-WiFi endpoint in
		// production while CI stays green.
		"teardownGuestWifi": func() {
			_, _ = svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{})
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

// TestRadioForNewSTAIsDeterministic pins a bug this branch's own new check
// exposed rather than caused.
//
// Connect used to pick the radio for a new uplink STA by taking the first entry
// found while ranging over the wireless-section map, so Go's randomised
// iteration order decided it. Downstream that was not cosmetic: the chosen radio
// decides whether the uplink shares a PHY with an access point, and the new
// splitAPOffUplinkRadio check refuses when it would. The result was an
// intermittent "refusing to run an access point and the WiFi uplink on the same
// radio" that depended on map order — a ~40% failure rate in
// TestWifiConnect_MultipleProfilesPersist.
func TestRadioForNewSTAIsDeterministic(t *testing.T) {
	svc, u := newTestWifiService()

	// Both radios carry an enabled access point, the stock layout, so nothing
	// is preferable and the sorted first radio must win every time.
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "mode", "ap"); err != nil {
			t.Fatal(err)
		}
		if err := u.Set("wireless", section, "device", section); err != nil {
			t.Fatal(err)
		}
		if err := u.Set("wireless", section, "disabled", "0"); err != nil {
			t.Fatal(err)
		}
	}

	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("read sections: %v", err)
	}

	first, err := svc.radioForNewSTA(sections)
	if err != nil {
		t.Fatalf("radioForNewSTA: %v", err)
	}
	if first != "radio0" {
		t.Errorf("radioForNewSTA = %q, want radio0 (sorted, not map order)", first)
	}

	// Repeat: the choice must not move between calls on identical input.
	for i := 0; i < 50; i++ {
		again, err := svc.radioForNewSTA(sections)
		if err != nil {
			t.Fatalf("radioForNewSTA call %d: %v", i, err)
		}
		if again != first {
			t.Fatalf("radioForNewSTA is not deterministic: call %d returned %q, first returned %q",
				i, again, first)
		}
	}
}

// TestRadioForNewSTAPrefersRadioWithoutAccessPoint pins the second property:
// when one radio is free of access points the uplink should go there, so a
// second Connect does not land on the radio holding the only remaining access
// point and get refused for it.
func TestRadioForNewSTAPrefersRadioWithoutAccessPoint(t *testing.T) {
	svc, u := newTestWifiService()

	// Simulate the state a first Connect leaves behind: radio0's AP disabled by
	// splitAPOffUplinkRadio, radio1 still carrying the only enabled AP.
	if err := u.Set("wireless", "default_radio0", "mode", "ap"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio0", "device", "radio0"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio1", "mode", "ap"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio1", "device", "radio1"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio1", "disabled", "0"); err != nil {
		t.Fatal(err)
	}

	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("read sections: %v", err)
	}
	got, err := svc.radioForNewSTA(sections)
	if err != nil {
		t.Fatalf("radioForNewSTA: %v", err)
	}
	if got != "radio0" {
		t.Errorf("radioForNewSTA = %q, want radio0: it has no enabled access point, so "+
			"choosing it avoids disabling a second one and avoids being refused for "+
			"sharing a PHY with the only remaining AP", got)
	}
}

// TestConnectTwiceSucceeds is the regression the two properties exist for: two
// consecutive connects with no band pinned must both succeed. Before, the second
// one could be refused because the new STA landed on the radio that still held
// the only enabled access point.
func TestConnectTwiceSucceeds(t *testing.T) {
	svc, _ := newTestWifiService()

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Coffee-Shop", Password: "coffee123", Encryption: "psk2",
	}); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Airport-WiFi", Password: "air456", Encryption: "psk2",
	}); err != nil {
		t.Fatalf("second connect was refused: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The probe reads netifd's real payload shape.
//
// goldenWirelessStatus is the trimmed, verbatim `ubus call network.wireless
// status` output captured on the test device (OpenWrt 25.12.3, netifd
// 2026.02.26-r1) with every radio up and both the 2 GHz AP and the 5 GHz uplink
// STA live. Only the WPA keys are redacted; they are not read by the probe and
// they are a real credential.
//
// It is embedded verbatim rather than synthesised because the previous fixture
// invented a per-interface "up" field: netifd reports liveness per RADIO
// (up/pending/retry_setup_failed) and a per-interface entry carries exactly
// {section, config, ifname, vlans, stations}. A fixture that adds a field the
// device does not emit makes a probe that can never pass on hardware look green.
// See docs/tests/on-device-verification.md.
const goldenWirelessStatus = `	{
	 "radio0": {
	  "up": true,
	  "pending": false,
	  "autostart": true,
	  "disabled": false,
	  "retry_setup_failed": false,
	  "config": {
	   "disabled": false,
	   "type": "mac80211",
	   "band": "5g",
	   "cell_density": 0,
	   "channel": "36",
	   "country": "US",
	   "htmode": "VHT80",
	   "path": "platform/soc@0/c000000.wifi"
	  },
	  "interfaces": [
	   {
	    "section": "sta0",
	    "config": {
	     "network": [
	      "wwan"
	     ],
	     "device": [
	      "radio0"
	     ],
	     "mode": "sta",
	     "disabled": false,
	     "encryption": "psk2",
	     "hidden": false,
	     "key": "<redacted>",
	     "macaddr": "9e:6c:d4:9d:05:d0",
	     "ssid": "Cappuxinno",
	     "radios": []
	    },
	    "ifname": "phy0-sta0",
	    "vlans": [],
	    "stations": []
	   }
	  ]
	 },
	 "radio1": {
	  "up": true,
	  "pending": false,
	  "autostart": true,
	  "disabled": false,
	  "retry_setup_failed": false,
	  "config": {
	   "disabled": false,
	   "type": "mac80211",
	   "band": "2g",
	   "cell_density": 0,
	   "channel": "auto",
	   "country": "US",
	   "htmode": "HT20",
	   "path": "platform/soc@0/c000000.wifi+1"
	  },
	  "interfaces": [
	   {
	    "section": "default_radio1",
	    "config": {
	     "network": [
	      "lan"
	     ],
	     "device": [
	      "radio1"
	     ],
	     "mode": "ap",
	     "disabled": false,
	     "encryption": "psk2",
	     "key": "<redacted>",
	     "ssid": "TravelTestNet",
	     "radios": []
	    },
	    "ifname": "phy1-ap0",
	    "vlans": [],
	    "stations": []
	   }
	  ]
	 }
	}`

// decodeWirelessStatus parses a captured `ubus call ... status` payload the way
// the ubus client does, so a fixture cannot drift from the real wire shape by
// being hand-built as Go maps.
func decodeWirelessStatus(t *testing.T, payload string) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("decoding the captured status payload: %v", err)
	}
	return resp
}

// putInRepeaterLikeDeviceConfig makes the mock config match the captured
// payload: the 5 GHz uplink STA plus the 2 GHz AP, and no other enabled AP.
func putInRepeaterLikeDeviceConfig(t *testing.T, u *uci.MockUCI) {
	t.Helper()
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
}

// The regression this whole section exists for: with the real payload, an AP
// and an uplink that netifd reports as up MUST confirm, or every wireless
// change on this firmware rolls back after 30 s forever.
func TestConfirmApply_RealDevicePayloadConfirmsUpradioAndSTA(t *testing.T) {
	svc, u := newTestWifiService()
	putInRepeaterLikeDeviceConfig(t, u)
	registerPayload(t, svc, decodeWirelessStatus(t, goldenWirelessStatus))
	registerDeviceStatus(t, svc, map[string]bool{"phy0-sta0": true, "phy1-ap0": true})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-real"); err != nil {
		t.Fatalf("an AP and STA netifd reports as up must confirm, got %v", err)
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

// Fail-closed, the direction a naive fix breaks: a section netifd does not
// LIST proves nothing, even when every radio says it is up.
func TestConfirmApply_UnlistedSectionIsNotUp(t *testing.T) {
	svc, u := newTestWifiService()
	putInRepeaterLikeDeviceConfig(t, u)
	// Drop the 2 GHz AP from the payload; the radio it belongs to stays up.
	resp := decodeWirelessStatus(t, goldenWirelessStatus)
	radio1 := resp["radio1"].(map[string]any)
	radio1["interfaces"] = []any{}
	registerPayload(t, svc, resp)
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	err := svc.ConfirmApply("session-unlisted")
	if !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("a section netifd does not list must stay unproven, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("an unlisted section must not cancel the rollback, got confirm %#v", fake.confirmed)
	}
}

// A radio that is up but still pending (or that netifd gave up on) is not up:
// reporting "up" while the interface is still being set up is exactly the state
// the rollback window exists for.
func TestConfirmApply_PendingOrRetryFailedRadioIsNotUp(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
	}{
		{"pending", "pending"},
		{"retry_setup_failed", "retry_setup_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, u := newTestWifiService()
			putInRepeaterLikeDeviceConfig(t, u)
			resp := decodeWirelessStatus(t, goldenWirelessStatus)
			resp["radio1"].(map[string]any)[tc.key] = true
			registerPayload(t, svc, resp)
			quietConfirmRetries(t)
			// The device status says the interface carries traffic, so only the
			// radio-level signal can keep it from counting as up.
			fake := &fakeWirelessApplier{}
			svc.applier = fake

			err := svc.ConfirmApply("session-" + tc.name)
			if !errors.Is(err, ErrWirelessNotUp) {
				t.Fatalf("a radio with %s must not count as up, got %v", tc.key, err)
			}
			if len(fake.confirmed) != 0 {
				t.Fatalf("an unsettled radio must not cancel the rollback, got confirm %#v",
					fake.confirmed)
			}
		})
	}
}

// The radio-level signal is only one of two. When the radio is not settled,
// `network.device status {"name":"<ifname>"}` is asked whether the interface
// itself exists, is up and has carrier.
func TestConfirmApply_DeviceStatusProvesAnInterfaceOnAnUnsettledRadio(t *testing.T) {
	svc, u := newTestWifiService()
	putInRepeaterLikeDeviceConfig(t, u)
	resp := decodeWirelessStatus(t, goldenWirelessStatus)
	resp["radio1"].(map[string]any)["up"] = false
	registerPayload(t, svc, resp)
	registerDeviceStatus(t, svc, map[string]bool{"phy1-ap0": true})
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-device"); err != nil {
		t.Fatalf("an interface the device reports present, up and with carrier must confirm, got %v", err)
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

// Without the device answer, the same unsettled radio must fail closed.
func TestConfirmApply_DeviceStatusDownDoesNotProveAnInterface(t *testing.T) {
	svc, u := newTestWifiService()
	putInRepeaterLikeDeviceConfig(t, u)
	resp := decodeWirelessStatus(t, goldenWirelessStatus)
	resp["radio1"].(map[string]any)["up"] = false
	registerPayload(t, svc, resp)
	registerDeviceStatus(t, svc, map[string]bool{"phy1-ap0": false})
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-device-down"); !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("a device-reported-down interface must not count as up, got %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("a down interface must not cancel the rollback, got confirm %#v", fake.confirmed)
	}
}

// Schema drift must not masquerade as operator error. When netifd stops

// reporting the keys the probe reads, the answer is "I cannot read this",
// which is diagnosable, and not "the access point did not come up", which
// sends the operator looking at their own config.
func TestConfirmApply_UnknownPayloadShapeIsDistinguishableFromAPNotUp(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{"radio without liveness", `{"radio0":{"interfaces":[]}}`},
		{"interface without ifname", `{"radio0":{"up":true,"pending":false,
			"retry_setup_failed":false,"interfaces":[{"section":"default_radio0"}]}}`},
		{"radio is not an object", `{"radio0":"up"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestWifiService()
			registerPayload(t, svc, decodeWirelessStatus(t, tc.payload))
			fake := &fakeWirelessApplier{}
			svc.applier = fake

			err := svc.ConfirmApply("session-shape")
			if !errors.Is(err, ErrWirelessUnverifiable) {
				t.Fatalf("expected ErrWirelessUnverifiable for %s, got %v", tc.name, err)
			}
			if errors.Is(err, ErrWirelessNotUp) {
				t.Fatalf("schema drift must not read as ErrWirelessNotUp, got %v", err)
			}
			if len(fake.confirmed) != 0 {
				t.Fatalf("an unreadable payload must not cancel the rollback, got confirm %#v",
					fake.confirmed)
			}
		})
	}
}

// flakyWirelessStatusUbus answers `network.wireless status` with a TRANSPORT
// error for its first `failures` calls and with a payload afterwards. It exists
// because MockUbus cannot return an error from Call, and "the bus never
// answered" is a condition the service must tell apart from "the bus answered
// with bad news": the socket is gone, netifd is restarting the object, or the
// call timed out. Unrelated to this file and deleted with it.
type flakyWirelessStatusUbus struct {
	payload  map[string]any
	failures int
	calls    int
}

func (f *flakyWirelessStatusUbus) Call(
	path, method string, _ map[string]any,
) (map[string]any, error) {
	if path == "network.device" && method == "status" {
		// The uplink STA is proven by carrier; see
		// TestConfirmApply_UplinkSTANeverAssociatedIsNotUp for the case this
		// deliberately does NOT model.
		return map[string]any{"present": true, "up": true, "carrier": true}, nil
	}
	if path != "network.wireless" || method != "status" {
		return nil, fmt.Errorf("unexpected ubus call %s %s", path, method)
	}
	f.calls++
	if f.calls <= f.failures {
		return nil, errors.New("ubus connection reset by peer")
	}
	return f.payload, nil
}

// A transport failure is the SAME condition as an unreadable payload: Travo
// cannot read the answer, so it knows nothing about the change. Reporting it as
// ErrWirelessNotUp claims the router rolled back while rpcd's rollback window is
// still open and the config is still live — and the frontend turns that message
// into "so it rolled back to the previous settings".
func TestConfirmApply_TransportFailureIsNotReportedAsARollback(t *testing.T) {
	svc, _ := newTestWifiService()
	ub := &flakyWirelessStatusUbus{failures: wirelessConfirmAttempts}
	svc.ubus = ub
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	err := svc.ConfirmApply("session-transport")
	if !errors.Is(err, ErrWirelessUnverifiable) {
		t.Fatalf("a transport failure must be ErrWirelessUnverifiable, got %v", err)
	}
	if errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("a transport failure must not read as a rollback, got %v", err)
	}
	if strings.Contains(err.Error(), "wireless apply not verified") {
		t.Fatalf("the operator-facing message claims a rollback that has not happened: %v", err)
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("an unreadable answer must not cancel the rollback, got confirm %#v", fake.confirmed)
	}
}

// A transport failure is transient by nature — netifd restarting the object is
// the normal case — so it is retried inside the budget instead of ending the
// confirmation on the first unreadable poll.
func TestConfirmApply_TransportFailureIsRetriedAndCanStillConfirm(t *testing.T) {
	svc, u := newTestWifiService()
	putInRepeaterLikeDeviceConfig(t, u)
	ub := &flakyWirelessStatusUbus{
		payload:  decodeWirelessStatus(t, goldenWirelessStatus),
		failures: 1,
	}
	svc.ubus = ub
	quietConfirmRetries(t)
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-transient"); err != nil {
		t.Fatalf("a status read that recovers on retry must confirm, got %v", err)
	}
	if ub.calls < 2 {
		t.Fatalf("expected the transport failure to be retried, got %d status reads", ub.calls)
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

// The probe safety margin is one number with two homes: the Go gate reserves it
// in seconds, the client reserves it in milliseconds, and the documentation
// asserts they are the same. Nothing derived one from the other, so they
// drifted (1 s here, 500 ms there, "half a second" in the comment) without a
// single test failing. This one reads the client's exported constant and fails
// when the two sides disagree.
func TestClientProbeSafetyMarginMatchesTheGoGate(t *testing.T) {
	clientPath := filepath.Join(
		"..", "..", "..", "frontend", "src", "lib", "wifi-apply.ts")
	body, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("reading %s: %v", clientPath, err)
	}
	match := regexp.MustCompile(`PROBE_SAFETY_MARGIN_MS\s*=\s*(\d+)`).
		FindSubmatch(body)
	if match == nil {
		t.Fatalf("no PROBE_SAFETY_MARGIN_MS constant found in %s", clientPath)
	}
	clientMillis, err := strconv.Atoi(string(match[1]))
	if err != nil {
		t.Fatalf("parsing the client margin %q: %v", match[1], err)
	}
	if clientMillis != wirelessProbeSafetyMarginSeconds*1000 {
		t.Fatalf("the client reserves %d ms of margin while the Go gate reserves %ds; "+
			"docs/adr/0002 says they are the same number", clientMillis,
			wirelessProbeSafetyMarginSeconds)
	}
}

// newRollbackTestService builds a WifiService whose applier is the REAL
// rpcd applier wired to temp config dirs, so the file-level snapshot/restore is
// exercised end to end. Its wireless config enables one access point, which the
// unreachable netifd makes unverifiable — that is the refused-confirm case.
func newRollbackTestService(t *testing.T) (*WifiService, *RealUCIApplyConfirm, string) {
	t.Helper()
	prev := wirelessRetrySleep
	wirelessRetrySleep = func(time.Duration) {}
	t.Cleanup(func() { wirelessRetrySleep = prev })

	etcDir := t.TempDir()
	cfg := filepath.Join(etcDir, "wireless")
	if err := os.WriteFile(cfg, []byte("config wifi-iface 'ap0'\n\tmode 'ap'\n"), 0600); err != nil {
		t.Fatalf("write wireless config: %v", err)
	}
	u := uci.NewMockUCI()
	if err := u.AddSection("wireless", "radio0", "wifi-device"); err != nil {
		t.Fatalf("seed radio: %v", err)
	}
	if err := u.AddSection("wireless", "ap0", "wifi-iface"); err != nil {
		t.Fatalf("seed AP: %v", err)
	}
	for k, v := range map[string]string{"device": "radio0", "mode": "ap"} {
		if err := u.Set("wireless", "ap0", k, v); err != nil {
			t.Fatalf("seed AP %s: %v", k, err)
		}
	}
	fu := &fakeUbusApply{loginSID: "sess-refused"}
	applier := NewRealUCIApplyConfirm(fu, auth.NewRootPassword())
	applier.etcConfigDir = etcDir
	applier.rpcdRunDir = filepath.Join(t.TempDir(), "rpcd")
	svc := NewWifiServiceWithApplier(u, fu, applier)
	svc.guardDir = testGuardDir()
	return svc, applier, cfg
}

// A refused confirm must be followed by the action it implies. rpcd's window
// snapshotted the already-committed config, so leaving the restore to it left
// the operator with the broken profile (confirmed on a GL-AXT1800, OpenWrt
// 25.12.3).
func TestConfirmApply_RefusedProbeRestoresThePreviousConfig(t *testing.T) {
	svc, applier, cfg := newRollbackTestService(t)
	before, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	res, err := svc.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		// The mutation commits: that is what put the previous config out of
		// reach of `uci revert` and of rpcd's rollback window.
		broken := "config wifi-iface 'broken'\n\tmode 'sta'\n"
		if err := os.WriteFile(cfg, []byte(broken), 0600); err != nil {
			return nil, err
		}
		sid, err := applier.StartApply([]string{"wireless"})
		if err != nil {
			return nil, err
		}
		return &WirelessApplyResult{Token: sid}, nil
	})
	if err != nil {
		t.Fatalf("mutateWireless: %v", err)
	}

	if err := svc.ConfirmApply(res.Token); err == nil {
		t.Fatal("expected the probe to refuse: netifd reports no wireless interfaces")
	}
	after, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config after refused confirm: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("wireless config after a refused confirm =\n%q\nwant the pre-mutation config\n%q",
			after, before)
	}
}

// Gap 2: a mutation that commits and then fails has no session to roll back, so
// the pending snapshot is the only thing standing between the operator and a
// config the API already reported as failed.
func TestMutateWireless_FailedMutationRestoresTheCommittedConfig(t *testing.T) {
	svc, _, cfg := newRollbackTestService(t)
	before, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	_, err = svc.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		if writeErr := os.WriteFile(cfg, []byte("half written\n"), 0600); writeErr != nil {
			return nil, writeErr
		}
		return nil, errors.New("mutation failed after committing")
	})
	if err == nil {
		t.Fatal("expected the mutation's error")
	}
	after, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config after failed mutation: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("wireless config after a failed mutation = %q, want the pre-mutation %q",
			after, before)
	}
}

// A failed mutation must not hide behind a failed restore: the caller gets one
// error that says the config may still be the one it rejected.
func TestMutateWireless_FailedMutationReportsAFailedRestore(t *testing.T) {
	cause := errors.New("mutation failed after committing")
	applier := &fakeWirelessApplier{rollbackErr: errors.New("snapshot unreadable")}
	svc := NewWifiServiceWithApplier(uci.NewMockUCI(), ubus.NewMockUbus(), applier)

	_, err := svc.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		return nil, cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want the mutation's error to stay unwrappable", err)
	}
	if !strings.Contains(err.Error(), "restoring the previous config also failed") {
		t.Errorf("error = %q, want it to say the restore failed too", err)
	}
	if len(applier.rolledBack) != 1 || applier.rolledBack[0] != "" {
		t.Errorf("expected a rollback of the pending snapshot, got %v", applier.rolledBack)
	}
}

// The applier inside the mutation may already have consumed the pending snapshot
// (ApplyAndConfirm does exactly that). That is not a failed restore.
func TestMutateWireless_FailedMutationToleratesAnAlreadyRestoredSnapshot(t *testing.T) {
	cause := errors.New("mutation failed after committing")
	applier := &fakeWirelessApplier{rollbackErr: ErrNoSnapshot}
	svc := NewWifiServiceWithApplier(uci.NewMockUCI(), ubus.NewMockUbus(), applier)

	_, err := svc.mutateWireless([]string{"wireless"}, func() (*WirelessApplyResult, error) {
		return nil, cause
	})
	if !errors.Is(err, cause) {
		t.Errorf("error = %v, want the mutation's error unchanged", err)
	}
	if err != cause { //nolint:errorlint // identity is the assertion
		t.Errorf("error = %v, want the mutation's error verbatim", err)
	}
}
