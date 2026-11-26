package mock

import (
	"context"
	_ "embed"
	"fmt"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

//go:embed testdata/dhcp-leases.txt
var dhcpLeases []byte

//go:embed testdata/ip-neigh-clients.json
var clientNeighbours []byte

//go:embed testdata/hostapd-phy1-ap0-get_clients.json
var apClients []byte

//go:embed testdata/interface-dump.json
var interfaceDump []byte

// SeedClients scripts one leased WiFi station on phy1-ap0, a wired neighbour and the
// interface dump that tells LAN from uplink neighbours, with the verbatim AXT1800 captures
// (testdata/PROVENANCE.md).
func SeedClients(d *Device) {
	d.SetLeases(dhcpLeases)
	d.SetNeighbours(clientNeighbours)
	d.SetUbusReply("hostapd.phy1-ap0", "get_clients", apClients)
	d.SetUbusReply("network.interface", "dump", interfaceDump)
}

// SetLeases scripts the dnsmasq lease file; unset reads as an empty file.
func (d *Device) SetLeases(b []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.leases = b
}

func (d *Device) DHCPLeases(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.LeasesErr != nil {
		return nil, d.LeasesErr
	}
	return slices.Clone(d.leases), nil
}

// KickStation records "ifname mac" when the AP's scripted get_clients lists the station.
func (d *Device) KickStation(ctx context.Context, ifname, mac string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.ubus["hostapd."+ifname+".get_clients"]; !ok {
		return fmt.Errorf("kick %s: %w", ifname, device.ErrNotFound)
	}
	d.Kicks = append(d.Kicks, ifname+" "+mac)
	return d.KickErr
}
