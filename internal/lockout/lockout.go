// Package lockout refuses changes that would leave the caller without a way back in
// (invariants §2). Pure: inputs are already-parsed facts; the parsers for neighbour
// table and hostapd station dumps live in internal/device/facts (device captures).
package lockout

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
)

// Method is how the caller reaches the router.
type Method string

const (
	Ethernet   Method = "ethernet"
	WifiAP     Method = "wifi-ap"     // associated to one of our APs
	WifiClient Method = "wifi-client" // reaches us over the uplink STA side (should be blocked anyway)
	Unknown    Method = "unknown"
)

// Facts are the parsed inputs. Neighbours: IP -> MAC (kernel neighbour table).
// Stations: AP ifname -> associated MACs (hostapd get_clients), one entry per enabled AP;
// an AP whose dump failed is ABSENT, never an empty list. EnabledAPs: ifnames that should
// have a station list. UplinkNets: subnets of uplink interfaces (wwan/wan/usbtether).
type Facts struct {
	Neighbours map[netip.Addr]string
	Stations   map[string][]string
	EnabledAPs []string
	BridgeNets []netip.Prefix // br-lan and guest subnets: wired and WiFi clients mixed
	UplinkNets []netip.Prefix
}

// Caller is the classification result.
type Caller struct {
	Method Method `json:"method"`
	// AP is the ifname the caller is associated to (WifiAP only).
	AP string `json:"ap,omitempty"`
}

// Classify. br-lan mixes wired and WiFi clients in one subnet (device-facts §5), so the
// only reliable signal is MAC membership in an AP station list. No MAC = Unknown.
func Classify(ip netip.Addr, f Facts) Caller {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return Caller{Method: Unknown}
	}
	for _, p := range f.UplinkNets {
		if p.Contains(ip) {
			return Caller{Method: WifiClient}
		}
	}
	inBridge := false
	for _, p := range f.BridgeNets {
		inBridge = inBridge || p.Contains(ip)
	}
	if !inBridge {
		return Caller{Method: Unknown}
	}
	mac := normMAC(f.Neighbours[ip])
	if mac == "" {
		return Caller{Method: Unknown}
	}
	aps := make([]string, 0, len(f.Stations))
	for ap := range f.Stations {
		aps = append(aps, ap)
	}
	slices.Sort(aps) // deterministic if a MAC shows up twice (roaming)
	for _, ap := range aps {
		for _, sta := range f.Stations[ap] {
			if normMAC(sta) == mac {
				return Caller{Method: WifiAP, AP: ap}
			}
		}
	}
	// Ethernet is only provable when every enabled AP's station list was read: a missing
	// dump could hide the caller (fail closed, invariants §2). EnabledAPs is the only
	// completeness signal, so without it nothing is provable.
	if len(f.EnabledAPs) == 0 {
		return Caller{Method: Unknown}
	}
	for _, ap := range f.EnabledAPs {
		if _, ok := f.Stations[ap]; !ok {
			return Caller{Method: Unknown}
		}
	}
	return Caller{Method: Ethernet}
}

func normMAC(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 17 || strings.Count(s, ":") != 5 {
		return ""
	}
	return s
}

// AP is an access point as configured after the proposed change.
type AP struct {
	Ifname  string // empty when not yet up; matched by Section then
	Section string
	Enabled bool
	Guest   bool
}

// Plan describes the AP layout before and after a change.
type Plan struct {
	Before []AP
	After  []AP
}

// ErrLockoutRisk is returned with stable code wifi_lockout_risk (409).
var ErrLockoutRisk = errors.New("change would disconnect the caller from every access point")

// Check refuses a plan that leaves a WiFi (or unknown) caller with no enabled AP they can
// use. Ethernet callers are never blocked. Acknowledge is the explicit override.
//
// "Can use" is conservative: the caller's own AP must stay enabled, OR another non-guest
// AP must stay enabled (they can rejoin it). An unknown caller needs a non-guest AP.
func Check(c Caller, p Plan, acknowledge bool) error {
	if acknowledge || c.Method == Ethernet {
		return nil
	}
	var after []AP
	for _, a := range p.After {
		if a.Enabled {
			after = append(after, a)
		}
	}
	if c.Method == WifiAP {
		own := sectionOf(p.Before, c.AP)
		for _, a := range after {
			if (own != "" && a.Section == own) || !a.Guest {
				return nil
			}
		}
		return ErrLockoutRisk
	}
	// Unknown or wifi-client: fail closed unless a non-guest AP stays up.
	for _, a := range after {
		if !a.Guest {
			return nil
		}
	}
	return ErrLockoutRisk
}

func sectionOf(aps []AP, ifname string) string {
	for _, a := range aps {
		if ifname != "" && a.Ifname == ifname {
			return a.Section
		}
	}
	return ""
}
