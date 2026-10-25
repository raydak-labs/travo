package wifi

import (
	"errors"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

func TestGetGuestWifi_NotConfigured(t *testing.T) {
	svc, _ := newTestWifiService()

	cfg, err := svc.GetGuestWifi()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected guest wifi to be disabled when not configured")
	}
}

func TestSetGuestWifi_Enable(t *testing.T) {
	svc, u := newTestWifiService()

	_, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled:    true,
		SSID:       "Guest-Travel",
		Encryption: "psk2",
		Key:        "guestpass123",
	}, LockoutRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify wireless guest section
	opts, err := u.GetAll("wireless", "guest")
	if err != nil {
		t.Fatalf("expected wireless.guest to exist: %v", err)
	}
	if opts["ssid"] != "Guest-Travel" {
		t.Errorf("expected ssid 'Guest-Travel', got %q", opts["ssid"])
	}
	if opts["isolate"] != "1" {
		t.Errorf("expected isolate '1', got %q", opts["isolate"])
	}
	if opts["disabled"] != "0" {
		t.Errorf("expected disabled '0', got %q", opts["disabled"])
	}
	if opts["network"] != "guest" {
		t.Errorf("expected network 'guest', got %q", opts["network"])
	}

	// Verify network.guest interface. The address is not pinned to a literal:
	// what matters is that the guest subnet does not collide with network.lan,
	// which is why SetGuestWifi refuses an overlap (see
	// TestSetGuestWifi_RefusesASubnetThatOverlapsLAN).
	netOpts, err := u.GetAll("network", "guest")
	if err != nil {
		t.Fatalf("expected network.guest to exist: %v", err)
	}
	lanOpts, err := u.GetAll("network", "lan")
	if err != nil {
		t.Fatalf("expected network.lan to exist: %v", err)
	}
	if sameSubnet(t, netOpts["ipaddr"], netOpts["netmask"], lanOpts["ipaddr"], lanOpts["netmask"]) {
		t.Errorf("guest subnet %s/%s overlaps network.lan %s/%s",
			netOpts["ipaddr"], netOpts["netmask"], lanOpts["ipaddr"], lanOpts["netmask"])
	}

	// Verify dhcp.guest
	dhcpOpts, err := u.GetAll("dhcp", "guest")
	if err != nil {
		t.Fatalf("expected dhcp.guest to exist: %v", err)
	}
	if dhcpOpts["interface"] != "guest" {
		t.Errorf("expected dhcp interface 'guest', got %q", dhcpOpts["interface"])
	}

	// Verify firewall guest zone
	fwOpts, err := u.GetAll("firewall", "guest_zone")
	if err != nil {
		t.Fatalf("expected firewall.guest_zone to exist: %v", err)
	}
	if fwOpts["forward"] != "REJECT" {
		t.Errorf("expected forward 'REJECT', got %q", fwOpts["forward"])
	}

	// Verify guest->wan forwarding
	fwdOpts, err := u.GetAll("firewall", "guest_fwd")
	if err != nil {
		t.Fatalf("expected firewall.guest_fwd to exist: %v", err)
	}
	if fwdOpts["dest"] != "wan" {
		t.Errorf("expected forwarding dest 'wan', got %q", fwdOpts["dest"])
	}

	// Verify GetGuestWifi returns correct config
	cfg, err := svc.GetGuestWifi()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected guest wifi to be enabled")
	}
	if cfg.SSID != "Guest-Travel" {
		t.Errorf("expected SSID 'Guest-Travel', got %q", cfg.SSID)
	}
}

func TestSetGuestWifi_Disable(t *testing.T) {
	svc, u := newTestWifiService()

	// Enable first
	_, _ = svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled:    true,
		SSID:       "Guest-Travel",
		Encryption: "psk2",
		Key:        "guestpass123",
	}, LockoutRequest{})

	// Disable
	_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, err := u.Get("wireless", "guest", "disabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "1" {
		t.Errorf("expected disabled '1', got %q", val)
	}

	cfg, _ := svc.GetGuestWifi()
	if cfg.Enabled {
		t.Error("expected guest wifi to be disabled")
	}
}

func TestSetGuestWifi_DisableWhenNotConfigured(t *testing.T) {
	svc, _ := newTestWifiService()

	_, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetGuestWifi_ReturnsApplyError(t *testing.T) {
	svc, _ := newTestWifiService()
	svc.applier = &fakeWirelessApplier{startErr: errors.New("apply failed")}

	_, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled:    true,
		SSID:       "Guest-Travel",
		Encryption: "psk2",
		Key:        "guestpass123",
	}, LockoutRequest{})
	if err == nil || !strings.Contains(err.Error(), "apply failed") {
		t.Fatalf("expected apply error, got %v", err)
	}
}

// The 2.4 GHz preference must not be a hard veto. On the archetypal travel
// router the uplink STA is on the 2.4 GHz radio, so preferring it made the
// guest access point a 409 with no way forward but allow_ap_on_sta_radio — i.e.
// deliberately re-enabling the crash state the guard exists to prevent. The
// guest AP belongs on the radio that does NOT carry the uplink, exactly like
// the downlink AP does.
func TestSetGuestWifi_FallsBackToTheRadioWithoutTheUplink(t *testing.T) {
	svc, u := clientModeService(t, false)

	if _, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{}); err != nil {
		t.Fatalf("SetGuestWifi on the 2.4 GHz uplink radio must fall back, got %v", err)
	}
	device, _ := u.Get("wireless", "guest", "device")
	if device != "radio1" {
		t.Errorf("expected the guest AP on radio1, the radio without the uplink, got %q", device)
	}
	if dis, _ := u.Get("wireless", "guest", "disabled"); dis != "0" {
		t.Errorf("expected the guest AP to be enabled, got disabled=%q", dis)
	}
	for _, radio := range []string{"radio0", "radio1"} {
		assertNoSameRadioAPSTA(t, u, radio)
	}
}

// Single-radio hardware has no split to make, and rejectAPOnUplinkRadio accepts
// the coexistence there — the same trade-off SetMode("repeater") and
// SetRadioRole already accept, and the only case left where preferredGuestRadio
// cannot fall back. Pinned so guest WiFi cannot silently become impossible on
// single-radio travel routers.
func TestSetGuestWifi_SingleRadioKeepsCoexistence(t *testing.T) {
	svc, u := repeaterModeService(t, false)
	for _, section := range []string{"radio1", "default_radio1"} {
		if err := u.DeleteSection("wireless", section); err != nil {
			t.Fatal(err)
		}
	}
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{}); err != nil {
		t.Fatalf("guest WiFi must stay available on single-radio hardware: %v", err)
	}
	device, _ := u.Get("wireless", "guest", "device")
	if device != "radio0" {
		t.Errorf("expected the guest AP on the only radio, got %q", device)
	}
}

func TestSetGuestWifi_AllowedOnTheRadioWithoutTheUplink(t *testing.T) {
	svc, u := clientModeService(t, false)
	// Move the uplink to the 5 GHz radio; the guest AP then lands on radio0,
	// which is what the split layout asks for.
	if err := u.Set("wireless", "sta0", "device", "radio1"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{}); err != nil {
		t.Fatalf("SetGuestWifi: %v", err)
	}
	device, _ := u.Get("wireless", "guest", "device")
	if device != "radio0" {
		t.Errorf("expected the guest AP on radio0, got %q", device)
	}
}

// The guest AP no longer needs the escape hatch to sit on the 2.4 GHz uplink
// radio: preferredGuestRadio falls back to the other one. The hatch is still
// what lets an access point STAY on the uplink radio, which is what the
// SetAPConfig sibling test covers.
func TestSetGuestWifi_AllowedWithAllowAPOnSTARadio(t *testing.T) {
	svc, u := clientModeService(t, true)

	if _, err := svc.SetGuestWifi(guestEnable(), LockoutRequest{}); err != nil {
		t.Fatalf("guest WiFi must be available with allow_ap_on_sta_radio: %v", err)
	}
	if dis, _ := u.Get("wireless", "guest", "disabled"); dis != "0" {
		t.Errorf("expected the guest AP to be enabled, got disabled=%q", dis)
	}
}

func TestSetGuestWifi_DisableTearsDownGuestNetwork(t *testing.T) {
	svc, u := newTestWifiService()

	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-Travel", Encryption: "psk2", Key: "guestpass123",
	}, LockoutRequest{}); err != nil {
		t.Fatalf("enable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{}); err != nil {
		t.Fatalf("disable guest: %v", err)
	}

	if dis, _ := u.Get("wireless", "guest", "disabled"); dis != "1" {
		t.Errorf("expected wireless.guest disabled=1, got %q", dis)
	}
	removed := []struct{ config, section string }{
		{"network", "guest"},
		{"dhcp", "guest"},
		{"firewall", "guest_zone"},
		{"firewall", "guest_fwd"},
		{"firewall", "guest_dns"},
		{"firewall", "guest_dhcp"},
	}
	for _, r := range removed {
		if _, err := u.GetAll(r.config, r.section); err == nil {
			t.Errorf("expected %s.%s to be removed when guest WiFi is disabled", r.config, r.section)
		}
	}
}

func TestSetGuestWifi_DisableThenEnableRestoresNetwork(t *testing.T) {
	svc, u := newTestWifiService()

	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-Travel", Encryption: "psk2", Key: "guestpass123",
	}, LockoutRequest{}); err != nil {
		t.Fatalf("enable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{Enabled: false}, LockoutRequest{}); err != nil {
		t.Fatalf("disable guest: %v", err)
	}
	if _, err := svc.SetGuestWifi(models.GuestWifiConfig{
		Enabled: true, SSID: "Guest-2", Encryption: "psk2", Key: "guestpass123",
	}, LockoutRequest{}); err != nil {
		t.Fatalf("re-enable guest: %v", err)
	}
	net, err := u.GetAll("network", "guest")
	if err != nil {
		t.Fatalf("expected network.guest to be restored: %v", err)
	}
	if net["ipaddr"] != "192.168.2.1" {
		t.Errorf("expected guest network to be rebuilt with ipaddr 192.168.2.1, got %q", net["ipaddr"])
	}
	fw, err := u.GetAll("firewall", "guest_zone")
	if err != nil {
		t.Fatalf("expected firewall.guest_zone to be restored: %v", err)
	}
	if fw["name"] != "guest" {
		t.Errorf("expected guest zone to be restored, got %v", fw)
	}
}
