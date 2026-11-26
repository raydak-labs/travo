package mock

import (
	"context"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Reorder records the request: the mock keeps no section order (Show sorts by name), so
// tests assert on Reorders; real ordering is covered by the container test.
func (d *Device) Reorder(_ context.Context, config, section string, index int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	c := d.view(config)
	if c == nil {
		return fmt.Errorf("%s: %w", config, device.ErrNotFound)
	}
	if _, ok := c.sections[section]; !ok {
		return fmt.Errorf("%s.%s: %w", config, section, device.ErrNotFound)
	}
	d.Reorders = append(d.Reorders, fmt.Sprintf("%s.%s=%d", config, section, index))
	return nil
}

var _ device.UCIReorderer = (*Device)(nil)
