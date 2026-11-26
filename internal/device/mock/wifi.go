package mock

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

//go:embed testdata/wireless-status.json
var wirelessStatus []byte

//go:embed testdata/iwinfo-info-phy0-sta0.json
var staInfoPHY0 []byte

//go:embed testdata/iwinfo-scan-phy0-sta0.json
var scanPHY0STA []byte

//go:embed testdata/iwinfo-scan-phy1-ap0.json
var scanPHY1AP []byte

// WirelessUCI is the `uci show wireless` capture matching the seeded wireless status.
// The caller parses it with the CLI adapter's parser and commits it with Load.
//
//go:embed testdata/uci-show-wireless.txt
var WirelessUCI []byte

// SetScanReply scripts a scan for one exact interface or radio name.
func (d *Device) SetScanReply(name string, reply json.RawMessage) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.scanReplies[name] = reply
}

// ScanWireless returns only the reply seeded for name; ignoring the target would be a false pass.
func (d *Device) ScanWireless(ctx context.Context, name string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	reply, ok := d.scanReplies[name]
	if !ok {
		return nil, fmt.Errorf("iwinfo scan %s: %w", name, device.ErrNotFound)
	}
	return reply, nil
}

// SeedWiFi scripts the connected radio summary and one scan for each radio.
func SeedWiFi(d *Device) {
	d.mu.Lock()
	d.staLinkIndex = 2
	d.mu.Unlock()
	d.SetUbusReply("network.wireless", "status", wirelessStatus)
	d.SetScanReply("phy0-sta0", scanPHY0STA)
	d.SetUbusDeviceReply("iwinfo", "info", "phy0-sta0", staInfoPHY0)
	d.SetScanReply("phy1-ap0", scanPHY1AP)
}

func (d *Device) LinkIndices(ctx context.Context) (map[string]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	links := map[string]int{"lo": 1}
	if d.staLinkIndex > 0 {
		links["phy0-sta0"] = d.staLinkIndex
	}
	return links, nil
}

// Neighbours returns the scripted `ip -j -4 neigh show` reply; unset means an empty table,
// so every caller classifies as unknown (fail closed).
func (d *Device) Neighbours(ctx context.Context) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.neighbours == nil {
		return json.RawMessage("[]"), nil
	}
	return d.neighbours, nil
}

// SetNeighbours scripts the neighbour table (use a verbatim capture).
func (d *Device) SetNeighbours(reply json.RawMessage) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.neighbours = reply
}
