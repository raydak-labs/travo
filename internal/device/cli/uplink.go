package cli

import (
	"context"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// ReconnectUplink runs `ifup wwan`, the recovery measured on the AXT1800 (device-facts §2).
// ifup only issues ubus calls and returns before the STA re-associates.
func (d *Device) ReconnectUplink(ctx context.Context) error {
	return d.RestartInterface(ctx, "wwan")
}

// RestartInterface runs `ifup <name>`; like ReconnectUplink it returns before the
// interface is up again. The name is passed as one argument, never through a shell.
func (d *Device) RestartInterface(ctx context.Context, name string) error {
	_, err := d.run(ctx, TierWrite, "ifup", name)
	return err
}

var (
	_ device.UplinkReconnector  = (*Device)(nil)
	_ device.InterfaceRestarter = (*Device)(nil)
)
