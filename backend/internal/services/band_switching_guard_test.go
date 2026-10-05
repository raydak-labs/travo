package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Automatic band switching: what happens to the access point the uplink would
// land on, and what the operator is told when a switch cannot happen.
//
// The switcher runs unattended, so both halves matter: it must never commit an
// enabled access point and an enabled uplink STA on one PHY (ADR 0002 §2), and
// it must never quietly do nothing while the UI reports the feature as on.

// testGuardOverride points the crash guard at a temp file for the duration of a
// test. Without it doSwitch would write to the real /etc/trafo.
type testGuardOverride struct{ prev string }

func newGuardOverride(t *testing.T) *testGuardOverride {
	t.Helper()
	o := &testGuardOverride{prev: bandSwitchGuardFile}
	bandSwitchGuardFile = filepath.Join(t.TempDir(), "band-switch-in-progress")
	t.Cleanup(func() { bandSwitchGuardFile = o.prev })
	return o
}

// The common layout is an access point on both radios, so the switcher's only
// candidate always carries one. Refusing there made the feature unreachable —
// it reports enabled and never fires — so the switch has to happen the way
// Connect does it: take the access point off the target radio, switch, and let
// the downlink come back on the other band.
func TestDoSwitch_MovesTheAPOffTheTargetRadio(t *testing.T) {
	svc, u := newTestWifiService()
	b := NewBandSwitchingService(svc, filepath.Join(t.TempDir(), "band-switch.json"))
	newGuardOverride(t)

	if err := b.doSwitch("radio1", "signal too weak"); err != nil {
		t.Fatalf("doSwitch onto a radio carrying an access point: %v", err)
	}
	assertUplinkRadio(t, u, "radio1")
	assertNoRadioRunsBothAnAPAndTheUplink(t, svc)
	assertAPEnabled(t, u, "default_radio1", false)
	assertAPEnabled(t, u, "default_radio0", true)
	if _, err := os.Stat(bandSwitchGuardFile); !os.IsNotExist(err) {
		t.Errorf("a completed switch must not leave the crash guard behind (stat err: %v)", err)
	}
	if b.GetStatus().LastSwitchAt == "" {
		t.Error("a completed switch must be reported in the status")
	}
}

// A switch that cannot be made must be visible. The switcher reports itself
// blocked instead of leaving the UI claiming a working feature, and it must not
// leave the crash guard behind either: nothing was applied, and a stale guard
// disables automatic switching until somebody removes the file by hand.
func TestDoSwitch_ReportsABlockedSwitchAndLeavesNoGuard(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	b := NewBandSwitchingService(svc, filepath.Join(t.TempDir(), "band-switch.json"))
	newGuardOverride(t)

	err := b.doSwitch("radio1", "signal too weak")
	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected ErrAPAndSTASameRadio, got %v", err)
	}
	if got := b.GetStatus().State; got != "blocked" {
		t.Errorf("switcher state = %q, want \"blocked\" so the UI can see it is stuck", got)
	}
	if _, err := os.Stat(bandSwitchGuardFile); !os.IsNotExist(err) {
		t.Errorf("a refused switch must not leave the crash guard behind (stat err: %v)", err)
	}
	// The block is not sticky: once the layout leaves room for a switch, the
	// switcher works again and reports itself working instead of staying
	// blocked for good.
	if err := u.Set("wireless", "default_radio0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	if err := b.doSwitch("radio1", "signal too weak"); err != nil {
		t.Fatalf("doSwitch once the downlink is back on another radio: %v", err)
	}
	assertUplinkRadio(t, u, "radio1")
	if got := b.liveState(0); got != "monitoring" {
		t.Errorf("switcher state after a successful switch = %q, want \"monitoring\"", got)
	}
}

// A blocked switcher keeps saying so on every tick instead of reporting the
// ordinary "monitoring" the UI reads as a working feature.
func TestBandSwitching_BlockedStateOutranksTheTickState(t *testing.T) {
	svc, u := newTestWifiService()
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	b := NewBandSwitchingService(svc, filepath.Join(t.TempDir(), "band-switch.json"))
	newGuardOverride(t)

	if err := b.doSwitch("radio1", "signal too weak"); !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected the switch to be refused, got %v", err)
	}
	for _, weak := range []int{0, 30} {
		if got := b.liveState(weak); got != "blocked" {
			t.Errorf("liveState(weak=%d) = %q, want \"blocked\"", weak, got)
		}
	}
}

// A block that outlives the layout that caused it is the same defect the state
// was added to fix: the card says blocked for a switch the switcher could make
// again, and the operator goes looking for a fault that is not there.
func TestBandSwitching_BlockedClearsWhenTheLayoutFreesTheTargetRadio(t *testing.T) {
	svc, u := newTestWifiService()
	// Only radio1 runs an access point, so the uplink has nowhere to go.
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	b := NewBandSwitchingService(svc, filepath.Join(t.TempDir(), "band-switch.json"))
	newGuardOverride(t)
	cfg := b.GetConfig()
	cfg.Enabled = true
	cfg.CheckIntervalSec = 1
	if err := b.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	b.Start()
	defer b.Stop()

	if err := b.doSwitch("radio1", "signal too weak"); !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected the switch to be refused, got %v", err)
	}
	if got := b.GetStatus().State; got != "blocked" {
		t.Fatalf("state = %q, want \"blocked\" while the layout blocks the switch", got)
	}

	// The operator brings the downlink back on the other radio: the same
	// switch is possible now, so the switcher must stop claiming otherwise
	// without needing a switch to succeed first.
	if err := u.Set("wireless", "default_radio0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && b.GetStatus().State == "blocked" {
		time.Sleep(50 * time.Millisecond)
	}
	if got := b.GetStatus().State; got == "blocked" {
		t.Errorf("state = %q after the layout allows the switch again, want it cleared", got)
	}
}
