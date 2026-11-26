package cli

import "context"

// sysupgrade uses the same /tmp/sysupgrade.conffiles for every backup.
var backupSlot = make(chan struct{}, 1)

func (d *Device) BackupConfig(ctx context.Context, destination string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case backupSlot <- struct{}{}:
		defer func() { <-backupSlot }()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := d.run(ctx, TierWrite, "sysupgrade", "-b", destination)
	return err
}
