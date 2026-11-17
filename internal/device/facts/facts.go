// Package facts parses verbatim device payloads (testdata/device/*) into typed values.
// Unknown or missing shapes are errors: "cannot read" must never look like "down" or "absent".
package facts

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

// ErrShape means the payload did not have the fields this firmware is known to emit.
var ErrShape = errors.New("unexpected payload shape")

// Interface is one netifd logical interface (network.interface dump).
type Interface struct {
	Name      string
	Up        bool
	L3Device  string
	IPv4      []netip.Prefix
	HasDefRte bool // netifd's view only; route truth is the kernel table (device-facts §1)
	// DNSServers are the resolvers the interface learned (DHCP/static), in netifd order.
	DNSServers []string
}

// ParseInterfaceDump parses `ubus call network.interface dump`. IPv4 arrives as
// {"address": "192.168.1.1", "mask": 24}, not as a CIDR string (device-facts §5).
func ParseInterfaceDump(b []byte) ([]Interface, error) {
	var raw struct {
		Interface []struct {
			Interface string `json:"interface"`
			Up        *bool  `json:"up"`
			L3Device  string `json:"l3_device"`
			IPv4      []struct {
				Address string `json:"address"`
				Mask    *int   `json:"mask"`
			} `json:"ipv4-address"`
			Route []struct {
				Target string `json:"target"`
				Mask   *int   `json:"mask"`
			} `json:"route"`
			DNSServer []string `json:"dns-server"`
		} `json:"interface"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("interface dump: %w", err)
	}
	if raw.Interface == nil {
		return nil, fmt.Errorf("interface dump: %w: no interface array", ErrShape)
	}
	out := make([]Interface, 0, len(raw.Interface))
	for _, r := range raw.Interface {
		if r.Interface == "" || r.Up == nil {
			return nil, fmt.Errorf("interface dump: %w: entry without name or up", ErrShape)
		}
		it := Interface{Name: r.Interface, Up: *r.Up, L3Device: r.L3Device}
		for _, d := range r.DNSServer {
			if _, err := netip.ParseAddr(d); err != nil {
				return nil, fmt.Errorf("interface %s: dns-server: %w", r.Interface, err)
			}
			it.DNSServers = append(it.DNSServers, d)
		}
		for _, a := range r.IPv4 {
			if a.Mask == nil {
				return nil, fmt.Errorf("interface %s: %w: address without mask", r.Interface, ErrShape)
			}
			ip, err := netip.ParseAddr(a.Address)
			if err != nil {
				return nil, fmt.Errorf("interface %s: %w", r.Interface, err)
			}
			pfx := netip.PrefixFrom(ip, *a.Mask)
			if !pfx.IsValid() {
				return nil, fmt.Errorf("interface %s: %w: bad mask %d", r.Interface, ErrShape, *a.Mask)
			}
			it.IPv4 = append(it.IPv4, pfx) // keeps the host address; Masked() gives the subnet
		}
		for _, rt := range r.Route {
			if rt.Target == "0.0.0.0" && rt.Mask != nil && *rt.Mask == 0 {
				it.HasDefRte = true
			}
		}
		out = append(out, it)
	}
	return out, nil
}

// Neighbour is one kernel neighbour entry with a usable MAC.
type Neighbour struct {
	IP  netip.Addr
	Dev string
	MAC string
}

// ParseNeighJSON parses `ip -j -4 neigh show`. Entries without lladdr (INCOMPLETE,
// FAILED) are skipped: they prove nothing about the caller.
func ParseNeighJSON(b []byte) ([]Neighbour, error) {
	var raw []struct {
		Dst    string   `json:"dst"`
		Dev    string   `json:"dev"`
		LLAddr string   `json:"lladdr"`
		State  []string `json:"state"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("neigh: %w", err)
	}
	out := []Neighbour{}
	for _, r := range raw {
		ip, err := netip.ParseAddr(r.Dst)
		if err != nil {
			return nil, fmt.Errorf("neigh: %w", err)
		}
		if r.LLAddr == "" || slices.Contains(r.State, "FAILED") || slices.Contains(r.State, "INCOMPLETE") {
			continue
		}
		out = append(out, Neighbour{IP: ip, Dev: r.Dev, MAC: strings.ToLower(r.LLAddr)})
	}
	return out, nil
}

// Link is per-device truth from `network.device status {"name": ...}` (device-facts §2):
// radio-level "up" proves nothing per interface; carrier proves STA association.
type Link struct {
	Present bool
	Up      bool
	Carrier bool
	MAC     string // lower case; empty when the reply has no macaddr
}

// ParseDeviceStatus requires all three fields; a missing one is "cannot verify".
func ParseDeviceStatus(b []byte) (Link, error) {
	var raw struct {
		Present *bool  `json:"present"`
		Up      *bool  `json:"up"`
		Carrier *bool  `json:"carrier"`
		MAC     string `json:"macaddr"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Link{}, fmt.Errorf("device status: %w", err)
	}
	if raw.Present == nil || raw.Up == nil || raw.Carrier == nil {
		return Link{}, fmt.Errorf("device status: %w: need present, up, carrier", ErrShape)
	}
	return Link{Present: *raw.Present, Up: *raw.Up, Carrier: *raw.Carrier, MAC: strings.ToLower(raw.MAC)}, nil
}

// RadioIface is one entry of network.wireless status.
type RadioIface struct {
	Radio   string
	RadioUp bool
	Section string
	Ifname  string
}

// ParseWirelessStatus parses `ubus call network.wireless status`. Per-interface entries
// carry no up flag on this firmware; liveness comes from ParseDeviceStatus per ifname.
func ParseWirelessStatus(b []byte) ([]RadioIface, error) {
	var raw map[string]struct {
		Up         *bool `json:"up"`
		Interfaces []struct {
			Section string `json:"section"`
			Ifname  string `json:"ifname"`
		} `json:"interfaces"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("wireless status: %w", err)
	}
	radios := make([]string, 0, len(raw))
	for r := range raw {
		radios = append(radios, r)
	}
	slices.Sort(radios)
	out := []RadioIface{}
	for _, r := range radios {
		v := raw[r]
		if v.Up == nil {
			return nil, fmt.Errorf("wireless status %s: %w: no up", r, ErrShape)
		}
		for _, i := range v.Interfaces {
			if i.Section == "" {
				return nil, fmt.Errorf("wireless status %s: %w: interface without section", r, ErrShape)
			}
			out = append(out, RadioIface{Radio: r, RadioUp: *v.Up, Section: i.Section, Ifname: i.Ifname})
		}
	}
	return out, nil
}

// ParseKernelDefaultRoute reads `ip -4 route show default`: any line means a way out.
// No error path by design: the caller already failed on a command error, and empty
// output genuinely means "no default route".
func ParseKernelDefaultRoute(out string) bool {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "default ") {
			return true
		}
	}
	return false
}

// ParseHostapdClients parses `ubus call hostapd.<ifname> get_clients`: clients is a map
// keyed by MAC. Only associated and authorized stations count.
func ParseHostapdClients(b []byte) ([]string, error) {
	var raw struct {
		Freq    *int `json:"freq"`
		Clients map[string]struct {
			Assoc      bool `json:"assoc"`
			Authorized bool `json:"authorized"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("get_clients: %w", err)
	}
	if raw.Freq == nil || raw.Clients == nil {
		return nil, fmt.Errorf("get_clients: %w: need freq and clients", ErrShape)
	}
	out := []string{}
	for mac, c := range raw.Clients {
		if c.Assoc && c.Authorized {
			out = append(out, strings.ToLower(mac))
		}
	}
	slices.Sort(out)
	return out, nil
}
