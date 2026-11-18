package cli

import "context"

// ReadSystemLog returns the whole `logread` ring buffer (RAM, read-only).
func (d *Device) ReadSystemLog(ctx context.Context) (string, error) {
	return d.run(ctx, TierRead, "logread")
}
