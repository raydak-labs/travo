package ipv6settings

import (
	"context"
	"errors"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Probe timing: the IPv4 uplink must stay as it was while netifd reloads the DHCPv6 clients.
const (
	ProbeWait   = 30 * time.Second
	ProbeSettle = 3 * time.Second
	probeTick   = time.Second
)

// ErrNotProven: the IPv4 default route changed or vanished after the IPv6 change.
var ErrNotProven = errors.New("the IPv4 uplink changed after the IPv6 change")

// Transactions is the slice of txn.Manager the change needs.
type Transactions interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
}

// Service reads and changes the IPv6 switch.
type Service struct {
	UCI    device.UCI
	Routes device.DefaultRouteReader
	Txns   Transactions
	// Zero values use ProbeWait, ProbeSettle and a 1 s tick.
	ProbeWait, ProbeSettle, Tick time.Duration
}

// Configs are the UCI configs one change stages, in one snapshot.
var Configs = []string{"network", "dhcp"}

// Settings never mutates.
func (s *Service) Settings(ctx context.Context) (View, error) {
	network, dhcp, err := s.read(ctx)
	if err != nil {
		return View{}, err
	}
	return Read(network, dhcp)
}

// Set plans against the current config and stages it in one transaction the user confirms.
func (s *Service) Set(ctx context.Context, in Request) (txn.Status, error) {
	if s.UCI == nil || s.Routes == nil || s.Txns == nil {
		return txn.Status{}, ErrUnavailable
	}
	if _, err := s.plan(ctx, in); err != nil {
		return txn.Status{}, err
	}
	c := &change{s: s, in: in}
	summary := "Turn IPv6 off"
	if in.Enabled {
		summary = "Turn IPv6 on"
	}
	return s.Txns.Begin(ctx, summary, Configs, c.stage, c.probe)
}

func (s *Service) read(ctx context.Context) (network, dhcp []device.Section, err error) {
	if network, err = s.UCI.Show(ctx, "network"); err != nil {
		return nil, nil, ErrUnavailable
	}
	if dhcp, err = s.UCI.Show(ctx, "dhcp"); err != nil {
		return nil, nil, ErrUnavailable
	}
	return network, dhcp, nil
}

func (s *Service) plan(ctx context.Context, in Request) ([]Write, error) {
	network, dhcp, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	w, err := Plan(network, dhcp, in)
	if err == nil && len(w) == 0 {
		return nil, txn.ErrNoChanges
	}
	return w, err
}

// change carries the IPv4 default route stage saw to the probe.
type change struct {
	s        *Service
	in       Request
	before   device.IPv4DefaultRoute
	hadRoute bool
}

// stage re-plans under the transaction lock and records the IPv4 default route.
func (c *change) stage(ctx context.Context) error {
	w, err := c.s.plan(ctx, c.in)
	if err != nil {
		return err
	}
	if c.before, c.hadRoute, err = c.s.Routes.DefaultIPv4Route(ctx); err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	return Stage(ctx, c.s.UCI, w)
}

// probe needs two consecutive readings with the IPv4 default route as stage saw it.
func (c *change) probe(ctx context.Context) error {
	wait, settle, tick := c.s.ProbeWait, c.s.ProbeSettle, c.s.Tick
	if wait <= 0 {
		wait = ProbeWait
	}
	if settle <= 0 {
		settle = ProbeSettle
	}
	if tick <= 0 {
		tick = probeTick
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	timer := time.NewTimer(settle)
	defer timer.Stop()
	passes := 0
	for passes < 2 {
		select {
		case <-ctx.Done():
			return errors.Join(ErrNotProven, ctx.Err())
		case <-timer.C:
		}
		r, found, err := c.s.Routes.DefaultIPv4Route(ctx)
		if err == nil && Unchanged(r, found, c.before, c.hadRoute) {
			passes++
		} else {
			passes = 0
		}
		timer.Reset(tick)
	}
	return nil
}

// Unchanged is the probe rule: the same IPv4 default route (device and gateway), or still none.
func Unchanged(r device.IPv4DefaultRoute, found bool, before device.IPv4DefaultRoute, hadRoute bool) bool {
	if found != hadRoute {
		return false
	}
	return !found || r.Device == before.Device && r.Gateway == before.Gateway
}

// Stage applies the writes in order.
func Stage(ctx context.Context, uci device.UCI, writes []Write) error {
	for _, w := range writes {
		var err error
		switch w.Op {
		case "type":
			err = uci.SetType(ctx, w.Config, w.Section, w.Value)
		case "set":
			err = uci.Set(ctx, w.Config, w.Section, w.Option, w.Value)
		default:
			err = uci.DeleteOption(ctx, w.Config, w.Section, w.Option)
			if errors.Is(err, device.ErrNotFound) {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}
