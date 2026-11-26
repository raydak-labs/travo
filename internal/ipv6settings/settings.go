// Package ipv6settings turns IPv6 on or off for the uplinks and the LAN (#245): the wan6
// and wwan6 DHCPv6 clients and odhcpd's router advertisements and DHCPv6 on dhcp.lan.
package ipv6settings

import (
	"errors"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Section names. wan6 is the image's DHCPv6 client on the cable uplink (testdata
// ipv6/uci-show-ipv6.txt); wwan6 is Travo's on the WiFi uplink, which the image lacks.
const (
	WAN6     = "wan6"
	WWAN6    = "wwan6"
	WWAN6Dev = "@wwan"
	LAN      = "lan"
	Tunnel   = "wg0"
)

var (
	// ErrForeign: relay mode or a wwan6 section Travo did not write; LuCI owns it.
	ErrForeign = errors.New("IPv6 is configured in a way Travo does not edit; use LuCI")
	// ErrVPNLeak: the WireGuard tunnel is IPv4-only, so IPv6 would bypass it.
	ErrVPNLeak = errors.New("turn the VPN tunnel off before enabling IPv6: IPv6 traffic would bypass it")
	// ErrUnavailable: network or dhcp config cannot be read.
	ErrUnavailable = errors.New("IPv6 settings are unavailable")
)

// View is the GET body.
type View struct {
	Enabled bool `json:"enabled" doc:"odhcpd advertises IPv6 on the LAN (dhcp.lan ra=server) and the uplink DHCPv6 clients run"`
}

// Request is the PUT body.
type Request struct {
	Enabled bool `json:"enabled"`
}

// Write is one staged UCI change. Op is set, delete (option) or type (create section).
type Write struct {
	Config, Section, Op, Option, Value string
}

// Read derives the view. dhcp.lan must exist and nothing may be in relay mode.
func Read(network, dhcp []device.Section) (View, error) {
	lan, ok := find(dhcp, LAN)
	if !ok || lan.Type != "dhcp" {
		return View{}, ErrForeign
	}
	for _, opt := range []string{"ra", "dhcpv6", "ndp"} {
		if lan.Options[opt] == "relay" {
			return View{}, ErrForeign
		}
	}
	if w, ok := find(network, WWAN6); ok && !ownWWAN6(w) {
		return View{}, ErrForeign
	}
	return View{Enabled: lan.Options["ra"] == "server"}, nil
}

// ownWWAN6 is the shape Plan creates: a DHCPv6 client on the wwan alias.
func ownWWAN6(s device.Section) bool {
	return s.Type == "interface" && s.Options["proto"] == "dhcpv6" && s.Options["device"] == WWAN6Dev
}

// Plan returns the set-if-different writes for in. Empty means nothing to change.
func Plan(network, dhcp []device.Section, in Request) ([]Write, error) {
	if _, err := Read(network, dhcp); err != nil {
		return nil, err
	}
	if in.Enabled && tunnelEnabled(network) {
		return nil, ErrVPNLeak
	}
	var w []Write
	lan, _ := find(dhcp, LAN)
	mode := "disabled"
	if in.Enabled {
		mode = "server"
	}
	for _, opt := range []string{"ra", "dhcpv6"} {
		if lan.Options[opt] != mode {
			w = append(w, Write{"dhcp", LAN, "set", opt, mode})
		}
	}
	for _, name := range []string{WAN6, WWAN6} {
		s, ok := find(network, name)
		switch {
		case !ok && name == WWAN6 && in.Enabled:
			w = append(w, Write{"network", WWAN6, "type", "", "interface"},
				Write{"network", WWAN6, "set", "proto", "dhcpv6"},
				Write{"network", WWAN6, "set", "device", WWAN6Dev})
		case !ok:
		case in.Enabled && s.Options["disabled"] != "":
			w = append(w, Write{"network", name, "delete", "disabled", ""})
		case !in.Enabled && s.Options["disabled"] != "1":
			w = append(w, Write{"network", name, "set", "disabled", "1"})
		}
	}
	return w, nil
}

// tunnelEnabled: network.wg0 exists and is not disabled. Its absence is "off".
func tunnelEnabled(network []device.Section) bool {
	s, ok := find(network, Tunnel)
	return ok && s.Options["disabled"] != "1"
}

func find(secs []device.Section, name string) (device.Section, bool) {
	i := slices.IndexFunc(secs, func(s device.Section) bool { return s.Name == name })
	if i < 0 {
		return device.Section{}, false
	}
	return secs[i], true
}
