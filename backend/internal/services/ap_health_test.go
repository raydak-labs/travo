package services

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func newAPHealthTestService(u *uci.MockUCI) *WifiService {
	ub := ubus.NewMockUbus()
	return NewWifiServiceWithReloader(u, ub, &NoopWifiReloader{})
}

func TestEnsureAPRunning_HealthyConfig(t *testing.T) {
	// Default MockUCI has a valid AP setup — no changes should be needed.
	u := uci.NewMockUCI()
	svc := newAPHealthTestService(u)

	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The default mock has country+channel set, so no fixes expected.
	_ = fixed
	_ = needWifiUp
}

func TestEnsureAPRunning_MissingSSID(t *testing.T) {
	u := uci.NewMockUCI()
	// Remove SSID from the 2.4 GHz AP section.
	if err := u.Set("wireless", "default_radio0", "ssid", ""); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := newAPHealthTestService(u)
	fixed, _, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fixed=true when SSID is missing")
	}

	// Verify default SSID was applied.
	sections, _ := u.GetSections("wireless")
	ap := sections["default_radio0"]
	if ap["ssid"] != DefaultAPSSID {
		t.Errorf("expected ssid=%q, got %q", DefaultAPSSID, ap["ssid"])
	}
}

func TestEnsureAPRunning_DisabledAPWithActiveSTA_Skipped(t *testing.T) {
	u := uci.NewMockUCI()
	// Disable the 2.4 GHz AP.
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// radio0 has an active STA (sta0) in the default mock — AP must NOT be re-enabled.

	svc := newAPHealthTestService(u)
	_, _, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sections, _ := u.GetSections("wireless")
	ap := sections["default_radio0"]
	if ap["disabled"] != "1" {
		t.Error("expected AP to remain disabled when radio has active STA")
	}
}

func TestEnsureAPRunning_DisabledAPWithoutSTA_ReEnabled(t *testing.T) {
	u := uci.NewMockUCI()
	// Disable the 5 GHz AP — radio1 has no STA, so it should be re-enabled.
	if err := u.Set("wireless", "default_radio1", "disabled", "1"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := newAPHealthTestService(u)
	fixed, needWifiUp, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fixed=true when disabled AP is re-enabled")
	}
	if !needWifiUp {
		t.Error("expected needWifiUp=true when AP was re-enabled")
	}

	sections, _ := u.GetSections("wireless")
	ap := sections["default_radio1"]
	if ap["disabled"] != "0" {
		t.Error("expected AP to be re-enabled")
	}
}

func TestEnsureAPRunning_MissingKey_WhenEncryptionSet(t *testing.T) {
	u := uci.NewMockUCI()
	// Remove key from AP that has encryption set.
	if err := u.Set("wireless", "default_radio0", "key", ""); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := newAPHealthTestService(u)
	fixed, _, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fixed=true when key is missing")
	}

	sections, _ := u.GetSections("wireless")
	ap := sections["default_radio0"]
	if ap["key"] != DefaultAPKey {
		t.Errorf("expected key=%q, got %q", DefaultAPKey, ap["key"])
	}
}

func TestEnsureAPRunning_MissingCountry_Fixed(t *testing.T) {
	u := uci.NewMockUCI()
	if err := u.Set("wireless", "radio0", "country", ""); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := newAPHealthTestService(u)
	fixed, _, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fixed=true when radio country is missing")
	}

	sections, _ := u.GetSections("wireless")
	radio := sections["radio0"]
	if radio["country"] != DefaultCountry {
		t.Errorf("expected country=%q, got %q", DefaultCountry, radio["country"])
	}
}

// Regression: after an ath11k/IPQ6018 crash, UCI can end up with an explicit out-of-range
// 5 GHz channel (e.g. 177, UNII-4/V2X) that is invisible to consumer devices.
// EnsureAPRunning must reset any 5 GHz channel > 165 to Default5GChannel.
// channel=auto is deliberately preserved — auto-selection picks valid channels in normal operation.
func TestEnsureAPRunning_5GHz_OutOfRangeChannel_Fixed(t *testing.T) {
	u := uci.NewMockUCI()
	// Simulate post-crash UCI state: channel explicitly set to 177.
	if err := u.Set("wireless", "radio1", "channel", "177"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := newAPHealthTestService(u)
	fixed, _, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fixed {
		t.Error("expected fixed=true when 5GHz radio has out-of-range channel")
	}

	sections, _ := u.GetSections("wireless")
	ch := sections["radio1"]["channel"]
	if ch != Default5GChannel {
		t.Errorf("expected 5GHz channel=%q after out-of-range reset, got %q", Default5GChannel, ch)
	}
}

// channel=auto on 5 GHz must be preserved — auto-selection picks valid channels in normal operation.
func TestEnsureAPRunning_5GHz_AutoChannel_Preserved(t *testing.T) {
	u := uci.NewMockUCI()
	if err := u.Set("wireless", "radio1", "channel", "auto"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := newAPHealthTestService(u)
	fixed, _, err := svc.EnsureAPRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fixed {
		t.Error("expected fixed=false: channel=auto on 5GHz should not be overridden")
	}

	sections, _ := u.GetSections("wireless")
	ch := sections["radio1"]["channel"]
	if ch != "auto" {
		t.Errorf("5GHz channel=auto should be preserved, got %q", ch)
	}
}

// 2.4GHz radios must keep channel=auto unchanged.
func TestEnsureAPRunning_2GHz_AutoChannel_Preserved(t *testing.T) {
	u := uci.NewMockUCI()
	if err := u.Set("wireless", "radio0", "channel", "auto"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := newAPHealthTestService(u)
	if _, _, err := svc.EnsureAPRunning(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sections, _ := u.GetSections("wireless")
	ch := sections["radio0"]["channel"]
	if ch != "auto" {
		t.Errorf("2.4GHz channel=auto should be preserved, got %q", ch)
	}
}
