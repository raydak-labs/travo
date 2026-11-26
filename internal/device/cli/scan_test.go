package cli

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestScanWirelessUsesScanTimeoutAndExactTarget(t *testing.T) {
	var calls []call
	var remaining time.Duration
	d := &Device{Run: func(ctx context.Context, name string, args ...string) (string, int, error) {
		calls = append(calls, call{name, strings.Join(args, " ")})
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("scan without deadline")
		}
		remaining = time.Until(deadline)
		return `{"results":[]}`, 0, nil
	}}
	if _, err := d.ScanWireless(context.Background(), "phy1-ap0"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].name != "ubus" || calls[0].args != `call iwinfo scan {"device":"phy1-ap0"}` {
		t.Fatalf("calls: %+v", calls)
	}
	if remaining <= 5*time.Second || remaining > TierScan || TierScan != 20*time.Second {
		t.Fatalf("deadline remaining %v, TierScan %v", remaining, TierScan)
	}
}

func TestScanWirelessRejectsNonJSON(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, map[string]string{`ubus call iwinfo scan {"device":"radio0"}`: "not JSON"}, "")}
	if _, err := d.ScanWireless(context.Background(), "radio0"); err == nil {
		t.Fatal("accepted non-JSON reply")
	}
}
