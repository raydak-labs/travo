package facts

import "testing"

func TestParseIwinfoInfoFromDevice(t *testing.T) {
	signal := func(v int) *int { return &v }
	for _, tc := range []struct {
		name string
		want IwinfoInfo
	}{
		{"wifi-scan/connected/iwinfo-info-phy0-sta0.json", IwinfoInfo{SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a", Signal: signal(-30)}},
		{"iwinfo-info-phy0-sta0.json", IwinfoInfo{SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a", Signal: signal(-63)}},
		// The disconnected STA keeps its last BSSID but reports no signal; only the SSID proves association.
		{"wwan-down/iwinfo-info-phy0-sta0-disconnected.json", IwinfoInfo{BSSID: "94:83:c4:1f:28:3a"}},
	} {
		got, err := ParseIwinfoInfo(read(t, tc.name))
		same := got.SSID == tc.want.SSID && got.BSSID == tc.want.BSSID &&
			(got.Signal == nil) == (tc.want.Signal == nil) && (got.Signal == nil || *got.Signal == *tc.want.Signal)
		if err != nil || !same {
			t.Fatalf("%s: %#v (signal %v) %v", tc.name, got, got.Signal, err)
		}
	}
	for _, raw := range []string{"", "null", "{}", `{"mode":"Client","ssid":1}`} {
		if _, err := ParseIwinfoInfo([]byte(raw)); err == nil {
			t.Fatalf("%q parsed", raw)
		}
	}
}
