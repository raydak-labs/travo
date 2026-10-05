package services

import (
	"encoding/json"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

// perDeviceUbus answers `network.device status` per interface name. MockUbus keys
// its responses by path and ignores the requested device, so it cannot express the
// case this file is about: an access point that is genuinely up and an uplink STA
// that is configured but has never associated.
type perDeviceUbus struct {
	*ubus.MockUbus
	devices map[string]map[string]any
}

func (p *perDeviceUbus) Call(path, method string, args map[string]any) (map[string]any, error) {
	if path == "network.device" && method == "status" {
		name, _ := args["name"].(string)
		if answer, ok := p.devices[name]; ok {
			return answer, nil
		}
		// netifd answers an unknown device with an error and no object.
		return nil, errUnknownDevice
	}
	return p.MockUbus.Call(path, method, args)
}

var errUnknownDevice = &deviceLookupError{}

type deviceLookupError struct{}

func (e *deviceLookupError) Error() string { return "Failed to look up device" }

// deviceUp / deviceNoCarrier build a `network.device status` answer. carrier is
// what distinguishes a configured uplink STA that never associated from an
// interface that is actually linked: on this device a non-associated STA reads
// up:true, carrier:false.
func deviceUp() map[string]any {
	return map[string]any{"present": true, "up": true, "carrier": true}
}

func deviceNoCarrier() map[string]any {
	return map[string]any{"present": true, "up": true, "carrier": false}
}

// Reproduced on 192.168.1.1 (GL.iNet GL-AXT1800, OpenWrt 25.12.3): connecting
// the uplink to a network that does not exist leaves
//
//	ubus call network.device status {"name":"phy0-sta0"}
//	  -> present:true, up:true, carrier:false
//
// while radio0 itself is up and settled. The probe took the radio flag as proof
// of the STA and confirmed the apply, which cancelled rpcd's rollback — leaving
// the previously working uplink disabled with no way back. That is the exact
// outcome the confirm probe exists to prevent, in the fail-open direction.
func TestConfirmApply_UplinkSTANeverAssociatedIsNotUp(t *testing.T) {
	svc, u := newTestWifiService()
	putInRepeaterLikeDeviceConfig(t, u)

	mock, ok := svc.ubus.(*ubus.MockUbus)
	if !ok {
		t.Fatal("expected a MockUbus")
	}
	svc.ubus = &perDeviceUbus{
		MockUbus: mock,
		devices: map[string]map[string]any{
			"phy0-sta0": deviceNoCarrier(),
			"phy1-ap0":  deviceUp(),
		},
	}
	// The captured payload: both radios up and settled, the STA listed on
	// radio0 and the access point on radio1.
	registerPayloadOn(t, svc.ubus, decodeWirelessStatus(t, goldenWirelessStatus))
	quietConfirmRetries(t)

	fake := &fakeWirelessApplier{}
	svc.applier = fake

	err := svc.ConfirmApply("session-sta-never-associated")
	if err == nil {
		t.Fatal("an uplink STA that never associated must not be confirmed as up")
	}
	if len(fake.confirmed) != 0 {
		t.Fatalf("a non-associated uplink must not cancel the rollback, got confirm %#v",
			fake.confirmed)
	}
}

// The converse, so the fix above cannot become "never confirm a client-mode
// change": an uplink STA that IS associated (carrier true) with its radio settled
// must still confirm.
func TestConfirmApply_AssociatedUplinkSTAStillConfirms(t *testing.T) {
	svc, u := newTestWifiService()
	putInRepeaterLikeDeviceConfig(t, u)

	mock, ok := svc.ubus.(*ubus.MockUbus)
	if !ok {
		t.Fatal("expected a MockUbus")
	}
	svc.ubus = &perDeviceUbus{
		MockUbus: mock,
		devices: map[string]map[string]any{
			"phy0-sta0": deviceUp(),
			"phy1-ap0":  deviceUp(),
		},
	}
	registerPayloadOn(t, svc.ubus, decodeWirelessStatus(t, goldenWirelessStatus))
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.ConfirmApply("session-sta-associated"); err != nil {
		t.Fatalf("an associated uplink STA must confirm, got %v", err)
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("expected the applier to be confirmed, got %#v", fake.confirmed)
	}
}

// An access point keeps the radio shortcut: proving it per-device would spend a
// ubus round-trip on the common single-AP layout for no extra assurance, and the
// budget counts those calls. This pins that the fix did not make APs pay for it.
func TestConfirmApply_SingleAccessPointStillUsesTheRadioShortcut(t *testing.T) {
	svc, u := newTestWifiService()
	putInRepeaterLikeDeviceConfig(t, u)
	registerPayload(t, svc, decodeWirelessStatus(t, goldenWirelessStatus))
	// No device answers registered at all: if the probe asked, it would get the
	// mock's default and this test would not be measuring the shortcut.
	fake := &fakeWirelessApplier{}
	svc.applier = fake

	if err := svc.appliedWirelessUp(
		map[string]map[string]string{"default_radio1": {"device": "radio1", "mode": "ap"}},
		[]string{"default_radio1"}, nil,
	); err != nil {
		t.Fatalf("a settled radio must still prove its only expected access point, got %v", err)
	}
}

// registerPayloadOn registers the wireless status payload on whichever ubus the
// service is using, so a test can swap in a wrapper without losing the payload.
func registerPayloadOn(t *testing.T, u ubus.Ubus, resp map[string]any) {
	t.Helper()
	type registerer interface {
		RegisterResponse(string, map[string]any)
	}
	reg, ok := u.(registerer)
	if !ok {
		t.Fatal("expected a ubus that accepts registered responses")
	}
	reg.RegisterResponse("network.wireless.status", resp)
	if _, err := json.Marshal(resp); err != nil {
		t.Fatalf("payload must be encodable: %v", err)
	}
}
