package mock

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// The embed cannot reach outside the package, so the copies must not drift from the captures.
func TestSeedCopiesMatchCaptures(t *testing.T) {
	for name, capture := range map[string]string{
		"system-board.json":          "system-board.json",
		"system-info.json":           "system-info.json",
		"net-counters.txt":           "wwan-down/net-counters.txt",
		"wireless-status.json":       "wifi-scan/connected/wireless-status.json",
		"iwinfo-scan-phy0-sta0.json": "wifi-scan/connected/iwinfo-scan-phy0-sta0.json",
		"iwinfo-scan-phy1-ap0.json":  "wifi-scan/no-sta/iwinfo-scan-phy1-ap0.json",
		"uci-export-dhcp.txt":        "uci-export-dhcp.txt",
		"uci-show-wireless.txt":      "wifi-scan/connected/uci-show-wireless.txt",
	} {
		orig, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/" + capture)
		if err != nil {
			t.Fatal(err)
		}
		cp, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(orig, cp) {
			t.Errorf("%s: mock copy differs from testdata/device/axt1800-25.12.6", name)
		}
	}
}

func TestSeedSystemAnswersUbus(t *testing.T) {
	d := New()
	SeedSystem(d)
	for _, m := range []string{"board", "info"} {
		if _, err := d.Call(context.Background(), "system", m, nil); err != nil {
			t.Errorf("system %s: %v", m, err)
		}
	}
}
