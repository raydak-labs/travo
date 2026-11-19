package cli

import (
	"context"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func (d *Device) MWAN3Interfaces(ctx context.Context) (string, error) {
	return d.run(ctx, TierRead, "mwan3", "interfaces")
}

func (d *Device) MangleRules(ctx context.Context) (string, error) {
	return d.run(ctx, TierRead, "iptables", "-t", "mangle", "-w", "-S")
}

var _ device.MWAN3Reader = (*Device)(nil)
