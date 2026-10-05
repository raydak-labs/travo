package services

import (
	"errors"
	"testing"
)

// A REFUSED wireless change must leave the configuration exactly as it was.
// The invariant is safety-critical: the operator who asked for the change is
// the one a refusal is supposed to leave with a working router, so a refusal
// that writes anything — a new STA profile, a wwan interface, a disabled
// access point — is worse than the failure it was reporting.
//
// The two cases below pin both halves of the report this lane investigated:
// the everyday repeater layout is NOT refused (the 409 the operator saw), and a
// refusal that does happen changes nothing.

// TestRepeater_EverydayLayoutIsNotRefused pins the layout a repeater router
// actually runs in: the uplink STA on one radio and an ENABLED access point on
// the other, with the uplink radio's own access point left disabled. Verified
// on the test device (OpenWrt 25.12.3: radio0 = 5 GHz uplink, radio1 = 2.4 GHz
// access point), where this same request is served with 200, not 409.
func TestRepeater_EverydayLayoutIsNotRefused(t *testing.T) {
	svc, u, _ := newLockoutService(t)
	// The device layout: radio0 carries the uplink, radio1 the access point.
	_ = u.Set("wireless", "radio0", "band", "5g")
	_ = u.Set("wireless", "radio1", "band", "2g")
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio0", "disabled", "1")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "default_radio1", "disabled", "0")
	_ = u.Set("wireless", "sta0", "device", "radio0")
	_ = u.Set("wireless", "sta0", "disabled", "0")

	if _, err := svc.SetMode("repeater", LockoutRequest{ClientIP: ethernetIP}); err != nil {
		t.Fatalf("SetMode(repeater) on the everyday layout: %v", err)
	}
	if got, _ := u.Get("wireless", "default_radio1", "disabled"); got != "0" {
		t.Errorf("the access point on the free radio must stay enabled, got disabled=%q", got)
	}
	if got, _ := u.Get("wireless", "default_radio0", "disabled"); got != "1" {
		t.Errorf("the access point on the uplink radio must stay disabled, got disabled=%q", got)
	}
	if got, _ := u.Get("wireless", "sta0", "disabled"); got != "0" {
		t.Errorf("the uplink must stay enabled, got disabled=%q", got)
	}
}

// TestRepeater_RefusalLeavesEveryPackageUntouched is the safety-critical
// invariant: a REFUSED repeater change leaves every UCI package byte-identical
// and stages no apply session.
//
// The fixture is the shape that refuses — no uplink profile yet, and no access
// point enabled off the radio the uplink would take — so SetMode has to CREATE
// that profile (and its wwan interface) before it can know the layout. Deciding
// after that write is what makes a refusal dirty.
func TestRepeater_RefusalLeavesEveryPackageUntouched(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	// No uplink profile at all: SetMode has to create one to compute the layout.
	_ = u.DeleteSection("wireless", "sta0")
	// Neither access point is enabled, so there is no downlink anywhere.
	_ = u.Set("wireless", "default_radio0", "device", "radio0")
	_ = u.Set("wireless", "default_radio0", "disabled", "1")
	_ = u.Set("wireless", "default_radio1", "device", "radio1")
	_ = u.Set("wireless", "default_radio1", "disabled", "1")
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetMode("repeater", LockoutRequest{ClientIP: ethernetIP})

	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("SetMode(repeater) err = %v, want ErrAPAndSTASameRadio", err)
	}
	if after := dumpUCI(t, u); after != before {
		t.Errorf("a refused repeater change rewrote the config:\nbefore %s\nafter  %s",
			before, after)
	}
	if len(applier.started) != sessions {
		t.Errorf("a refused repeater change staged %v apply session(s), want %d",
			applier.started, sessions)
	}
	if len(applier.confirmed) != 0 {
		t.Errorf("a refused repeater change confirmed an apply session: %v",
			applier.confirmed)
	}
}
