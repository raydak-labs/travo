// Package wansettings reads and changes the wan uplink's protocol (network.wan): DHCP,
// static or PPPoE. Every change is a `network` transaction the user confirms; the probe
// reads the kernel default route, never netifd's (device-facts §1).
package wansettings

import (
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Interface is the netifd interface Travo edits; wan6 and the device stay untouched.
const Interface = "wan"

// Protocols Travo edits. Any other proto (LuCI's qmi, none, ...) is read-only here.
const (
	ProtoDHCP   = "dhcp"
	ProtoStatic = "static"
	ProtoPPPoE  = "pppoe"
)

// MaxDNS bounds the static DNS list.
const MaxDNS = 4

var (
	ErrInvalid = errors.New("invalid wan settings")
	// ErrForeign: network.wan is missing, has no device or holds a layout Travo does not edit.
	ErrForeign = errors.New("network.wan is configured in a way Travo does not edit")
	// ErrOverlapsLAN: the static subnet overlaps lan or another local network.
	ErrOverlapsLAN = errors.New("the wan subnet overlaps a local network")
)

// FieldError names the request field a value failed on.
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

// View is GET /network/wan. The PPPoE password is never returned, only whether one is stored.
type View struct {
	Proto       string   `json:"proto" enum:"dhcp,static,pppoe" doc:"WAN protocol"`
	IPAddr      string   `json:"ipaddr" doc:"Static IPv4 address with prefix, e.g. 203.0.113.5/24; empty otherwise"`
	Gateway     string   `json:"gateway" doc:"Static default gateway; empty otherwise"`
	DNS         []string `json:"dns" nullable:"false" doc:"Static DNS servers; empty otherwise"`
	Username    string   `json:"username" doc:"PPPoE user name; empty otherwise"`
	PasswordSet bool     `json:"password_set" doc:"A PPPoE password is stored (it is never returned)"`
	Device      string   `json:"device" doc:"Kernel device the wan interface runs on (read-only)"`
}

// Request is PUT /network/wan. Fields of another protocol must be empty.
type Request struct {
	Proto    string   `json:"proto" enum:"dhcp,static,pppoe" doc:"WAN protocol"`
	IPAddr   string   `json:"ipaddr,omitempty" maxLength:"18" doc:"static: IPv4 address with prefix"`
	Gateway  string   `json:"gateway,omitempty" maxLength:"15" doc:"static: default gateway inside the subnet"`
	DNS      []string `json:"dns,omitempty" maxItems:"4" doc:"static: DNS servers (IPv4)"`
	Username string   `json:"username,omitempty" maxLength:"128" doc:"pppoe: user name"`
	Password string   `json:"password,omitempty" maxLength:"128" doc:"pppoe: password; omitted keeps the stored one (write-only)"`
}

// current is network.wan as read, including the stored password (never leaves the package).
type current struct {
	View
	password string
}

func wanSection(network []device.Section) (device.Section, bool) {
	at := slices.IndexFunc(network, func(s device.Section) bool { return s.Name == Interface && s.Type == "interface" })
	if at < 0 {
		return device.Section{}, false
	}
	return network[at], true
}

// read takes network.wan; a proto Travo does not edit or a multi-address static is foreign.
func read(network []device.Section) (current, error) {
	s, ok := wanSection(network)
	if !ok || s.Options["device"] == "" {
		return current{}, ErrForeign
	}
	c := current{View: View{Proto: s.Options["proto"], DNS: []string{}, Device: s.Options["device"]}}
	switch c.Proto {
	case ProtoDHCP:
	case ProtoStatic:
		addrs := append(strings.Fields(s.Options["ipaddr"]), s.Lists["ipaddr"]...)
		if len(addrs) != 1 {
			return current{}, ErrForeign
		}
		p, ok := configuredPrefix(addrs[0], s.Options["netmask"])
		if !ok {
			return current{}, ErrForeign
		}
		c.IPAddr, c.Gateway = p.String(), s.Options["gateway"]
		c.DNS = append(strings.Fields(s.Options["dns"]), s.Lists["dns"]...)
	case ProtoPPPoE:
		c.Username, c.password = s.Options["username"], s.Options["password"]
		c.PasswordSet = c.password != ""
	default:
		return current{}, ErrForeign
	}
	return c, nil
}

// Read is the GET view of network.wan.
func Read(network []device.Section) (View, error) {
	c, err := read(network)
	return c.View, err
}

// configuredPrefix reads `ipaddr` in CIDR form or with a dotted `netmask` (the host part kept).
func configuredPrefix(addr, netmask string) (netip.Prefix, bool) {
	if p, err := netip.ParsePrefix(addr); err == nil && p.Addr().Is4() {
		return p, true
	}
	ip, err := netip.ParseAddr(addr)
	mask, merr := netip.ParseAddr(netmask)
	if err != nil || !ip.Is4() {
		return netip.Prefix{}, false
	}
	if netmask == "" {
		return netip.PrefixFrom(ip, 32), true
	}
	if merr != nil || !mask.Is4() {
		return netip.Prefix{}, false
	}
	b := mask.As4()
	n := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	bits := 0
	for n&0x80000000 != 0 {
		bits++
		n <<= 1
	}
	if n != 0 {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(ip, bits), true
}

var (
	pppoeUser     = regexp.MustCompile(`^[!-~]{1,128}$`)
	pppoePassword = regexp.MustCompile(`^[ -~]{1,128}$`)
)

// Validate checks a request; network supplies the local prefixes a static wan must not
// overlap. passwordSet says whether a PPPoE password is already stored.
func Validate(in Request, network []device.Section, passwordSet bool) (Request, error) {
	switch in.Proto {
	case ProtoDHCP:
		return in, onlyFields(in)
	case ProtoStatic:
		if err := onlyFields(in, "ipaddr", "gateway", "dns"); err != nil {
			return Request{}, err
		}
		return validateStatic(in, network)
	case ProtoPPPoE:
		if err := onlyFields(in, "username", "password"); err != nil {
			return Request{}, err
		}
		if !pppoeUser.MatchString(in.Username) {
			return Request{}, &FieldError{"username", "must be 1-128 printable characters without spaces"}
		}
		if in.Password == "" && !passwordSet {
			return Request{}, &FieldError{"password", "is required"}
		}
		if in.Password != "" && !pppoePassword.MatchString(in.Password) {
			return Request{}, &FieldError{"password", "must be 1-128 printable characters"}
		}
		return in, nil
	}
	return Request{}, &FieldError{"proto", "must be dhcp, static or pppoe"}
}

// onlyFields refuses values for fields the protocol does not use: silent acceptance is a defect.
func onlyFields(in Request, allowed ...string) error {
	set := map[string]bool{"ipaddr": in.IPAddr != "", "gateway": in.Gateway != "", "dns": len(in.DNS) > 0,
		"username": in.Username != "", "password": in.Password != ""}
	for _, f := range []string{"ipaddr", "gateway", "dns", "username", "password"} {
		if set[f] && !slices.Contains(allowed, f) {
			return &FieldError{f, "is not used by " + in.Proto}
		}
	}
	return nil
}

func validateStatic(in Request, network []device.Section) (Request, error) {
	p, err := netip.ParsePrefix(in.IPAddr)
	if err != nil || !p.Addr().Is4() || p.Bits() < 8 || p.Bits() > 30 {
		return Request{}, &FieldError{"ipaddr", "must be an IPv4 address with a prefix of /8 to /30"}
	}
	if !usableHost(p, p.Addr()) {
		return Request{}, &FieldError{"ipaddr", "must be a unicast host address of its subnet"}
	}
	gw, err := netip.ParseAddr(in.Gateway)
	if err != nil || !gw.Is4() || gw == p.Addr() || !usableHost(p, gw) {
		return Request{}, &FieldError{"gateway", "must be another host address inside " + p.Masked().String()}
	}
	if len(in.DNS) > MaxDNS {
		return Request{}, &FieldError{"dns", "at most 4 servers"}
	}
	dns := make([]string, 0, len(in.DNS))
	for _, d := range in.DNS {
		a, err := netip.ParseAddr(strings.TrimSpace(d))
		if err != nil || !a.Is4() || !a.IsGlobalUnicast() {
			return Request{}, &FieldError{"dns", "must be IPv4 unicast addresses"}
		}
		if !slices.Contains(dns, a.String()) {
			dns = append(dns, a.String())
		}
	}
	if overlapsLocal(p.Masked(), network) {
		return Request{}, ErrOverlapsLAN
	}
	return Request{Proto: ProtoStatic, IPAddr: p.String(), Gateway: gw.String(), DNS: dns}, nil
}

// usableHost: a in p, unicast, neither the network nor the broadcast address.
func usableHost(p netip.Prefix, a netip.Addr) bool {
	if !p.Contains(a) || !a.IsGlobalUnicast() || a == p.Masked().Addr() {
		return false
	}
	b := p.Masked().Addr().As4()
	n := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	n |= 1<<(32-p.Bits()) - 1
	return a != netip.AddrFrom4([4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
}

// overlapsLocal: any other interface's configured IPv4 prefix (lan, guest, ...) overlaps p.
func overlapsLocal(p netip.Prefix, network []device.Section) bool {
	for _, s := range network {
		if s.Type != "interface" || s.Name == Interface || s.Name == "loopback" {
			continue
		}
		for _, addr := range append(strings.Fields(s.Options["ipaddr"]), s.Lists["ipaddr"]...) {
			if q, ok := configuredPrefix(addr, s.Options["netmask"]); ok && q.Masked().Overlaps(p) {
				return true
			}
		}
	}
	return false
}
