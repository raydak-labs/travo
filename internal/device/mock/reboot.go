package mock

import (
	"context"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// SetBootID sets the value returned by BootID.
func (d *Device) SetBootID(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bootID = id
}

// BootID returns the mock kernel boot id.
func (d *Device) BootID(context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.BootIDErr != nil {
		return "", d.BootIDErr
	}
	return d.bootID, nil
}

// Reboot records a scheduled restart and optionally rotates the mock boot id.
func (d *Device) Reboot(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Reboots++
	if d.RebootErr != nil {
		return d.RebootErr
	}
	if d.RotateBootOnReboot {
		d.bootSequence++
		next := fmt.Sprintf("mock-boot-%d", d.bootSequence)
		if next == d.bootID {
			d.bootSequence++
			next = fmt.Sprintf("mock-boot-%d", d.bootSequence)
		}
		d.bootID = next
	}
	return nil
}

var _ device.Rebooter = (*Device)(nil)
var _ device.BootIDReader = (*Device)(nil)
