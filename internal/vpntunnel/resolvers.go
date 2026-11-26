package vpntunnel

import (
	"net/netip"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// ResolversOption is the Travo-owned option on network.wg0 holding the applied profile's
// `DNS =` resolvers, space-separated, or noResolvers (#289). Enable stages it in the tunnel
// transaction, so rollback restores it with wg0 and selecting another profile changes
// nothing until it is applied. netifd ignores the unknown option (procd container, 25.12.6).
const ResolversOption = "travo_dns"

const noResolvers = "none"

func resolversValue(dns []netip.Addr) string {
	if len(dns) == 0 {
		return noResolvers
	}
	out := make([]string, 0, len(dns))
	for _, a := range dns {
		out = append(out, a.String())
	}
	return strings.Join(out, " ")
}

// recordedResolvers reads ResolversOption from network.wg0. recorded is false when there
// is no wg0 or it was applied before #289; a malformed value or a foreign wg0 section is
// errUnavailable.
func recordedResolvers(network []device.Section) (resolvers []netip.Addr, recorded bool, err error) {
	for _, s := range network {
		if s.Name != iface {
			continue
		}
		if s.Type != "interface" || s.Options["proto"] != "wireguard" || len(s.Lists[ResolversOption]) != 0 {
			return nil, false, errUnavailable
		}
		raw, ok := s.Options[ResolversOption]
		switch {
		case !ok:
			return nil, false, nil
		case raw == noResolvers:
			return nil, true, nil
		}
		for _, v := range strings.Fields(raw) {
			a, perr := netip.ParseAddr(v)
			if perr != nil || a.Zone() != "" {
				return nil, false, errUnavailable
			}
			resolvers = append(resolvers, a)
		}
		if len(resolvers) == 0 {
			return nil, false, errUnavailable
		}
		return resolvers, true, nil
	}
	return nil, false, nil
}

// TunnelResolvers are the `DNS =` resolvers the configured wg0 runs with: the recorded ones,
// else the selected profile's for a wg0 applied before #289. vpnprofiles.ErrNotFound means
// neither exists.
func TunnelResolvers(network []device.Section, selected ProfileStore) ([]netip.Addr, error) {
	dns, recorded, err := recordedResolvers(network)
	if err != nil || recorded {
		return dns, err
	}
	cfg, err := selected.SelectedConfig()
	return cfg.DNS, err
}
