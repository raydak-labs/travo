package wifi

import (
	"context"
	"errors"
	"maps"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// replay applies writes one by one and fails if any intermediate staged state has an
// enabled AP and an enabled STA on one radio (device-facts §3).
func replay(t *testing.T, wireless []device.Section, writes []uciWrite) map[string]map[string]string {
	t.Helper()
	state := map[string]map[string]string{}
	for _, s := range wireless {
		state[s.Name] = maps.Clone(s.Options)
		if state[s.Name] == nil {
			state[s.Name] = map[string]string{}
		}
		state[s.Name][".type"] = s.Type
	}
	for i, w := range writes {
		if w.config != "wireless" {
			continue
		}
		if state[w.section] == nil {
			state[w.section] = map[string]string{}
		}
		switch w.action {
		case "type":
			state[w.section][".type"] = w.value
		case "set":
			state[w.section][w.option] = w.value
		case "delete":
			delete(state[w.section], w.option)
		}
		modes := map[string][]string{}
		for _, o := range state {
			if o[".type"] == "wifi-iface" && o["disabled"] != "1" && o["device"] != "" {
				modes[o["device"]] = append(modes[o["device"]], o["mode"])
			}
		}
		for radio, m := range modes {
			if slices.Contains(m, "ap") && slices.Contains(m, "sta") {
				t.Fatalf("write %d %v stages AP and STA on %s", i, w, radio)
			}
		}
	}
	return state
}

func reconcileFixture(t *testing.T, dir string) ([]device.Section, []facts.Radio) {
	t.Helper()
	wireless := configFixture(t, "wireless", dir+"uci-show-wireless.txt")
	radios, err := facts.ParseWirelessRadios(fixture(t, dir+"wireless-status.json"))
	if err != nil {
		t.Fatal(err)
	}
	return wireless, radios
}

func TestReconcileMovesTheAPBeforeTheSTA(t *testing.T) {
	network := configFixture(t, "network", "uci-show-network.txt")
	firewall := configFixture(t, "firewall", "uci-show-firewall.txt")
	for _, dir := range []string{"wifi-scan/no-sta/", "wifi-scan/connected/"} {
		t.Run(dir, func(t *testing.T) {
			wireless, radios := reconcileFixture(t, dir)
			// Only radio1 (2.4 GHz) fits and it carries the AP.
			if pickRadio(radios, []string{"2g"}, wireless) != "" {
				t.Fatal("a free radio exists")
			}
			sta, target, moved := reconcileRadio(radios, []string{"2g"}, wireless)
			if sta != "radio1" || target != "radio0" || !slices.Equal(moved, []string{"default_radio1"}) {
				t.Fatalf("%s %s %v", sta, target, moved)
			}
			staWrites, _, _, err := planJoin(wireless, network, firewall, JoinRequest{SSID: "Uplink2G", Security: SecurityOpen}, sta)
			if err != nil {
				t.Fatal(err)
			}
			writes := reconcileWrites(wireless, staWrites, moved, target)
			move := slices.Index(writes, uciWrite{"set", "wireless", "default_radio1", "device", "radio0"})
			staDevice := slices.IndexFunc(writes, func(w uciWrite) bool { return w.option == "device" && w.value == "radio1" })
			if move < 0 || staDevice < move || writes[len(writes)-1].option != "disabled" || writes[len(writes)-1].value != "0" {
				t.Fatalf("order %v", writes)
			}
			state := replay(t, wireless, writes)
			if state["default_radio1"]["device"] != "radio0" || state["default_radio1"]["ssid"] != "TravelAP" {
				t.Fatal("AP not moved intact")
			}
		})
	}
}

func TestReconcileRefusesATargetWithItsOwnAP(t *testing.T) {
	wireless, radios := reconcileFixture(t, "wifi-scan/no-sta/")
	for i := range wireless {
		if wireless[i].Name == "default_radio0" {
			wireless[i].Options["disabled"] = "0"
		}
	}
	if sta, _, _ := reconcileRadio(radios, []string{"2g"}, wireless); sta != "" {
		t.Fatal("moved onto a radio with an AP of the same role")
	}
}

func TestJoinMoveLockout(t *testing.T) {
	radios, err := facts.ParseWirelessRadios(fixture(t, "with-ap-client/wireless-status.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		caller  string
		ack     bool
		refused bool
	}{
		{"192.168.1.2", false, false},  // Ethernet laptop
		{"192.168.1.151", false, true}, // phone on the AP that moves
		{"192.168.1.151", true, false}, // acknowledged
		{"192.168.1.200", false, true}, // unknown fails closed
	} {
		s, _, _ := apService(t, apUbus(t))
		plan, err := s.prepareJoin(context.Background(), JoinRequest{SSID: "Uplink2G", Security: SecurityOpen, Caller: netip.MustParseAddr(tc.caller), Acknowledge: tc.ack}, radios, []string{"2g"})
		if errors.Is(err, lockout.ErrLockoutRisk) != tc.refused {
			t.Fatalf("%s: %v", tc.caller, err)
		}
		if !tc.refused && (plan.radio != "radio1" || !slices.Contains(plan.writes, uciWrite{"set", "wireless", "default_radio1", "device", "radio0"})) {
			t.Fatalf("%s: plan %+v", tc.caller, plan)
		}
	}
}

func TestJoinWithMoveRollsBackWhenTheAPStaysDown(t *testing.T) {
	ctx := context.Background()
	service, _ := noSTAScanService(t)
	var req JoinRequest
	scan, err := service.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range scan.Networks {
		if slices.Equal(n.Bands, []string{"2g"}) && (n.Security == string(SecurityOpen) || n.Security == string(SecurityWPA2)) {
			req = JoinRequest{SSID: n.SSID, Security: Security(n.Security), Acknowledge: true}
			if req.Security == SecurityWPA2 {
				req.Key = "example-only"
			}
			break
		}
	}
	if req.SSID == "" {
		t.Fatal("capture has no 2.4 GHz-only network")
	}
	base := mock.New()
	seedJoinDevice(t, base)
	ubus := service.ubus.(*fakeUbus)
	// The STA comes up on radio1, but the moved AP never appears on radio0.
	manager, configDir := fileTxnManager(t, base, func() {})
	service.WithJoin(JoinDeps{UCI: manager.Device, Ubus: ubus, Routes: joinRoute{"phy1-sta0"}, Keys: base, Neighbours: base,
		Links: fixedJoinLinks(3), Txns: manager, ProbeBudget: 30 * time.Millisecond, ProbeEvery: time.Millisecond})
	st, err := service.Join(ctx, req)
	if !errors.Is(err, txn.ErrProbeFailed) || st.State != txn.RolledBack {
		t.Fatal(st, err)
	}
	for _, config := range []string{"wireless", "network", "firewall"} {
		content, err := os.ReadFile(filepath.Join(configDir, config))
		if err != nil || string(content) != "original "+config {
			t.Fatalf("%s not restored: %v", config, err)
		}
	}
}
