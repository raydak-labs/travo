package wansettings

import (
	"context"
	"errors"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// PPPoEDevice is the kernel link netifd's ppp proto creates for network.wan unless
// `pppname` names another (testdata wan/netifd-proto-ppp.sh.txt).
const PPPoEDevice = "pppoe-" + Interface

// Probe timing: DHCP or PPPoE on a fresh link takes seconds, the transaction's apply budget
// (65 s) bounds the whole commit + reload + probe.
const (
	ProbeWait = 40 * time.Second
	// ProbeSettle lets netifd tear the old protocol down before the first reading, so the
	// previous route is not mistaken for the new one.
	ProbeSettle = 3 * time.Second
	probeTick   = time.Second
)

// ErrUnavailable: network config or the probe inputs cannot be read.
var ErrUnavailable = errors.New("wan settings are unavailable")

// ErrNotProven: the probe did not see the expected kernel default route in time.
var ErrNotProven = errors.New("no default route proven after the wan change")

// Transactions is the slice of txn.Manager the wan change needs.
type Transactions interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
}

// Service reads and changes network.wan.
type Service struct {
	UCI    device.UCI
	Keys   device.WANPasswordWriter
	Ubus   device.Ubus
	Routes device.DefaultRouteReader
	Txns   Transactions
	// Zero values use ProbeWait, ProbeSettle and a 1 s tick.
	ProbeWait, ProbeSettle, Tick time.Duration
}

// Settings reads network.wan. GET never mutates and never returns the password.
func (s *Service) Settings(ctx context.Context) (View, error) {
	network, err := s.UCI.Show(ctx, "network")
	if err != nil {
		return View{}, ErrUnavailable
	}
	return Read(network)
}

// Set validates in and stages it in one `network` transaction the user confirms.
func (s *Service) Set(ctx context.Context, in Request) (txn.Status, error) {
	if s.UCI == nil || s.Ubus == nil || s.Routes == nil || s.Txns == nil {
		return txn.Status{}, ErrUnavailable
	}
	network, err := s.UCI.Show(ctx, "network")
	if err != nil {
		return txn.Status{}, ErrUnavailable
	}
	if _, err := s.plan(network, in); err != nil {
		return txn.Status{}, err
	}
	c := &change{s: s, in: in}
	return s.Txns.Begin(ctx, "Change WAN to "+in.Proto, []string{"network"}, c.stage, c.probe)
}

// change carries what stage records under the transaction lock to the probe after commit.
type change struct {
	s        *Service
	in       Request
	before   device.IPv4DefaultRoute
	hadRoute bool
	wanDev   string
	pppDev   string
}

// stage re-reads and re-plans under the lock, records the default route before the commit.
func (c *change) stage(ctx context.Context) error {
	network, err := c.s.UCI.Show(ctx, "network")
	if err != nil {
		return ErrUnavailable
	}
	p, err := c.s.plan(network, c.in)
	if err != nil {
		return err
	}
	cur, _ := wanSection(network)
	c.wanDev, c.pppDev = cur.Options["device"], cur.Options["pppname"]
	if c.pppDev == "" {
		c.pppDev = PPPoEDevice
	}
	if c.before, c.hadRoute, err = c.s.Routes.DefaultIPv4Route(ctx); err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	return Stage(ctx, c.s.UCI, c.s.Keys, p)
}

func (c *change) probe(ctx context.Context) error {
	routeDev := c.wanDev
	if c.in.Proto == ProtoPPPoE {
		routeDev = c.pppDev
	}
	wasActive := c.hadRoute && (c.before.Device == c.wanDev || c.before.Device == c.pppDev)
	return c.s.prove(ctx, func(ctx context.Context) bool {
		r, found, err := c.s.Routes.DefaultIPv4Route(ctx)
		if err != nil {
			return false
		}
		return Proven(Reading{Carrier: c.s.carrier(ctx, c.wanDev), Route: r, Found: found},
			routeDev, wasActive, c.before, c.hadRoute)
	})
}

// plan validates against the current config and refuses an unchanged request.
func (s *Service) plan(network []device.Section, in Request) (Plan, error) {
	cur, err := read(network)
	if err != nil {
		return Plan{}, err
	}
	v, err := Validate(in, network, cur.PasswordSet)
	if err != nil {
		return Plan{}, err
	}
	p, err := Diff(network, v)
	if err == nil && p.Empty() {
		return Plan{}, txn.ErrNoChanges
	}
	return p, err
}

// Reading is one probe sample: wan carrier and the kernel default route.
type Reading struct {
	Carrier bool
	Route   device.IPv4DefaultRoute
	Found   bool
}

// Proven is the probe rule. With carrier, a kernel default route via routeDev proves the new
// config. Otherwise, when wan was not the active uplink, the previous default route (or its
// absence) must be unchanged: the change did not disturb the uplink in use. When wan was the
// active uplink, only a route via routeDev counts. netifd's view is never consulted.
func Proven(r Reading, routeDev string, wasActive bool, before device.IPv4DefaultRoute, hadRoute bool) bool {
	if r.Carrier && r.Found && r.Route.Device == routeDev {
		return true
	}
	if wasActive {
		return false
	}
	if r.Found != hadRoute {
		return false
	}
	return !hadRoute || r.Route.Device == before.Device && r.Route.Gateway == before.Gateway
}

// prove waits ProbeSettle, then needs two consecutive passing readings within ProbeWait.
func (s *Service) prove(ctx context.Context, ok func(context.Context) bool) error {
	wait, settle, tick := s.ProbeWait, s.ProbeSettle, s.Tick
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
	for {
		select {
		case <-ctx.Done():
			return errors.Join(ErrNotProven, ctx.Err())
		case <-timer.C:
		}
		if ok(ctx) {
			passes++
		} else {
			passes = 0
		}
		if passes == 2 {
			return nil
		}
		timer.Reset(tick)
	}
}

func (s *Service) carrier(ctx context.Context, dev string) bool {
	raw, err := s.Ubus.Call(ctx, "network.device", "status", map[string]string{"name": dev})
	if err != nil {
		return false
	}
	link, err := facts.ParseDeviceStatus(raw)
	return err == nil && link.Carrier
}
