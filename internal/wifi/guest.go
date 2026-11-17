package wifi

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Guest AP layout (invariants §10): own subnet, own zone, no access to lan or the admin.
const (
	guestName    = "guest"
	guestBridge  = "br-guest"
	guestAddress = "192.168.2.1/24"
)

var (
	GuestSubnet = netip.MustParsePrefix("192.168.2.0/24")
	// ErrGuestOverlap: 192.168.2.0/24 is already used by another interface.
	ErrGuestOverlap = errors.New("guest subnet overlaps another network")
	ErrNoGuestRadio = errors.New("no radio for the guest access point")
	// ErrGuestConflict: a guest-named section exists that this feature did not shape.
	ErrGuestConflict = errors.New("existing guest configuration is not managed here")
)

// GuestConfig is the configured guest AP. Keys are never returned.
type GuestConfig struct {
	Configured bool   `json:"configured"`
	Enabled    bool   `json:"enabled"`
	SSID       string `json:"ssid"`
	Radio      string `json:"radio"`
	Subnet     string `json:"subnet"`
}

// GuestRequest configures the guest AP. Key is required on first creation; an empty
// key keeps the current one. An empty Radio keeps the current radio or picks one.
type GuestRequest struct {
	SSID, Key, Radio string
	Enabled          bool
	Acknowledge      bool
	Caller           netip.Addr
}

func (r GuestRequest) validate() error {
	if err := validSSID(r.SSID); err != nil {
		return err
	}
	if r.Radio != "" && !sectionPattern.MatchString(r.Radio) {
		return ValidationError{"wifi_radio_invalid"}
	}
	if r.Key != "" {
		return validKey(r.Key)
	}
	return nil
}

type guestConfigs struct{ wireless, network, firewall, dhcp []device.Section }

func (s *Service) readGuestConfigs(ctx context.Context) (guestConfigs, error) {
	var c guestConfigs
	for config, dst := range map[string]*[]device.Section{"wireless": &c.wireless, "network": &c.network, "firewall": &c.firewall, "dhcp": &c.dhcp} {
		sections, err := s.join.UCI.Show(ctx, config)
		if err != nil {
			return guestConfigs{}, ErrUnavailable
		}
		*dst = sections
	}
	return c, nil
}

// Guest reports the guest AP from UCI.
func (s *Service) Guest(ctx context.Context) (GuestConfig, error) {
	if s.join == nil {
		return GuestConfig{}, ErrUnavailable
	}
	wireless, err := s.join.UCI.Show(ctx, "wireless")
	if err != nil {
		return GuestConfig{}, ErrUnavailable
	}
	out := GuestConfig{Subnet: GuestSubnet.String()}
	if at := slices.IndexFunc(wireless, isGuestAP); at >= 0 {
		ap := wireless[at]
		out.Configured, out.Enabled, out.SSID, out.Radio = true, ap.Options["disabled"] != "1", ap.Options["ssid"], ap.Options["device"]
	}
	return out, nil
}

func isGuestAP(s device.Section) bool { return s.Name == guestName && isAP(s) }

// SetGuest applies the guest AP, its interface, DHCP and firewall zone in one transaction.
func (s *Service) SetGuest(ctx context.Context, req GuestRequest) (txn.Status, error) {
	if err := req.validate(); err != nil {
		return txn.Status{}, err
	}
	if s.join == nil {
		return txn.Status{}, ErrUnavailable
	}
	return s.applyAPPlan(ctx, "wifi: guest access point", []string{"wireless", "network", "firewall", "dhcp"}, req.Key,
		func(ctx context.Context) (apPlan, error) { return s.prepareGuest(ctx, req) })
}

func (s *Service) prepareGuest(ctx context.Context, req GuestRequest) (apPlan, error) {
	c, err := s.readGuestConfigs(ctx)
	if err != nil {
		return apPlan{}, err
	}
	raw, err := s.join.Ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return apPlan{}, ErrUnavailable
	}
	live, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return apPlan{}, ErrUnavailable
	}
	writes, before, after, err := planGuest(c, live, req)
	if err != nil {
		return apPlan{}, err
	}
	if err := s.checkAPLockout(ctx, c.wireless, before, after, req.Caller, req.Acknowledge); err != nil {
		return apPlan{}, err
	}
	expect := apExpectations(c.wireless, guestName, req.Enabled, req.SSID)
	if !slices.ContainsFunc(expect, func(e apExpectation) bool { return e.section == guestName }) {
		expect = append(expect, apExpectation{section: guestName, enabled: req.Enabled, ssid: req.SSID})
	}
	return apPlan{writes, expect}, nil
}

// planGuest is pure: current configs and live interfaces in, ordered writes out.
func planGuest(c guestConfigs, live []facts.Interface, req GuestRequest) (writes []uciWrite, before, after []lockout.AP, err error) {
	if err := guestOverlap(c.network, live); err != nil {
		return nil, nil, nil, err
	}
	existing := slices.IndexFunc(c.wireless, isGuestAP)
	// Never repurpose an unrelated section named guest or shadow a zone named guest.
	foreignZone := slices.ContainsFunc(c.firewall, func(s device.Section) bool {
		return s.Type == "zone" && s.Options["name"] == guestName && s.Name != guestName
	})
	if foreignZone || (existing < 0 && hasSection(c.wireless, guestName)) {
		return nil, nil, nil, ErrGuestConflict
	}
	if existing < 0 && req.Key == "" {
		return nil, nil, nil, ValidationError{"wifi_key_required"}
	}
	radio := req.Radio
	if radio == "" && existing >= 0 {
		radio = c.wireless[existing].Options["device"]
	}
	if radio == "" {
		radio = guestRadio(c.wireless)
	}
	if !slices.ContainsFunc(c.wireless, func(s device.Section) bool { return s.Type == "wifi-device" && s.Name == radio }) {
		return nil, nil, nil, ErrNoGuestRadio
	}
	if req.Enabled && radioHasSTA(c.wireless, radio) {
		return nil, nil, nil, ErrRadioInUse
	}
	before = apLayout(c.wireless)
	after = slices.Clone(before)
	if i := slices.IndexFunc(after, func(ap lockout.AP) bool { return ap.Section == guestName }); i >= 0 {
		after[i].Enabled = req.Enabled
	} else {
		after = append(after, lockout.AP{Section: guestName, Enabled: req.Enabled, Guest: true})
	}

	w := planner{}
	w.section(c.network, "network", "guest_dev", "device", [][2]string{{"name", guestBridge}, {"type", "bridge"}, {"bridge_empty", "1"}})
	w.section(c.network, "network", guestName, "interface", [][2]string{{"proto", "static"}, {"device", guestBridge}, {"ipaddr", guestAddress}})
	w.section(c.dhcp, "dhcp", guestName, "dhcp", [][2]string{{"interface", guestName}, {"start", "100"}, {"limit", "150"}, {"leasetime", "1h"}, {"dhcpv4", "server"}})
	w.section(c.firewall, "firewall", guestName, "zone", [][2]string{{"name", guestName}, {"network", guestName}, {"input", "REJECT"}, {"output", "ACCEPT"}, {"forward", "REJECT"}})
	w.section(c.firewall, "firewall", "guest_wan", "forwarding", [][2]string{{"src", guestName}, {"dest", "wan"}})
	w.section(c.firewall, "firewall", "guest_dhcp", "rule", [][2]string{{"name", "Allow-Guest-DHCP"}, {"src", guestName}, {"proto", "udp"}, {"dest_port", "67-68"}, {"family", "ipv4"}, {"target", "ACCEPT"}})
	w.section(c.firewall, "firewall", "guest_dns", "rule", [][2]string{{"name", "Allow-Guest-DNS"}, {"src", guestName}, {"proto", "tcp udp"}, {"dest_port", "53"}, {"target", "ACCEPT"}})
	encryption := "psk2"
	if existing >= 0 && req.Key == "" && c.wireless[existing].Options["key"] == "" {
		encryption = "none"
	}
	w.section(c.wireless, "wireless", guestName, "wifi-iface", [][2]string{{"device", radio}, {"mode", "ap"}, {"network", guestName}, {"ssid", req.SSID}, {"encryption", encryption}, {"isolate", "1"}})
	if req.Key != "" && (existing < 0 || c.wireless[existing].Options["key"] != req.Key) {
		w.writes = append(w.writes, uciWrite{"key", "wireless", guestName, "key", ""})
	}
	w.section(c.wireless, "wireless", guestName, "wifi-iface", [][2]string{{"disabled", map[bool]string{true: "0", false: "1"}[req.Enabled]}})
	return w.writes, before, after, nil
}

// planner emits set-if-different writes, creating a named section when absent, so a
// re-apply of the same request stages nothing.
type planner struct{ writes []uciWrite }

func (p *planner) section(current []device.Section, config, name, typ string, options [][2]string) {
	at := slices.IndexFunc(current, func(s device.Section) bool { return s.Name == name })
	var have map[string]string
	if at < 0 || current[at].Type != typ {
		if !slices.ContainsFunc(p.writes, func(w uciWrite) bool { return w.action == "type" && w.config == config && w.section == name }) {
			p.writes = append(p.writes, uciWrite{"type", config, name, "", typ})
		}
	} else {
		have = current[at].Options
	}
	for _, kv := range options {
		if have[kv[0]] != kv[1] {
			p.writes = append(p.writes, uciWrite{"set", config, name, kv[0], kv[1]})
		}
	}
}

// guestRadio picks deterministically: the first radio by name with no enabled STA.
func guestRadio(wireless []device.Section) string {
	var radios []string
	for _, s := range wireless {
		if s.Type == "wifi-device" && s.Options["disabled"] != "1" && !radioHasSTA(wireless, s.Name) {
			radios = append(radios, s.Name)
		}
	}
	slices.Sort(radios)
	if len(radios) == 0 {
		return ""
	}
	return radios[0]
}

// guestOverlap refuses 192.168.2.0/24 when any other interface, configured or live
// (an uplink DHCP lease can land there), already uses an overlapping prefix.
func guestOverlap(network []device.Section, live []facts.Interface) error {
	for _, s := range network {
		if s.Type != "interface" || s.Name == guestName {
			continue
		}
		for _, addr := range append(strings.Fields(s.Options["ipaddr"]), s.Lists["ipaddr"]...) {
			if prefix, ok := configuredPrefix(addr, s.Options["netmask"]); ok && prefix.Overlaps(GuestSubnet) {
				return ErrGuestOverlap
			}
		}
	}
	for _, iface := range live {
		if iface.Name == guestName {
			continue
		}
		for _, prefix := range iface.IPv4 {
			if prefix.Masked().Overlaps(GuestSubnet) {
				return ErrGuestOverlap
			}
		}
	}
	return nil
}

// configuredPrefix reads `ipaddr` in CIDR form or with a dotted `netmask`.
func configuredPrefix(addr, netmask string) (netip.Prefix, bool) {
	if prefix, err := netip.ParsePrefix(addr); err == nil {
		return prefix.Masked(), true
	}
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return netip.Prefix{}, false
	}
	bits := 32
	if mask, err := netip.ParseAddr(netmask); err == nil && mask.Is4() {
		b := mask.As4()
		bits = 0
		for _, octet := range b {
			for i := 7; i >= 0; i-- {
				if octet&(1<<i) == 0 {
					return netip.PrefixFrom(ip, bits).Masked(), true
				}
				bits++
			}
		}
	}
	return netip.PrefixFrom(ip, bits).Masked(), true
}
