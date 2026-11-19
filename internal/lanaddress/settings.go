// Package lanaddress changes the router's own lan IPv4 address (network.lan ipaddr) in a
// transaction the user confirms from the NEW address (ADR 0028).
package lanaddress

import (
	"errors"
	"net/netip"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Interface is the only network section Travo edits here.
const Interface = "lan"

// Prefix bounds for the new lan subnet: /24 is OpenWrt's default; /30 still leaves the
// router and one client.
const (
	MinBits = 24
	MaxBits = 30
)

var (
	// ErrInvalid wraps every FieldError.
	ErrInvalid = errors.New("invalid lan address")
	// ErrForeign: network.lan is not a static interface with one IPv4 address; LuCI owns it.
	ErrForeign = errors.New("network.lan is configured in a way Travo does not edit")
	// ErrOverlap: the new subnet overlaps another interface (guest, wan, wwan, VPN, ...).
	ErrOverlap = errors.New("the new lan subnet overlaps another interface")
	// ErrPoolOutside: the DHCP pool (dhcp.lan start + limit) does not fit the new subnet.
	ErrPoolOutside = errors.New("the DHCP pool does not fit the new lan subnet")
	// ErrDependents: reservations, DNS names or port forwards point outside the new subnet.
	ErrDependents = errors.New("settings still point at addresses outside the new lan subnet")
	// ErrAdGuardExpert: the AdGuard expert topology binds DNS to the current lan address
	// (ADR 0021); switch back to the default topology before moving lan.
	ErrAdGuardExpert = errors.New("switch AdGuard Home back to the default DNS topology before changing the lan address")
)

// FieldError is a validation failure on one request field.
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

// Request is PUT /network/lan.
type Request struct {
	Address string `json:"address" maxLength:"18" doc:"New router lan address in CIDR form, e.g. 192.168.8.1/24 (private IPv4, /24 to /30)"`
}

// View is GET /network/lan.
type View struct {
	Address string `json:"address" doc:"Router lan address in CIDR form (committed network.lan ipaddr)"`
	Subnet  string `json:"subnet" doc:"The lan subnet"`
	// Pending is set while a lan change waits for the confirm from its new address.
	Pending *Pending `json:"pending,omitempty" doc:"A lan change waiting for the confirm from the new address"`
}

// Pending lets the page opened on the new address find the transaction to confirm.
type Pending struct {
	ID               string `json:"id" doc:"Transaction to confirm or roll back"`
	Address          string `json:"address" doc:"New lan address in CIDR form"`
	RemainingSeconds int    `json:"remaining_seconds" doc:"Seconds left in the confirm window"`
}

// lanState is network.lan as Travo reads it.
type lanState struct {
	prefix  netip.Prefix // unmasked: the router's address and the subnet length
	list    bool         // ipaddr is a one-element list (25.12 default, wan/uci-export-network.txt)
	netmask bool
}

// read takes network.lan: proto static with exactly one IPv4 address, CIDR or ipaddr + netmask.
func read(network []device.Section) (lanState, error) {
	at := slices.IndexFunc(network, func(s device.Section) bool { return s.Name == Interface && s.Type == "interface" })
	if at < 0 {
		return lanState{}, ErrForeign
	}
	s := network[at]
	if s.Options["proto"] != "static" {
		return lanState{}, ErrForeign
	}
	addrs := s.Lists["ipaddr"]
	list := len(addrs) > 0
	if v := s.Options["ipaddr"]; v != "" {
		addrs = append(addrs, v)
	}
	if len(addrs) != 1 {
		return lanState{}, ErrForeign
	}
	_, netmask := s.Options["netmask"]
	p, ok := configuredPrefix(addrs[0], s.Options["netmask"])
	if !ok || !p.Addr().Is4() {
		return lanState{}, ErrForeign
	}
	return lanState{prefix: p, list: list, netmask: netmask}, nil
}

// Validate parses the request: a private IPv4 host address of a /24 to /30 subnet.
func Validate(in Request) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(in.Address))
	if err != nil || !p.Addr().Is4() {
		return netip.Prefix{}, &FieldError{"address", "must be an IPv4 address in CIDR form, e.g. 192.168.8.1/24"}
	}
	if p.Bits() < MinBits || p.Bits() > MaxBits {
		return netip.Prefix{}, &FieldError{"address", "the prefix must be /24 to /30"}
	}
	if !p.Addr().IsPrivate() {
		return netip.Prefix{}, &FieldError{"address", "must be a private address (10/8, 172.16/12 or 192.168/16)"}
	}
	if !usableHost(p, p.Addr()) {
		return netip.Prefix{}, &FieldError{"address", "must be a host address of its subnet, not the network or broadcast address"}
	}
	return p, nil
}

// usableHost: a in p, neither the network nor the broadcast address.
func usableHost(p netip.Prefix, a netip.Addr) bool {
	if !p.Contains(a) || a == p.Masked().Addr() {
		return false
	}
	n := u32(p.Masked().Addr()) | (1<<(32-p.Bits()) - 1)
	return u32(a) != n
}

func u32(a netip.Addr) uint32 {
	b := a.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// configuredPrefix reads `ipaddr` in CIDR form or with a dotted `netmask` (no mask = /32).
func configuredPrefix(addr, netmask string) (netip.Prefix, bool) {
	if p, err := netip.ParsePrefix(addr); err == nil {
		return p, true
	}
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return netip.Prefix{}, false
	}
	if netmask == "" {
		return netip.PrefixFrom(ip, ip.BitLen()), true
	}
	mask, err := netip.ParseAddr(netmask)
	if err != nil || !mask.Is4() {
		return netip.Prefix{}, false
	}
	n, bits := u32(mask), 0
	for n&0x80000000 != 0 {
		bits++
		n <<= 1
	}
	if n != 0 {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(ip, bits), true
}
