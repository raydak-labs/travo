package cli

import (
	"context"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// ReconnectUplink runs `ifup wwan`, the recovery measured on the AXT1800 (device-facts §2).
// ifup only issues ubus calls and returns before the STA re-associates.
func (d *Device) ReconnectUplink(ctx context.Context) error {
	_, err := d.run(ctx, TierWrite, "ifup", "wwan")
	return err
}

var _ device.UplinkReconnector = (*Device)(nil)
