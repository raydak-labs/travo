package wifi

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

func TestPlanAP(t *testing.T) {
	connected := configFixture(t, "wireless", "uci-show-wireless.txt")
	noSTA := configFixture(t, "wireless", "wifi-scan/no-sta/uci-show-wireless.txt")
	for _, tc := range []struct {
		name     string
		wireless []device.Section
		req      APRequest
		want     []uciWrite
		err      error
	}{
		{"ssid only", connected, APRequest{Section: "default_radio1", SSID: "NewName", Enabled: true},
			[]uciWrite{{"set", "wireless", "default_radio1", "ssid", "NewName"}}, nil},
		{"unchanged", connected, APRequest{Section: "default_radio1", SSID: "TravelAP", Enabled: true}, nil, nil},
		{"same key", connected, APRequest{Section: "default_radio1", SSID: "TravelAP", Key: "REDACTED", Enabled: true}, nil, nil},
		{"new key", connected, APRequest{Section: "default_radio1", SSID: "TravelAP", Key: "example-only", Enabled: true},
			[]uciWrite{{"key", "wireless", "default_radio1", "key", ""}}, nil},
		{"disable", connected, APRequest{Section: "default_radio1", SSID: "TravelAP", Enabled: false},
			[]uciWrite{{"set", "wireless", "default_radio1", "disabled", "1"}}, nil},
		{"enable on the STA radio", connected, APRequest{Section: "default_radio0", SSID: "TravelAP", Enabled: true}, nil, ErrRadioInUse},
		{"key on an open AP", noSTA, APRequest{Section: "default_radio0", SSID: "OpenWrt", Key: "example-only", Enabled: true},
			[]uciWrite{{"set", "wireless", "default_radio0", "encryption", "psk2"}, {"key", "wireless", "default_radio0", "key", ""}, {"set", "wireless", "default_radio0", "disabled", "0"}}, nil},
		{"STA section", connected, APRequest{Section: "wifinet3", SSID: "x", Enabled: true}, nil, ErrAPNotFound},
		{"missing", connected, APRequest{Section: "nope", SSID: "x", Enabled: true}, nil, ErrAPNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, before, after, err := planAP(tc.wireless, tc.req)
			if !errors.Is(err, tc.err) || !slices.Equal(writes, tc.want) {
				t.Fatalf("got %#v %v", writes, err)
			}
			if err == nil && len(before) != len(after) {
				t.Fatal("layouts differ in length")
			}
		})
	}
}

// apUbus answers with verbatim captures: the phone 192.168.1.151 is on phy1-ap0 and the
// laptop 192.168.1.2 is wired (ADR 0011, spike S3).
func apUbus(t *testing.T) *fakeUbus {
	t.Helper()
	return &fakeUbus{replies: map[ubusKey]json.RawMessage{
		{object: "network.interface", method: "dump"}:                   fixture(t, "interface-dump.json"),
		{object: "network.wireless", method: "status"}:                  fixture(t, "with-ap-client/wireless-status.json"),
		{object: "hostapd.phy1-ap0", method: "get_clients"}:             fixture(t, "with-ap-client/hostapd-phy1-ap0-get_clients.json"),
		{object: "network.device", method: "status", name: "phy1-ap0"}:  fixture(t, "wifi-scan/connected/device-status-phy1-ap0.json"),
		{object: "iwinfo", method: "info", name: "phy1-ap0"}:            fixture(t, "wifi-scan/connected/iwinfo-info-phy1-ap0.json"),
		{object: "network.device", method: "status", name: "phy0-sta0"}: fixture(t, "wifi-scan/connected/device-status-phy0-sta0.json"),
	}}
}

type recordingBegin struct {
	calls int
	err   error
}

func (b *recordingBegin) Begin(ctx context.Context, _ string, _ []string, stage func(context.Context) error, _ txn.Probe) (txn.Status, error) {
	b.calls++
	if err := stage(ctx); err != nil {
		return txn.Status{}, err
	}
	return txn.Status{State: txn.Pending}, b.err
}

func apService(t *testing.T, ubus *fakeUbus) (*Service, *mock.Device, *recordingBegin) {
	t.Helper()
	d := mock.New()
	seedConfigs(t, d, "uci-show-wireless.txt", nil)
	d.SetNeighbours(fixture(t, "with-ap-client/ip-neigh.json"))
	begin := &recordingBegin{}
	s := New(context.Background(), ubus, nil).WithJoin(JoinDeps{UCI: d, Ubus: ubus, Keys: d, Neighbours: d, Txns: begin, ProbeBudget: 20 * time.Millisecond, ProbeEvery: time.Millisecond})
	return s, d, begin
}

func TestUpdateAPLockout(t *testing.T) {
	disable := APRequest{Section: "default_radio1", SSID: "TravelAP", Enabled: false}
	for _, tc := range []struct {
		name        string
		caller      string
		acknowledge bool
		ssid        string
		noStations  bool
		refused     bool
	}{
		{name: "ethernet", caller: "192.168.1.2"},
		{name: "phone on the AP", caller: "192.168.1.151", refused: true},
		{name: "phone acknowledges", caller: "192.168.1.151", acknowledge: true},
		{name: "unknown", caller: "192.168.1.200", refused: true},
		{name: "loopback", caller: "127.0.0.1", refused: true},
		{name: "station list unreadable", caller: "192.168.1.2", noStations: true, refused: true},
		{name: "phone renames its AP", caller: "192.168.1.151", ssid: "Renamed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ubus := apUbus(t)
			if tc.noStations {
				delete(ubus.replies, ubusKey{object: "hostapd.phy1-ap0", method: "get_clients"})
			}
			s, d, begin := apService(t, ubus)
			req := disable
			req.Caller, req.Acknowledge = netip.MustParseAddr(tc.caller), tc.acknowledge
			if tc.ssid != "" {
				req.SSID, req.Enabled = tc.ssid, true
			}
			_, err := s.UpdateAP(context.Background(), req)
			if errors.Is(err, lockout.ErrLockoutRisk) != tc.refused {
				t.Fatalf("err %v", err)
			}
			pending, _ := d.Pending(context.Background(), "wireless")
			if tc.refused && (begin.calls != 0 || pending) {
				t.Fatal("refusal wrote")
			}
			if !tc.refused && (err != nil || !pending) {
				t.Fatalf("not staged: %v", err)
			}
		})
	}
}

func TestUpdateAPRefusesTheSTARadio(t *testing.T) {
	s, d, begin := apService(t, apUbus(t))
	_, err := s.UpdateAP(context.Background(), APRequest{Section: "default_radio0", SSID: "TravelAP", Enabled: true, Caller: netip.MustParseAddr("192.168.1.2")})
	pending, _ := d.Pending(context.Background(), "wireless")
	if !errors.Is(err, ErrRadioInUse) || begin.calls != 0 || pending {
		t.Fatal(err)
	}
}

func TestAPValidation(t *testing.T) {
	for _, tc := range []struct {
		req APRequest
		err error
	}{
		{APRequest{Section: "default_radio1"}, ValidationError{"wifi_ssid_invalid"}},
		{APRequest{Section: "default_radio1", SSID: "ok", Key: "short"}, ValidationError{"wifi_key_invalid"}},
		{APRequest{Section: "../x", SSID: "ok"}, ErrAPNotFound},
	} {
		if err := tc.req.validate(); err != tc.err {
			t.Fatalf("%+v: %v", tc.req, err)
		}
	}
}

func TestAPProbeChecksEachInterface(t *testing.T) {
	enabled := []apExpectation{{section: "default_radio1", enabled: true, ssid: "TravelAP"}}
	for _, tc := range []struct {
		name   string
		expect []apExpectation
		mutate func(*fakeUbus)
		pass   bool
	}{
		{"up with the new SSID", enabled, nil, true},
		{"old SSID still live", []apExpectation{{section: "default_radio1", enabled: true, ssid: "Renamed"}}, nil, false},
		{"device status unreadable", enabled, func(u *fakeUbus) {
			delete(u.replies, ubusKey{"network.device", "status", "phy1-ap0"})
		}, false},
		{"AP link down", enabled, func(u *fakeUbus) {
			u.replies[ubusKey{"network.device", "status", "phy1-ap0"}] = json.RawMessage(`{"present":true,"up":false,"carrier":false}`)
		}, false},
		{"iwinfo unreadable", enabled, func(u *fakeUbus) { delete(u.replies, ubusKey{"iwinfo", "info", "phy1-ap0"}) }, false},
		{"disabled AP still listed", []apExpectation{{section: "default_radio1"}}, nil, false},
		{"disabled AP gone", []apExpectation{{section: "default_radio0"}}, nil, true},
		{"expected AP missing", []apExpectation{{section: "default_radio0", enabled: true}}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := apUbus(t)
			if tc.mutate != nil {
				tc.mutate(u)
			}
			s, _, _ := apService(t, u)
			if got := s.apsProven(context.Background(), tc.expect); got != tc.pass {
				t.Fatalf("proven=%v", got)
			}
		})
	}
}

func TestUpdateAPFailingProbeRollsBack(t *testing.T) {
	ctx := context.Background()
	ubus := apUbus(t)
	base := mock.New()
	seedConfigs(t, base, "uci-show-wireless.txt", nil)
	manager, configDir := fileTxnManager(t, base, func() {
		// The renamed AP never comes up.
		delete(ubus.replies, ubusKey{"network.device", "status", "phy1-ap0"})
	})
	s := New(ctx, ubus, nil).WithJoin(JoinDeps{UCI: manager.Device, Ubus: ubus, Keys: base, Neighbours: base, Txns: manager, ProbeBudget: 30 * time.Millisecond, ProbeEvery: time.Millisecond})
	st, err := s.UpdateAP(ctx, APRequest{Section: "default_radio1", SSID: "Renamed", Enabled: true})
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

func TestAccessPointsListsDisabledAPs(t *testing.T) {
	s, _, _ := apService(t, apUbus(t))
	aps, err := s.AccessPoints(context.Background())
	want := []APConfig{
		{Section: "default_radio0", Radio: "radio0", Band: "5g", SSID: "TravelAP", Enabled: false, Security: "protected"},
		{Section: "default_radio1", Radio: "radio1", Band: "2g", SSID: "TravelAP", Enabled: true, Security: "protected"},
	}
	if err != nil || !slices.Equal(aps, want) {
		t.Fatalf("%+v %v", aps, err)
	}
}
