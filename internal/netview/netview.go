// Package netview is the read-only network view: netifd interfaces with their uptime and
// addresses, and the firewall zones that hold them. Edits stay in LuCI.
package netview

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// ErrUnavailable means a required source (the interface dump or the firewall config) could not be read.
var ErrUnavailable = errors.New("network data is unavailable")

// UplinkInterfaces are the netifd interfaces that carry the uplink (GLOSSARY "Uplink"; wan6 is wan's IPv6 side).
var UplinkInterfaces = []string{"wan", "wan6", "wwan", "usbtether"}

// Interface is one netifd logical interface.
type Interface struct {
	Name          string   `json:"name" doc:"netifd interface name"`
	Proto         string   `json:"proto" doc:"netifd protocol (static, dhcp, dhcpv6, wireguard, ...)"`
	Up            bool     `json:"up"`
	UptimeSeconds int64    `json:"uptime_seconds" doc:"Seconds since the interface came up; 0 while down"`
	Device        string   `json:"device,omitempty" doc:"Kernel device carrying layer 3 (l3_device); absent while down"`
	IPv4          []string `json:"ipv4" nullable:"false" doc:"IPv4 addresses in CIDR form"`
	IPv6          []string `json:"ipv6" nullable:"false" doc:"IPv6 addresses in CIDR form, including the router's address in delegated prefixes"`
	DNSServers    []string `json:"dns_servers" nullable:"false" doc:"Resolvers the interface learned, in netifd order"`
	Uplink        bool     `json:"uplink" doc:"The interface is an uplink (wan, wan6, wwan, usbtether)"`
	Zone          string   `json:"zone,omitempty" doc:"Firewall zone whose network list holds the interface; absent for none"`
}

// Zone is one named firewall zone.
type Zone struct {
	Name       string   `json:"name"`
	Input      string   `json:"input" doc:"Policy for traffic to the router (ACCEPT, REJECT, DROP); inherited from the defaults section, then DROP, when unset"`
	Output     string   `json:"output" doc:"Policy for traffic from the router"`
	Forward    string   `json:"forward" doc:"Policy for traffic forwarded between the zone's own networks"`
	Masq       bool     `json:"masq" doc:"Masquerading (NAT) on egress"`
	Networks   []string `json:"networks" nullable:"false" doc:"netifd interfaces in the zone"`
	ForwardsTo []string `json:"forwards_to" nullable:"false" doc:"Zones this zone may forward to (forwarding sections with src = this zone)"`
}

// Listing is GET /network/interfaces: Partial is true when the firewall could not be read
// and zones are missing.
type Listing struct {
	Interfaces []Interface
	Partial    bool
}

// Service reads the view. GET never mutates.
type Service struct {
	UCI  device.UCI
	Ubus device.Ubus
}

// Interfaces reads the interface dump (required) and the firewall (optional, sets Partial).
func (s *Service) Interfaces(ctx context.Context) (Listing, error) {
	raw, err := s.Ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return Listing{}, ErrUnavailable
	}
	dump, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return Listing{}, errors.Join(ErrUnavailable, err)
	}
	zones, err := s.Zones(ctx)
	return Listing{Interfaces: Interfaces(dump, zones), Partial: err != nil}, nil
}

// Zones reads the firewall config.
func (s *Service) Zones(ctx context.Context) ([]Zone, error) {
	fw, err := s.UCI.Show(ctx, "firewall")
	if err != nil {
		return nil, ErrUnavailable
	}
	return Zones(fw), nil
}

// Interfaces joins the dump with the zones, in netifd order.
func Interfaces(dump []facts.Interface, zones []Zone) []Interface {
	out := make([]Interface, 0, len(dump))
	for _, d := range dump {
		it := Interface{Name: d.Name, Proto: d.Proto, Up: d.Up, UptimeSeconds: d.Uptime, Device: d.L3Device,
			IPv4: prefixes(d.IPv4), IPv6: prefixes(d.IPv6), DNSServers: append([]string{}, d.DNSServers...),
			Uplink: slices.Contains(UplinkInterfaces, d.Name)}
		if at := slices.IndexFunc(zones, func(z Zone) bool { return slices.Contains(z.Networks, d.Name) }); at >= 0 {
			it.Zone = zones[at].Name
		}
		out = append(out, it)
	}
	return out
}

// Zones lists zone sections by their name option, anonymous (@zone[n]) and named sections
// alike, in config order. fw4 skips a zone without a name or with enabled '0', so it does here too. An unset
// policy inherits the defaults section's, then DROP, as fw4 does.
func Zones(firewall []device.Section) []Zone {
	var defaults device.Section
	if at := slices.IndexFunc(firewall, func(s device.Section) bool { return s.Type == "defaults" }); at >= 0 {
		defaults = firewall[at]
	}
	out := []Zone{}
	for _, s := range firewall {
		name := s.Options["name"]
		if s.Type != "zone" || name == "" || s.Options["enabled"] == "0" {
			continue
		}
		out = append(out, Zone{Name: name,
			Input:    policy(s, defaults, "input"),
			Output:   policy(s, defaults, "output"),
			Forward:  policy(s, defaults, "forward"),
			Masq:     s.Options["masq"] == "1",
			Networks: values(s, "network"), ForwardsTo: forwardsTo(firewall, name)})
	}
	return out
}

func forwardsTo(firewall []device.Section, zone string) []string {
	out := []string{}
	for _, s := range firewall {
		dest := s.Options["dest"]
		if s.Type == "forwarding" && s.Options["src"] == zone && s.Options["enabled"] != "0" && dest != "" && !slices.Contains(out, dest) {
			out = append(out, dest)
		}
	}
	return out
}

func policy(zone, defaults device.Section, key string) string {
	if v := zone.Options[key]; v != "" {
		return strings.ToUpper(v)
	}
	if v := defaults.Options[key]; v != "" {
		return strings.ToUpper(v)
	}
	return "DROP" // fw4's built-in default for an unset defaults policy
}

// values reads an option that may be a list or a space-separated option (both occur in the
// same capture: `list network 'wg0'` and `option network 'guest'`).
func values(s device.Section, key string) []string {
	out := append([]string{}, s.Lists[key]...)
	return append(out, strings.Fields(s.Options[key])...)
}

func prefixes(ps []netip.Prefix) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}
