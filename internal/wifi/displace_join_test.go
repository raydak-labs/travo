package wifi

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/raydak-labs/travo/internal/device/mock"
	"github.com/raydak-labs/travo/internal/txn"
)

// connectedWithDisplacedAP is the verbatim connected capture (STA phy0-sta0 on radio0,
// default_radio1 on radio1) with the displaced default_radio0 AP still listed on radio0,
// copied from dualAPWirelessStatus: the state a displace leaves when the AP never stops.
func connectedWithDisplacedAP(t *testing.T) json.RawMessage {
	t.Helper()
	var connected, dual map[string]map[string]any
	if err := json.Unmarshal(fixture(t, "wifi-scan/connected/wireless-status.json"), &connected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(dualAPWirelessStatus(t), &dual); err != nil {
		t.Fatal(err)
	}
	r0 := connected["radio0"]
	r0["interfaces"] = append(r0["interfaces"].([]any), dual["radio0"]["interfaces"].([]any)...)
	out, err := json.Marshal(connected)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The STA is fully proven (fresh link, carrier, default route) in both cases, so the
// displaced AP's expectation alone decides between Pending and rollback.
func TestJoinDisplaceProbeGatesOnDisplacedAP(t *testing.T) {
	for _, tc := range []struct {
		name      string
		after     func(*testing.T) json.RawMessage
		wantState txn.State
		wantErr   error
	}{
		{"displaced AP gone", func(t *testing.T) json.RawMessage { return fixture(t, "wifi-scan/connected/wireless-status.json") }, txn.Pending, nil},
		{"displaced AP still listed", connectedWithDisplacedAP, txn.RolledBack, txn.ErrProbeFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			service, scanner := noSTAScanService(t)
			scanner.replies["phy0-ap0"] = fixture(t, "wifi-scan/no-sta/iwinfo-scan-radio0.json")
			ubus := service.ubus.(*fakeUbus)
			ubus.replies[ubusKey{"network.wireless", "status", ""}] = dualAPWirelessStatus(t)
			ubus.replies[ubusKey{"network.interface", "dump", ""}] = fixture(t, "interface-dump.json")
			ubus.replies[ubusKey{"hostapd.phy0-ap0", "get_clients", ""}] = fixture(t, "hostapd-phy1-ap0-get_clients.json")
			ubus.replies[ubusKey{"hostapd.phy1-ap0", "get_clients", ""}] = fixture(t, "hostapd-phy1-ap0-get_clients.json")
			ubus.replies[ubusKey{"network.device", "status", "phy0-ap0"}] = fixture(t, "wifi-scan/connected/device-status-phy1-ap0.json")
			ubus.replies[ubusKey{"network.device", "status", "phy1-ap0"}] = fixture(t, "wifi-scan/connected/device-status-phy1-ap0.json")
			ubus.replies[ubusKey{"network.device", "status", "phy0-sta0"}] = fixture(t, "wifi-scan/connected/device-status-phy0-sta0.json")

			base := mock.New()
			seedWirelessSections(t, base, dualAPWireless(t))
			base.SetNeighbours(fixture(t, "with-ap-client/ip-neigh.json"))
			ssid := joinFixtureSSID(t, service)
			committed := false
			manager, configDir := fileTxnManager(t, base, func() {
				committed = true
				ubus.replies[ubusKey{"network.wireless", "status", ""}] = tc.after(t)
			})
			// No STA link before the commit; a fresh phy0-sta0 index after it.
			links := joinLinks(func() (map[string]int, error) {
				if committed {
					return map[string]int{"lo": 1, "phy0-sta0": 7}, nil
				}
				return map[string]int{"lo": 1}, nil
			})
			service.WithJoin(JoinDeps{
				UCI: manager.Device, Ubus: ubus, Routes: joinRoute{"phy0-sta0"}, Keys: base, Neighbours: base,
				Links: links, Txns: manager, ProbeBudget: 50 * time.Millisecond, ProbeEvery: time.Millisecond,
			})
			// Ethernet caller (verbatim capture laptop): the STA takes radio0.
			st, err := service.Join(ctx, JoinRequest{SSID: ssid, Security: SecurityWPA2, Key: "example-only", Caller: netip.MustParseAddr("192.168.1.2")})
			if st.State != tc.wantState || !errors.Is(err, tc.wantErr) {
				t.Fatalf("got state=%s err=%v, want %s/%v", st.State, err, tc.wantState, tc.wantErr)
			}
			if tc.wantState == txn.Pending {
				if _, err := manager.Rollback(st.ID); err != nil {
					t.Fatal(err)
				}
			}
			for _, config := range []string{"wireless", "network", "firewall"} {
				content, err := os.ReadFile(filepath.Join(configDir, config))
				if err != nil || string(content) != "original "+config {
					t.Fatalf("%s not restored: %v", config, err)
				}
			}
		})
	}
}
