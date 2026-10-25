package wifi

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetConnection_DerivesRepeaterMode(t *testing.T) {
	svc, _ := newTestWifiService()

	conn, err := svc.GetConnection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Mode != "repeater" {
		t.Fatalf("expected repeater mode, got %q", conn.Mode)
	}
}

// iwinfo on OpenWrt 25.12 reports no "band" for a STA, only its frequency
// (GL-AXT1800: channel 149, frequency 5745); the WiFi summary showed no band.
func TestGetConnection_DerivesBandFromFrequency(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("iwinfo.info", map[string]any{
		"ssid": "Cappuxinno", "channel": float64(149), "frequency": float64(5745),
		"signal": float64(-27), "quality": float64(70),
	})
	svc := New(Deps{UCI: u, Ubus: ub, Reloader: &NoopWifiReloader{}, PriorityFile: t.TempDir() + "/p.json"})

	conn, err := svc.GetConnection()
	if err != nil {
		t.Fatal(err)
	}
	if conn.Band != "5g" {
		t.Errorf("band = %q, want 5g", conn.Band)
	}
}

// Regression for the "connected but no IP" bug: Connect() chose sta_B, disabling sta_A.
// Later SetMode("repeater") must not blindly re-enable sta_A; the currently-active profile
// (sta_B) stays the sole STA on wwan.
func TestConnectThenSetModeRepeater_PreservesSingleActiveSTA(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	tmpFile := t.TempDir() + "/priorities.json"
	svc := New(Deps{UCI: u, Ubus: ub, Reloader: &NoopWifiReloader{}, PriorityFile: tmpFile})

	// Existing sta0 (Hotel-WiFi) is the mock default.
	// Connect to a second network — Connect() creates sta1 and disables sta0.
	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Password: "latte", Encryption: "psk2",
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	sta0Before, _ := u.Get("wireless", "sta0", "disabled")
	sta1Before, _ := u.Get("wireless", "sta1", "disabled")
	if sta0Before != "1" || sta1Before != "0" {
		t.Fatalf("setup: expected sta0=1 sta1=0 after Connect, got %q %q", sta0Before, sta1Before)
	}

	if _, err := svc.SetMode("repeater", LockoutRequest{}); err != nil {
		t.Fatalf("SetMode: %v", err)
	}

	sta0After, _ := u.Get("wireless", "sta0", "disabled")
	sta1After, _ := u.Get("wireless", "sta1", "disabled")
	if sta0After != "1" {
		t.Errorf("SetMode re-enabled stale STA: expected sta0 disabled=1, got %q", sta0After)
	}
	if sta1After != "0" {
		t.Errorf("SetMode disabled active STA: expected sta1 disabled=0, got %q", sta1After)
	}
}

// A second enabled access point on ANOTHER radio is the layout ADR 0002 §2 asks
// for, so Connect is not refused; with no split available the uplink would put
// an AP and a STA on the only PHY that runs an access point, which is refused
// rather than silently taking the router's WiFi away.
func TestConnect_RefusesWhenTheTargetRadioHasAnEnabledAP(t *testing.T) {
	svc, u := clientModeWithAPService(t)
	// An access point on radio0 and none anywhere else: freeing radio0 would
	// mean disabling the router's only WiFi.
	if err := u.Set("wireless", "default_radio0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	if err := u.DeleteSection("wireless", "default_radio1"); err != nil {
		t.Fatal(err)
	}
	applier := &fakeWirelessApplier{startToken: "must-not-be-used"}
	svc.applier = applier

	_, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Band: "2g", Encryption: "psk2", Password: "cafepass1",
	})
	if !errors.Is(err, ErrAPAndSTASameRadio) {
		t.Fatalf("expected ErrAPAndSTASameRadio, got %v", err)
	}
	if dis, _ := u.Get("wireless", "default_radio0", "disabled"); dis == "1" {
		t.Error("the refused connect disabled the access point anyway")
	}
	for _, c := range u.commitCalls() {
		if c == "wireless" {
			t.Error("wireless was committed after the connect was refused")
		}
	}
	if len(applier.started) != 0 {
		t.Errorf("no apply may be started for a refused connect, got %v", applier.started)
	}
}

// The split layout is not a refusal: with an enabled access point on the other
// radio the uplink is welcome, and the AP sharing the target radio is disabled
// so the PHY is not asked to do both — the same move SetMode("repeater") makes,
// applied here in client mode too.
func TestConnect_MovesTheEnabledAPOffTheUplinkRadio(t *testing.T) {
	svc, u := clientModeWithAPService(t)
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "disabled", "0"); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Cafe-WiFi", Band: "2g", Encryption: "psk2", Password: "cafepass1",
	}); err != nil {
		t.Fatalf("Connect must be allowed when the downlink AP has the other radio: %v", err)
	}
	dis, _ := u.Get("wireless", "default_radio0", "disabled")
	if dis != "1" {
		t.Errorf("expected the access point on the uplink radio to be disabled, got %q", dis)
	}
	if dis, _ := u.Get("wireless", "default_radio1", "disabled"); dis != "0" {
		t.Errorf("expected the access point on the other radio to stay enabled, got %q", dis)
	}
	assertNoSameRadioAPSTA(t, u, "radio0")
}

func TestConnect_RevertsStagedWritesOnFailure(t *testing.T) {
	u := &revertingUCI{MockUCI: uci.NewMockUCI()}
	u.failSet = func(config, section, option string) error {
		if config == "wireless" && option == "ssid" {
			return fmt.Errorf("disk full")
		}
		return nil
	}
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus(), Reloader: &NoopWifiReloader{}})

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", Password: "newpass123", Encryption: "psk2",
	}); err == nil {
		t.Fatal("expected Connect to fail")
	}
	// The staged delta must be dropped: the uci CLI keeps it in the
	// process-global /tmp/.uci/wireless/changes, where a later unrelated
	// `uci commit wireless` would persist it.
	if !slices.Contains(u.revertCalls(), "wireless") {
		t.Errorf("expected the staged wireless delta to be reverted, got %v", u.revertCalls())
	}
}

func TestConnect_DoesNotRevertOnSuccess(t *testing.T) {
	u := &revertingUCI{MockUCI: uci.NewMockUCI()}
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus(), Reloader: &NoopWifiReloader{}})

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Hotel-WiFi", Password: "newpass123", Encryption: "psk2",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls := u.revertCalls(); len(calls) != 0 {
		t.Errorf("successful Connect must not revert the config it just committed, got %v", calls)
	}
}

func TestGetSavedNetworks_SurfacesGetSectionsError(t *testing.T) {
	svc := New(Deps{UCI: &uci.FailingSectionsUCI{MockUCI: uci.NewMockUCI()}, Ubus: ubus.NewMockUbus(), Reloader: &NoopWifiReloader{}})

	networks, err := svc.GetSavedNetworks()
	if err == nil {
		t.Fatal("expected an error instead of an empty saved-network list when the UCI read fails")
	}
	if networks != nil {
		t.Errorf("expected no list on error, got %v", networks)
	}
}

// TestRadioForNewSTAIsDeterministic pins a bug this branch's own new check
// exposed rather than caused.
//
// Connect used to pick the radio for a new uplink STA by taking the first entry
// found while ranging over the wireless-section map, so Go's randomised
// iteration order decided it. Downstream that was not cosmetic: the chosen radio
// decides whether the uplink shares a PHY with an access point, and the new
// splitAPOffUplinkRadio check refuses when it would. The result was an
// intermittent "refusing to run an access point and the WiFi uplink on the same
// radio" that depended on map order — a ~40% failure rate in
// TestWifiConnect_MultipleProfilesPersist.
func TestRadioForNewSTAIsDeterministic(t *testing.T) {
	svc, u := newTestWifiService()

	// Both radios carry an enabled access point, the stock layout, so nothing
	// is preferable and the sorted first radio must win every time.
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "mode", "ap"); err != nil {
			t.Fatal(err)
		}
		if err := u.Set("wireless", section, "device", section); err != nil {
			t.Fatal(err)
		}
		if err := u.Set("wireless", section, "disabled", "0"); err != nil {
			t.Fatal(err)
		}
	}

	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("read sections: %v", err)
	}

	first, err := svc.radioForNewSTA(sections)
	if err != nil {
		t.Fatalf("radioForNewSTA: %v", err)
	}
	if first != "radio0" {
		t.Errorf("radioForNewSTA = %q, want radio0 (sorted, not map order)", first)
	}

	// Repeat: the choice must not move between calls on identical input.
	for i := range 50 {
		again, err := svc.radioForNewSTA(sections)
		if err != nil {
			t.Fatalf("radioForNewSTA call %d: %v", i, err)
		}
		if again != first {
			t.Fatalf("radioForNewSTA is not deterministic: call %d returned %q, first returned %q",
				i, again, first)
		}
	}
}

// TestRadioForNewSTAPrefersRadioWithoutAccessPoint pins the second property:
// when one radio is free of access points the uplink should go there, so a
// second Connect does not land on the radio holding the only remaining access
// point and get refused for it.
func TestRadioForNewSTAPrefersRadioWithoutAccessPoint(t *testing.T) {
	svc, u := newTestWifiService()

	// Simulate the state a first Connect leaves behind: radio0's AP disabled by
	// splitAPOffUplinkRadio, radio1 still carrying the only enabled AP.
	if err := u.Set("wireless", "default_radio0", "mode", "ap"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio0", "device", "radio0"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio1", "mode", "ap"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio1", "device", "radio1"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "default_radio1", "disabled", "0"); err != nil {
		t.Fatal(err)
	}

	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("read sections: %v", err)
	}
	got, err := svc.radioForNewSTA(sections)
	if err != nil {
		t.Fatalf("radioForNewSTA: %v", err)
	}
	if got != "radio0" {
		t.Errorf("radioForNewSTA = %q, want radio0: it has no enabled access point, so "+
			"choosing it avoids disabling a second one and avoids being refused for "+
			"sharing a PHY with the only remaining AP", got)
	}
}

// TestConnectTwiceSucceeds is the regression the two properties exist for: two
// consecutive connects with no band pinned must both succeed. Before, the second
// one could be refused because the new STA landed on the radio that still held
// the only enabled access point.
func TestConnectTwiceSucceeds(t *testing.T) {
	svc, _ := newTestWifiService()

	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Coffee-Shop", Password: "coffee123", Encryption: "psk2",
	}); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	if _, err := svc.Connect(models.WifiConfig{
		SSID: "Airport-WiFi", Password: "air456", Encryption: "psk2",
	}); err != nil {
		t.Fatalf("second connect was refused: %v", err)
	}
}
