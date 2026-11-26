package facts

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseWirelessRadiosFromDevice(t *testing.T) {
	for _, tc := range []struct {
		name string
		want []Radio
	}{
		{
			name: "connected",
			want: []Radio{
				{Name: "radio0", Band: "5g", Up: true, Ifaces: []WirelessIface{{Section: "wwan", Ifname: "phy0-sta0", Mode: "sta", SSID: "HomeUplink"}}},
				{Name: "radio1", Band: "2g", Up: true, Ifaces: []WirelessIface{{Section: "default_radio1", Ifname: "phy1-ap0", Mode: "ap", SSID: "TravelAP"}}},
			},
		},
		{
			name: "no-sta",
			want: []Radio{
				{Name: "radio0", Band: "5g", Up: true, Ifaces: []WirelessIface{}},
				{Name: "radio1", Band: "2g", Up: true, Ifaces: []WirelessIface{{Section: "default_radio1", Ifname: "phy1-ap0", Mode: "ap", SSID: "TravelAP"}}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseWirelessRadios(read(t, "wifi-scan/"+tc.name+"/wireless-status.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v\nwant %#v", got, tc.want)
			}
		})
	}
}

func TestParseWirelessRadiosRequiresRadioFlags(t *testing.T) {
	for _, key := range []string{"up", "disabled"} {
		t.Run(key, func(t *testing.T) {
			var payload map[string]any
			if err := json.Unmarshal(read(t, "wifi-scan/connected/wireless-status.json"), &payload); err != nil {
				t.Fatal(err)
			}
			delete(payload["radio0"].(map[string]any), key)
			b, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseWirelessRadios(b); !errors.Is(err, ErrShape) {
				t.Fatalf("missing %s: %v", key, err)
			}
		})
	}
}

func TestParseIwinfoScanFromDevice(t *testing.T) {
	for _, tc := range []struct {
		name          string
		total, hidden int
	}{
		{"wifi-scan/connected/iwinfo-scan-phy0-sta0.json", 8, 3},
		{"wifi-scan/no-sta/iwinfo-scan-phy0.json", 8, 3},
		{"wifi-scan/no-sta/iwinfo-scan-radio0.json", 8, 3},
		{"wifi-scan/no-sta/iwinfo-scan-phy1.json", 5, 1},
		{"wifi-scan/no-sta/iwinfo-scan-phy1-ap0.json", 6, 1},
		{"wwan-down/iwinfo-scan-phy0-sta0.json", 8, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			aps, err := ParseIwinfoScan(read(t, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			hidden := 0
			for _, ap := range aps {
				if ap.SSID == "" {
					hidden++
				}
				if ap.BSSID != strings.ToLower(ap.BSSID) {
					t.Errorf("BSSID not lower-cased: %q", ap.BSSID)
				}
			}
			if len(aps) != tc.total || hidden != tc.hidden {
				t.Fatalf("got total/hidden %d/%d, want %d/%d", len(aps), hidden, tc.total, tc.hidden)
			}
		})
	}
}

func TestParseIwinfoScanRejectsUnreadablePayload(t *testing.T) {
	if _, err := ParseIwinfoScan([]byte(`{}`)); !errors.Is(err, ErrShape) {
		t.Fatalf("missing results: %v", err)
	}
	for _, key := range []string{"bssid", "band", "channel", "mhz", "signal", "quality", "quality_max", "enabled"} {
		t.Run(key, func(t *testing.T) {
			var payload map[string]any
			if err := json.Unmarshal(read(t, "wifi-scan/connected/iwinfo-scan-phy0-sta0.json"), &payload); err != nil {
				t.Fatal(err)
			}
			entry := payload["results"].([]any)[0].(map[string]any)
			if key == "enabled" {
				delete(entry["encryption"].(map[string]any), key)
			} else {
				delete(entry, key)
			}
			b, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseIwinfoScan(b); !errors.Is(err, ErrShape) {
				t.Fatalf("missing %s: %v", key, err)
			}
		})
	}
}

func TestParseIwinfoAPSSIDFromDevice(t *testing.T) {
	ssid, err := ParseIwinfoAPSSID(read(t, "iwinfo-info-phy1-ap0.json"))
	if err != nil || ssid != "TravelAP" {
		t.Fatalf("%q %v", ssid, err)
	}
	// A STA reply never proves an AP.
	if _, err := ParseIwinfoAPSSID(read(t, "iwinfo-info-phy0-sta0.json")); !errors.Is(err, ErrShape) {
		t.Fatal(err)
	}
	if _, err := ParseIwinfoAPSSID([]byte(`{"mode":"Master"}`)); !errors.Is(err, ErrShape) {
		t.Fatal(err)
	}
}
