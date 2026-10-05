package services

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// The three mutators left out of the lockout guard when it was written —
// POST /wifi/connect, POST /wifi/disconnect and POST /wifi/repeater/reconcile —
// are pinned HERE as deliberately unguarded, because "it is safe" is only worth
// something if a test says so.
//
// What these tests assert is the property the guard is built on (ADR 0002 §5.2:
// refuse when the caller is on WiFi and no enabled mode=ap wifi-iface would be
// left on an enabled radio), read back out of the configuration the caller would
// actually be left with. They deliberately do NOT assert that a guard was
// called, that a LockoutRequest exists, or that some error type is absent: none
// of those is the property, and a test that could only pass against an
// acknowledge-aware signature would be testing the fiction that these endpoints
// are lockout vectors at all.
//
// The reader is independent of the guard's own enabledAPRemains on purpose: a
// test that reuses the implementation's definition of "an access point
// survives" proves only that the implementation agrees with itself.
func enabledAPsLeft(t *testing.T, u *uci.MockUCI) []string {
	t.Helper()
	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("reading wireless: %v", err)
	}
	var left []string
	for name, opts := range sections {
		if opts["mode"] != "ap" || opts["disabled"] == "1" {
			continue
		}
		radio, err := u.GetAll("wireless", opts["device"])
		if err != nil || radio["disabled"] == "1" {
			continue
		}
		left = append(left, name)
	}
	sort.Strings(left)
	return left
}

func hasAP(left []string, section string) bool {
	for _, name := range left {
		if name == section {
			return true
		}
	}
	return false
}

// newLockoutServiceWithRuntime is newLockoutService plus a `network.wireless
// status` answer that names the UCI sections this configuration actually has.
//
// The ubus default fixture reports the uplink as section `wifinet2`, which
// exists in no UCI fixture here — so Disconnect against it created that section
// and "disconnected" a profile that was never there. That is the fixture-shaped-
// to-what-the-code-reads defect this branch keeps hitting: the runtime answer
// and the config answer have to describe the same device, so this one is built
// from the config rather than invented.
func newLockoutServiceWithRuntime(t *testing.T) (*WifiService, *uci.MockUCI, *fakeWirelessApplier) {
	t.Helper()
	svc, u, applier := newLockoutService(t)
	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("reading wireless: %v", err)
	}
	radio0 := map[string]any{"interfaces": []any{}}
	for name, opts := range sections {
		if opts["mode"] != "sta" {
			continue
		}
		radio0["interfaces"] = append(radio0["interfaces"].([]any), map[string]any{
			"ifname":  "phy0-" + name,
			"section": name,
			"config":  map[string]any{"mode": "sta", "ssid": opts["ssid"]},
		})
	}
	svc.ubus.(*ubus.MockUbus).RegisterResponse("network.wireless.status",
		map[string]any{"radio0": radio0})
	return svc, u, applier
}

// ---------------------------------------------------------------------------
// POST /api/v1/wifi/connect
// ---------------------------------------------------------------------------

// Connect moves the uplink STA and turns an access point OFF to give that STA a
// radio. The outcome that would strand the operator is the one where the uplink
// radio carries the router's ONLY access point — and splitAPOffUplinkRadio
// refuses that before writing, so the "would strand" state is unreachable by
// construction rather than guarded after the fact.
func TestConnect_NeverLeavesTheRouterWithoutAnAccessPoint(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	// The uplink goes to the 5 GHz radio, which carries default_radio1.
	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Band: "5g", Encryption: "psk2", Password: "cafepass1",
	}); err != nil {
		t.Fatalf("connecting the uplink to a radio that carries an access point: %v", err)
	}
	if dis, _ := u.Get("wireless", "default_radio1", "disabled"); dis != "1" {
		t.Fatalf("fixture did not exercise the split: default_radio1 disabled = %q", dis)
	}
	left := enabledAPsLeft(t, u)
	if !hasAP(left, "default_radio0") {
		t.Errorf("enabled access points = %v, want default_radio0 to survive", left)
	}
	if len(applier.started) == 0 {
		t.Error("expected the safe connect to be staged for apply")
	}
}

// The refusal that keeps the property: the only access point up is on the radio
// the uplink wants, so freeing that radio would take the router's WiFi away.
//
// Note what is NOT asserted here: that the configuration is byte-identical
// afterwards. uci.MockUCI does not model `uci revert` — Revert on the real
// client drops the staged delta, on the mock it does not exist — so a refusal
// raised INSIDE mutateWireless leaves the delta it had already written visible
// in the mock, and a byte-comparison would be asserting a property of the mock.
// What is asserted is what a caller can observe either way: the request is
// refused, the access points it would have taken away are untouched, and nothing
// was staged for apply. (That the restore depends on the applier snapshot
// rather than on the refusal itself is a real difference from the lockout guard,
// and is reported rather than papered over here.)
func TestConnect_RefusesRatherThanRemovingTheLastAccessPoint(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	before, sessions := enabledAPsLeft(t, u), len(applier.started)

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Band: "5g", Encryption: "psk2", Password: "cafepass1",
	})

	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("err = %v, want ErrAPAndSTASameRadio", err)
	}
	if after := enabledAPsLeft(t, u); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Errorf("refused connect changed the access points: before %v, after %v", before, after)
	}
	if len(applier.started) != sessions {
		t.Errorf("refused connect staged %v apply session(s), want %d",
			applier.started, sessions)
	}
}

// allow_ap_on_sta_radio is the operator's own override, and it is the one Connect
// outcome that commits an access point and a STA onto the same PHY. The access
// point stays ENABLED in the configuration, so an enabled access point is still
// left and the single rule has nothing to refuse — while the cost is a driver
// that can take that access point down with a failing STA. Recorded rather than
// hidden: this is the one Connect outcome the rule cannot express.
func TestConnect_AllowAPOnSTARadioKeepsTheAccessPointEnabled(t *testing.T) {
	svc, u, _ := newLockoutService(t)
	svc.repeaterOptionsFile = writeTempFile(t, "repeater-options",
		`{"allow_ap_on_sta_radio":true}`)
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Band: "5g", Encryption: "psk2", Password: "cafepass1",
	}); err != nil {
		t.Fatalf("connecting with allow_ap_on_sta_radio: %v", err)
	}
	left := enabledAPsLeft(t, u)
	if !hasAP(left, "default_radio1") {
		t.Errorf("enabled access points = %v, want default_radio1 (the uplink radio's) up", left)
	}
}

// ---------------------------------------------------------------------------
// POST /api/v1/wifi/disconnect
// ---------------------------------------------------------------------------

// Disconnect disables the uplink STA and writes nothing else: no wifi-iface in
// mode=ap is touched, so every access point that was up is still up. Losing the
// uplink is not a lockout — the router still has its own WiFi to be reached on.
func TestDisconnect_LeavesEveryAccessPointEnabled(t *testing.T) {
	svc, u, applier := newLockoutServiceWithRuntime(t)
	before := enabledAPsLeft(t, u)
	if len(before) != 2 {
		t.Fatalf("fixture does not have two access points up: %v", before)
	}

	if _, err := svc.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	if after := enabledAPsLeft(t, u); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Errorf("access points before %v, after %v", before, after)
	}
	if dis, _ := u.Get("wireless", "sta0", "disabled"); dis != "1" {
		t.Errorf("sta0 disabled = %q, want \"1\"", dis)
	}
	if len(applier.started) == 0 {
		t.Error("expected the disconnect to be staged for apply")
	}
}

// The repeater case, where the operator reaches the router through the access
// point the uplink feeds: dropping the uplink leaves the downlink up. Compared
// by identity, not by count — see anyAccessPointSurvives.
func TestDisconnect_InRepeaterModeLeavesTheDownlinkUp(t *testing.T) {
	svc, u, _ := newLockoutServiceWithRuntime(t)
	if got := svc.deriveWifiMode(); got != "repeater" {
		t.Fatalf("fixture is not in repeater mode, got %q", got)
	}
	before := enabledAPsLeft(t, u)

	if _, err := svc.Disconnect(); err != nil {
		t.Fatalf("Disconnect in repeater mode: %v", err)
	}

	// Which access points, not how many: a mutator that turned default_radio0
	// off and default_radio1 on leaves the same count and has taken away the
	// operator's way back in.
	if after := enabledAPsLeft(t, u); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Errorf("access points before %v, after %v", before, after)
	}
}

// ---------------------------------------------------------------------------
// POST /api/v1/wifi/repeater/reconcile
// ---------------------------------------------------------------------------

// reconcile re-derives the AP layout. The only path that turns an access point
// OFF needs an ENABLED access point on another radio first, so with the uplink
// and the only access point up both on radio0 it has nothing to move and leaves
// them alone.
func TestReconcileRepeaterAPLayout_CannotDisableTheLastAccessPoint(t *testing.T) {
	svc, u, _ := newLockoutService(t)
	if err := u.Set("wireless", "default_radio1", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	// sta0 sits on radio0, next to the only access point that is up.
	if err := u.Set("wireless", "sta0", "device", "radio0"); err != nil {
		t.Fatal(err)
	}
	if before := enabledAPsLeft(t, u); !hasAP(before, "default_radio0") {
		t.Fatalf("fixture is wrong: %v", before)
	}

	if _, err := svc.ReconcileRepeaterAPLayout(); err != nil {
		t.Fatalf("reconcile with the uplink and the only access point on one radio: %v", err)
	}

	if left := enabledAPsLeft(t, u); !hasAP(left, "default_radio0") {
		t.Errorf("enabled access points = %v, want default_radio0 to survive", left)
	}
}

// The layout reconcile does change: the uplink shares radio0, so that radio's
// access point goes and the one on radio1 takes over as the operator's way back
// in. An access point survives, which is what the rule asks for.
func TestReconcileRepeaterAPLayout_MovesTheDownlinkRatherThanRemovingIt(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	if err := u.Set("wireless", "sta0", "device", "radio0"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ReconcileRepeaterAPLayout(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	left := enabledAPsLeft(t, u)
	if !hasAP(left, "default_radio1") {
		t.Errorf("enabled access points = %v, want default_radio1 to carry the downlink", left)
	}
	if len(applier.started) == 0 {
		t.Error("expected the reconcile to be staged for apply")
	}
}
