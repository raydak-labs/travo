package mock

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func TestScanWirelessUsesExactName(t *testing.T) {
	d := New()
	d.SetScanReply("radio0", json.RawMessage(`{"results":[]}`))
	if _, err := d.ScanWireless(context.Background(), "radio0"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"phy0", "phy1-ap0", "unknown"} {
		if _, err := d.ScanWireless(context.Background(), name); !errors.Is(err, device.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestSeedWiFiAnswersStatusAndEachScanTarget(t *testing.T) {
	d := New()
	SeedWiFi(d)
	if _, err := d.Call(context.Background(), "network.wireless", "status", nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"phy0-sta0", "phy1-ap0"} {
		if _, err := d.ScanWireless(context.Background(), name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := d.ScanWireless(context.Background(), "radio0"); !errors.Is(err, device.ErrNotFound) {
		t.Fatalf("unseeded radio0: %v", err)
	}
}
