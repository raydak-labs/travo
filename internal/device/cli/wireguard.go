package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

var wireGuardInterface = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,14}$`)

// LatestHandshakes reads only peer public keys and handshake times, never private keys.
func (d *Device) LatestHandshakes(ctx context.Context, iface string) ([]device.PeerHandshake, error) {
	if iface == "all" || iface == "interfaces" || !wireGuardInterface.MatchString(iface) {
		return nil, fmt.Errorf("invalid WireGuard interface %q", iface)
	}
	out, err := d.run(ctx, TierRead, "wg", "show", iface, "latest-handshakes")
	if err != nil {
		if hasDiagnostic(err, "Unable to access interface: No such device") {
			return nil, fmt.Errorf("%w: %v", device.ErrNotFound, err)
		}
		return nil, err
	}
	peers := make([]device.PeerHandshake, 0)
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, rawEpoch, ok := strings.Cut(line, "\t")
		if !ok || key == "" || rawEpoch == "" {
			return nil, fmt.Errorf("wg latest-handshakes: malformed line %q", line)
		}
		epoch, err := strconv.ParseInt(rawEpoch, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("wg latest-handshakes: malformed epoch: %w", err)
		}
		var at time.Time
		if epoch != 0 {
			at = time.Unix(epoch, 0).UTC()
		}
		peers = append(peers, device.PeerHandshake{PublicKey: key, At: at})
	}
	return peers, nil
}

var _ device.WireGuardReader = (*Device)(nil)
