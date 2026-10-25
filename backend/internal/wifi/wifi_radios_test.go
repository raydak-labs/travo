package wifi

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

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

func TestSetRadioRole_ValidatesBeforeCommit(t *testing.T) {
	u := &revertingUCI{MockUCI: uci.NewMockUCI()}
	svc := New(Deps{UCI: &dirtySTAUCI{u.MockUCI}, Ubus: ubus.NewMockUbus(), Reloader: &NoopWifiReloader{}})

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

// Finding 1: a failed `uci show` must never look like an empty config in the
// wireless read paths either.
func TestGetRadios_SurfacesGetSectionsError(t *testing.T) {
	svc := New(Deps{UCI: &uci.FailingSectionsUCI{MockUCI: uci.NewMockUCI()}, Ubus: ubus.NewMockUbus(), Reloader: &NoopWifiReloader{}})

	radios, err := svc.GetRadios()
	if err == nil {
		t.Fatal("expected an error instead of an empty radio list when the UCI read fails")
	}
	if radios != nil {
		t.Errorf("expected no list on error, got %v", radios)
	}
}
