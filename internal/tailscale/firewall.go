package tailscale

import (
	"errors"
	"maps"
	"net/netip"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Firewall sections Travo owns by name (docs/adr/0017). tailscaled has no netifd interface,
// so the zone holds the kernel device instead of a network.
const (
	ZoneName   = "travo_ts"
	LANForward = "travo_ts_lan" // lan -> tailnet: LAN clients reach peers and the exit node
	InForward  = "travo_ts_in"  // tailnet -> lan: only while the LAN prefix is advertised
	TunDevice  = "tailscale0"
	lanZone    = "lan"
)

// ErrFirewallConflict means a section Travo would own exists with another type, or another
// section already uses the zone name. Nothing is written.
var ErrFirewallConflict = errors.New("firewall layout conflicts with the Tailscale zone")

// ErrNoLAN means the lan interface has no readable IPv4 prefix to advertise.
var ErrNoLAN = errors.New("no IPv4 prefix on the lan interface")

func owned(name string) bool { return name == ZoneName || name == LANForward || name == InForward }

// FirewallSections is pure: the owned sections Tailscale needs. The zone matches the wan
// pattern (masq, mtu_fix) but rejects input, so the admin UI is never reachable from the
// tailnet (invariants §3); Tailscale SSH is answered inside tailscaled, not by the router.
func FirewallSections(advertiseLAN bool) []device.Section {
	out := []device.Section{
		{Name: ZoneName, Type: "zone", Options: map[string]string{
			"name": ZoneName, "input": "REJECT", "output": "ACCEPT", "forward": "REJECT", "masq": "1", "mtu_fix": "1",
		}, Lists: map[string][]string{"device": {TunDevice}}},
		forwarding(LANForward, lanZone, ZoneName),
	}
	if advertiseLAN {
		out = append(out, forwarding(InForward, ZoneName, lanZone))
	}
	return out
}

func forwarding(name, src, dest string) device.Section {
	return device.Section{Name: name, Type: "forwarding", Options: map[string]string{"src": src, "dest": dest}, Lists: map[string][]string{}}
}

// CheckFirewall refuses layouts Travo must not touch: a foreign type on an owned name, a
// zone named travo_ts in a foreign section, or no lan zone to forward from.
func CheckFirewall(current []device.Section) error {
	hasLAN := false
	for _, s := range current {
		if s.Type == "zone" && s.Options["name"] == lanZone {
			hasLAN = true
		}
		if s.Type == "zone" && s.Options["name"] == ZoneName && s.Name != ZoneName {
			return ErrFirewallConflict
		}
		if (s.Name == ZoneName && s.Type != "zone") || ((s.Name == LANForward || s.Name == InForward) && s.Type != "forwarding") {
			return ErrFirewallConflict
		}
	}
	if !hasLAN {
		return ErrFirewallConflict
	}
	return nil
}

// FirewallInSync reports whether the owned sections in current are exactly want.
func FirewallInSync(current, want []device.Section) bool {
	var have []device.Section
	for _, s := range current {
		if owned(s.Name) {
			have = append(have, s)
		}
	}
	if len(have) != len(want) {
		return false
	}
	for _, w := range want {
		i := slices.IndexFunc(have, func(s device.Section) bool { return s.Name == w.Name })
		if i < 0 || have[i].Type != w.Type || !maps.Equal(have[i].Options, w.Options) ||
			!maps.EqualFunc(nonEmpty(have[i].Lists), nonEmpty(w.Lists), slices.Equal[[]string]) {
			return false
		}
	}
	return true
}

func nonEmpty(lists map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range lists {
		if len(v) > 0 {
			out[k] = v
		}
	}
	return out
}

// LANPrefix is pure: `uci show network` sections in, the masked IPv4 prefix of the `lan`
// interface out. 25.12 writes CIDR (`ipaddr='192.168.1.1/24'`, uci-show-network.txt);
// older configs use `ipaddr` + `netmask`.
func LANPrefix(network []device.Section) (netip.Prefix, error) {
	i := slices.IndexFunc(network, func(s device.Section) bool { return s.Name == "lan" && s.Type == "interface" })
	if i < 0 {
		return netip.Prefix{}, ErrNoLAN
	}
	lan := network[i]
	addrs := lan.Lists["ipaddr"]
	if v := lan.Options["ipaddr"]; v != "" {
		addrs = append([]string{v}, addrs...)
	}
	for _, a := range addrs {
		if p, err := parseLAN(a, lan.Options["netmask"]); err == nil {
			return p, nil
		}
	}
	return netip.Prefix{}, ErrNoLAN
}

func parseLAN(addr, netmask string) (netip.Prefix, error) {
	if strings.Contains(addr, "/") {
		p, err := netip.ParsePrefix(addr)
		if err != nil || !p.Addr().Is4() {
			return netip.Prefix{}, ErrNoLAN
		}
		return p.Masked(), nil
	}
	ip, err := netip.ParseAddr(addr)
	mask, merr := netip.ParseAddr(netmask)
	if err != nil || merr != nil || !ip.Is4() || !mask.Is4() {
		return netip.Prefix{}, ErrNoLAN
	}
	bits, ok := maskBits(mask.As4())
	if !ok {
		return netip.Prefix{}, ErrNoLAN
	}
	return netip.PrefixFrom(ip, bits).Masked(), nil
}

func maskBits(m [4]byte) (int, bool) {
	v := uint32(m[0])<<24 | uint32(m[1])<<16 | uint32(m[2])<<8 | uint32(m[3])
	bits := 0
	for v&0x80000000 != 0 {
		bits++
		v <<= 1
	}
	return bits, v == 0
}
