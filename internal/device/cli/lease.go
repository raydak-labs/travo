package cli

import (
	"context"
	"fmt"
	"strings"
)

// RenewLease runs netifd's renew method. Its success reply is empty, which the generic
// JSON Call would reject; any output is unexpected and reported.
func (d *Device) RenewLease(ctx context.Context, logical string) error {
	if !logicalInterface.MatchString(logical) {
		return fmt.Errorf("renew: invalid interface %q", logical)
	}
	out, err := d.run(ctx, TierWrite, "ubus", "call", "network.interface."+logical, "renew")
	if err != nil {
		return err
	}
	if s := strings.TrimSpace(out); s != "" && s != "{}" {
		return fmt.Errorf("renew %s: unexpected reply %q", logical, s)
	}
	return nil
}
