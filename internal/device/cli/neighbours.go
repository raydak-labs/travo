package cli

import (
	"context"
	"encoding/json"
)

// Neighbours reads the kernel IPv4 neighbour table (read-only).
func (d *Device) Neighbours(ctx context.Context) (json.RawMessage, error) {
	out, err := d.run(ctx, TierRead, "ip", "-j", "-4", "neigh", "show")
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}
