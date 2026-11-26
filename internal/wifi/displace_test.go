package wifi

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/raydak-labs/travo/internal/device"
	"github.com/raydak-labs/travo/internal/device/facts"
	"github.com/raydak-labs/travo/internal/device/mock"
	"github.com/raydak-labs/travo/internal/lockout"
)

// dualAPWireless derives a post-ADR-0034 UCI config (both radios carrying an enabled
// main AP with DefaultSSID) from the verbatim factory-default wireless capture.
// UCI sections are router configuration, not device status payloads.
func dualAPWireless(t *testing.T) []device.Section {
	t.Helper()
	sections := configFixture(t, "wireless", "factory-default/uci-show-wireless.txt")
	for i := range sections {
		if isAP(sections[i]) {
			sections[i].Options["ssid"] = DefaultSSID
			sections[i].Options["encryption"] = SetupEncryption
			sections[i].Options["key"] = "REDACTED"
			sections[i].Options["disabled"] = "0"
		}
	}
	return sections
}

// dualAPWirelessStatus derives a dual-AP network.wireless status payload from the
// verbatim wifi-scan/no-sta/wireless-status.json capture by cloning radio1's AP
// entry onto radio0 (default_radio0 / phy0-ap0). No payload fields are invented.
func dualAPWirelessStatus(t *testing.T) json.RawMessage {
	t.Helper()
	var doc map[string]map[string]any
	if err := json.Unmarshal(fixture(t, "wifi-scan/no-sta/wireless-status.json"), &doc); err != nil {
		t.Fatal(err)
	}
	r1AP := doc["radio1"]["interfaces"].([]any)[0].(map[string]any)
	r1AP["config"].(map[string]any)["ssid"] = DefaultSSID
	rawAP, err := json.Marshal(r1AP)
	if err != nil {
		t.Fatal(err)
	}
	var r0AP map[string]any
	if err := json.Unmarshal(rawAP, &r0AP); err != nil {
		t.Fatal(err)
	}
	r0AP["section"], r0AP["ifname"] = "default_radio0", "phy0-ap0"
	r0AP["config"].(map[string]any)["device"] = []string{"radio0"}
	doc["radio0"]["interfaces"] = []any{r0AP}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func cloneSections(in []device.Section) []device.Section {
	out := make([]device.Section, len(in))
	for i, s := range in {
		s.Options = maps.Clone(s.Options)
		if s.Lists != nil {
			s.Lists = make(map[string][]string, len(s.Lists))
			for k, v := range in[i].Lists {
				s.Lists[k] = slices.Clone(v)
			}
		}
		out[i] = s
	}
	return out
}

func withOption(in []device.Section, section, key, value string) []device.Section {
	out := cloneSections(in)
	for i := range out {
		if out[i].Name == section {
			out[i].Options[key] = value
		}
	}
	return out
}

func withGuestAP(in []device.Section, name, radio, ssid string) []device.Section {
	return append(cloneSections(in), device.Section{
		Name: name, Type: "wifi-iface",
		Options: map[string]string{"device": radio, "mode": "ap", "network": "guest", "ssid": ssid, "disabled": "0"},
	})
}

func seedWirelessSections(t *testing.T, d *mock.Device, wireless []device.Section) {
	t.Helper()
	ctx := context.Background()
	seedConfigs(t, d, "factory-default/uci-show-wireless.txt", func(device.Section) bool { return true })
	for _, s := range wireless {
		if err := d.SetType(ctx, "wireless", s.Name, s.Type); err != nil {
			t.Fatal(err)
		}
		for k, v := range s.Options {
			if err := d.Set(ctx, "wireless", s.Name, k, v); err != nil {
				t.Fatal(err)
			}
		}
		for k, vs := range s.Lists {
			for _, v := range vs {
				if err := d.AddToList(ctx, "wireless", s.Name, k, v); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := d.Commit(ctx, "wireless"); err != nil {
		t.Fatal(err)
	}
}

func seedDualAPService(t *testing.T, wireless []device.Section, callerOnRadio0 bool) (*Service, *mock.Device, *fakeUbus, []facts.Radio) {
	t.Helper()
	ctx := context.Background()
	statusRaw := dualAPWirelessStatus(t)
	radios, err := facts.ParseWirelessRadios(statusRaw)
	if err != nil {
		t.Fatal(err)
	}
	// Verbatim captures: phone 192.168.1.151 is associated on the chosen AP;
	// laptop 192.168.1.2 is wired Ethernet.
	withPhone := fixture(t, "with-ap-client/hostapd-phy1-ap0-get_clients.json")
	emptyAP := fixture(t, "hostapd-phy1-ap0-get_clients.json")
	r0Clients, r1Clients := emptyAP, withPhone
	if callerOnRadio0 {
		r0Clients, r1Clients = withPhone, emptyAP
	}
	ubus := &fakeUbus{replies: map[ubusKey]json.RawMessage{
		{object: "network.interface", method: "dump"}:                  fixture(t, "interface-dump.json"),
		{object: "network.wireless", method: "status"}:                 statusRaw,
		{object: "hostapd.phy0-ap0", method: "get_clients"}:            r0Clients,
		{object: "hostapd.phy1-ap0", method: "get_clients"}:            r1Clients,
		{object: "network.device", method: "status", name: "phy0-ap0"}: fixture(t, "wifi-scan/connected/device-status-phy1-ap0.json"),
		{object: "network.device", method: "status", name: "phy1-ap0"}: fixture(t, "wifi-scan/connected/device-status-phy1-ap0.json"),
	}}
	d := mock.New()
	seedWirelessSections(t, d, wireless)
	d.SetNeighbours(fixture(t, "with-ap-client/ip-neigh.json"))
	s := New(ctx, ubus, nil).WithJoin(JoinDeps{
		UCI: d, Ubus: ubus, Keys: d, Neighbours: d, Links: fixedJoinLinks(2),
		ProbeBudget: 30 * time.Millisecond, ProbeEvery: time.Millisecond,
	})
	return s, d, ubus, radios
}

func TestDisplaceRadioSelection(t *testing.T) {
	base := dualAPWireless(t)
	radios, err := facts.ParseWirelessRadios(dualAPWirelessStatus(t))
	if err != nil {
		t.Fatal(err)
	}
	radio1LiveOff := slices.Clone(radios)
	radio1LiveOff[1].Disabled = true
	guestBlocked := withGuestAP(withGuestAP(base, "guest0", "radio0", "Guest5G"), "guest1", "radio1", "Guest2G")
	guestMovable := withGuestAP(base, "guest", "radio0", "TravoGuest")
	// radio1 has a different-SSID main AP (blocks move) plus a guest AP with DefaultSSID (wrong role for a twin).
	roleMismatch := withGuestAP(withOption(base, "default_radio1", "ssid", "OtherMain"), "guest1", "radio1", DefaultSSID)
	eth := lockout.Caller{Method: lockout.Ethernet}
	// Three radios, only radio0 band-compatible. radio0's guest "Shared" has a twin on
	// radio2, guest "Unique" has none; radio1 has room for one guest AP. Unique must get
	// that room, so Shared is disabled instead of moved.
	threeRadios := append(slices.Clone(radios), facts.Radio{Name: "radio2", Band: "6g", Up: true})
	twinBeforeUnique := withGuestAP(withGuestAP(withGuestAP(append(cloneSections(base), device.Section{
		Name: "main2", Type: "wifi-iface",
		Options: map[string]string{"device": "radio2", "mode": "ap", "network": "lan", "ssid": DefaultSSID, "disabled": "0"},
	}), "shared0", "radio0", "Shared"), "unique0", "radio0", "Unique"), "shared2", "radio2", "Shared")

	for _, tc := range []struct {
		name         string
		radios       []facts.Radio
		bands        []string
		wireless     []device.Section
		caller       lockout.Caller
		wantRadio    string
		wantDisabled []string
		wantMoves    []apMove
	}{
		{"dual-band caller on radio0 spares radio0", radios, []string{"5g", "2g"}, base, lockout.Caller{Method: lockout.WifiAP, AP: "phy0-ap0"}, "radio1", []string{"default_radio1"}, nil},
		{"dual-band caller on radio1 spares radio1", radios, []string{"5g", "2g"}, base, lockout.Caller{Method: lockout.WifiAP, AP: "phy1-ap0"}, "radio0", []string{"default_radio0"}, nil},
		{"dual-band ethernet picks radio0", radios, []string{"5g", "2g"}, base, eth, "radio0", []string{"default_radio0"}, nil},
		{"dual-band unknown picks radio0", radios, []string{"5g", "2g"}, base, lockout.Caller{Method: lockout.Unknown}, "radio0", []string{"default_radio0"}, nil},
		{"5g-only caller on radio0 displaces radio0", radios, []string{"5g"}, base, lockout.Caller{Method: lockout.WifiAP, AP: "phy0-ap0"}, "radio0", []string{"default_radio0"}, nil},
		{"twin different SSID refused", radios, []string{"5g", "2g"}, withOption(base, "default_radio1", "ssid", "OtherSSID"), eth, "", nil, nil},
		{"twin on UCI-disabled radio refused", radios, []string{"5g", "2g"}, withOption(base, "radio1", "disabled", "1"), eth, "", nil, nil},
		{"twin on live-disabled radio refused", radio1LiveOff, []string{"5g", "2g"}, base, eth, "", nil, nil},
		{"guest AP with no twin and no move target refused", radios, []string{"5g", "2g"}, guestBlocked, eth, "", nil, nil},
		{"guest AP on R moves while main twin disabled", radios, []string{"5g"}, guestMovable, eth, "radio0", []string{"default_radio0"}, []apMove{{section: "guest", target: "radio1"}}},
		{"AP without twin gets the move target first", threeRadios, []string{"5g"}, twinBeforeUnique, eth, "radio0", []string{"default_radio0", "shared0"}, []apMove{{section: "unique0", target: "radio1"}}},
		{"role mismatch main vs guest is not a twin", radios, []string{"5g"}, roleMismatch, eth, "", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := displaceRadio(tc.radios, tc.bands, tc.wireless, tc.caller)
			if got.radio != tc.wantRadio || !slices.Equal(got.disabled, tc.wantDisabled) || !slices.Equal(got.moves, tc.wantMoves) {
				t.Fatalf("got %+v, want radio=%q disabled=%v moves=%v", got, tc.wantRadio, tc.wantDisabled, tc.wantMoves)
			}
		})
	}
}

func TestPrepareJoinDisplace(t *testing.T) {
	base := dualAPWireless(t)
	guestBlocked := withGuestAP(withGuestAP(base, "guest0", "radio0", "Guest5G"), "guest1", "radio1", "Guest2G")
	guestOnlyTwins := withOption(withOption(base, "default_radio0", "network", "guest"), "default_radio1", "network", "guest")

	for _, tc := range []struct {
		name         string
		wireless     []device.Section
		callerIP     string
		acknowledge  bool
		bands        []string
		wantErr      error
		wantRadio    string
		wantDisabled string
		wantTwin     string
	}{
		{"dual-band caller on radio0 AP displaces radio1", base, "192.168.1.151", false, []string{"5g", "2g"}, nil, "radio1", "default_radio1", "default_radio0"},
		{"dual-band ethernet displaces radio0", base, "192.168.1.2", false, []string{"5g", "2g"}, nil, "radio0", "default_radio0", "default_radio1"},
		{"5g-only caller on radio0 AP displaces radio0 without lockout", base, "192.168.1.151", false, []string{"5g"}, nil, "radio0", "default_radio0", "default_radio1"},
		{"different SSID returns ErrNoFreeRadio", withOption(base, "default_radio1", "ssid", "OtherSSID"), "192.168.1.2", false, []string{"5g", "2g"}, ErrNoFreeRadio, "", "", ""},
		{"twin on disabled radio returns ErrNoFreeRadio", withOption(base, "radio1", "disabled", "1"), "192.168.1.2", false, []string{"5g", "2g"}, ErrNoFreeRadio, "", "", ""},
		{"guest AP blocked returns ErrNoFreeRadio", guestBlocked, "192.168.1.2", false, []string{"5g", "2g"}, ErrNoFreeRadio, "", "", ""},
		{"guest-only displace without main AP triggers lockout", guestOnlyTwins, "192.168.1.151", false, []string{"5g"}, lockout.ErrLockoutRisk, "", "", ""},
		{"guest-only displace acknowledged succeeds", guestOnlyTwins, "192.168.1.151", true, []string{"5g"}, nil, "radio0", "default_radio0", "default_radio1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, d, _, radios := seedDualAPService(t, tc.wireless, true)
			req := JoinRequest{SSID: "HotelWiFi", Security: SecurityOpen, Caller: netip.MustParseAddr(tc.callerIP), Acknowledge: tc.acknowledge}
			plan, err := s.prepareJoin(context.Background(), req, radios, tc.bands)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got err %v, want %v", err, tc.wantErr)
				}
				if pending, _ := d.Pending(context.Background(), "wireless"); pending {
					t.Fatal("refusal staged writes")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			assertDisplacePlan(t, tc.wireless, plan, tc.wantRadio, tc.wantDisabled, tc.wantTwin)
		})
	}
}

func assertDisplacePlan(t *testing.T, wireless []device.Section, plan joinPlan, wantRadio, wantDisabled, wantTwin string) {
	t.Helper()
	if plan.radio != wantRadio {
		t.Fatalf("radio=%q, want %q", plan.radio, wantRadio)
	}
	disableIdx := slices.Index(plan.writes, uciWrite{"set", "wireless", wantDisabled, "disabled", "1"})
	staDeviceIdx := slices.IndexFunc(plan.writes, func(w uciWrite) bool {
		return w.config == "wireless" && w.section == "wwan" && w.option == "device" && w.value == wantRadio
	})
	last := plan.writes[len(plan.writes)-1]
	if disableIdx < 0 || staDeviceIdx < disableIdx || last != (uciWrite{"set", "wireless", "wwan", "disabled", "0"}) {
		t.Fatalf("unexpected write order: %#v", plan.writes)
	}
	state := replay(t, wireless, plan.writes)
	if state[wantDisabled]["disabled"] != "1" || state[wantTwin]["disabled"] == "1" {
		t.Fatalf("AP states wrong: disabled=%v twin=%v", state[wantDisabled], state[wantTwin])
	}
	if state["wwan"]["device"] != wantRadio || state["wwan"]["disabled"] != "0" {
		t.Fatalf("STA state wrong: %v", state["wwan"])
	}
	wantExpect := []apExpectation{
		{section: "default_radio0", enabled: wantTwin == "default_radio0"},
		{section: "default_radio1", enabled: wantTwin == "default_radio1"},
	}
	if !slices.Equal(plan.expect, wantExpect) {
		t.Fatalf("expect=%+v, want %+v", plan.expect, wantExpect)
	}
}

func TestDisplaceWritesDisablesExistingSTABeforeAP(t *testing.T) {
	network := configFixture(t, "network", "uci-show-network.txt")
	firewall := configFixture(t, "firewall", "uci-show-firewall.txt")
	wireless := append(dualAPWireless(t), device.Section{
		Name: "wwan", Type: "wifi-iface",
		Options: map[string]string{"device": "radio0", "mode": "sta", "network": "wwan", "ssid": "Old", "encryption": "none"},
	})
	staWrites, _, _, err := planJoin(wireless, network, firewall, JoinRequest{SSID: "New", Security: SecurityOpen}, "radio1")
	if err != nil {
		t.Fatal(err)
	}
	writes := displaceWrites(wireless, staWrites, displacePlan{radio: "radio1", disabled: []string{"default_radio1"}})
	staOff := slices.Index(writes, uciWrite{"set", "wireless", "wwan", "disabled", "1"})
	apOff := slices.Index(writes, uciWrite{"set", "wireless", "default_radio1", "disabled", "1"})
	staDev := slices.Index(writes, uciWrite{"set", "wireless", "wwan", "device", "radio1"})
	if staOff != 0 || apOff < staOff || staDev < apOff || writes[len(writes)-1] != (uciWrite{"set", "wireless", "wwan", "disabled", "0"}) {
		t.Fatalf("wrong staging order: %#v", writes)
	}
	state := replay(t, wireless, writes)
	if state["default_radio1"]["disabled"] != "1" || state["wwan"]["device"] != "radio1" || state["wwan"]["disabled"] != "0" {
		t.Fatalf("unexpected final state: %v", state)
	}
}

func TestDisplaceProbeExpectations(t *testing.T) {
	expect := []apExpectation{
		{section: "default_radio0", enabled: false},
		{section: "default_radio1", enabled: true},
	}
	for _, tc := range []struct {
		name   string
		mutate func(*fakeUbus)
		pass   bool
	}{
		{
			// Verbatim connected capture: default_radio0 absent, default_radio1 (phy1-ap0) up.
			name: "displaced AP gone and twin up",
			mutate: func(u *fakeUbus) {
				u.replies[ubusKey{"network.wireless", "status", ""}] = fixture(t, "wifi-scan/connected/wireless-status.json")
			},
			pass: true,
		},
		{name: "displaced AP still listed", pass: false},
		{
			name: "displaced AP gone but twin down",
			mutate: func(u *fakeUbus) {
				u.replies[ubusKey{"network.wireless", "status", ""}] = fixture(t, "wifi-scan/connected/wireless-status.json")
				delete(u.replies, ubusKey{"network.device", "status", "phy1-ap0"})
			},
			pass: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, ubus, _ := seedDualAPService(t, dualAPWireless(t), true)
			if tc.mutate != nil {
				tc.mutate(ubus)
			}
			if got := s.apsProven(context.Background(), expect); got != tc.pass {
				t.Fatalf("apsProven=%v, want %v", got, tc.pass)
			}
		})
	}
}
