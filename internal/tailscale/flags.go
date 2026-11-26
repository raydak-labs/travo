package tailscale

import (
	"net/netip"
	"strconv"
	"strings"
)

// Desired is the complete set of tailscaled preferences Travo owns. Every write passes all
// of them, so `tailscale up --reset` and `tailscale set` leave the same prefs behind.
type Desired struct {
	ExitNodeIP      string
	AdvertiseRoutes []string
	SSH             bool
}

// Flags is pure: the desired prefs in, the CLI flags out. `--accept-dns=false` is always
// present (invariants §11): MagicDNS reaches LAN clients through the dnsmasq layer instead.
// LAN access stays on while an exit node is used so the router keeps reaching its own LAN.
func Flags(d Desired) []string {
	return []string{
		"--accept-dns=false",
		"--exit-node=" + d.ExitNodeIP,
		"--exit-node-allow-lan-access=" + strconv.FormatBool(d.ExitNodeIP != ""),
		"--advertise-routes=" + strings.Join(d.AdvertiseRoutes, ","),
		"--ssh=" + strconv.FormatBool(d.SSH),
	}
}

// CurrentDesired reads the prefs Travo owns back out of a parsed backend, resolving the
// exit node's stable ID to the IPv4 address the CLI takes. An unresolvable ID drops the
// exit node rather than guessing.
func CurrentDesired(b Backend) Desired {
	d := Desired{AdvertiseRoutes: b.Prefs.AdvertiseRoutes, SSH: b.Prefs.SSH}
	ref := b.Prefs.ExitNode
	if _, err := netip.ParseAddr(ref); err == nil {
		d.ExitNodeIP = ref
		return d
	}
	for _, n := range b.ExitNodes {
		if n.ID == ref && ref != "" {
			d.ExitNodeIP, _ = n.IPv4()
		}
	}
	return d
}

// LoginArgs are the `tailscale up` arguments for a login: --reset so tailscaled never
// refuses with "requires mentioning all non-default flags" (up-revert.txt), then Travo's full prefs.
func LoginArgs(d Desired) []string {
	return append([]string{"--reset"}, Flags(d)...)
}
