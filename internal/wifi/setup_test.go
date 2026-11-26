package wifi

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/raydak-labs/travo/internal/device"
	"github.com/raydak-labs/travo/internal/device/mock"
)

func TestPlanSetup(t *testing.T) {
	factory := configFixture(t, "wireless", "factory-default/uci-show-wireless.txt")
	connected := configFixture(t, "wireless", "uci-show-wireless.txt")
	req := SetupRequest{SSID: DefaultSSID, Key: "example-only"}
	enable := func(section string) []uciWrite {
		return []uciWrite{
			{"set", "wireless", section, "ssid", DefaultSSID},
			{"set", "wireless", section, "encryption", SetupEncryption},
			{"key", "wireless", section, "key", ""},
			{"set", "wireless", section, "disabled", "0"},
		}
	}
	radioOff := slices.Clone(factory)
	for i := range radioOff {
		if radioOff[i].Name == "radio1" {
			radioOff[i].Options = map[string]string{"band": "2g", "disabled": "1"}
		}
	}
	guestOnly := slices.DeleteFunc(slices.Clone(factory), func(s device.Section) bool { return isAP(s) })
	for _, tc := range []struct {
		name     string
		wireless []device.Section
		writes   []uciWrite
		expect   []apExpectation
		err      error
	}{
		{"factory default: both APs", factory,
			append(enable("default_radio0"), enable("default_radio1")...),
			[]apExpectation{{"default_radio0", true, DefaultSSID}, {"default_radio1", true, DefaultSSID}}, nil},
		{"radio switched off", radioOff,
			append(enable("default_radio0"), append(enable("default_radio1"), uciWrite{"set", "wireless", "radio1", "disabled", "0"})...),
			[]apExpectation{{"default_radio0", true, DefaultSSID}, {"default_radio1", true, DefaultSSID}}, nil},
		// radio0 carries the uplink STA: its AP stays off, the one on radio1 is renamed.
		{"uplink on radio0", connected,
			[]uciWrite{{"set", "wireless", "default_radio1", "ssid", DefaultSSID}, {"set", "wireless", "default_radio1", "encryption", SetupEncryption}, {"key", "wireless", "default_radio1", "key", ""}},
			[]apExpectation{{"default_radio1", true, DefaultSSID}}, nil},
		{"no AP at all", guestOnly, nil, nil, ErrAPNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, expect, err := planSetup(tc.wireless, req)
			if !errors.Is(err, tc.err) || !slices.Equal(writes, tc.writes) || !slices.Equal(expect, tc.expect) {
				t.Fatalf("got %#v %#v %v", writes, expect, err)
			}
		})
	}
}

func TestPlanSetupEveryRadioCarriesAnUplink(t *testing.T) {
	connected := configFixture(t, "wireless", "uci-show-wireless.txt")
	// Put a second enabled STA on radio1, so no radio is free for an AP.
	sta := device.Section{Name: "wifinet9", Type: "wifi-iface", Options: map[string]string{"device": "radio1", "mode": "sta", "network": "wwan"}}
	if _, _, err := planSetup(append(slices.Clone(connected), sta), SetupRequest{SSID: "x", Key: "example-only"}); !errors.Is(err, ErrRadioInUse) {
		t.Fatal(err)
	}
}

func TestSetupValidation(t *testing.T) {
	s, _, begin := apService(t, apUbus(t))
	for _, req := range []SetupRequest{{SSID: "", Key: "example-only"}, {SSID: "Travo", Key: "short"}, {SSID: "Travo"}} {
		var v ValidationError
		if _, err := s.SetupAPs(context.Background(), req); !errors.As(err, &v) {
			t.Fatalf("%+v: %v", req, err)
		}
	}
	if begin.calls != 0 {
		t.Fatal("transaction begun for an invalid request")
	}
}

func TestNotSetUp(t *testing.T) {
	open := APConfig{Section: "default_radio0", SSID: "OpenWrt", Security: "open"}
	for _, tc := range []struct {
		name string
		aps  []APConfig
		want bool
	}{
		{"factory default", []APConfig{open, {Section: "default_radio1", Security: "open"}}, true},
		{"one AP enabled", []APConfig{open, {Section: "default_radio1", Enabled: true, Security: "open"}}, false},
		{"off for the night, key kept", []APConfig{{Section: "default_radio0", Security: "protected"}}, false},
		{"only a guest AP", []APConfig{{Section: "guest", Guest: true, Security: "open"}}, false},
		{"no AP", nil, false},
	} {
		if got := NotSetUp(tc.aps); got != tc.want {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}

func TestNotSetUpOnTheFactoryConfig(t *testing.T) {
	d := mock.New()
	seedConfigs(t, d, "factory-default/uci-show-wireless.txt", nil)
	s := New(context.Background(), apUbus(t), nil).WithJoin(JoinDeps{UCI: d})
	aps, err := s.AccessPoints(context.Background())
	if err != nil || !NotSetUp(aps) {
		t.Fatalf("%+v %v", aps, err)
	}
}
