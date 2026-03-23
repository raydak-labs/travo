package services

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func newTestWifiService() (*WifiService, *uci.MockUCI) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := NewWifiServiceWithReloader(u, ub, &NoopWifiReloader{})
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
	val, _ := u.Get("wireless", "wifinet2", "ssid")
	if val != "Test-Network" {
		t.Errorf("expected ssid 'Test-Network', got %q", val)
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

	// Disconnect
	if _, err := svc.Disconnect(); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
	val, _ := u.Get("wireless", "wifinet2", "disabled")
	if val != "1" {
		t.Errorf("expected disabled='1' after disconnect, got %q", val)
	}

	// Reconnect
	_, err := svc.Connect(models.WifiConfig{
		SSID: "New-Network", Password: "newpass123", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	val, _ = u.Get("wireless", "wifinet2", "disabled")
	if val != "0" {
		t.Errorf("expected disabled='0' after connect, got %q", val)
	}
	val, _ = u.Get("wireless", "wifinet2", "ssid")
	if val != "New-Network" {
		t.Errorf("expected ssid 'New-Network', got %q", val)
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

func TestWifiDisconnectFallsBackToUCI(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	// Register wireless status with no STA interfaces (simulating disabled STA)
	ub.RegisterResponse("network.wireless.status", map[string]interface{}{
		"radio0": map[string]interface{}{
			"interfaces": []interface{}{},
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
	ub.RegisterResponse("network.wireless.status", map[string]interface{}{
		"radio0": map[string]interface{}{
			"interfaces": []interface{}{},
		},
	})
	svc := NewWifiServiceWithReloader(u, ub, &NoopWifiReloader{})

	_, err := svc.Connect(models.WifiConfig{
		SSID: "New-Network", Password: "newpass123", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("wireless", "sta0", "ssid")
	if val != "New-Network" {
		t.Errorf("expected ssid 'New-Network', got %q", val)
	}
	val, _ = u.Get("wireless", "sta0", "disabled")
	if val != "0" {
		t.Errorf("expected disabled='0', got %q", val)
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
	val, _ := u.Get("wireless", "wifinet2", "ssid")
	if val != "Hidden-Net" {
		t.Errorf("expected ssid 'Hidden-Net', got %q", val)
	}
	val, _ = u.Get("wireless", "wifinet2", "hidden")
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
	val, _ := u.Get("wireless", "wifinet2", "hidden")
	if val != "0" {
		t.Errorf("expected hidden='0', got %q", val)
	}
}

func TestWifiConnect_NormalizesMissingNetworkToWwan(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "wifinet2", "network", "")

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Visible-Net", Password: "secretpass", Encryption: "psk2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, _ := u.Get("wireless", "wifinet2", "network")
	if val != "wwan" {
		t.Errorf("expected network='wwan', got %q", val)
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

	_, err := svc.SetAPConfig("default_radio0", models.APConfig{
		SSID:       "MyTravelRouter",
		Encryption: "psk2",
		Key:        "newpassword123",
		Enabled:    true,
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
		if c.Section == "default_radio0" {
			found = &c
			break
		}
	}
	if found == nil {
		t.Fatal("expected to find default_radio0 config")
	}
	if found.SSID != "MyTravelRouter" {
		t.Errorf("expected SSID 'MyTravelRouter', got '%s'", found.SSID)
	}
	if !found.Enabled {
		t.Error("expected AP to be enabled")
	}
}

func TestSetAPConfig_InvalidSection(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.SetAPConfig("nonexistent", models.APConfig{
		SSID:       "Test",
		Encryption: "none",
		Enabled:    true,
	})
	if err == nil {
		t.Error("expected error for nonexistent section")
	}
}

func TestSetAPConfig_ReturnsApplyError(t *testing.T) {
	svc, _ := newTestWifiService()
	svc.applier = &fakeWirelessApplier{startErr: errors.New("apply failed")}

	_, err := svc.SetAPConfig("default_radio0", models.APConfig{
		SSID:       "MyTravelRouter",
		Encryption: "psk2",
		Key:        "newpassword123",
		Enabled:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "apply failed") {
		t.Fatalf("expected apply error, got %v", err)
	}
}

func TestSetMACAddress(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify MAC was set
	opts, err := u.GetAll("wireless", "sta0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts["macaddr"] != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected macaddr 'AA:BB:CC:DD:EE:FF', got '%s'", opts["macaddr"])
	}
}

func TestSetMACAddress_Reset(t *testing.T) {
	svc, u := newTestWifiService()

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
	svc, u := newTestWifiService()

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
	svc, _ := newTestWifiService()

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

func TestWifiSetMode_RepeaterEnablesSTAAndAPs(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "disabled", "1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1")
	_ = u.Set("wireless", "sta0", "disabled", "1")

	_, err := svc.SetMode("repeater")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ap0, _ := u.Get("wireless", "default_radio0", "disabled")
	ap1, _ := u.Get("wireless", "default_radio1", "disabled")
	sta, _ := u.Get("wireless", "sta0", "disabled")
	if ap0 != "0" || ap1 != "0" || sta != "0" {
		t.Fatalf("expected APs and STA enabled, got ap0=%q ap1=%q sta=%q", ap0, ap1, sta)
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
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-456"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.confirmed) != 1 || fake.confirmed[0] != "session-456" {
		t.Fatalf("expected confirm to be called for session-456, got %#v", fake.confirmed)
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
