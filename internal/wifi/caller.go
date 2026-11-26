package wifi

import (
	"context"
	"net/netip"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
)

var uplinkNetworks = []string{"wan", "wwan", "usbtether"}

// classifyCaller reads the live facts ADR 0011 needs and classifies ip. Any unreadable
// input leaves the caller Unknown (fail closed); a station list that cannot be read is
// left out so Classify cannot prove Ethernet.
func classifyCaller(ctx context.Context, ubus device.Ubus, neigh device.NeighbourReader, bridges []string, ip netip.Addr) lockout.Caller {
	unknown := lockout.Caller{Method: lockout.Unknown}
	if ubus == nil || neigh == nil || !ip.IsValid() {
		return unknown
	}
	f, ok := callerFacts(ctx, ubus, neigh, bridges)
	if !ok {
		return unknown
	}
	return lockout.Classify(ip, f)
}

func callerFacts(ctx context.Context, ubus device.Ubus, neigh device.NeighbourReader, bridges []string) (lockout.Facts, bool) {
	f := lockout.Facts{Neighbours: map[netip.Addr]string{}, Stations: map[string][]string{}}
	raw, err := ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return f, false
	}
	ifaces, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return f, false
	}
	for _, iface := range ifaces {
		for _, prefix := range iface.IPv4 {
			switch {
			case slices.Contains(uplinkNetworks, iface.Name):
				f.UplinkNets = append(f.UplinkNets, prefix.Masked())
			case slices.Contains(bridges, iface.Name):
				f.BridgeNets = append(f.BridgeNets, prefix.Masked())
			}
		}
	}
	if raw, err = neigh.Neighbours(ctx); err != nil {
		return f, false
	}
	neighbours, err := facts.ParseNeighJSON(raw)
	if err != nil {
		return f, false
	}
	for _, n := range neighbours {
		f.Neighbours[n.IP] = n.MAC
	}
	if raw, err = ubus.Call(ctx, "network.wireless", "status", nil); err != nil {
		return f, false
	}
	radios, err := facts.ParseWirelessRadios(raw)
	if err != nil {
		return f, false
	}
	for _, radio := range radios {
		for _, iface := range radio.Ifaces {
			if iface.Mode != "ap" || iface.Ifname == "" {
				continue
			}
			f.EnabledAPs = append(f.EnabledAPs, iface.Ifname)
			if raw, err := ubus.Call(ctx, "hostapd."+iface.Ifname, "get_clients", nil); err == nil {
				if stations, err := facts.ParseHostapdClients(raw); err == nil {
					f.Stations[iface.Ifname] = stations
				}
			}
		}
	}
	return f, true
}

// apNetworks lists the logical networks APs bridge into: callers there may be on WiFi.
func apNetworks(wireless []device.Section) []string {
	networks := []string{"lan"}
	for _, section := range wireless {
		if section.Type != "wifi-iface" || section.Options["mode"] != "ap" {
			continue
		}
		for _, network := range sectionNetworks(section) {
			if !slices.Contains(networks, network) {
				networks = append(networks, network)
			}
		}
	}
	return networks
}
