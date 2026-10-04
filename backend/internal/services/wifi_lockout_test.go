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

// Clients the guard has to tell apart. The IPs are chosen to sit inside the
// prefixes of the interface dump registered below, because that dump — not a
// separate notion of "who is the caller" — is what the router really knows.
const (
	wifiClientIP = "10.0.0.50"    // matches wwan0 -> wifi-client
	wifiAPIP     = "192.168.8.50" // matches br-lan -> wifi-ap
	ethernetIP   = "172.16.0.10"  // matches eth0  -> ethernet
)

// lockoutDump is the shape `ubus call network.interface dump` really returns:
// an "interface" array of per-L3-device objects carrying up/interface flags and
// ipv4-prefix CIDR strings. The lockout guard classifies the caller through
// NetworkService.GetConnectionMethod, so it reads exactly this.
func lockoutDump() map[string]any {
	entry := func(l3 string, prefix string) map[string]any {
		return map[string]any{
			"interface":   true,
			"up":          true,
			"device":      l3,
			"l3_device":   l3,
			"ipv4-prefix": []any{map[string]any{"address": prefix}},
		}
	}
	return map[string]any{
		"interface": []any{
			entry("wwan0", "10.0.0.0/24"),
			entry("br-lan", "192.168.8.0/24"),
			entry("eth0", "172.16.0.0/12"),
		},
	}
}

// newLockoutService wires a wifi service whose caller classification is
// deterministic and whose apply sessions are recorded, so a refusal can be
// asserted to have staged nothing at all.
func newLockoutService(t *testing.T) (*WifiService, *uci.MockUCI, *fakeWirelessApplier) {
	t.Helper()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", lockoutDump())
	applier := &fakeWirelessApplier{startToken: "lockout-token"}
	svc := NewWifiServiceWithApplier(u, ub, applier)
	svc.guardDir = testGuardDir()
	return svc, u, applier
}

// dumpUCI renders every config the wireless mutators touch as a stable string,
// so a refusal can be asserted to have left the configuration byte-identical
// rather than merely "close enough".
func dumpUCI(t *testing.T, u *uci.MockUCI) string {
	t.Helper()
	var b strings.Builder
	for _, config := range []string{"wireless", "network", "dhcp", "firewall", "system"} {
		sections, err := u.GetSections(config)
		if err != nil {
			t.Fatalf("reading %s: %v", config, err)
		}
		names := make([]string, 0, len(sections))
		for name := range sections {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			opts := sections[name]
			keys := make([]string, 0, len(opts))
			for k := range opts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			b.WriteString(config + "." + name + "{")
			for _, k := range keys {
				b.WriteString(k + "=" + opts[k] + ";")
			}
			b.WriteString("}")
		}
	}
	return b.String()
}

func assertRefusedAndUntouched(
	t *testing.T, err error, before string,
	u *uci.MockUCI, applier *fakeWirelessApplier, sessionsBefore int,
) {
	t.Helper()
	if !errors.Is(err, ErrLockoutRefused) {
		t.Fatalf("err = %v, want ErrLockoutRefused", err)
	}
	if after := dumpUCI(t, u); after != before {
		t.Errorf("config changed on a refused request:\nbefore %s\nafter  %s", before, after)
	}
	if len(applier.started) != sessionsBefore {
		t.Errorf("refused request staged %v apply session(s), want %d",
			applier.started, sessionsBefore)
	}
}

// ---------------------------------------------------------------------------
// PUT /api/v1/wifi/mode — SetMode("client") disables every access point.
// ---------------------------------------------------------------------------

func TestSetMode_WifiCallerIsRefusedWithoutAcknowledgement(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetMode("client", LockoutRequest{ClientIP: wifiAPIP})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

func TestSetMode_WifiClientUplinkIsRefusedWithoutAcknowledgement(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetMode("client", LockoutRequest{ClientIP: wifiClientIP})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

func TestSetMode_AcknowledgementAppliesTheChange(t *testing.T) {
	svc, u, _ := newLockoutService(t)

	if _, err := svc.SetMode("client", LockoutRequest{
		ClientIP:           wifiAPIP,
		AcknowledgeLockout: true,
	}); err != nil {
		t.Fatalf("acknowledged SetMode: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v != "1" {
		t.Errorf("default_radio0 disabled = %q, want \"1\"", v)
	}
}

func TestSetMode_EthernetCallerIsNeverStranded(t *testing.T) {
	svc, u, _ := newLockoutService(t)

	if _, err := svc.SetMode("client", LockoutRequest{ClientIP: ethernetIP}); err != nil {
		t.Fatalf("SetMode from Ethernet: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v != "1" {
		t.Errorf("default_radio0 disabled = %q, want \"1\"", v)
	}
}

func TestSetMode_RepeaterKeepsAnAccessPointSoItIsAllowed(t *testing.T) {
	svc, _, _ := newLockoutService(t)

	if _, err := svc.SetMode("repeater", LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("SetMode(repeater) over WiFi: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PUT /api/v1/wifi/ap/{section} — SetAPConfig
// ---------------------------------------------------------------------------

func TestSetAPConfig_DisablingTheLastAccessPointIsRefused(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	off := false

	// The first radio may go: radio1's access point is still up.
	if _, err := svc.SetAPConfig("default_radio0",
		models.APConfigUpdate{Enabled: &off}, LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("disabling the first of two access points: %v", err)
	}
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetAPConfig("default_radio1",
		models.APConfigUpdate{Enabled: &off}, LockoutRequest{ClientIP: wifiAPIP})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

func TestSetAPConfig_AcknowledgementAppliesTheDisable(t *testing.T) {
	svc, u, _ := newLockoutService(t)
	off := false

	for _, section := range []string{"default_radio0", "default_radio1"} {
		if _, err := svc.SetAPConfig(section, models.APConfigUpdate{Enabled: &off},
			LockoutRequest{ClientIP: wifiAPIP, AcknowledgeLockout: true}); err != nil {
			t.Fatalf("acknowledged disable of %s: %v", section, err)
		}
	}
	if v, _ := u.Get("wireless", "default_radio1", "disabled"); v != "1" {
		t.Errorf("default_radio1 disabled = %q, want \"1\"", v)
	}
}

func TestSetAPConfig_EthernetCallerIsNeverStranded(t *testing.T) {
	svc, u, _ := newLockoutService(t)
	off := false

	for _, section := range []string{"default_radio0", "default_radio1"} {
		if _, err := svc.SetAPConfig(section, models.APConfigUpdate{Enabled: &off},
			LockoutRequest{ClientIP: ethernetIP}); err != nil {
			t.Fatalf("disable of %s from Ethernet: %v", section, err)
		}
	}
	if v, _ := u.Get("wireless", "default_radio1", "disabled"); v != "1" {
		t.Errorf("default_radio1 disabled = %q, want \"1\"", v)
	}
}

// The false-positive guard: renaming an access point leaves it up, so a WiFi
// caller must not be refused. Without this the rule is too broad to use.
func TestSetAPConfig_SSIDChangeOverWiFiApplies(t *testing.T) {
	svc, u, _ := newLockoutService(t)

	if _, err := svc.SetAPConfig("default_radio0",
		models.APConfigUpdate{SSID: "Renamed", Encryption: "psk2", Key: "travelrouter"},
		LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("SSID change over WiFi: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio0", "ssid"); v != "Renamed" {
		t.Errorf("ssid = %q, want \"Renamed\"", v)
	}
}

// ---------------------------------------------------------------------------
// PUT /api/v1/wifi/radio and /radios/{name}/role
// ---------------------------------------------------------------------------

func TestSetRadioEnabled_DisablingEveryRadioIsRefused(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetRadioEnabled(false, LockoutRequest{ClientIP: wifiAPIP})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

func TestSetRadioEnabled_EthernetCallerIsNeverStranded(t *testing.T) {
	svc, u, _ := newLockoutService(t)

	if _, err := svc.SetRadioEnabled(false, LockoutRequest{ClientIP: ethernetIP}); err != nil {
		t.Fatalf("disabling radios from Ethernet: %v", err)
	}
	if v, _ := u.Get("wireless", "radio0", "disabled"); v != "1" {
		t.Errorf("radio0 disabled = %q, want \"1\"", v)
	}
}

// Turning off ONE radio while the other radio's access point stays up must go
// through: that is the everyday "5G off, 2.4G stays" request.
func TestSetRadioRole_NoneOnOneRadioAppliesWhileAnotherAPStaysUp(t *testing.T) {
	svc, u, applier := newLockoutService(t)

	if _, err := svc.SetRadioRole("radio1", "none",
		LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("role none on radio1: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio1", "disabled"); v != "1" {
		t.Errorf("default_radio1 disabled = %q, want \"1\"", v)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v == "1" {
		t.Error("default_radio0 must stay enabled")
	}
	if len(applier.started) == 0 {
		t.Error("expected the safe change to be applied")
	}
}

func TestSetRadioRole_NoneOnTheLastActiveRadioIsRefused(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	off := false
	if _, err := svc.SetAPConfig("default_radio0",
		models.APConfigUpdate{Enabled: &off}, LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("disabling default_radio0: %v", err)
	}
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetRadioRole("radio1", "none", LockoutRequest{ClientIP: wifiAPIP})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

func TestSetRadioEnabled_AcknowledgementAppliesTheDisable(t *testing.T) {
	svc, u, _ := newLockoutService(t)

	if _, err := svc.SetRadioEnabled(false,
		LockoutRequest{ClientIP: wifiAPIP, AcknowledgeLockout: true}); err != nil {
		t.Fatalf("acknowledged radio disable: %v", err)
	}
	if v, _ := u.Get("wireless", "radio1", "disabled"); v != "1" {
		t.Errorf("radio1 disabled = %q, want \"1\"", v)
	}
}

func TestSetRadioRole_AcknowledgementAppliesTheLastRadioOff(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	off := false
	if _, err := svc.SetAPConfig("default_radio0",
		models.APConfigUpdate{Enabled: &off}, LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("disabling default_radio0: %v", err)
	}

	if _, err := svc.SetRadioRole("radio1", "none", LockoutRequest{
		ClientIP:           wifiAPIP,
		AcknowledgeLockout: true,
	}); err != nil {
		t.Fatalf("acknowledged role none on radio1: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio1", "disabled"); v != "1" {
		t.Errorf("default_radio1 disabled = %q, want \"1\"", v)
	}
	if len(applier.started) == 0 {
		t.Error("expected the acknowledged change to be applied")
	}
}

// ---------------------------------------------------------------------------
// PUT /api/v1/wifi/guest — SetGuestWifi
// ---------------------------------------------------------------------------

func TestSetGuestWifi_DisablingWhileAnotherAccessPointIsUpApplies(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest", Encryption: "psk2", Key: "guestpass",
	}, LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("enabling guest WiFi: %v", err)
	}

	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false},
		LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("disabling guest WiFi while the default APs are up: %v", err)
	}
	if v, _ := u.Get("wireless", "guest", "disabled"); v != "1" {
		t.Errorf("guest disabled = %q, want \"1\"", v)
	}
	_ = applier
}

func TestSetGuestWifi_DisablingTheOnlyAccessPointIsRefused(t *testing.T) {
	svc, u, applier := newLockoutService(t)
	off := false
	ack := LockoutRequest{ClientIP: wifiAPIP, AcknowledgeLockout: true}
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if _, err := svc.SetAPConfig(section, models.APConfigUpdate{Enabled: &off}, ack); err != nil {
			t.Fatalf("disabling %s: %v", section, err)
		}
	}
	// Guest WiFi is now the only access point left.
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest", Encryption: "psk2", Key: "guestpass",
	}, LockoutRequest{ClientIP: wifiAPIP}); err != nil {
		t.Fatalf("enabling guest WiFi: %v", err)
	}
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false},
		LockoutRequest{ClientIP: wifiAPIP})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}
