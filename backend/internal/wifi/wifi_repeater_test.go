package wifi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

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

// Regression: SetMode("repeater") must never enable more than one STA section on network=wwan.
// When the user toggles mode with multiple saved STA profiles, the wrong STA may win the wwan
// binding in netifd, leaving the actually-connected STA without DHCP — "connected, no IP".
func TestSetModeRepeater_WithMultipleSavedSTAs_EnablesOnlyHighestPriority(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpFile := t.TempDir() + "/priorities.json"
	svc := New(Deps{UCI: u, Ubus: ub, Reloader: &NoopWifiReloader{}, PriorityFile: tmpFile})

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
	svc := New(Deps{UCI: u, Ubus: ub, Reloader: &NoopWifiReloader{}, PriorityFile: tmpFile})

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
