// Package dhcpserver reads and changes the lan DHCP server (dhcp.lan) and the local DNS
// names dnsmasq answers (dhcp `domain` sections). Both write the dhcp config through
// txn.Apply and never touch the dnsmasq section the DNS layer stack owns (invariants §8).
package dhcpserver

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strconv"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// LANSection is the dhcp section serving the lan interface.
const LANSection = "lan"

// Defaults of /etc/init.d/dnsmasq dhcp_add (testdata dhcp/init-dnsmasq-dhcp-range.txt).
const (
	defaultStart     = 100
	defaultLimit     = 150
	defaultLeaseTime = "12h"
)

var (
	ErrInvalid = errors.New("invalid dhcp settings")
	// ErrNoSection: dhcp has no `lan` section of type dhcp.
	ErrNoSection = errors.New("dhcp.lan is missing")
	// ErrForeignValue: start or limit is not a plain offset (e.g. an address); LuCI owns it.
	ErrForeignValue = errors.New("dhcp.lan holds a value Travo does not edit")
)

// FieldError names the request field a value failed on.
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

// DHCPSettings is the editable part of dhcp.lan.
type DHCPSettings struct {
	Enabled   bool   `json:"enabled" doc:"IPv4 DHCP server on (dhcp.lan has neither ignore nor dhcpv4 disabled)"`
	Start     int    `json:"start" minimum:"1" maximum:"254" doc:"First pool address as an offset into the lan subnet"`
	Limit     int    `json:"limit" minimum:"1" maximum:"254" doc:"Pool size"`
	LeaseTime string `json:"leasetime" maxLength:"16" doc:"<n>m, <n>h or <n>d (at least 2m), or infinite"`
}

// DHCPView is GET /network/dhcp: the settings plus what they mean on the committed lan subnet.
type DHCPView struct {
	DHCPSettings
	Subnet string `json:"subnet,omitempty" doc:"Committed lan IPv4 subnet; empty when unknown"`
	First  string `json:"first,omitempty" doc:"First pool address"`
	Last   string `json:"last,omitempty" doc:"Last pool address"`
	// CallerLease: the requesting device holds a DHCP lease from this router, so turning the
	// server off strands it when the lease expires. False when unknown.
	CallerLease bool `json:"callerLease" doc:"The caller holds a lease from this router"`
}

// uciTrue mirrors config_get_bool in OpenWrt's functions.sh.
var uciTrue = map[string]bool{"1": true, "on": true, "true": true, "yes": true, "enabled": true}

func lanSection(dhcp []device.Section) (device.Section, bool) {
	at := slices.IndexFunc(dhcp, func(s device.Section) bool { return s.Name == LANSection && s.Type == "dhcp" })
	if at < 0 {
		return device.Section{}, false
	}
	return dhcp[at], true
}

// Read takes dhcp.lan as dnsmasq's init script does: unset start, limit and leasetime fall
// back to its defaults; `ignore` or `dhcpv4 disabled` turn the IPv4 server off.
func Read(dhcp []device.Section) (DHCPSettings, error) {
	s, ok := lanSection(dhcp)
	if !ok {
		return DHCPSettings{}, ErrNoSection
	}
	start, err := offset(s.Options, "start", defaultStart)
	if err != nil {
		return DHCPSettings{}, err
	}
	limit, err := offset(s.Options, "limit", defaultLimit)
	if err != nil {
		return DHCPSettings{}, err
	}
	lease := s.Options["leasetime"]
	if lease == "" {
		lease = defaultLeaseTime
	}
	on := !uciTrue[s.Options["ignore"]] && s.Options["dhcpv4"] != "disabled"
	return DHCPSettings{Enabled: on, Start: start, Limit: limit, LeaseTime: lease}, nil
}

func offset(opts map[string]string, key string, def int) (int, error) {
	v, set := opts[key]
	if !set {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, ErrForeignValue
	}
	return n, nil
}

var leaseTime = regexp.MustCompile(`^([1-9][0-9]{0,5})([mhd])$`)

// Validate checks a request against the committed lan prefix: the pool lies inside the
// subnet's host range (ipcalc would clamp it otherwise) and the lease is at least 2m, the
// shortest lease dnsmasq hands out.
func Validate(in DHCPSettings, lan netip.Prefix) (DHCPSettings, error) {
	if in.Start < 1 || in.Start > 254 {
		return DHCPSettings{}, &FieldError{"start", "must be 1-254"}
	}
	if in.Limit < 1 || in.Limit > 254 {
		return DHCPSettings{}, &FieldError{"limit", "must be 1-254"}
	}
	if hosts := hostCount(lan); in.Start+in.Limit-1 > hosts {
		return DHCPSettings{}, &FieldError{"limit", "the pool must end inside " + lan.Masked().String()}
	}
	if in.LeaseTime != "infinite" {
		m := leaseTime.FindStringSubmatch(in.LeaseTime)
		if m == nil || (m[2] == "m" && m[1] == "1") {
			return DHCPSettings{}, &FieldError{"leasetime", "must be <n>m, <n>h or <n>d (at least 2m), or infinite"}
		}
	}
	return in, nil
}

// hostCount is the last usable host offset of p (broadcast excluded).
func hostCount(p netip.Prefix) int { return 1<<(32-p.Bits()) - 2 }

// Pool is the first and last address of the range on lan.
func Pool(s DHCPSettings, lan netip.Prefix) (first, last netip.Addr) {
	base := lan.Masked().Addr().As4()
	n := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	at := func(off int) netip.Addr {
		v := n + uint32(off)
		return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
	}
	return at(s.Start), at(s.Start + s.Limit - 1)
}

// Stage writes s into dhcp.lan set-if-different and reports whether anything was staged.
// Turning the server on clears both off switches; turning it off sets `ignore '1'` only.
func Stage(ctx context.Context, uci device.UCI, dhcp []device.Section, s DHCPSettings) (bool, error) {
	cur, ok := lanSection(dhcp)
	if !ok {
		return false, ErrNoSection
	}
	old, err := Read(dhcp)
	if err != nil {
		return false, err
	}
	changed := false
	set := func(opt, v string) error {
		changed = true
		return uci.Set(ctx, "dhcp", LANSection, opt, v)
	}
	for _, kv := range []struct {
		opt      string
		old, new string
	}{
		{"start", strconv.Itoa(old.Start), strconv.Itoa(s.Start)},
		{"limit", strconv.Itoa(old.Limit), strconv.Itoa(s.Limit)},
		{"leasetime", old.LeaseTime, s.LeaseTime},
	} {
		if kv.old != kv.new {
			if err := set(kv.opt, kv.new); err != nil {
				return false, err
			}
		}
	}
	if s.Enabled == old.Enabled {
		return changed, nil
	}
	if !s.Enabled {
		return true, set("ignore", "1")
	}
	if _, has := cur.Options["ignore"]; has {
		changed = true
		if err := uci.DeleteOption(ctx, "dhcp", LANSection, "ignore"); err != nil {
			return false, err
		}
	}
	if cur.Options["dhcpv4"] == "disabled" {
		return true, set("dhcpv4", "server")
	}
	return changed, nil
}
