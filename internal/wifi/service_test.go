package wifi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const fixtureRoot = "../../testdata/device/axt1800-25.12.6"

var errInjected = errors.New("injected")

type ubusKey struct {
	object, method, name string
}

type fakeUbus struct {
	replies  map[ubusKey]json.RawMessage
	failures map[ubusKey]error
	calls    []ubusKey
}

func (f *fakeUbus) Call(_ context.Context, object, method string, args any) (json.RawMessage, error) {
	key := ubusKey{object: object, method: method}
	if args != nil {
		fields, ok := args.(map[string]string)
		if !ok {
			return nil, fmt.Errorf("unexpected args %#v", args)
		}
		key.name = fields["name"]
		if key.name == "" {
			key.name = fields["device"]
		}
	}
	f.calls = append(f.calls, key)
	if err := f.failures[key]; err != nil {
		return nil, err
	}
	if reply, ok := f.replies[key]; ok {
		return reply, nil
	}
	return nil, device.ErrNotFound
}

func fixture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtureRoot, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mutatedFixture(t *testing.T, name string, mutate func(map[string]any)) json.RawMessage {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(fixture(t, name), &payload); err != nil {
		t.Fatal(err)
	}
	mutate(payload)
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func summaryUbus(t *testing.T, wireless string) *fakeUbus {
	t.Helper()
	return &fakeUbus{replies: map[ubusKey]json.RawMessage{
		{object: "network.wireless", method: "status"}:                  fixture(t, wireless),
		{object: "network.device", method: "status", name: "phy0-sta0"}: fixture(t, "wifi-scan/connected/device-status-phy0-sta0.json"),
		{object: "network.device", method: "status", name: "phy1-ap0"}:  fixture(t, "wifi-scan/connected/device-status-phy1-ap0.json"),
	}}
}

func boolPtr(v bool) *bool { return &v }

func TestSummaryConnected(t *testing.T) {
	u := summaryUbus(t, "wifi-scan/connected/wireless-status.json")
	got, err := New(context.Background(), u, nil).Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Summary{
		Radios: []RadioInfo{{Name: "radio0", Band: "5g", Up: true}, {Name: "radio1", Band: "2g", Up: true}},
		STAs:   []STA{{Section: "wwan", Radio: "radio0", SSID: "HomeUplink", Ifname: "phy0-sta0", Associated: boolPtr(true)}},
		APs:    []AP{{Section: "default_radio1", Radio: "radio1", SSID: "TravelAP", Ifname: "phy1-ap0", Up: boolPtr(true)}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
	wantCalls := []ubusKey{
		{object: "network.wireless", method: "status"},
		{object: "network.device", method: "status", name: "phy0-sta0"},
		{object: "network.device", method: "status", name: "phy1-ap0"},
	}
	if !reflect.DeepEqual(u.calls, wantCalls) {
		t.Fatalf("read calls %v, want %v", u.calls, wantCalls)
	}
}

func TestSummaryWithoutSTAKeepsInterfaceLessRadio(t *testing.T) {
	u := summaryUbus(t, "wifi-scan/no-sta/wireless-status.json")
	got, err := New(context.Background(), u, nil).Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Radios) != 2 || got.Radios[0].Name != "radio0" {
		t.Fatalf("radios: %+v", got.Radios)
	}
	if got.STAs == nil || len(got.STAs) != 0 {
		t.Fatalf("STAs must be []: %#v", got.STAs)
	}
	if len(got.APs) != 1 || got.APs[0].Up == nil || !*got.APs[0].Up {
		t.Fatalf("AP: %+v", got.APs)
	}
}

func TestSummaryInterfaceFailuresAreIndependent(t *testing.T) {
	u := summaryUbus(t, "wifi-scan/connected/wireless-status.json")
	u.replies[ubusKey{object: "network.device", method: "status", name: "phy0-sta0"}] = fixture(t, "wwan-down/device-status-phy0-sta0-disconnected.json")
	u.failures = map[ubusKey]error{{object: "network.device", method: "status", name: "phy1-ap0"}: errInjected}
	got, err := New(context.Background(), u, nil).Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.STAs) != 1 || got.STAs[0].Associated == nil || *got.STAs[0].Associated {
		t.Fatalf("STA: %+v", got.STAs)
	}
	if len(got.APs) != 1 || got.APs[0].Up != nil {
		t.Fatalf("AP read failure must be null, not down or absent: %+v", got.APs)
	}
}

func TestSummaryUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
		reply   json.RawMessage
	}{
		{name: "call", failure: errInjected},
		{name: "payload", reply: json.RawMessage("not JSON")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := ubusKey{object: "network.wireless", method: "status"}
			u := &fakeUbus{replies: map[ubusKey]json.RawMessage{key: tc.reply}, failures: map[ubusKey]error{key: tc.failure}}
			_, err := New(context.Background(), u, nil).Summary(context.Background())
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("%v", err)
			}
		})
	}
}
