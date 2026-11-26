package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const (
	bootIDPath         = "/proc/sys/kernel/random/boot_id"
	rebootShellCommand = "(sleep 2; reboot) </dev/null >/dev/null 2>&1 &"
)

// Reboot schedules a detached restart and returns before the delay expires.
// The scheduling shell-out is bounded; the terminal reboot itself has no timeout tier.
func (d *Device) Reboot(ctx context.Context) error {
	_, err := d.run(ctx, TierWrite, "setsid", "sh", "-c", rebootShellCommand)
	return err
}

// BootID reads the kernel's random per-boot identifier without a shell-out.
func (d *Device) BootID(context.Context) (string, error) {
	read := d.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	raw, err := read(bootIDPath)
	if err != nil {
		return "", fmt.Errorf("read boot id: %w", err)
	}
	id := strings.TrimSpace(string(raw))
	if id == "" {
		return "", errors.New("read boot id: empty")
	}
	return id, nil
}

var _ device.Rebooter = (*Device)(nil)
var _ device.BootIDReader = (*Device)(nil)
