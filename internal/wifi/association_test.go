package wifi

import (
	"context"
	"errors"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// showUCI only reads; any write reaches the nil embedded interface and panics.
type showUCI struct {
	device.UCI
	sections []device.Section
	err      error
}

func (u showUCI) Show(context.Context, string) ([]device.Section, error) { return u.sections, u.err }

func associationService(t *testing.T, uci device.UCI, mutate func(*fakeUbus)) *Service {
	t.Helper()
	ubus := summaryUbus(t, "wifi-scan/connected/wireless-status.json")
	ubus.replies[ubusKey{"iwinfo", "info", "phy0-sta0"}] = fixture(t, "wifi-scan/connected/iwinfo-info-phy0-sta0.json")
	if mutate != nil {
		mutate(ubus)
	}
	return New(context.Background(), ubus, nil).WithJoin(JoinDeps{UCI: uci, Ubus: ubus})
}

func TestAssociatedUplink(t *testing.T) {
	connected := configFixture(t, "wireless", "wifi-scan/connected/uci-show-wireless.txt")
	withSection := func(extra device.Section) []device.Section {
		return append(configFixture(t, "wireless", "wifi-scan/connected/uci-show-wireless.txt"), extra)
	}
	disabled := configFixture(t, "wireless", "wifi-scan/connected/uci-show-wireless.txt")
	for i := range disabled {
		if disabled[i].Name == "wwan" {
			disabled[i].Options["disabled"] = "1"
		}
	}
	secondSTA := device.Section{Name: "wwan2", Type: "wifi-iface", Options: map[string]string{"mode": "sta", "network": "wwan", "device": "radio1"}}
	for _, tc := range []struct {
		name     string
		uci      showUCI
		mutate   func(*fakeUbus)
		want     Association
		wantFail bool
	}{
		{name: "connected", uci: showUCI{sections: connected}, want: Association{SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a"}},
		{name: "disabled STA", uci: showUCI{sections: disabled}},
		{name: "no STA", uci: showUCI{sections: configFixture(t, "wireless", "wifi-scan/no-sta/uci-show-wireless.txt")}},
		{name: "lost carrier", uci: showUCI{sections: connected}, mutate: func(u *fakeUbus) {
			u.replies[ubusKey{"network.device", "status", "phy0-sta0"}] = fixture(t, "wwan-down/device-status-phy0-sta0-disconnected.json")
		}},
		{name: "disconnected iwinfo keeps BSSID", uci: showUCI{sections: connected}, mutate: func(u *fakeUbus) {
			u.replies[ubusKey{"iwinfo", "info", "phy0-sta0"}] = fixture(t, "wwan-down/iwinfo-info-phy0-sta0-disconnected.json")
		}},
		{name: "two enabled STAs", uci: showUCI{sections: withSection(secondSTA)}, wantFail: true},
		{name: "UCI unreadable", uci: showUCI{err: errInjected}, wantFail: true},
		{name: "duplicate runtime section", uci: showUCI{sections: connected}, mutate: func(u *fakeUbus) {
			u.replies[ubusKey{"network.wireless", "status", ""}] = mutatedFixture(t, "wifi-scan/connected/wireless-status.json", func(p map[string]any) {
				radio1 := p["radio1"].(map[string]any)
				radio0 := p["radio0"].(map[string]any)
				radio1["interfaces"] = append(radio1["interfaces"].([]any), radio0["interfaces"].([]any)[0])
			})
		}, wantFail: true},
		{name: "missing ifname", uci: showUCI{sections: connected}, mutate: func(u *fakeUbus) {
			u.replies[ubusKey{"network.wireless", "status", ""}] = mutatedFixture(t, "wifi-scan/connected/wireless-status.json", func(p map[string]any) {
				p["radio0"].(map[string]any)["interfaces"].([]any)[0].(map[string]any)["ifname"] = ""
			})
		}, wantFail: true},
		{name: "iwinfo read error", uci: showUCI{sections: connected}, mutate: func(u *fakeUbus) {
			u.failures = map[ubusKey]error{{"iwinfo", "info", "phy0-sta0"}: errInjected}
		}, wantFail: true},
		{name: "malformed BSSID", uci: showUCI{sections: connected}, mutate: func(u *fakeUbus) {
			u.replies[ubusKey{"iwinfo", "info", "phy0-sta0"}] = mutatedFixture(t, "wifi-scan/connected/iwinfo-info-phy0-sta0.json", func(p map[string]any) {
				p["bssid"] = "not-a-mac"
			})
		}, wantFail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := associationService(t, tc.uci, tc.mutate).AssociatedUplink(context.Background())
			if tc.wantFail != errors.Is(err, ErrUnavailable) || (err != nil && !tc.wantFail) || got != tc.want {
				t.Fatalf("got %#v, %v", got, err)
			}
		})
	}
	if _, err := New(context.Background(), &fakeUbus{}, nil).AssociatedUplink(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatal("missing read dependencies must be unavailable", err)
	}
}

func TestUplinkEnabled(t *testing.T) {
	connected := configFixture(t, "wireless", "wifi-scan/connected/uci-show-wireless.txt")
	disabled := configFixture(t, "wireless", "wifi-scan/connected/uci-show-wireless.txt")
	for i := range disabled {
		if disabled[i].Name == "wwan" {
			disabled[i].Options["disabled"] = "1"
		}
	}
	two := append(configFixture(t, "wireless", "wifi-scan/connected/uci-show-wireless.txt"),
		device.Section{Name: "wwan2", Type: "wifi-iface", Options: map[string]string{"mode": "sta", "network": "wwan", "device": "radio1"}})
	for _, tc := range []struct {
		name     string
		uci      showUCI
		want     bool
		wantFail bool
	}{
		{name: "enabled STA", uci: showUCI{sections: connected}, want: true},
		{name: "disabled STA", uci: showUCI{sections: disabled}},
		{name: "two enabled STAs", uci: showUCI{sections: two}, wantFail: true},
		{name: "UCI unreadable", uci: showUCI{err: errInjected}, wantFail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := associationService(t, tc.uci, nil).UplinkEnabled(context.Background())
			if got != tc.want || tc.wantFail != errors.Is(err, ErrUnavailable) {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
}
