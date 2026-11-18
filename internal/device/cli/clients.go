package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// LeaseFile is dnsmasq's lease file (`dhcp.@dnsmasq[0].leasefile` on the AXT1800).
const LeaseFile = "/tmp/dhcp.leases"

var (
	netdevName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,14}$`)
	colonMAC   = regexp.MustCompile(`^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`)
)

// DHCPLeases reads the lease file; /tmp is RAM, so an absent file (no lease yet) is empty.
func (d *Device) DHCPLeases(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	read := d.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	b, err := read(LeaseFile)
	if errors.Is(err, os.ErrNotExist) {
		return []byte{}, nil
	}
	return b, err
}

// KickStation runs hostapd's del_client with deauth and no ban: the station is disconnected
// once and may rejoin. hostapd answers UBUS_STATUS_NOT_FOUND for an unknown station.
func (d *Device) KickStation(ctx context.Context, ifname, mac string) error {
	if !netdevName.MatchString(ifname) || !colonMAC.MatchString(mac) {
		return fmt.Errorf("kick: invalid interface %q or MAC", ifname)
	}
	args, err := json.Marshal(map[string]any{"addr": mac, "reason": 5, "deauth": true, "ban_time": 0})
	if err != nil {
		return err
	}
	out, err := d.run(ctx, TierWrite, "ubus", "call", "hostapd."+ifname, "del_client", string(args))
	if err != nil {
		if hasDiagnostic(err, "Command failed: Not found") {
			return fmt.Errorf("kick %s: %w", ifname, device.ErrNotFound)
		}
		return err
	}
	if s := strings.TrimSpace(out); s != "" && s != "{}" {
		return fmt.Errorf("kick %s: unexpected reply %q", ifname, s)
	}
	return nil
}
