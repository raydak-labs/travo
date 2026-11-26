package cli

import (
	"context"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Reorder stages `uci reorder <config>.<section>=<index>`.
func (d *Device) Reorder(ctx context.Context, config, section string, index int) error {
	if index < 0 {
		return fmt.Errorf("uci reorder: negative index %d", index)
	}
	_, err := d.run(ctx, TierWrite, "uci", "reorder", fmt.Sprintf("%s.%s=%d", config, section, index))
	return err
}

var _ device.UCIReorderer = (*Device)(nil)
