// Package portfwd reads and changes port forwards: firewall `redirect` sections with target
// DNAT from the wan zone to a host in the lan zone. Travo owns travo_fwd_<n>; every other
// redirect (LuCI's, hand-written) is listed read-only.
package portfwd

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/netview"
)

// Prefix names the redirect sections Travo creates: travo_fwd_<n>.
const Prefix = "travo_fwd_"

// MaxOwned bounds Travo's own forwards.
const MaxOwned = 32

// Zone names the forwards use; fw4 resolves a redirect's src/dest by zone name.
const (
	WANZone = "wan"
	LANZone = "lan"
)

// Protocols a request may name; both is stored as fw4's flattened list `tcp udp`.
const (
	ProtoTCP  = "tcp"
	ProtoUDP  = "udp"
	ProtoBoth = "both"
)

var (
	ErrInvalid = errors.New("invalid port forward")
	// ErrPortInUse: an enabled forward already takes that protocol and external port.
	ErrPortInUse = errors.New("an enabled forward already uses that external port")
	ErrNotFound  = errors.New("no port forward with that id")
	// ErrForeign: the redirect was not created by Travo; LuCI owns it.
	ErrForeign = errors.New("the port forward is not managed by Travo")
	ErrFull    = errors.New("too many port forwards")
	// ErrNoZone: the firewall has no enabled zone named wan or lan.
	ErrNoZone = errors.New("the firewall has no wan or lan zone")
)

// FieldError names the request field a validation failed on.
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

// Forward is one redirect section as fw4 reads it (testdata portfwd/fw4-parse-redirect.uc.txt):
// enabled defaults to on, target to DNAT, proto to `tcpudp`.
type Forward struct {
	ID           string `json:"id" doc:"UCI section"`
	Name         string `json:"name"`
	Proto        string `json:"proto" doc:"tcp, udp, both, or the raw fw4 value of a foreign section"`
	ExternalPort string `json:"external_port" doc:"src_dport: port or range a-b on the wan side"`
	TargetIP     string `json:"target_ip" doc:"dest_ip; empty means the router itself"`
	TargetPort   string `json:"target_port" doc:"dest_port; empty means the external port"`
	Src          string `json:"src" doc:"Source zone"`
	Dest         string `json:"dest" doc:"Destination zone"`
	Target       string `json:"target" doc:"DNAT or SNAT"`
	Enabled      bool   `json:"enabled"`
	Owned        bool   `json:"owned" doc:"Created by Travo (travo_fwd_<n>); others are read-only"`
}

// Request is one new forward. TargetPort empty forwards to the same port.
type ForwardRequest struct {
	Name         string `json:"name,omitempty" maxLength:"64" doc:"Label; defaults to the external port"`
	Proto        string `json:"proto" enum:"tcp,udp,both"`
	ExternalPort string `json:"external_port" minLength:"1" maxLength:"11" doc:"Port 1-65535 or range a-b"`
	TargetIP     string `json:"target_ip" minLength:"7" maxLength:"15" doc:"IPv4 host in the lan subnet, not the router"`
	TargetPort   string `json:"target_port,omitempty" maxLength:"11" doc:"Port or range; empty keeps the external port"`
}

// PortRange is an inclusive port range; a single port has Lo == Hi.
type PortRange struct{ Lo, Hi int }

func (r PortRange) String() string {
	if r.Lo == r.Hi {
		return strconv.Itoa(r.Lo)
	}
	return strconv.Itoa(r.Lo) + "-" + strconv.Itoa(r.Hi)
}

func (r PortRange) overlaps(o PortRange) bool { return r.Lo <= o.Hi && o.Lo <= r.Hi }

// ParsePorts reads `n` or `a-b` (fw4 also accepts `a:b`), 1-65535, a <= b.
func ParsePorts(s string) (PortRange, bool) {
	lo, hi, isRange := strings.Cut(strings.TrimSpace(s), "-")
	if !isRange {
		lo, hi, isRange = strings.Cut(lo, ":")
	}
	a, ok := port(lo)
	if !ok {
		return PortRange{}, false
	}
	if !isRange {
		return PortRange{a, a}, true
	}
	b, ok := port(hi)
	return PortRange{a, b}, ok && a <= b
}

func port(s string) (int, bool) {
	if s == "" || strings.Trim(s, "0123456789") != "" || len(s) > 5 {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 1 && n <= 65535
}

// protocols is fw4's flattened proto list; an unset proto is `tcpudp`.
func protocols(s device.Section) []string {
	out := slices.Clone(s.Lists["proto"])
	out = append(out, strings.Fields(strings.ToLower(s.Options["proto"]))...)
	if len(out) == 0 {
		return []string{ProtoTCP, ProtoUDP}
	}
	var flat []string
	for _, p := range out {
		if p == "tcpudp" {
			flat = append(flat, ProtoTCP, ProtoUDP)
		} else {
			flat = append(flat, p)
		}
	}
	return flat
}

func protoLabel(ps []string) string {
	switch {
	case slices.Equal(ps, []string{ProtoTCP}):
		return ProtoTCP
	case slices.Equal(ps, []string{ProtoUDP}):
		return ProtoUDP
	case len(ps) == 2 && slices.Contains(ps, ProtoTCP) && slices.Contains(ps, ProtoUDP):
		return ProtoBoth
	}
	return strings.Join(ps, " ")
}

// enabled reads a fw4 bool (parse_bool spellings); unset is on.
func enabled(v string) bool {
	switch strings.ToLower(v) {
	case "0", "no", "off", "false", "disabled":
		return false
	}
	return true
}

// List returns every redirect section in section order.
func List(firewall []device.Section) []Forward {
	out := []Forward{}
	for _, s := range firewall {
		if s.Type != "redirect" {
			continue
		}
		target := strings.ToUpper(s.Options["target"])
		if target == "" {
			target = "DNAT"
		}
		out = append(out, Forward{ID: s.Name, Name: s.Options["name"], Proto: protoLabel(protocols(s)),
			ExternalPort: s.Options["src_dport"], TargetIP: s.Options["dest_ip"], TargetPort: s.Options["dest_port"],
			Src: s.Options["src"], Dest: s.Options["dest"], Target: target, Enabled: enabled(s.Options["enabled"]),
			Owned: owned(s)})
	}
	return out
}

func owned(s device.Section) bool {
	n, ok := strings.CutPrefix(s.Name, Prefix)
	if !ok || s.Type != "redirect" {
		return false
	}
	_, err := strconv.Atoi(n)
	return err == nil
}

// Valid is a checked request ready to stage.
type Valid struct {
	Name     string
	Protos   []string
	External PortRange
	Target   netip.Addr
	// TargetPort is empty when it equals the external port.
	TargetPort string
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,63}$`)

// Validate checks in against the committed lan prefix, whose address is the router's. The
// router itself is never a target: a forward to it would expose its admin services (web UI,
// LuCI, SSH) to the wan side (invariants §port forwards).
func Validate(in ForwardRequest, lan netip.Prefix) (Valid, error) {
	var v Valid
	switch in.Proto {
	case ProtoTCP, ProtoUDP:
		v.Protos = []string{in.Proto}
	case ProtoBoth:
		v.Protos = []string{ProtoTCP, ProtoUDP}
	default:
		return Valid{}, &FieldError{"proto", "must be tcp, udp or both"}
	}
	ext, ok := ParsePorts(in.ExternalPort)
	if !ok {
		return Valid{}, &FieldError{"external_port", "must be a port 1-65535 or a range a-b"}
	}
	v.External = ext
	if tp := strings.TrimSpace(in.TargetPort); tp != "" {
		t, ok := ParsePorts(tp)
		if !ok {
			return Valid{}, &FieldError{"target_port", "must be a port 1-65535 or a range a-b"}
		}
		// fw4 maps a range onto a range only when both have the same size.
		if t.Hi-t.Lo != ext.Hi-ext.Lo {
			return Valid{}, &FieldError{"target_port", "must cover as many ports as the external port"}
		}
		if t != ext {
			v.TargetPort = t.String()
		}
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(in.TargetIP))
	if err != nil || !addr.Is4() || !lan.Contains(addr) || (lan.Bits() < 31 && addr == lan.Masked().Addr()) || isBroadcast(addr, lan) {
		return Valid{}, &FieldError{"target_ip", "must be a host address in the lan subnet " + lan.Masked().String()}
	}
	if addr == lan.Addr() {
		return Valid{}, &FieldError{"target_ip", "must not be the router itself"}
	}
	v.Target = addr
	v.Name = strings.TrimSpace(in.Name)
	if v.Name == "" {
		v.Name = "Forward " + ext.String()
	}
	if !nameRe.MatchString(v.Name) {
		return Valid{}, &FieldError{"name", "letters, digits, space, dot, underscore or hyphen, at most 64"}
	}
	return v, nil
}

func isBroadcast(a netip.Addr, lan netip.Prefix) bool {
	host := ^uint32(0) >> lan.Bits()
	return lan.Bits() < 31 && binary.BigEndian.Uint32(a.AsSlice())&host == host
}

// Conflict reports whether an enabled DNAT redirect from the wan zone already takes one of
// v's protocols on an overlapping external port.
func Conflict(firewall []device.Section, v Valid) bool {
	for _, s := range firewall {
		if s.Type != "redirect" || !enabled(s.Options["enabled"]) || s.Options["src"] != WANZone {
			continue
		}
		if t := strings.ToUpper(s.Options["target"]); t != "" && t != "DNAT" {
			continue
		}
		r, ok := ParsePorts(s.Options["src_dport"])
		if !ok || !r.overlaps(v.External) {
			continue
		}
		if slices.ContainsFunc(protocols(s), func(p string) bool { return slices.Contains(v.Protos, p) }) {
			return true
		}
	}
	return false
}

// zonesPresent reports whether enabled zones named wan and lan exist.
func zonesPresent(firewall []device.Section) bool {
	zs := netview.Zones(firewall)
	has := func(n string) bool { return slices.ContainsFunc(zs, func(z netview.Zone) bool { return z.Name == n }) }
	return has(WANZone) && has(LANZone)
}
