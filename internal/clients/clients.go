// Package clients lists the devices on the router's LAN side and manages their aliases,
// DHCP reservations (dhcp host sections), blocks (firewall rules) and kicks (hostapd).
// Reads merge the lease file, the kernel neighbour table and hostapd station lists;
// GET never mutates (invariants §4).
package clients

import (
	"cmp"
	"maps"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// Connection says how the router last saw a client.
type Connection string

const (
	// WiFi: an associated, authorized station of an enabled AP (hostapd get_clients).
	WiFi Connection = "wifi"
	// LAN: in the LAN-side neighbour table but on no AP's station list. br-lan carries wired
	// and WiFi clients alike (device-facts §5), so this is Ethernet or WiFi that just left.
	LAN Connection = "lan"
	// Offline: known only from a lease, reservation, alias or block.
	Offline Connection = "offline"
)

// Client is one device, keyed by MAC.
type Client struct {
	MAC        string     `json:"mac" doc:"Lower-case colon MAC"`
	IP         string     `json:"ip,omitempty" doc:"Current IPv4 address (neighbour table, else lease)"`
	Hostname   string     `json:"hostname,omitempty" doc:"Name the client sent with its DHCP request"`
	Alias      string     `json:"alias,omitempty" doc:"Name the admin gave the client"`
	Connection Connection `json:"connection" enum:"wifi,lan,offline"`
	AP         string     `json:"ap,omitempty" doc:"AP interface the station is associated with"`
	SSID       string     `json:"ssid,omitempty" doc:"SSID of that AP"`
	Signal     *int       `json:"signal,omitempty" doc:"Station signal in dBm"`
	// LeaseExpiresIn is relative so clients never compare server time with their clock.
	LeaseExpiresIn *int64 `json:"lease_expires_in,omitempty" doc:"Seconds until the DHCP lease expires; absent for no or an infinite lease"`
	Reserved       bool   `json:"reserved" doc:"A DHCP reservation holds this MAC"`
	Blocked        bool   `json:"blocked" doc:"A Travo block rule rejects this MAC's forwarded traffic"`
}

// AP is one enabled access point interface and its stations.
type AP struct {
	Ifname   string
	SSID     string
	Stations []facts.Station
}

// Inputs are the facts one listing merges. Neighbours must already be LAN-side only.
type Inputs struct {
	Leases       []facts.Lease
	Neighbours   []facts.Neighbour
	APs          []AP
	Reservations []Reservation
	Blocked      []string
	Aliases      map[string]string
	Now          time.Time
}

var macPattern = regexp.MustCompile(`^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`)

// NormalizeMAC lower-cases a colon MAC; ok is false for anything else or a multicast MAC.
func NormalizeMAC(mac string) (string, bool) {
	mac = strings.ToLower(strings.TrimSpace(mac))
	if !macPattern.MatchString(mac) || mac == "00:00:00:00:00:00" {
		return "", false
	}
	if strings.IndexByte("13579bdf", mac[1]) >= 0 {
		return "", false // group bit set
	}
	return mac, true
}

// Merge builds one row per MAC. Pure: every input is passed in.
func Merge(in Inputs) []Client {
	rows := map[string]*Client{}
	row := func(mac string) *Client {
		if c, ok := rows[mac]; ok {
			return c
		}
		c := &Client{MAC: mac, Connection: Offline}
		rows[mac] = c
		return c
	}
	for _, l := range in.Leases {
		c := row(l.MAC)
		c.IP, c.Hostname = l.IP.String(), l.Hostname
		if !l.Expires.IsZero() {
			left := max(int64(l.Expires.Sub(in.Now)/time.Second), 0)
			c.LeaseExpiresIn = &left
		}
	}
	for _, n := range in.Neighbours {
		c := row(n.MAC)
		c.IP, c.Connection = n.IP.String(), LAN
	}
	for _, ap := range in.APs {
		for _, st := range ap.Stations {
			c := row(st.MAC)
			c.Connection, c.AP, c.SSID, c.Signal = WiFi, ap.Ifname, ap.SSID, st.Signal
		}
	}
	for _, r := range in.Reservations {
		row(r.MAC).Reserved = true
	}
	for _, mac := range in.Blocked {
		row(mac).Blocked = true
	}
	for mac, alias := range in.Aliases {
		row(mac).Alias = alias
	}
	out := make([]Client, 0, len(rows))
	for _, mac := range slices.Sorted(maps.Keys(rows)) {
		out = append(out, *rows[mac])
	}
	slices.SortStableFunc(out, compareClients)
	return out
}

var connectionRank = map[Connection]int{WiFi: 0, LAN: 0, Offline: 1}

// compareClients puts online clients first, then orders by IPv4 address; MAC breaks ties.
func compareClients(a, b Client) int {
	if r := cmp.Compare(connectionRank[a.Connection], connectionRank[b.Connection]); r != 0 {
		return r
	}
	ia, ea := netip.ParseAddr(a.IP)
	ib, eb := netip.ParseAddr(b.IP)
	switch {
	case ea == nil && eb == nil:
		return ia.Compare(ib)
	case ea == nil:
		return -1
	case eb == nil:
		return 1
	}
	return 0
}
