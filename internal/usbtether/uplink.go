package usbtether

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Metric keeps the USB uplink behind wan/wwan (netifd default metric 0): it carries the
// kernel default only while they have none. Failover (#34) orders uplinks itself.
const Metric = "10"

// ProbeWait bounds the enable probe: phone DHCP after the link comes up (Android hands out
// a lease in a few seconds; 30 s matches the STA join budget, device-facts §2).
const ProbeWait = 30 * time.Second

var (
	ErrNoDevice    = errors.New("no tethered phone detected")
	ErrAmbiguous   = errors.New("more than one tethered phone; choose a device")
	ErrUnsupported = errors.New("the firewall or network layout is not supported for USB tethering")
	ErrNotUp       = errors.New("the USB uplink did not come up")
	ErrStillUp     = errors.New("the USB uplink is still up")
)

// Transactions is the slice of txn.Manager the uplink change needs.
type Transactions interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
}

// Request enables or disables the USB uplink. Device picks a tether link; empty means the only one.
type Request struct {
	Enabled bool
	Device  string
}

var uplinkConfigs = []string{"network", "firewall"}

// Set changes `network.usbtether` and its wan zone entry in one transaction the user confirms.
// Travo owns only the interface named usbtether (invariants §7) and that one list entry.
func (s *Service) Set(ctx context.Context, req Request) (txn.Status, error) {
	if s.UCI == nil || s.Ubus == nil || s.Links == nil || s.Txns == nil {
		return txn.Status{}, ErrUnavailable
	}
	network, firewall, err := s.read(ctx)
	if err != nil {
		return txn.Status{}, err
	}
	zone, err := wanZone(network, firewall)
	if err != nil {
		return txn.Status{}, err
	}
	if !req.Enabled {
		if !slices.ContainsFunc(network, isUplink) && !slices.Contains(zone.Lists["network"], Interface) {
			return txn.Status{}, txn.ErrNoChanges
		}
		return s.Txns.Begin(ctx, "Disable USB tethering uplink", uplinkConfigs,
			func(c context.Context) error { return s.stageDisable(c, zone) }, s.proveDown)
	}
	dev, err := s.pickDevice(ctx, req.Device)
	if err != nil {
		return txn.Status{}, err
	}
	if enabledAs(network, zone, dev) {
		return txn.Status{}, txn.ErrNoChanges
	}
	return s.Txns.Begin(ctx, "Enable USB tethering uplink on "+dev, uplinkConfigs,
		func(c context.Context) error { return s.stageEnable(c, zone, dev) },
		func(c context.Context) error { return s.proveUp(c, dev) })
}

func (s *Service) read(ctx context.Context) (network, firewall []device.Section, err error) {
	if network, err = s.UCI.Show(ctx, "network"); err != nil {
		return nil, nil, errors.Join(ErrUnavailable, err)
	}
	if firewall, err = s.UCI.Show(ctx, "firewall"); err != nil {
		return nil, nil, errors.Join(ErrUnavailable, err)
	}
	return network, firewall, nil
}

// wanZone refuses before any write: no wan zone, a wan zone whose networks are an option
// (`option network 'wan wan6'`, old syntax), or a non-interface section named usbtether.
func wanZone(network, firewall []device.Section) (device.Section, error) {
	zone, ok := WANZoneSection(firewall)
	if !ok {
		return zone, fmt.Errorf("%w: no zone named %s", ErrUnsupported, WANZone)
	}
	if _, isOption := zone.Options["network"]; isOption {
		return zone, fmt.Errorf("%w: zone %s lists its networks as an option", ErrUnsupported, WANZone)
	}
	if i := slices.IndexFunc(network, func(x device.Section) bool { return x.Name == Interface }); i >= 0 && network[i].Type != "interface" {
		return zone, fmt.Errorf("%w: network.%s is a %s", ErrUnsupported, Interface, network[i].Type)
	}
	return zone, nil
}

func isUplink(x device.Section) bool { return x.Name == Interface && x.Type == "interface" }

func (s *Service) pickDevice(ctx context.Context, want string) (string, error) {
	all, err := s.Links.NetLinks(ctx)
	if err != nil {
		return "", errors.Join(ErrUnavailable, err)
	}
	links := TetherLinks(all)
	if want != "" {
		if slices.ContainsFunc(links, func(l device.NetLink) bool { return l.Name == want }) {
			return want, nil
		}
		return "", ErrNoDevice
	}
	switch len(links) {
	case 0:
		return "", ErrNoDevice
	case 1:
		return links[0].Name, nil
	default:
		return "", ErrAmbiguous
	}
}

func wantOptions(dev string) map[string]string {
	return map[string]string{"proto": "dhcp", "device": dev, "metric": Metric}
}

func enabledAs(network []device.Section, zone device.Section, dev string) bool {
	i := slices.IndexFunc(network, isUplink)
	if i < 0 || !slices.Contains(zone.Lists["network"], Interface) {
		return false
	}
	for k, v := range wantOptions(dev) {
		if network[i].Options[k] != v {
			return false
		}
	}
	return network[i].Options["disabled"] != "1"
}

func (s *Service) stageEnable(ctx context.Context, zone device.Section, dev string) error {
	if err := s.UCI.SetType(ctx, "network", Interface, "interface"); err != nil {
		return err
	}
	for _, k := range []string{"proto", "device", "metric"} {
		if err := s.UCI.Set(ctx, "network", Interface, k, wantOptions(dev)[k]); err != nil {
			return err
		}
	}
	if err := s.UCI.DeleteOption(ctx, "network", Interface, "disabled"); err != nil && !errors.Is(err, device.ErrNotFound) {
		return err
	}
	if slices.Contains(zone.Lists["network"], Interface) {
		return nil
	}
	return s.UCI.AddToList(ctx, "firewall", zone.Name, "network", Interface)
}

// stageDisable deletes the interface and rewrites the zone's network list without
// usbtether, keeping the other entries in order (the port has no del_list).
func (s *Service) stageDisable(ctx context.Context, zone device.Section) error {
	network, err := s.UCI.Show(ctx, "network")
	if err != nil {
		return err
	}
	if slices.ContainsFunc(network, isUplink) {
		if err := s.UCI.DeleteSection(ctx, "network", Interface); err != nil {
			return err
		}
	}
	nets := zone.Lists["network"]
	if !slices.Contains(nets, Interface) {
		return nil
	}
	if err := s.UCI.DeleteOption(ctx, "firewall", zone.Name, "network"); err != nil {
		return err
	}
	for _, n := range nets {
		if n == Interface {
			continue
		}
		if err := s.UCI.AddToList(ctx, "firewall", zone.Name, "network", n); err != nil {
			return err
		}
	}
	return nil
}

// proveUp: the phone link has carrier and netifd has the interface up with a DHCP address.
// netifd's address is the proof here, not its routes (device-facts §1); the kernel default
// may stay on wan/wwan because of Metric.
func (s *Service) proveUp(ctx context.Context, dev string) error {
	return s.poll(ctx, ErrNotUp, func(c context.Context) bool {
		carrier := s.carrier(c, dev)
		if carrier == nil || !*carrier {
			return false
		}
		st, err := s.interfaceStatus(c)
		return err == nil && st.Up && len(st.IPv4) > 0
	})
}

// proveDown: netifd no longer runs usbtether.
func (s *Service) proveDown(ctx context.Context) error {
	return s.poll(ctx, ErrStillUp, func(c context.Context) bool {
		raw, err := s.Ubus.Call(c, "network.interface."+Interface, "status", nil)
		if errors.Is(err, device.ErrNotFound) {
			return true
		}
		if err != nil {
			return false
		}
		st, err := facts.ParseInterfaceStatus(raw)
		return err == nil && !st.Up
	})
}

func (s *Service) poll(ctx context.Context, failed error, ok func(context.Context) bool) error {
	wait := s.ProbeWait
	if wait <= 0 {
		wait = ProbeWait
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		if ok(ctx) {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(failed, ctx.Err())
		case <-tick.C:
		}
	}
}
