package cli

import "context"

// factoryResetShellCommand wipes the overlay and restarts after the HTTP reply flushed.
// On 25.12.6 /sbin/firstboot is a wrapper around /sbin/factoryreset (device-facts §10).
const factoryResetShellCommand = "(sleep 2; firstboot -y && reboot) </dev/null >/dev/null 2>&1 &"

// FactoryReset schedules the detached wipe. Scheduling is bounded; the wipe and reboot
// are terminal operations without a timeout tier (invariants §1).
func (d *Device) FactoryReset(ctx context.Context) error {
	_, err := d.run(ctx, TierWrite, "setsid", "sh", "-c", factoryResetShellCommand)
	return err
}
