package facts

import "testing"

func TestParseIwinfoInfoFromDevice(t *testing.T) {
	for _, tc := range []struct {
		name string
		want IwinfoInfo
	}{
		{"wifi-scan/connected/iwinfo-info-phy0-sta0.json", IwinfoInfo{SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a"}},
		// The disconnected STA keeps its last BSSID; only the SSID proves association.
		{"wwan-down/iwinfo-info-phy0-sta0-disconnected.json", IwinfoInfo{BSSID: "94:83:c4:1f:28:3a"}},
	} {
		got, err := ParseIwinfoInfo(read(t, tc.name))
		if err != nil || got != tc.want {
			t.Fatalf("%s: %#v %v", tc.name, got, err)
		}
	}
	for _, raw := range []string{"", "null", "{}", `{"mode":"Client","ssid":1}`} {
		if _, err := ParseIwinfoInfo([]byte(raw)); err == nil {
			t.Fatalf("%q parsed", raw)
		}
	}
}
