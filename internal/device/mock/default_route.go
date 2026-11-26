package mock

import (
	"context"
	_ "embed"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

//go:embed testdata/device-status-phy0-sta0.json
var staDeviceStatus []byte

// SeedConnectivity scripts the connected uplink captured on the AXT1800.
func SeedConnectivity(d *Device) {
	d.DefaultRoute = true
	d.SetUbusReply("network.device", "status", staDeviceStatus)
}

// DefaultIPv4Route mirrors the captured AXT1800 default-route tuple.
func (d *Device) DefaultIPv4Route(ctx context.Context) (device.IPv4DefaultRoute, bool, error) {
	if err := ctx.Err(); err != nil {
		return device.IPv4DefaultRoute{}, false, err
	}
	found, err := d.HasDefaultRoute(ctx)
	if !found || err != nil {
		return device.IPv4DefaultRoute{}, false, err
	}
	return device.IPv4DefaultRoute{Device: "phy0-sta0", Gateway: "192.168.90.2", Source: "192.168.90.241"}, true, nil
}
