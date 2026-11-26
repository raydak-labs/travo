package mock

import (
	"context"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// RenewLease records the renew like netifd would accept it; the lease itself is unchanged.
func (d *Device) RenewLease(ctx context.Context, logical string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.RenewErr != nil {
		return d.RenewErr
	}
	d.renews = append(d.renews, logical)
	return nil
}

// Renews lists the interfaces RenewLease was called for, in order.
func (d *Device) Renews() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.renews)
}

var _ device.LeaseRenewer = (*Device)(nil)
