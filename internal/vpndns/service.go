package vpndns

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

// Tunnel reads the classified WireGuard state.
type Tunnel interface {
	Status(ctx context.Context) (vpnstatus.TunnelStatus, error)
}

// Profiles reads the selected WireGuard profile; its DNS lines are the tunnel resolvers.
type Profiles interface {
	SelectedConfig() (vpnprofiles.Config, error)
}

// Layers is the shared DNS layer stack (dnslayer.Service).
type Layers interface {
	Load() (*dnslayer.Record, error)
	Status() (dnslayer.Status, error)
	Push(ctx context.Context, l dnslayer.Layer) error
	Pop(ctx context.Context, name dnslayer.Name) error
}

// Lookup resolves host through the router's own dnsmasq. A not-found answer is a
// success: the upstream answered. Any other error means resolution failed.
type Lookup func(ctx context.Context, host string) error

type Service struct {
	Tunnel   Tunnel
	Profiles Profiles
	Layers   Layers
	// ReadDNS reads the live dnsmasq options the layer stack owns.
	ReadDNS func(ctx context.Context) (dnslayer.State, error)
	// TxnOpen reports a pending transaction (guard present).
	TxnOpen func() (bool, error)
	Lookup  Lookup
	// Nonce returns a fresh DNS label so the verify lookup cannot be answered from cache.
	Nonce func() string

	mu sync.Mutex
}

// VerifyDomain is the zone the verify lookup asks under; spec item 3 already contacts it.
const VerifyDomain = "connectivitycheck.gstatic.com"

var ErrUnavailable = errors.New("VPN DNS is unavailable")

// Reconcile moves the `vpn` layer one step towards what the tunnel state asks for.
// It is the scheduler of invariants §8: its writes run under the layer stack's guard.
func (s *Service) Reconcile(ctx context.Context) (Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	layers, err := s.Layers.Status()
	if err != nil {
		return Keep, err
	}
	if layers.Pending || layers.Corrupt != "" {
		return Keep, nil // a failed stack change waits for its explicit retry
	}
	st, err := s.Tunnel.Status(ctx)
	if err != nil {
		return Keep, nil
	}
	open, err := s.TxnOpen()
	if err != nil {
		return Keep, err
	}
	resolvers, err := s.resolvers()
	if err != nil {
		return Keep, nil // unreadable profile store: change nothing
	}
	rec, err := s.Layers.Load()
	if err != nil {
		return Keep, err
	}
	action := Decide(st, resolvers, vpnLayer(rec), open)
	switch action {
	case Push:
		err = s.Layers.Push(ctx, Layer(resolvers))
	case Pop:
		err = s.Layers.Pop(ctx, dnslayer.VPN)
	}
	return action, err
}

func (s *Service) resolvers() ([]netip.Addr, error) {
	cfg, err := s.Profiles.SelectedConfig()
	if errors.Is(err, vpnprofiles.ErrNotFound) {
		return nil, nil
	}
	return cfg.DNS, err
}

func vpnLayer(rec *dnslayer.Record) *dnslayer.Layer {
	if rec == nil {
		return nil
	}
	for i := range rec.Layers {
		if rec.Layers[i].Name == dnslayer.VPN {
			return &rec.Layers[i]
		}
	}
	return nil
}

// LayerState is the `vpn` layer as the UI sees it.
type LayerState string

const (
	Active   LayerState = "active"
	Inactive LayerState = "inactive"
	// Failed: a layer stack change did not finish; the guard is kept until an explicit retry.
	Failed LayerState = "failed"
)

type DNSStatus struct {
	Layer LayerState `json:"layer" enum:"active,inactive,failed"`
	// Resolvers are the selected profile's DNS servers ([] when it has none).
	Resolvers []string `json:"resolvers"`
	// Effective is dnsmasq's live server list; null when it could not be read.
	Effective []string `json:"effective" nullable:"true"`
	// Top is the highest-precedence active replace layer, null without one.
	Top *string `json:"top" nullable:"true" doc:"captive, adguard or vpn"`
}

// Status never mutates (invariants §8).
func (s *Service) Status(ctx context.Context) (DNSStatus, error) {
	st, _, err := s.status(ctx)
	return st, err
}

func (s *Service) status(ctx context.Context) (DNSStatus, *dnslayer.State, error) {
	layers, err := s.Layers.Status()
	if err != nil {
		return DNSStatus{}, nil, ErrUnavailable
	}
	out := DNSStatus{Layer: Inactive, Resolvers: []string{}}
	if resolvers, err := s.resolvers(); err == nil {
		out.Resolvers = servers(resolvers)
	}
	var rec *dnslayer.Record
	if layers.Corrupt == "" {
		if rec, err = s.Layers.Load(); err != nil {
			return DNSStatus{}, nil, ErrUnavailable
		}
	}
	switch {
	case layers.Pending || layers.Corrupt != "":
		out.Layer = Failed
	case vpnLayer(rec) != nil:
		out.Layer = Active
	}
	if rec != nil {
		out.Top = top(rec.Layers)
	}
	live, err := s.ReadDNS(ctx)
	if err != nil {
		return out, nil, nil
	}
	out.Effective = slices.Clone(live.Server.Values)
	if out.Effective == nil {
		out.Effective = []string{}
	}
	return out, &live, nil
}

// top mirrors dnslayer precedence: the first replace layer in the stored (sorted) order.
func top(layers []dnslayer.Layer) *string {
	for _, l := range layers {
		if l.Name != dnslayer.Tailscale {
			name := string(l.Name)
			return &name
		}
	}
	return nil
}

// Result of a DNS verification.
type Result string

const (
	OK Result = "ok"
	// NotActive: the `vpn` layer is not applied (tunnel off, not yet connected, or no resolvers).
	NotActive Result = "not_active"
	// Overridden: a higher layer (captive, adguard) owns dnsmasq's servers right now.
	Overridden Result = "overridden"
	// Mismatch: dnsmasq's live servers differ from the tunnel resolvers.
	Mismatch     Result = "mismatch"
	LookupFailed Result = "lookup_failed"
	// LayerFailed: the layer stack has an unfinished change.
	LayerFailed Result = "layer_failed"
)

type DNSVerification struct {
	Result Result    `json:"result" enum:"ok,not_active,overridden,mismatch,lookup_failed,layer_failed"`
	Status DNSStatus `json:"status"`
}

// Verify checks configuration first, then resolves a fresh name through dnsmasq.
func (s *Service) Verify(ctx context.Context) (DNSVerification, error) {
	st, live, err := s.status(ctx)
	if err != nil {
		return DNSVerification{}, err
	}
	v := DNSVerification{Status: st}
	switch {
	case st.Layer == Failed:
		v.Result = LayerFailed
	case st.Layer != Active:
		v.Result = NotActive
	case st.Top == nil || *st.Top != string(dnslayer.VPN):
		v.Result = Overridden
	case live == nil:
		return DNSVerification{}, ErrUnavailable
	case !slices.Equal(live.Server.Values, st.Resolvers) || live.NoResolv.Value != "1":
		v.Result = Mismatch
	default:
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.Lookup(ctx, s.Nonce()+"."+VerifyDomain); err != nil {
			v.Result = LookupFailed
		} else {
			v.Result = OK
		}
	}
	return v, nil
}

// LocalLookup resolves through the DNS server at addr (dnsmasq, "127.0.0.1:53").
func LocalLookup(addr string) Lookup {
	r := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}}
	return func(ctx context.Context, host string) error {
		_, err := r.LookupHost(ctx, host)
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return nil
		}
		return err
	}
}
