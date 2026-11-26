package lanaddress

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/dhcpserver"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Inputs is everything the checks read: committed configs and netifd's live addresses.
type Inputs struct {
	Network, DHCP, Firewall []device.Section
	// Travo is the travo-greenfield config; its AdGuard topology gates the change.
	Travo []device.Section
	Live  []facts.Interface
}

// OverlapError names the interface whose prefix the new subnet overlaps.
type OverlapError struct{ Interface, Prefix string }

func (e *OverlapError) Error() string {
	return fmt.Sprintf("%v: %s (%s)", ErrOverlap, e.Interface, e.Prefix)
}
func (e *OverlapError) Unwrap() error { return ErrOverlap }

// DependentsError lists the sections that would point outside the new subnet.
type DependentsError struct{ Sections []string }

func (e *DependentsError) Error() string {
	return ErrDependents.Error() + ": " + strings.Join(e.Sections, ", ")
}
func (e *DependentsError) Unwrap() error { return ErrDependents }

// Plan validates next against in and returns the network.lan writes. Pure.
func Plan(in Inputs, next netip.Prefix) ([]Write, error) {
	cur, err := read(in.Network)
	if err != nil {
		return nil, err
	}
	if cur.prefix == next {
		return nil, txn.ErrNoChanges
	}
	// Expert mode pins the old lan address in dhcp.lan `dhcp_option 6,<addr>` and in
	// AdGuard Home's bind_hosts; moving lan under it would leave LAN clients without DNS.
	if adguard.Mode(in.Travo) == adguard.TopologyExpert {
		return nil, ErrAdGuardExpert
	}
	if err := overlap(next, in.Network, in.Live); err != nil {
		return nil, err
	}
	if err := poolFits(next, in.DHCP); err != nil {
		return nil, err
	}
	if err := dependents(next, in.DHCP, in.Firewall); err != nil {
		return nil, err
	}
	return writes(cur, next), nil
}

// overlap refuses a subnet that meets any other interface prefix, configured (ipaddr, or a
// WireGuard `addresses`) or live (an uplink lease can land anywhere). lan and loopback skip.
func overlap(next netip.Prefix, network []device.Section, live []facts.Interface) error {
	want := next.Masked()
	skip := func(name string) bool { return name == Interface || name == "loopback" }
	for _, s := range network {
		if s.Type != "interface" || skip(s.Name) {
			continue
		}
		for _, opt := range []string{"ipaddr", "addresses"} {
			for _, a := range append(strings.Fields(s.Options[opt]), s.Lists[opt]...) {
				if p, ok := configuredPrefix(a, s.Options["netmask"]); ok && p.Masked().Overlaps(want) {
					return &OverlapError{s.Name, p.Masked().String()}
				}
			}
		}
	}
	for _, i := range live {
		if skip(i.Name) {
			continue
		}
		for _, p := range i.IPv4 {
			if p.Masked().Overlaps(want) {
				return &OverlapError{i.Name, p.Masked().String()}
			}
		}
	}
	return nil
}

// poolFits keeps dnsmasq's range inside the new subnet: start and limit are offsets, so an
// enabled pool that ends past the last host would be clamped by ipcalc. A dhcp.lan Travo does
// not read (missing, foreign offsets) or a server that is off is not checked.
func poolFits(next netip.Prefix, dhcp []device.Section) error {
	s, err := dhcpserver.Read(dhcp)
	if err != nil || !s.Enabled {
		return nil
	}
	if _, err := dhcpserver.Validate(s, next); err != nil {
		return ErrPoolOutside
	}
	return nil
}

// dependents lists every section that names an IPv4 address the change would strand:
// dhcp `host` reservations and `domain` names, firewall `redirect` targets. An address
// outside the new subnet, or equal to the new router address, blocks the change.
func dependents(next netip.Prefix, dhcp, firewall []device.Section) error {
	var bad []string
	check := func(config string, s device.Section, opt string) {
		for _, v := range strings.Fields(s.Options[opt]) {
			a, err := netip.ParseAddr(v)
			if err != nil || !a.Is4() {
				continue
			}
			if !next.Masked().Contains(a) || a == next.Addr() {
				bad = append(bad, fmt.Sprintf("%s.%s (%s)", config, s.Name, a))
			}
		}
	}
	for _, s := range dhcp {
		if s.Type == "host" || s.Type == "domain" {
			check("dhcp", s, "ip")
		}
	}
	for _, s := range firewall {
		if s.Type == "redirect" {
			check("firewall", s, "dest_ip")
		}
	}
	if len(bad) > 0 {
		return &DependentsError{bad}
	}
	return nil
}

// Write is one staged change to network.lan.
type Write struct {
	Op     string // set, delete, add (list entry)
	Option string
	Value  string
}

// writes keeps the stored form: a one-element list stays a list in its place, the address is
// CIDR and a netmask option is dropped (25.12 writes CIDR, uci-show-network.txt).
func writes(cur lanState, next netip.Prefix) []Write {
	var w []Write
	if cur.list {
		w = append(w, Write{"delete", "ipaddr", ""}, Write{"add", "ipaddr", next.String()})
		w = append(w, cur.tail...)
	} else {
		w = append(w, Write{"set", "ipaddr", next.String()})
	}
	if cur.netmask {
		w = append(w, Write{"delete", "netmask", ""})
	}
	return w
}

// Stage applies the writes to network.lan.
func Stage(ctx context.Context, uci device.UCI, ws []Write) error {
	for _, w := range ws {
		var err error
		switch w.Op {
		case "set":
			err = uci.Set(ctx, "network", Interface, w.Option, w.Value)
		case "add":
			err = uci.AddToList(ctx, "network", Interface, w.Option, w.Value)
		default:
			err = uci.DeleteOption(ctx, "network", Interface, w.Option)
			if errors.Is(err, device.ErrNotFound) {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}
