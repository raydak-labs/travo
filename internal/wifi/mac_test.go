package wifi

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

func TestPlanMAC(t *testing.T) {
	sta, ok := enabledUplink(configFixture(t, "wireless", "uci-show-wireless.txt"))
	if !ok || sta.Name != "wifinet3" {
		t.Fatalf("uplink %+v", sta)
	}
	custom := sta
	custom.Options = map[string]string{"macaddr": "9e:6c:d4:9d:05:d0"}
	neighbours, err := facts.ParseNeighJSON(fixture(t, "with-ap-client/ip-neigh.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		sta  device.Section
		req  MACRequest
		want []uciWrite
		err  error
	}{
		{"default unchanged", sta, MACRequest{Mode: MACDefault}, nil, nil},
		{"default from custom", custom, MACRequest{Mode: MACDefault}, []uciWrite{{"delete", "wireless", "wifinet3", "macaddr", ""}}, nil},
		{"random", sta, MACRequest{Mode: MACRandom}, []uciWrite{{"set", "wireless", "wifinet3", "macaddr", "random"}}, nil},
		{"custom upper case", sta, MACRequest{Mode: MACCustom, MAC: "02:AA:BB:CC:DD:EE"}, []uciWrite{{"set", "wireless", "wifinet3", "macaddr", "02:aa:bb:cc:dd:ee"}}, nil},
		{"custom same", custom, MACRequest{Mode: MACCustom, MAC: "9e:6c:d4:9d:05:d0"}, nil, nil},
		{"multicast", sta, MACRequest{Mode: MACCustom, MAC: "01:00:5e:00:00:01"}, nil, ValidationError{"wifi_mac_invalid"}},
		{"all zero", sta, MACRequest{Mode: MACCustom, MAC: "00:00:00:00:00:00"}, nil, ValidationError{"wifi_mac_invalid"}},
		{"clone the wired laptop", sta, MACRequest{Mode: MACClone, Caller: netip.MustParseAddr("192.168.1.2")}, []uciWrite{{"set", "wireless", "wifinet3", "macaddr", "9c:eb:e8:d3:f8:d1"}}, nil},
		{"clone without neighbour", sta, MACRequest{Mode: MACClone, Caller: netip.MustParseAddr("192.168.1.200")}, nil, ErrMACCloneUnavailable},
		{"unknown mode", sta, MACRequest{Mode: "other"}, nil, ValidationError{"wifi_mac_mode_invalid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, _, err := planMAC(tc.sta, tc.req, neighbours)
			if err != tc.err || !slices.Equal(writes, tc.want) {
				t.Fatalf("%#v %v", writes, err)
			}
		})
	}
	if _, ok := enabledUplink(configFixture(t, "wireless", "wifi-scan/no-sta/uci-show-wireless.txt")); ok {
		t.Fatal("disabled STA counted as uplink")
	}
}

func TestMACCheck(t *testing.T) {
	factory := "94:83:c4:1f:28:3a" // device-status-phy0-sta0.json
	laa := "9e:6c:d4:9d:05:d0"     // uci-show-wireless.txt sta0.macaddr
	for _, tc := range []struct {
		value, live string
		want        bool
	}{
		{"", factory, true}, {"", laa, false},
		{"random", laa, true}, {"random", factory, false},
		{laa, laa, true}, {laa, factory, false},
	} {
		if got := macCheck(tc.value)(tc.live); got != tc.want {
			t.Errorf("%q on %s: %v", tc.value, tc.live, got)
		}
	}
}

func macService(t *testing.T, links joinLinks) (*Service, *mock.Device, *fakeUbus) {
	t.Helper()
	ubus := apUbus(t)
	ubus.replies[ubusKey{"network.device", "status", "phy0-sta0"}] = fixture(t, "device-status-phy0-sta0.json")
	s, d, _ := apService(t, ubus)
	s.join.Links, s.join.Routes = links, joinRoute{"phy0-sta0"}
	return s, d, ubus
}

func TestMACProbe(t *testing.T) {
	s, _, _ := macService(t, fixedJoinLinks(3))
	old := map[int]struct{}{1: {}, 2: {}}
	if !s.macProven(context.Background(), "wifinet3", macCheck(""), old) {
		t.Fatal("factory MAC on a fresh link not proven")
	}
	if s.macProven(context.Background(), "wifinet3", macCheck("random"), old) {
		t.Fatal("factory MAC proved random")
	}
	if s.macProven(context.Background(), "wifinet3", macCheck(""), map[int]struct{}{3: {}}) {
		t.Fatal("stale link proved the policy")
	}
}

func TestUplinkMACReportsEffective(t *testing.T) {
	s, _, _ := macService(t, fixedJoinLinks(3))
	got, err := s.UplinkMAC(context.Background())
	if err != nil || got.Mode != MACDefault || got.Effective == nil || *got.Effective != "94:83:c4:1f:28:3a" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestSetUplinkMACFailingProbeRollsBack(t *testing.T) {
	ctx := context.Background()
	ubus := apUbus(t)
	ubus.replies[ubusKey{"network.device", "status", "phy0-sta0"}] = fixture(t, "device-status-phy0-sta0.json")
	base := mock.New()
	seedConfigs(t, base, "uci-show-wireless.txt", nil)
	manager, configDir := fileTxnManager(t, base, func() {})
	// The STA keeps its factory MAC, so random is never proven.
	s := New(ctx, ubus, nil).WithJoin(JoinDeps{UCI: manager.Device, Ubus: ubus, Keys: base, Neighbours: base, Links: fixedJoinLinks(3),
		Routes: joinRoute{"phy0-sta0"}, Txns: manager, ProbeBudget: 30 * time.Millisecond, ProbeEvery: time.Millisecond})
	st, err := s.SetUplinkMAC(ctx, MACRequest{Mode: MACRandom})
	if !errors.Is(err, txn.ErrProbeFailed) || st.State != txn.RolledBack {
		t.Fatal(st, err)
	}
	content, err := os.ReadFile(filepath.Join(configDir, "wireless"))
	if err != nil || string(content) != "original wireless" {
		t.Fatal("wireless not restored")
	}
}

func TestSetUplinkMACWithoutUplink(t *testing.T) {
	s, d, _ := macService(t, fixedJoinLinks(3))
	if err := d.Set(context.Background(), "wireless", "wifinet3", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetUplinkMAC(context.Background(), MACRequest{Mode: MACRandom}); !errors.Is(err, ErrNoUplink) {
		t.Fatal(err)
	}
}
