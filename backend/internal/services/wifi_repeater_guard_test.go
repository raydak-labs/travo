package services

import (
	"errors"
	"os"
	"testing"
)

// Repeater downlink policy guard. SetMode("repeater") enables every AP section,
// so on multi-radio hardware the split policy is the only thing standing between
// the uplink STA's PHY and an access point on it — the state ADR 0002 §2 calls
// enough to crash ath11k/IPQ6018. Connect refuses it (splitAPOffUplinkRadio);
// SetMode used to perform it silently.

// TestRepeater_RefusesWhenNoEnabledAPIsOnAnotherRadio is the inversion case:
// every enabled AP is on the uplink radio, so there is nowhere to put the
// downlink and enabling it would commit the crash state. SetMode must refuse
// instead of force-enabling the AP onto the STA's PHY.
func TestRepeater_RefusesWhenNoEnabledAPIsOnAnotherRadio(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio0", "disabled", "0")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1")
	_ = u.Set("wireless", "sta0", "device", "radio0")
	_ = u.Set("wireless", "sta0", "disabled", "0")

	if _, err := svc.SetMode("repeater", LockoutRequest{}); !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("SetMode(repeater) err = %v, want ErrAPAndSTASameRadio", err)
	}
	if got, _ := u.Get("wireless", "default_radio0", "disabled"); got != "0" {
		t.Errorf("refused repeater still changed the uplink-radio AP: disabled=%q", got)
	}
}

// TestRepeater_RefusesWhenTheOnlyOtherRadioAPIsDisabled pins the reason the split
// must look at the enabled state: a disabled section on the other radio hosts no
// downlink, so it must not vouch for the STA-radio AP either.
func TestRepeater_RefusesWhenTheOnlyOtherRadioAPIsDisabled(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio0", "disabled", "1")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1")
	_ = u.Set("wireless", "sta0", "device", "radio0")
	_ = u.Set("wireless", "sta0", "disabled", "0")

	if _, err := svc.SetMode("repeater", LockoutRequest{}); !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("SetMode(repeater) err = %v, want ErrAPAndSTASameRadio", err)
	}
	if got, _ := u.Get("wireless", "default_radio1", "disabled"); got != "1" {
		t.Errorf("refused repeater still touched an AP section: disabled=%q", got)
	}
}

// TestRepeater_KeepsSplitWhenAnotherRadioHasAnEnabledAP is the happy path that
// must not regress: a working downlink exists elsewhere, so the split happens.
func TestRepeater_KeepsSplitWhenAnotherRadioHasAnEnabledAP(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio0", "disabled", "0")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "default_radio1", "disabled", "0")
	_ = u.Set("wireless", "sta0", "device", "radio0")
	_ = u.Set("wireless", "sta0", "disabled", "0")

	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
		t.Fatalf("SetMode(repeater): %v", err)
	}
	if got, _ := u.Get("wireless", "default_radio0", "disabled"); got != "1" {
		t.Errorf("AP on the uplink radio should be disabled, got disabled=%q", got)
	}
	if got, _ := u.Get("wireless", "default_radio1", "disabled"); got != "0" {
		t.Errorf("downlink AP on the free radio should be enabled, got disabled=%q", got)
	}
	if got, _ := u.Get("wireless", "sta0", "disabled"); got != "0" {
		t.Errorf("uplink STA should be enabled, got disabled=%q", got)
	}
}

// TestRepeater_AllowAPOnSTARadioIsTheDocumentedEscape keeps the refusal from
// becoming a dead end: the explicit option still enables the downlink on the
// uplink radio.
func TestRepeater_AllowAPOnSTARadioIsTheDocumentedEscape(t *testing.T) {
	svc, u := newTestWifiService()
	svc.repeaterOptionsFile = t.TempDir() + "/repeater-options.json"
	const opts = `{"allow_ap_on_sta_radio":true}`
	if err := os.WriteFile(svc.repeaterOptionsFile, []byte(opts), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1")
	_ = u.Set("wireless", "sta0", "device", "radio0")

	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
		t.Fatalf("SetMode(repeater) with allow_ap_on_sta_radio: %v", err)
	}
	if got, _ := u.Get("wireless", "default_radio0", "disabled"); got != "0" {
		t.Errorf("with allow_ap_on_sta_radio the AP should be enabled, got disabled=%q", got)
	}
}

// TestRepeater_SingleRadioStillCoexists keeps the unavoidable case working: with
// one radio there is no split to make, so the request is served.
func TestRepeater_SingleRadioStillCoexists(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.DeleteSection("wireless", "default_radio1")
	_ = u.DeleteSection("wireless", "radio1")
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "sta0", "device", "radio0")

	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
		t.Fatalf("SetMode(repeater) on single-radio hardware: %v", err)
	}
	if got, _ := u.Get("wireless", "default_radio0", "disabled"); got != "0" {
		t.Errorf("single-radio AP should stay enabled, got disabled=%q", got)
	}
}

// TestRepeater_ApModeIsUnaffected: refusing applies to the repeater downlink
// only. A plain AP switch has no uplink, so it must never be refused.
func TestRepeater_ApModeIsUnaffected(t *testing.T) {
	svc, u := newTestWifiService()
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1")

	if _, err := svc.SetMode("ap", LockoutRequest{}); err != nil {
		t.Fatalf("SetMode(ap): %v", err)
	}
	if got, _ := u.Get("wireless", "default_radio1", "disabled"); got != "0" {
		t.Errorf("SetMode(ap) should enable the AP, got disabled=%q", got)
	}
}
