package wifi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

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
