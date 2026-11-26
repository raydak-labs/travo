package mock

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// The STA capture answers only the device it was taken on, so the AP broadcast check
// (internal/wifi/ap.go apBroadcasts, iwinfo info on phy1-ap0) still sees no reply.
func TestIwinfoInfoAnswersOnlyTheSTA(t *testing.T) {
	d := New()
	SeedWiFi(d)
	ctx := context.Background()
	got, err := d.Call(ctx, "iwinfo", "info", map[string]string{"device": "phy0-sta0"})
	if err != nil || !bytes.Equal(got, staInfoPHY0) {
		t.Fatalf("phy0-sta0: %v, %s", err, got)
	}
	if _, err := d.Call(ctx, "iwinfo", "info", map[string]string{"device": "phy1-ap0"}); !errors.Is(err, device.ErrNotFound) {
		t.Fatalf("phy1-ap0: err = %v, want ErrNotFound", err)
	}
}
