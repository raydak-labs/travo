package mock

import (
	"context"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// ReconnectUplink counts the request; the mock has no radio to re-associate.
func (d *Device) ReconnectUplink(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.UplinkReconnects++
	return nil
}

// RestartInterface records the name; RestartErr makes it fail after recording.
func (d *Device) RestartInterface(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Restarts = append(d.Restarts, name)
	return d.RestartErr
}

var (
	_ device.UplinkReconnector  = (*Device)(nil)
	_ device.InterfaceRestarter = (*Device)(nil)
)
