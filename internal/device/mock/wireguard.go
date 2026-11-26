package mock

import (
	"context"
	"fmt"

	"github.com/raydak-labs/travo/internal/device"
)

// LatestHandshakes reports no WireGuard device in the unconfigured mock network.
func (d *Device) LatestHandshakes(context.Context, string) ([]device.PeerHandshake, error) {
	return nil, fmt.Errorf("wg0: %w", device.ErrNotFound)
}

var _ device.WireGuardReader = (*Device)(nil)
