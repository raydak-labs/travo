package services

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// Radio-choice and same-radio guard behaviour: which radio a newly created
// uplink STA lands on, whether a radio switch is allowed to put the uplink on a
// PHY that runs an access point, whether the radio list is stable across calls,
// and what counts as "the uplink radio".

// The uplink's radio is not cosmetic: it decides whether the uplink shares a
// PHY with an access point, so the same request must never land on different
// radios from one call to the next. Stock layout: an access point on both
// radios, so the choice falls back to the first radio by name.
func TestEnsureSTASectionForScan_ChoosesTheSameRadioEveryTime(t *testing.T) {
	svc, u := newTestWifiService()
	seen := map[string]int{}
	for i := 0; i < 25; i++ {
		if err := u.DeleteSection("wireless", "sta0"); err != nil {
			t.Fatalf("deleting sta0: %v", err)
		}
		if _, err := svc.ensureSTASectionForScan(); err != nil {
			t.Fatalf("ensureSTASectionForScan: %v", err)
		}
		device, err := u.Get("wireless", "sta0", "device")
		if err != nil {
			t.Fatalf("reading wireless.sta0.device: %v", err)
		}
		seen[device]++
	}
	if len(seen) != 1 {
		t.Fatalf("uplink radio choice is not deterministic: %v", seen)
	}
	if _, ok := seen["radio0"]; !ok {
		t.Errorf("expected the first radio by name (radio0), got %v", seen)
	}
}

// With radio1's access point off, the bare radio is the better home for the
// uplink: the downlink keeps radio0 and nothing has to be torn down later.
func TestEnsureSTASectionForScan_PrefersTheRadioWithoutAnEnabledAP(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "default_radio1", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	if err := u.DeleteSection("wireless", "sta0"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ensureSTASectionForScan(); err != nil {
		t.Fatalf("ensureSTASectionForScan: %v", err)
	}
	device, err := u.Get("wireless", "sta0", "device")
	if err != nil {
		t.Fatalf("reading wireless.sta0.device: %v", err)
	}
	if device != "radio1" {
		t.Errorf("expected the uplink on radio1, the radio without an enabled AP, got %q", device)
	}
}

// A stock config runs an access point on EVERY radio, so the section this
// creates lands on a PHY that already hosts one. Creating it anyway commits an
// enabled AP and an enabled uplink STA on a single radio — the state ADR 0002
// §2 says is enough to crash ath11k/IPQ6018 — so the downlink has to be moved
// off that radio first, exactly as Connect does.
func TestEnsureSTASectionForScan_KeepsTheAPOffTheUplinkRadio(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.DeleteSection("wireless", "sta0"); err != nil {
		t.Fatal(err)
	}
	// Both radios run an access point: there is no bare radio to fall back to.
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if disabled, _ := u.Get("wireless", section, "disabled"); disabled == "1" {
			t.Fatalf("precondition: %s must start enabled", section)
		}
	}

	if _, err := svc.ensureSTASectionForScan(); err != nil {
		t.Fatalf("ensureSTASectionForScan: %v", err)
	}
	assertNoRadioRunsBothAnAPAndTheUplink(t, svc)
	assertUplinkRadio(t, u, "radio0")
	assertAPEnabled(t, u, "default_radio0", false)
	assertAPEnabled(t, u, "default_radio1", true)
}

// The split is a one-way move, so the second request has to land on the same
// radio again instead of chasing the downlink: taking the AP off the uplink
// radio leaves the NEXT request with a free radio and no second AP to tear
// down, and never with a refusal.
func TestEnsureSTASectionForScan_SplitsOnceAndStaysPut(t *testing.T) {
	svc, u := newTestWifiService()
	for i := range 3 {
		if err := u.DeleteSection("wireless", "sta0"); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.ensureSTASectionForScan(); err != nil {
			t.Fatalf("request %d: ensureSTASectionForScan: %v", i, err)
		}
		assertNoRadioRunsBothAnAPAndTheUplink(t, svc)
		assertUplinkRadio(t, u, "radio0")
	}
}

// With one PHY there is no split to make, so the uplink and the access point
// coexist — the trade-off every other same-radio guard already accepts.
func TestEnsureSTASectionForScan_SingleRadioKeepsCoexistence(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.DeleteSection("wireless", "radio1"); err != nil {
		t.Fatal(err)
	}
	if err := u.DeleteSection("wireless", "default_radio1"); err != nil {
		t.Fatal(err)
	}
	if err := u.DeleteSection("wireless", "sta0"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ensureSTASectionForScan(); err != nil {
		t.Fatalf("ensureSTASectionForScan: %v", err)
	}
	assertUplinkRadio(t, u, "radio0")
	assertAPEnabled(t, u, "default_radio0", true)
}

// SwitchSTAToRadio applies immediately, so it may not leave an enabled access
// point on the PHY the uplink moves to. It does not refuse the common layout —
// an access point on both radios, which is every stock config and every repeater
// — because that refusal is the same one the whole feature is built around:
// there would be nothing left to switch to. It splits instead, the way Connect
// does, and the downlink comes back when the uplink leaves again.
func TestSwitchSTAToRadio_SplitsTheAPOffTheTargetRadio(t *testing.T) {
	svc, u := newTestWifiService()

	if err := svc.SwitchSTAToRadio("radio1"); err != nil {
		t.Fatalf("SwitchSTAToRadio onto a radio carrying an access point: %v", err)
	}
	assertUplinkRadio(t, u, "radio1")
	assertNoRadioRunsBothAnAPAndTheUplink(t, svc)
	assertAPEnabled(t, u, "default_radio1", false)
	assertAPEnabled(t, u, "default_radio0", true)
}

// The switcher moves the uplink automatically, with no operator watching. If
// the target radio runs the ONLY access point, splitting would take the
// router's WiFi away to make room for a background decision, so that layout is
// refused — with the same typed error the API maps to 409.
func TestSwitchSTAToRadio_RefusesWhenTheTargetHoldsTheOnlyAccessPoint(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}

	err := svc.SwitchSTAToRadio("radio1")
	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected ErrAPAndSTASameRadio, got %v", err)
	}
	assertUplinkRadio(t, u, "radio0")
	assertAPEnabled(t, u, "default_radio1", true)
}

// A switch onto a radio with no access point on it is the ordinary background
// band switch and has to keep working.
func TestSwitchSTAToRadio_SucceedsWhenTheTargetRadioHasNoEnabledAP(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "default_radio1", "disabled", "1"); err != nil {
		t.Fatal(err)
	}

	if err := svc.SwitchSTAToRadio("radio1"); err != nil {
		t.Fatalf("SwitchSTAToRadio onto a bare radio: %v", err)
	}
	if device, _ := u.Get("wireless", "sta0", "device"); device != "radio1" {
		t.Errorf("expected the uplink on radio1, got %q", device)
	}
}

// Radio and AP lists are ordered, so every consumer of them — the band
// switcher above all — makes the same choice on every tick.
func TestGetRadios_ReturnsRadiosInAStableOrder(t *testing.T) {
	svc, _ := newTestWifiService()
	for i := 0; i < 25; i++ {
		radios, err := svc.GetRadios()
		if err != nil {
			t.Fatalf("GetRadios: %v", err)
		}
		var names []string
		for _, r := range radios {
			names = append(names, r.Name)
		}
		if len(names) != 2 || names[0] != "radio0" || names[1] != "radio1" {
			t.Fatalf("expected [radio0 radio1], got %v", names)
		}
	}
}

func TestGetAPConfigs_ReturnsSectionsInAStableOrder(t *testing.T) {
	svc, _ := newTestWifiService()
	for i := 0; i < 25; i++ {
		configs, err := svc.GetAPConfigs()
		if err != nil {
			t.Fatalf("GetAPConfigs: %v", err)
		}
		var sections []string
		for _, c := range configs {
			sections = append(sections, c.Section)
		}
		if len(sections) != 2 ||
			sections[0] != "default_radio0" || sections[1] != "default_radio1" {
			t.Fatalf("expected [default_radio0 default_radio1], got %v", sections)
		}
	}
}

// The band switcher picks "the radio for this band" and "the other radio" off
// the radio list every tick. When both radios report the same band an unstable
// list makes it oscillate between them forever, so the choice must not move.
func TestBandSwitching_PicksTheSameRadioOnEveryTick(t *testing.T) {
	svc, u := newTestWifiService()
	// Two radios on the same band: the by-band pick is then only decidable by
	// name, which is exactly the case that used to be a coin toss.
	if err := u.Set("wireless", "radio1", "band", "2g"); err != nil {
		t.Fatal(err)
	}
	b := NewBandSwitchingService(svc, filepath.Join(t.TempDir(), "band-switch.json"))

	for i := 0; i < 25; i++ {
		radios := b.getRadios()
		if got := b.findRadioByBand(radios, "2g"); got != "radio0" {
			t.Fatalf("tick %d: expected radio0 for band 2g, got %q", i, got)
		}
		if got := b.findAlternateRadio(radios, "radio0"); got != "radio1" {
			t.Fatalf("tick %d: expected radio1 as the alternate, got %q", i, got)
		}
	}
}

// Guest WiFi owns a fixed 192.168.2.0/24 subnet. On a router whose LAN is
// already on that subnet, creating it would put two interfaces on one network,
// so the request is refused instead.
func TestSetGuestWifi_RefusesASubnetThatOverlapsLAN(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("network", "lan", "ipaddr", "192.168.2.50"); err != nil {
		t.Fatal(err)
	}

	_, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{})
	if err == nil {
		t.Fatal("expected guest WiFi to be refused on a LAN that overlaps its subnet")
	}
	if _, err := u.GetAll("network", "guest"); err == nil {
		t.Error("a refused guest network must not create network.guest")
	}
	if _, err := u.GetAll("wireless", "guest"); err == nil {
		t.Error("a refused guest network must not create wireless.guest")
	}
}

// network=wwan is what makes a STA the uplink, but a hand-written STA without
// it still runs a client interface on that PHY — so it counts as the uplink for
// the guard as well, and the health role agrees.
func TestUplinkDefinition_CountsAHandWrittenSTAWithoutWwan(t *testing.T) {
	svc, u := newTestWifiService()
	// Mock config: sta0 is enabled on radio0 and has no network=wwan.
	if net, _ := u.Get("wireless", "sta0", "network"); net == "wwan" {
		t.Fatal("precondition: this test needs an STA without network=wwan")
	}

	err := svc.rejectAPOnUplinkRadio("radio0", "access point enabled")
	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Errorf("expected the guard to treat the hand-written STA as the uplink, got %v", err)
	}
	assertRadioRole(t, svc, "radio0", "both")
}

// The guard and the health role report must not disagree: a radio the guard
// calls the uplink's must never be reported as plain AP-only, or the health API
// declares safe the state the guard just refused.
func TestUplinkDefinition_AgreesWithGetRadiosRole(t *testing.T) {
	svc, u := newTestWifiService()
	// radio0: enabled AP + enabled STA. radio1: enabled AP only.
	assertRadioRole(t, svc, "radio0", "both")
	if err := u.Set("wireless", "sta0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	assertRadioRole(t, svc, "radio0", "ap")

	// Enabling the STA on radio0 again makes the two agree on "both".
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	assertRadioRole(t, svc, "radio0", "both")

	for _, radio := range []string{"radio0", "radio1"} {
		err := svc.rejectAPOnUplinkRadio(radio, "access point enabled")
		shares := radio == "radio0"
		if shares && !errors.Is(err, ErrAPAndSTASameRadio) {
			t.Errorf("%s carries the uplink and an AP: expected a refusal, got %v", radio, err)
		}
		if !shares && err != nil {
			t.Errorf("%s carries no uplink STA: expected no refusal, got %v", radio, err)
		}
	}
}

// assertNoRadioRunsBothAnAPAndTheUplink is the invariant the same-radio guards
// exist for, asserted through the role the health API reports rather than
// through the UCI options behind it.
func assertNoRadioRunsBothAnAPAndTheUplink(t *testing.T, svc *WifiService) {
	t.Helper()
	radios, err := svc.GetRadios()
	if err != nil {
		t.Fatalf("GetRadios: %v", err)
	}
	for _, r := range radios {
		if r.Role == "both" {
			t.Errorf("radio %s runs an access point and the uplink STA on one PHY", r.Name)
		}
	}
}

// assertUplinkRadio reads the radio the created uplink STA is bound to.
func assertUplinkRadio(t *testing.T, u *uci.MockUCI, want string) {
	t.Helper()
	device, err := u.Get("wireless", "sta0", "device")
	if err != nil {
		t.Fatalf("reading wireless.sta0.device: %v", err)
	}
	if device != want {
		t.Errorf("uplink STA device = %q, want %q", device, want)
	}
}

// assertAPEnabled checks whether an access point section is left enabled. A
// missing disabled option means enabled — that is how the stock config ships.
func assertAPEnabled(t *testing.T, u *uci.MockUCI, section string, want bool) {
	t.Helper()
	opts, err := u.GetAll("wireless", section)
	if err != nil {
		t.Fatalf("reading wireless.%s: %v", section, err)
	}
	if got := opts["disabled"] != "1"; got != want {
		t.Errorf("wireless.%s enabled = %v, want %v", section, got, want)
	}
}

func assertRadioRole(t *testing.T, svc *WifiService, radio, want string) {
	t.Helper()
	radios, err := svc.GetRadios()
	if err != nil {
		t.Fatalf("GetRadios: %v", err)
	}
	for _, r := range radios {
		if r.Name == radio {
			if r.Role != want {
				t.Errorf("%s role: expected %q, got %q", radio, want, r.Role)
			}
			return
		}
	}
	t.Fatalf("radio %s not reported by GetRadios", radio)
}

// sameSubnet reports whether two address/netmask pairs describe one IPv4
// network. Used instead of pinning the guest address to a literal, so the
// assertion is about the collision the guard exists to prevent.
func sameSubnet(t *testing.T, ipA, maskA, ipB, maskB string) bool {
	t.Helper()
	a, b := ipv4Net(ipA, maskA), ipv4Net(ipB, maskB)
	if a == nil || b == nil {
		t.Fatalf("not an IPv4 network: %s/%s and %s/%s", ipA, maskA, ipB, maskB)
	}
	return subnetsOverlap(a, b)
}

// Guest WiFi must not be placed on the PHY a hand-written uplink STA is using,
// even without network=wwan on it.
func TestSetGuestWifi_StaysOffAHandWrittenSTARadio(t *testing.T) {
	svc, u := newTestWifiService()
	// radio1 is the 2.4 GHz radio here, so the guest AP would prefer it by band
	// unless the uplink on it counts.
	if err := u.Set("wireless", "radio1", "band", "2g"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "radio0", "band", "5g"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "device", "radio1"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio1", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled:    true,
		SSID:       "Guest-Travel",
		Encryption: "psk2",
		Key:        "guestpass123",
	}, LockoutRequest{}); err != nil {
		t.Fatalf("SetGuestWifi: %v", err)
	}
	device, err := u.Get("wireless", "guest", "device")
	if err != nil {
		t.Fatalf("reading wireless.guest.device: %v", err)
	}
	if device != "radio0" {
		t.Errorf("expected the guest AP on radio0, away from the uplink on radio1, got %q", device)
	}
}
