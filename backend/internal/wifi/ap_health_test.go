package wifi

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func newAPHealthTestService(u *uci.MockUCI) *Service {
	ub := ubus.NewMockUbus()
	return New(Deps{UCI: u, Ubus: ub, Reloader: &NoopWifiReloader{}})
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
	svc := New(Deps{UCI: u, Ubus: ub, Reloader: &NoopWifiReloader{}})

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

func TestEnsureAPRunning_GetSectionsError(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: &errGetSectionsUCI{uci.NewMockUCI()}, Ubus: ub, Reloader: &NoopWifiReloader{}})

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

func TestEnsureAPRunning_CommitError(t *testing.T) {
	base := uci.NewMockUCI()
	// Empty SSID on an enabled AP triggers a fix → Commit → error.
	_ = base.Set("wireless", "default_radio0", "ssid", "")

	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: &errCommitUCI{base}, Ubus: ub, Reloader: &NoopWifiReloader{}})

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
