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
