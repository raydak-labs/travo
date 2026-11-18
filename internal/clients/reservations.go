package clients

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// HostPrefix names the dhcp host sections Travo creates: travo_host_<mac without colons>.
const HostPrefix = "travo_host_"

var (
	ErrReservationInvalid = errors.New("invalid reservation")
	// ErrReservationShared: the MAC sits in a user host section with other MACs; editing it
	// would change the other devices' reservation too.
	ErrReservationShared   = errors.New("the MAC shares a host section with other MACs")
	ErrReservationNotFound = errors.New("no reservation for this MAC")
	// ErrIPTaken: another MAC already holds that address.
	ErrIPTaken = errors.New("the address is reserved for another device")
	ErrNoLAN   = errors.New("the lan IPv4 prefix is unknown")
)

// FieldError names the request field a value failed on.
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrReservationInvalid }

// Reservation is one MAC of a dhcp host section.
type Reservation struct {
	MAC   string `json:"mac"`
	IP    string `json:"ip,omitempty" doc:"Reserved IPv4 address; empty for a name-only host entry"`
	Name  string `json:"name,omitempty" doc:"Host name dnsmasq hands out and resolves"`
	Owned bool   `json:"owned" doc:"Created by Travo (section travo_host_<mac>)"`
	// Section is the UCI section; not part of the API.
	Section string `json:"-"`
}

// hostMACs reads a host section's MACs: a list, or an option holding space-separated MACs.
func hostMACs(s device.Section) []string {
	raw := slices.Clone(s.Lists["mac"])
	if v := s.Options["mac"]; v != "" {
		raw = append(raw, strings.Fields(v)...)
	}
	out := []string{}
	for _, m := range raw {
		if mac, ok := NormalizeMAC(m); ok && !slices.Contains(out, mac) {
			out = append(out, mac)
		}
	}
	return out
}

// Reservations lists every MAC of every dhcp host section, in section order.
func Reservations(dhcp []device.Section) []Reservation {
	out := []Reservation{}
	for _, s := range dhcp {
		if s.Type != "host" {
			continue
		}
		for _, mac := range hostMACs(s) {
			out = append(out, Reservation{MAC: mac, IP: s.Options["ip"], Name: s.Options["name"],
				Owned: s.Name == HostSection(mac), Section: s.Name})
		}
	}
	return out
}

// HostSection is the section Travo uses for mac.
func HostSection(mac string) string { return HostPrefix + strings.ReplaceAll(mac, ":", "") }

// LANPrefix reads network.lan's IPv4 address: CIDR in ipaddr (25.12, see
// testdata clients/uci-show-network-lan.txt) or ipaddr plus netmask (older firmware).
func LANPrefix(network []device.Section) (netip.Prefix, error) {
	at := slices.IndexFunc(network, func(s device.Section) bool { return s.Name == "lan" && s.Type == "interface" })
	if at < 0 {
		return netip.Prefix{}, ErrNoLAN
	}
	lan := network[at]
	addr := lan.Options["ipaddr"]
	if addr == "" && len(lan.Lists["ipaddr"]) > 0 {
		addr = lan.Lists["ipaddr"][0]
	}
	if p, err := netip.ParsePrefix(addr); err == nil && p.Addr().Is4() {
		return p, nil
	}
	ip, err := netip.ParseAddr(addr)
	mask, merr := netip.ParseAddr(lan.Options["netmask"])
	if err != nil || merr != nil || !ip.Is4() || !mask.Is4() {
		return netip.Prefix{}, ErrNoLAN
	}
	bits, ok := maskBits(mask)
	if !ok {
		return netip.Prefix{}, ErrNoLAN
	}
	return netip.PrefixFrom(ip, bits), nil
}

func maskBits(mask netip.Addr) (int, bool) {
	b := mask.As4()
	n := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	bits := 0
	for n&0x80000000 != 0 {
		bits++
		n <<= 1
	}
	return bits, n == 0
}

var label = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

// ValidateReservation checks one request against the lan prefix and the existing hosts.
// It returns the normalised request.
func ValidateReservation(r Reservation, lan netip.Prefix, existing []Reservation) (Reservation, error) {
	mac, ok := NormalizeMAC(r.MAC)
	if !ok {
		return Reservation{}, &FieldError{"mac", "not a unicast MAC address"}
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(r.IP))
	if err != nil || !ip.Is4() {
		return Reservation{}, &FieldError{"ip", "not an IPv4 address"}
	}
	subnet := lan.Masked()
	if !subnet.Contains(ip) || ip == subnet.Addr() || ip == broadcast(subnet) || ip == lan.Addr() {
		return Reservation{}, &FieldError{"ip", "must be a host address in " + subnet.String() + " other than the router"}
	}
	name := strings.TrimSpace(r.Name)
	if name != "" && !label.MatchString(name) {
		return Reservation{}, &FieldError{"name", "must be one DNS label (letters, digits, hyphen)"}
	}
	if slices.ContainsFunc(existing, func(x Reservation) bool { return x.MAC != mac && x.IP == ip.String() }) {
		return Reservation{}, ErrIPTaken
	}
	return Reservation{MAC: mac, IP: ip.String(), Name: name}, nil
}

func broadcast(p netip.Prefix) netip.Addr {
	b := p.Addr().As4()
	host := uint32(1)<<(32-p.Bits()) - 1
	n := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) | host
	return netip.AddrFrom4([4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
}

// StageReservation writes r set-if-different. A user section holding only this MAC is edited
// in place; one shared with other MACs is refused. It reports whether anything was staged.
func StageReservation(ctx context.Context, uci device.UCI, dhcp []device.Section, r Reservation) (bool, error) {
	section := HostSection(r.MAC)
	at := slices.IndexFunc(dhcp, func(s device.Section) bool { return s.Type == "host" && slices.Contains(hostMACs(s), r.MAC) })
	if at >= 0 {
		if len(hostMACs(dhcp[at])) > 1 {
			return false, ErrReservationShared
		}
		section = dhcp[at].Name
	} else if slices.ContainsFunc(dhcp, func(s device.Section) bool { return s.Name == section }) {
		return false, ErrReservationShared // a foreign section squats on the owned name
	} else if err := stageNewHost(ctx, uci, section, r.MAC); err != nil {
		return false, err
	}
	var cur device.Section
	if at >= 0 {
		cur = dhcp[at]
	}
	changed := at < 0
	for _, kv := range [][2]string{{"ip", r.IP}, {"name", r.Name}} {
		old, set := cur.Options[kv[0]]
		switch {
		case kv[1] == "" && set:
			if err := uci.DeleteOption(ctx, "dhcp", section, kv[0]); err != nil {
				return false, err
			}
			changed = true
		case kv[1] != "" && old != kv[1]:
			if err := uci.Set(ctx, "dhcp", section, kv[0], kv[1]); err != nil {
				return false, err
			}
			changed = true
		}
	}
	return changed, nil
}

func stageNewHost(ctx context.Context, uci device.UCI, section, mac string) error {
	if err := uci.SetType(ctx, "dhcp", section, "host"); err != nil {
		return err
	}
	return uci.AddToList(ctx, "dhcp", section, "mac", mac)
}

// StageUnreserve removes mac from its host section; a section left without MACs is deleted.
func StageUnreserve(ctx context.Context, uci device.UCI, dhcp []device.Section, mac string) error {
	at := slices.IndexFunc(dhcp, func(s device.Section) bool { return s.Type == "host" && slices.Contains(hostMACs(s), mac) })
	if at < 0 {
		return ErrReservationNotFound
	}
	s := dhcp[at]
	rest := slices.DeleteFunc(hostMACs(s), func(m string) bool { return m == mac })
	if len(rest) == 0 {
		return uci.DeleteSection(ctx, "dhcp", s.Name)
	}
	if err := uci.DeleteOption(ctx, "dhcp", s.Name, "mac"); err != nil {
		return err
	}
	for _, m := range rest {
		if err := uci.AddToList(ctx, "dhcp", s.Name, "mac", m); err != nil {
			return err
		}
	}
	return nil
}
