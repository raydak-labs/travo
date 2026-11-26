package clients

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

var (
	ErrUnavailable = errors.New("client data is unavailable")
	// ErrIsCaller: the change would cut off the device the request came from.
	ErrIsCaller = errors.New("the target is the caller's own device")
	// ErrNotAssociated: no enabled AP lists the station.
	ErrNotAssociated = errors.New("the client is not associated with an AP")
	// ErrTransactionPending: another guarded change awaits confirmation.
	ErrTransactionPending = errors.New("a transaction is already pending")
)

// Transactions is the slice of txn.Manager the service uses: Begin for firewall (blocks),
// Apply for dhcp (reservations), which cannot cut the caller off.
type Transactions interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
	Apply(ctx context.Context, configs []string, stage func(context.Context) error) error
}

// Guards is the slice of guard.Dir the kick uses.
type Guards interface {
	Acquire(name string) error
	Release(name string) error
}

// Service reads and changes clients. Every port is required; New-style wiring lives in cmd.
type Service struct {
	UCI    device.UCI
	Ubus   device.Ubus
	Neigh  device.NeighbourReader
	Leases device.DHCPLeaseReader
	Kicker device.StationKicker
	// Reload finishes a dhcp commit whose reload failed (idempotent reload_config).
	Reload  device.Reloader
	Txns    Transactions
	Guards  Guards
	Aliases *AliasStore
	Now     func() time.Time
}

// Listing is GET /clients: Partial is true when an optional source could not be read.
type Listing struct {
	Clients []Client
	Partial bool
}

// uplinkInterfaces are netifd interfaces on the internet side (invariants §7) plus the tunnel.
var uplinkInterfaces = []string{"wan", "wan6", "wwan", "usbtether", "wg0", "loopback"}

// List merges every source. UCI dhcp and the alias store are required; leases, neighbours,
// stations and firewall are optional and drop out with Partial set. GET never mutates.
func (s *Service) List(ctx context.Context) (Listing, error) {
	dhcp, err := s.UCI.Show(ctx, "dhcp")
	if err != nil {
		return Listing{}, ErrUnavailable
	}
	aliases, err := s.Aliases.All()
	if err != nil {
		return Listing{}, err
	}
	in := Inputs{Reservations: Reservations(dhcp), Aliases: aliases, Now: s.Now(), Blocked: []string{}}
	partial := false
	if in.Leases, err = s.leases(ctx); err != nil {
		partial = true
	}
	if in.Neighbours, err = s.lanNeighbours(ctx); err != nil {
		partial = true
	}
	if in.APs, err = s.stations(ctx); err != nil {
		partial = true
	}
	if fw, err := s.UCI.Show(ctx, "firewall"); err == nil {
		in.Blocked = Blocked(fw)
	} else {
		partial = true
	}
	return Listing{Clients: Merge(in), Partial: partial}, nil
}

func (s *Service) leases(ctx context.Context) ([]facts.Lease, error) {
	raw, err := s.Leases.DHCPLeases(ctx)
	if err != nil {
		return nil, err
	}
	return facts.ParseLeases(string(raw))
}

// lanNeighbours keeps neighbours on a non-uplink interface's device (br-lan, br-guest), so the
// uplink's gateway and hotel neighbours never show as clients, even when a hotel network
// uses the same prefix as the LAN.
func (s *Service) lanNeighbours(ctx context.Context) ([]facts.Neighbour, error) {
	raw, err := s.Ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return nil, err
	}
	ifaces, err := facts.ParseInterfaceDump(raw)
	if err != nil {
		return nil, err
	}
	var lanDevs []string
	for _, it := range ifaces {
		if !slices.Contains(uplinkInterfaces, it.Name) && it.L3Device != "" {
			lanDevs = append(lanDevs, it.L3Device)
		}
	}
	if raw, err = s.Neigh.Neighbours(ctx); err != nil {
		return nil, err
	}
	all, err := facts.ParseNeighJSON(raw)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(n facts.Neighbour) bool { return !slices.Contains(lanDevs, n.Dev) }), nil
}

// stations reads get_clients of every AP in network.wireless status. An AP whose station
// list cannot be read makes the result partial but keeps the others (device-facts §2).
func (s *Service) stations(ctx context.Context) ([]AP, error) {
	raw, err := s.Ubus.Call(ctx, "network.wireless", "status", nil)
	if err != nil {
		return nil, err
	}
	radios, err := facts.ParseWirelessRadios(raw)
	if err != nil {
		return nil, err
	}
	var out []AP
	var failed error
	for _, r := range radios {
		for _, iface := range r.Ifaces {
			if iface.Mode != "ap" || iface.Ifname == "" {
				continue
			}
			raw, err := s.Ubus.Call(ctx, "hostapd."+iface.Ifname, "get_clients", nil)
			if err != nil {
				failed = err
				continue
			}
			st, err := facts.ParseHostapdStations(raw)
			if err != nil {
				failed = err
				continue
			}
			out = append(out, AP{Ifname: iface.Ifname, SSID: iface.SSID, Stations: st})
		}
	}
	return out, failed
}

// SetAlias names a client; an empty alias clears it. Not device config: no transaction.
func (s *Service) SetAlias(mac, alias string) error {
	m, ok := NormalizeMAC(mac)
	if !ok {
		return &FieldError{"mac", "not a unicast MAC address"}
	}
	return s.Aliases.Set(m, alias)
}

// callerMAC maps the caller's address to a MAC with the neighbour table and the leases.
// Unknown callers (loopback, no entry) have no MAC; an unreadable table is an error.
func (s *Service) callerMAC(ctx context.Context, caller netip.Addr) (string, error) {
	if !caller.IsValid() {
		return "", nil
	}
	caller = caller.Unmap()
	neigh, err := s.lanNeighbours(ctx)
	if err != nil {
		return "", ErrUnavailable
	}
	var macs []string
	for _, n := range neigh {
		if n.IP == caller && !slices.Contains(macs, n.MAC) {
			macs = append(macs, n.MAC)
		}
	}
	switch len(macs) {
	case 1:
		return macs[0], nil
	case 0:
	default:
		return "", ErrUnavailable // two LAN devices claim the caller's address: cannot tell
	}
	leases, err := s.leases(ctx)
	if err != nil {
		return "", ErrUnavailable
	}
	if at := slices.IndexFunc(leases, func(l facts.Lease) bool { return l.IP == caller }); at >= 0 {
		return leases[at].MAC, nil
	}
	return "", nil
}

func (s *Service) refuseCaller(ctx context.Context, mac string, caller netip.Addr) error {
	own, err := s.callerMAC(ctx, caller)
	if err != nil {
		return err
	}
	if own == mac {
		return ErrIsCaller
	}
	return nil
}
