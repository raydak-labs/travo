package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// These tests run the guard against the payload captured from the test device
// rather than a hand-written one, because the bug they cover was a hand-written
// payload: the previous fixture advertised an `ipv4-prefix` CIDR that netifd
// does not emit, so every client fell through to `unknown` and the guard never
// fired on real hardware.
//
// The three callers are the real ones, all reachable through br-lan or the
// uplink on the same router:
//
//	192.168.1.2   -> 9c:eb:e8:d3:f8:d1  wired, not associated with any AP
//	192.168.1.151 -> 22:4e:76:6c:2d:62  the iPhone, associated with phy1-ap0
//	8.8.8.8                          outside every prefix: cannot be placed
func newDeviceLockoutService(t *testing.T, stations map[string]string) (
	*WifiService, *uci.MockUCI, *fakeWirelessApplier,
) {
	t.Helper()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", deviceInterfaceDump(t))
	applier := &fakeWirelessApplier{startToken: "lockout-token"}
	svc := NewWifiServiceWithApplier(u, ub, applier)
	svc.guardDir = testGuardDir()
	svc.cmd = &MockCommandRunner{RunFunc: fakeIw(stations)}
	svc.arpFile = writeTempFile(t, "arp", deviceNeighborTable(t))
	return svc, u, applier
}

// The false positive this whole change exists to avoid: the caller is on the
// LAN bridge, the change removes the last access point, and they are on a wire.
// Refusing here would make the guard unusable for the operator it protects.
func TestGuard_DeviceWiredClientIsNotRefused(t *testing.T) {
	svc, u, _ := newDeviceLockoutService(t, noAssociatedAPs())

	_, err := svc.SetMode("client", LockoutRequest{ClientIP: deviceWiredClientIP})
	if err != nil {
		t.Fatalf("SetMode from the wired client: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v != "1" {
		t.Errorf("default_radio0 disabled = %q, want \"1\"", v)
	}
}

// The case that actually stranded a phone: the caller is associated with
// phy1-ap0, and the change takes every access point down.
func TestGuard_DeviceWiFiClientIsRefused(t *testing.T) {
	svc, u, applier := newDeviceLockoutService(t,
		map[string]string{"phy1-ap0": phy1ApStationDump})
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetMode("client", LockoutRequest{ClientIP: deviceWiFiClientIP})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

// FAIL CLOSED ON `unknown`.
//
// The classifier cannot prove this caller is wired, so it cannot prove the
// change will not strand them. The safe direction is to refuse: an operator who
// is actually on WiFi gets a dialog they can acknowledge, and one who is not
// loses one click. Allowing it is the unrecoverable direction — a router with
// no reachable access point cannot explain itself and needs physical access.
func TestGuard_UnknownCallerIsRefused(t *testing.T) {
	svc, u, applier := newDeviceLockoutService(t, noAssociatedAPs())
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetMode("client", LockoutRequest{ClientIP: "8.8.8.8"})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

// An unresolvable MAC on br-lan is `unknown` for the same reason, and is the
// case a real operator hits when their neighbour entry has not aged in yet.
func TestGuard_UnresolvableMACOnTheLANBridgeIsRefused(t *testing.T) {
	svc, u, applier := newDeviceLockoutService(t, noAssociatedAPs())
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetMode("client", LockoutRequest{ClientIP: "192.168.1.99"})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

func TestGuard_UnknownCallerProceedsWhenAcknowledged(t *testing.T) {
	svc, u, _ := newDeviceLockoutService(t, noAssociatedAPs())

	_, err := svc.SetMode("client", LockoutRequest{
		ClientIP:           "8.8.8.8",
		AcknowledgeLockout: true,
	})
	if err != nil {
		t.Fatalf("acknowledged SetMode: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v != "1" {
		t.Errorf("default_radio0 disabled = %q, want \"1\"", v)
	}
}

// Refusal is about the RESULT, not about the caller's medium: an unknown caller
// is fine when an access point stays up, exactly like a WiFi caller is.
func TestGuard_UnknownCallerAppliesWhileAnAPRemains(t *testing.T) {
	svc, u, applier := newDeviceLockoutService(t, noAssociatedAPs())

	_, err := svc.SetRadioRole("radio1", "none", LockoutRequest{ClientIP: "8.8.8.8"})
	if err != nil {
		t.Fatalf("SetRadioRole from an unknown caller: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio1", "disabled"); v != "1" {
		t.Errorf("default_radio1 disabled = %q, want \"1\"", v)
	}
	if len(applier.started) == 0 {
		t.Error("expected the safe change to be applied")
	}
}

func TestGuard_WiFiClientStillAppliesWhenAnAPRemains(t *testing.T) {
	svc, u, applier := newDeviceLockoutService(t,
		map[string]string{"phy1-ap0": phy1ApStationDump})

	_, err := svc.SetRadioRole("radio1", "none", LockoutRequest{ClientIP: deviceWiFiClientIP})
	if err != nil {
		t.Fatalf("SetRadioRole from a WiFi caller while radio0 keeps its AP: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v == "1" {
		t.Error("default_radio0 must stay enabled")
	}
	_ = applier
}

func TestGuard_WiredClientMayTakeBothAccessPointsDown(t *testing.T) {
	svc, u, _ := newDeviceLockoutService(t, noAssociatedAPs())
	off := false

	// The wired client may take the FIRST access point down: one remains.
	if _, err := svc.SetAPConfig("default_radio0",
		models.APConfigUpdate{Enabled: &off},
		LockoutRequest{ClientIP: deviceWiredClientIP}); err != nil {
		t.Fatalf("disabling the first of two access points from Ethernet: %v", err)
	}
	// The SECOND one strands nobody who is on a wire, so it also goes through.
	if _, err := svc.SetAPConfig("default_radio1",
		models.APConfigUpdate{Enabled: &off},
		LockoutRequest{ClientIP: deviceWiredClientIP}); err != nil {
		t.Fatalf("disabling the last access point from Ethernet: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio1", "disabled"); v != "1" {
		t.Errorf("default_radio1 disabled = %q, want \"1\"", v)
	}
}

// The uplink-STA caller keeps its classification: reachable through phy0-sta0,
// it is `wifi-client` and is refused.
func TestGuard_UplinkSTAClientIsRefused(t *testing.T) {
	svc, u, applier := newDeviceLockoutService(t, noAssociatedAPs())
	before, sessions := dumpUCI(t, u), len(applier.started)

	_, err := svc.SetMode("client", LockoutRequest{ClientIP: deviceUplinkClientIP})

	assertRefusedAndUntouched(t, err, before, u, applier, sessions)
}

func TestGuard_NoCallerContextStillPasses(t *testing.T) {
	svc, u, _ := newDeviceLockoutService(t, noAssociatedAPs())

	// A background flow has no caller to strand. Refusing here would break every
	// non-HTTP mutator to protect against nothing.
	if _, err := svc.SetMode("client", LockoutRequest{}); err != nil {
		t.Fatalf("SetMode with no caller context: %v", err)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v != "1" {
		t.Errorf("default_radio0 disabled = %q, want \"1\"", v)
	}
}

// The refusal must name the remedy. "Ethernet" is the word that tells the
// operator what to do next.
func TestGuard_UnknownRefusalNamesTheRemedy(t *testing.T) {
	svc, _, _ := newDeviceLockoutService(t, noAssociatedAPs())

	_, err := svc.SetMode("client", LockoutRequest{ClientIP: "8.8.8.8"})
	if !errors.Is(err, ErrLockoutRefused) {
		t.Fatalf("err = %v, want ErrLockoutRefused", err)
	}
	if !strings.Contains(err.Error(), "Ethernet") {
		t.Errorf("refusal does not name the remedy: %q", err.Error())
	}
}
