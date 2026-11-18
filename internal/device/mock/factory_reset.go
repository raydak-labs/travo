package mock

import "context"

// FactoryReset records the request; mock mode never wipes anything.
func (d *Device) FactoryReset(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.FactoryResets++
	return d.FactoryResetErr
}
