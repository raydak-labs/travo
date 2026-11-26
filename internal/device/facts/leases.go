package facts

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Lease is one dnsmasq IPv4 lease from /tmp/dhcp.leases (device-facts §14: there is no
// `ubus call dhcp ipv4leases` on 25.12.6).
type Lease struct {
	MAC      string // lower case
	IP       netip.Addr
	Hostname string    // empty when the client sent none (`*`)
	Expires  time.Time // zero for an infinite lease
}

// ParseLeases parses dnsmasq's lease file: `<expiry epoch> <mac> <ip> <hostname|*> <client-id|*>`.
// IPv6 lines (dnsmasq writes a `duid` line first) and lines with an IPv6 address are skipped.
func ParseLeases(text string) ([]Lease, error) {
	out := []Lease{}
	for n, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "duid" {
			continue
		}
		if len(fields) < 4 {
			return nil, fmt.Errorf("leases: line %d: %w: %d fields", n+1, ErrShape, len(fields))
		}
		epoch, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("leases: line %d: expiry: %w", n+1, err)
		}
		ip, err := netip.ParseAddr(fields[2])
		if err != nil {
			return nil, fmt.Errorf("leases: line %d: %w", n+1, err)
		}
		if !ip.Is4() {
			continue
		}
		l := Lease{MAC: strings.ToLower(fields[1]), IP: ip}
		if fields[3] != "*" {
			l.Hostname = fields[3]
		}
		if epoch > 0 {
			l.Expires = time.Unix(epoch, 0).UTC()
		}
		out = append(out, l)
	}
	return out, nil
}

// Station is one associated, authorized station of `hostapd.<ifname> get_clients`.
type Station struct {
	MAC    string // lower case
	Signal *int   // dBm; nil when the reply has none
}

// ParseHostapdStations parses `ubus call hostapd.<ifname> get_clients`: clients is a map
// keyed by MAC. Only associated and authorized stations count. Sorted by MAC.
func ParseHostapdStations(b []byte) ([]Station, error) {
	var raw struct {
		Freq    *int `json:"freq"`
		Clients map[string]struct {
			Assoc      bool `json:"assoc"`
			Authorized bool `json:"authorized"`
			Signal     *int `json:"signal"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("get_clients: %w", err)
	}
	if raw.Freq == nil || raw.Clients == nil {
		return nil, fmt.Errorf("get_clients: %w: need freq and clients", ErrShape)
	}
	out := []Station{}
	for mac, c := range raw.Clients {
		if c.Assoc && c.Authorized {
			out = append(out, Station{MAC: strings.ToLower(mac), Signal: c.Signal})
		}
	}
	slices.SortFunc(out, func(a, b Station) int { return strings.Compare(a.MAC, b.MAC) })
	return out, nil
}
