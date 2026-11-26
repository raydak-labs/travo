package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TierScan accommodates the measured ~4.7 s 5 GHz scan, unlike TierRead's 5 s ceiling.
const TierScan = 20 * time.Second

// ScanWireless performs a read-only site survey through an interface or radio name.
func (d *Device) ScanWireless(ctx context.Context, name string) (json.RawMessage, error) {
	payload, err := json.Marshal(struct {
		Device string `json:"device"`
	}{Device: name})
	if err != nil {
		return nil, err
	}
	out, err := d.run(ctx, TierScan, "ubus", "call", "iwinfo", "scan", string(payload))
	if err != nil {
		return nil, err
	}
	if !json.Valid([]byte(out)) {
		return nil, fmt.Errorf("ubus iwinfo.scan: reply is not JSON")
	}
	return json.RawMessage(out), nil
}
