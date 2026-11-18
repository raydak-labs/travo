// Package usbtether makes a tethered phone an uplink (#30): detection by medium
// (device-facts §12), the guarded driver install and the `usbtether` interface in the wan zone.
package usbtether

import (
	"context"
	"errors"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

const (
	// Interface is the logical uplink name of invariants §7.
	Interface = "usbtether"
	// WANZone is the firewall zone the uplink joins, resolved by its `name` (invariants §7).
	WANZone = "wan"
)

// ErrUnavailable: a port this read needs is missing or failed.
var ErrUnavailable = errors.New("USB tethering status is unavailable")

// Phone selects a driver set.
type Phone string

const (
	Android Phone = "android"
	IOS     Phone = "ios"
)

// Drivers per phone, checked against the 25.12.6 feed
// (testdata/device/axt1800-25.12.6/usbtether/apk-packages.txt). cdc_ncm and rndis pull
// kmod-usb-net-cdc-ether; usbmuxd pulls libimobiledevice and libusbmuxd.
var Drivers = map[Phone][]string{
	Android: {"kmod-usb-net-rndis", "kmod-usb-net-cdc-ncm"},
	IOS:     {"kmod-usb-net-ipheth", "usbmuxd"},
}

// Link is a detected tether netdev.
type Link struct {
	Name string `json:"name" doc:"Kernel netdev name (usb0, or eth* for iPhones and some Android phones)"`
	Kind string `json:"kind" enum:"rndis,ncm,ecm,ipheth" doc:"Tether protocol from the kernel driver"`
	// Carrier is null when netifd could not be read.
	Carrier *bool `json:"carrier" nullable:"true"`
}

// Driver is one package of a driver set.
type Driver struct {
	Package string `json:"package"`
	Phone   string `json:"phone" enum:"android,ios"`
	// State unknown: the package manager query failed; never read as absent (invariants §13).
	State string `json:"state" enum:"installed,absent,unknown"`
}

// Status is a read-only view; GET never mutates.
type Status struct {
	Links   []Link   `json:"links"`
	Drivers []Driver `json:"drivers"`
	// Configured: `network.usbtether` exists.
	Configured bool   `json:"configured"`
	Device     string `json:"device" doc:"Netdev network.usbtether binds; empty when not configured"`
	InWANZone  bool   `json:"in_wan_zone" doc:"The wan firewall zone lists usbtether"`
	Up         bool   `json:"up" doc:"netifd reports usbtether up"`
	IPv4       string `json:"ipv4" doc:"First IPv4 address in CIDR form; empty when none"`
}

// Service reads detection and configuration state.
type Service struct {
	UCI      device.UCI
	Ubus     device.Ubus
	Links    device.NetLinkReader
	Packages pkgmgr.PackageManager
}

// Status reads links, driver packages, UCI and netifd. A failed link, UCI or ubus read is
// ErrUnavailable; a failed package query only marks that driver unknown.
func (s *Service) Status(ctx context.Context) (Status, error) {
	if s.UCI == nil || s.Ubus == nil || s.Links == nil {
		return Status{}, ErrUnavailable
	}
	links, err := s.Links.NetLinks(ctx)
	if err != nil {
		return Status{}, errors.Join(ErrUnavailable, err)
	}
	network, err := s.UCI.Show(ctx, "network")
	if err != nil {
		return Status{}, errors.Join(ErrUnavailable, err)
	}
	firewall, err := s.UCI.Show(ctx, "firewall")
	if err != nil {
		return Status{}, errors.Join(ErrUnavailable, err)
	}
	st := Status{Links: s.tetherLinks(ctx, links), Drivers: s.drivers(ctx)}
	if sec, ok := find(network, Interface); ok && sec.Type == "interface" {
		st.Configured, st.Device = true, sec.Options["device"]
	}
	if zone, ok := WANZoneSection(firewall); ok {
		st.InWANZone = slices.Contains(zone.Lists["network"], Interface)
	}
	if st.Configured {
		iface, err := s.interfaceStatus(ctx)
		if err != nil {
			return Status{}, errors.Join(ErrUnavailable, err)
		}
		st.Up = iface.Up
		if len(iface.IPv4) > 0 {
			st.IPv4 = iface.IPv4[0].String()
		}
	}
	return st, nil
}

// TetherLinks keeps the USB netdevs a phone driver owns, in name order.
func TetherLinks(links []device.NetLink) []device.NetLink {
	var out []device.NetLink
	for _, l := range links {
		if _, ok := facts.Tether(l); ok {
			out = append(out, l)
		}
	}
	return out
}

func (s *Service) tetherLinks(ctx context.Context, links []device.NetLink) []Link {
	out := []Link{}
	for _, l := range TetherLinks(links) {
		kind, _ := facts.Tether(l)
		out = append(out, Link{Name: l.Name, Kind: string(kind), Carrier: s.carrier(ctx, l.Name)})
	}
	return out
}

func (s *Service) carrier(ctx context.Context, name string) *bool {
	raw, err := s.Ubus.Call(ctx, "network.device", "status", map[string]string{"name": name})
	if err != nil {
		return nil
	}
	link, err := facts.ParseDeviceStatus(raw)
	if err != nil {
		return nil
	}
	return &link.Carrier
}

func (s *Service) drivers(ctx context.Context) []Driver {
	out := []Driver{}
	for _, phone := range []Phone{Android, IOS} {
		for _, pkg := range Drivers[phone] {
			out = append(out, Driver{Package: pkg, Phone: string(phone), State: s.packageState(ctx, pkg)})
		}
	}
	return out
}

func (s *Service) packageState(ctx context.Context, pkg string) string {
	if s.Packages == nil {
		return "unknown"
	}
	_, err := s.Packages.Version(ctx, pkg)
	switch {
	case err == nil:
		return "installed"
	case errors.Is(err, pkgmgr.ErrNotInstalled):
		return "absent"
	default:
		return "unknown"
	}
}

// interfaceStatus: netifd has no object for an interface it has not set up yet; that is down.
func (s *Service) interfaceStatus(ctx context.Context) (facts.InterfaceStatus, error) {
	raw, err := s.Ubus.Call(ctx, "network.interface."+Interface, "status", nil)
	if errors.Is(err, device.ErrNotFound) {
		return facts.InterfaceStatus{}, nil
	}
	if err != nil {
		return facts.InterfaceStatus{}, err
	}
	return facts.ParseInterfaceStatus(raw)
}

// WANZoneSection finds the first zone named wan, often the anonymous `@zone[1]`.
func WANZoneSection(firewall []device.Section) (device.Section, bool) {
	i := slices.IndexFunc(firewall, func(x device.Section) bool {
		return x.Type == "zone" && x.Options["name"] == WANZone
	})
	if i < 0 {
		return device.Section{}, false
	}
	return firewall[i], true
}

func find(secs []device.Section, name string) (device.Section, bool) {
	i := slices.IndexFunc(secs, func(x device.Section) bool { return x.Name == name })
	if i < 0 {
		return device.Section{}, false
	}
	return secs[i], true
}
