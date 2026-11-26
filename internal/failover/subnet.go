package failover

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// ErrSameSubnet: two chosen uplinks sit in one IPv4 subnet (Ethernet and WiFi to the same
// home router). On the AXT1800 the backup's tracking then fails together with the primary,
// so failover would leave no uplink (SPIKES.md "Failover wan+wwan same subnet").
var ErrSameSubnet = errors.New("two uplinks are in the same IPv4 subnet; failover cannot keep the backup tracked when the first one fails")

// SharedSubnet returns the first two uplinks, in list order, whose IPv4 prefixes overlap.
func SharedSubnet(uplinks []string, addrs map[string][]netip.Prefix) (string, string, bool) {
	for i, a := range uplinks {
		for _, b := range uplinks[i+1:] {
			for _, p := range addrs[a] {
				for _, q := range addrs[b] {
					if p.Masked().Overlaps(q.Masked()) {
						return a, b, true
					}
				}
			}
		}
	}
	return "", "", false
}

// refuseSharedSubnet reads netifd's IPv4 prefixes of the chosen uplinks; a down uplink has
// none and passes here (the probe needs it online anyway). A failed read is an error.
func (s *Service) refuseSharedSubnet(ctx context.Context, uplinks []string) error {
	addrs := map[string][]netip.Prefix{}
	for _, u := range uplinks {
		raw, err := s.Ubus.Call(ctx, "network.interface."+u, "status", nil)
		if errors.Is(err, device.ErrNotFound) {
			continue // not configured (no usbtether yet): nothing to compare
		}
		if err != nil {
			return fmt.Errorf("%s status: %w", u, err)
		}
		st, err := facts.ParseInterfaceStatus(raw)
		if err != nil {
			return fmt.Errorf("%s status: %w", u, err)
		}
		addrs[u] = st.IPv4
	}
	if a, b, ok := SharedSubnet(uplinks, addrs); ok {
		return fmt.Errorf("%w: %s and %s", ErrSameSubnet, a, b)
	}
	return nil
}
