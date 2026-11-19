package lanaddress

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Window is the confirm window of a lan change (ADR 0028): the caller reconnects to get a
// lease in the new subnet, opens the new address and logs in again before confirming.
const Window = 180 * time.Second

// ProbeWait bounds the wait for netifd to carry the new address (apply budget 65 s).
const (
	ProbeWait = 20 * time.Second
	probeTick = time.Second
)

var (
	// ErrUnavailable: a config or the live interface dump cannot be read.
	ErrUnavailable = errors.New("lan settings are unavailable")
	// ErrNotProven: the lan interface did not come up with the new address in time.
	ErrNotProven = errors.New("the lan interface did not take the new address")
)

// Transactions is the slice of txn.Manager the lan change needs.
type Transactions interface {
	BeginWindow(ctx context.Context, summary string, configs []string, window time.Duration, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
	Get(id string) (txn.Status, error)
}

// Service reads and changes network.lan's address.
type Service struct {
	UCI  device.UCI
	Ubus device.Ubus
	Txns Transactions
	// Now is a monotonic clock for the remaining window (time.Now in production).
	Now func() time.Time
	// OnMoved runs after the change is armed so the source and Host checks accept the new
	// subnet at once instead of after the next periodic refresh (trust.Provider.Refresh).
	OnMoved func(context.Context)
	// Zero values use ProbeWait and a 1 s tick.
	ProbeWait, Tick time.Duration

	mu      sync.Mutex
	pending pendingChange
}

type pendingChange struct {
	id      string
	address netip.Prefix
	armed   time.Time
	window  time.Duration
}

// Settings reads the committed network.lan and the change waiting for its confirm, if any.
func (s *Service) Settings(ctx context.Context) (View, error) {
	if s.UCI == nil {
		return View{}, ErrUnavailable
	}
	network, err := s.UCI.Show(ctx, "network")
	if err != nil {
		return View{}, ErrUnavailable
	}
	cur, err := read(network)
	if err != nil {
		return View{}, err
	}
	return View{Address: cur.prefix.String(), Subnet: cur.prefix.Masked().String(), Pending: s.open()}, nil
}

// open reports the recorded change while its transaction is still pending.
func (s *Service) open() *Pending {
	s.mu.Lock()
	p := s.pending
	s.mu.Unlock()
	if p.id == "" || s.Txns == nil {
		return nil
	}
	st, err := s.Txns.Get(p.id)
	if err != nil || st.State != txn.Pending {
		return nil
	}
	left := max(p.window-s.Now().Sub(p.armed), 0)
	return &Pending{ID: p.id, Address: p.address.String(), RemainingSeconds: int(left.Round(time.Second) / time.Second)}
}

// Set validates in against the live config and runs the change in one `network` transaction
// with the long Window. The caller's connection to the old address breaks during the apply.
func (s *Service) Set(ctx context.Context, in Request) (txn.Status, error) {
	if s.UCI == nil || s.Ubus == nil || s.Txns == nil || s.Now == nil {
		return txn.Status{}, ErrUnavailable
	}
	next, err := Validate(in)
	if err != nil {
		return txn.Status{}, err
	}
	if _, err := s.plan(ctx, next); err != nil {
		return txn.Status{}, err
	}
	stage := func(ctx context.Context) error {
		ws, err := s.plan(ctx, next) // re-plan under the network lock
		if err != nil {
			return err
		}
		return Stage(ctx, s.UCI, ws)
	}
	probe := func(ctx context.Context) error { return s.prove(ctx, next) }
	st, err := s.Txns.BeginWindow(ctx, "Change LAN address to "+next.String(), []string{"network"}, Window, stage, probe)
	if err == nil && st.State == txn.Pending {
		s.mu.Lock()
		s.pending = pendingChange{id: st.ID, address: next, armed: s.Now(), window: time.Duration(st.WindowSeconds) * time.Second}
		s.mu.Unlock()
		if s.OnMoved != nil {
			s.OnMoved(ctx)
		}
	}
	return st, err
}

func (s *Service) plan(ctx context.Context, next netip.Prefix) ([]Write, error) {
	in, err := s.inputs(ctx)
	if err != nil {
		return nil, err
	}
	return Plan(in, next)
}

func (s *Service) inputs(ctx context.Context) (Inputs, error) {
	var in Inputs
	for _, c := range []struct {
		name string
		dst  *[]device.Section
	}{{"network", &in.Network}, {"dhcp", &in.DHCP}, {"firewall", &in.Firewall}} {
		secs, err := s.UCI.Show(ctx, c.name)
		if err != nil {
			return Inputs{}, ErrUnavailable
		}
		*c.dst = secs
	}
	live, err := s.live(ctx)
	if err != nil {
		return Inputs{}, ErrUnavailable // overlap must not be judged on a partial view
	}
	in.Live = live
	return in, nil
}

func (s *Service) live(ctx context.Context) ([]facts.Interface, error) {
	raw, err := s.Ubus.Call(ctx, "network.interface", "dump", nil)
	if err != nil {
		return nil, err
	}
	return facts.ParseInterfaceDump(raw)
}

// Carries reports whether netifd has the lan interface up with exactly next. netifd sets the
// address itself, so its dump is the source here (route truth is a different matter).
func Carries(live []facts.Interface, next netip.Prefix) bool {
	for _, i := range live {
		if i.Name == Interface && i.Up {
			for _, p := range i.IPv4 {
				if p == next {
					return true
				}
			}
		}
	}
	return false
}

// prove polls the interface dump until lan carries next, bounded by ProbeWait.
func (s *Service) prove(ctx context.Context, next netip.Prefix) error {
	wait, tick := s.ProbeWait, s.Tick
	if wait <= 0 {
		wait = ProbeWait
	}
	if tick <= 0 {
		tick = probeTick
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		if live, err := s.live(ctx); err == nil && Carries(live, next) {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(ErrNotProven, ctx.Err())
		case <-t.C:
		}
	}
}
