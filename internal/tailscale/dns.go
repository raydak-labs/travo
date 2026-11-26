package tailscale

import (
	"context"
	"regexp"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
)

// QuadHundred is tailscaled's in-process MagicDNS resolver; with --accept-dns=false it still
// answers, only the OS resolver config is left alone (docs/adr/0017).
const QuadHundred = "100.100.100.100"

// Layers is the shared DNS layer stack (dnslayer.Service).
type Layers interface {
	Load() (*dnslayer.Record, error)
	Status() (dnslayer.Status, error)
	Push(ctx context.Context, l dnslayer.Layer) error
	Pop(ctx context.Context, name dnslayer.Name) error
}

// Action is what one reconcile tick did.
type Action int

const (
	Keep Action = iota
	PushLayer
	PopLayer
)

// suffixPattern accepts DNS names only: the suffix lands inside a dnsmasq `server=/<suffix>/`
// entry, where `/` or `#` would change its meaning.
var suffixPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)+$`)

// Want is pure: the observed status in, the desired layer out. known=false means the
// state is transient (Starting, NeedsMachineAuth) and the stack keeps what it has.
func Want(st Status) (layer *dnslayer.Layer, known bool) {
	if !st.Installed || !st.Running {
		return nil, true
	}
	switch st.State {
	case StateRunning:
	case StateNeedsLogin, StateStopped, StateNoState:
		return nil, true
	default:
		return nil, false
	}
	if !st.MagicDNS.Enabled || !suffixPattern.MatchString(st.MagicDNS.Suffix) {
		return nil, true
	}
	return &dnslayer.Layer{Name: dnslayer.Tailscale, Servers: []string{"/" + st.MagicDNS.Suffix + "/" + QuadHundred}}, true
}

// Decide is pure: desired and current layer in, the step out.
func Decide(want *dnslayer.Layer, known bool, current *dnslayer.Layer) Action {
	switch {
	case !known:
		return Keep
	case want == nil && current == nil:
		return Keep
	case want == nil:
		return PopLayer
	case current != nil && slices.Equal(current.Servers, want.Servers):
		return Keep
	default:
		return PushLayer
	}
}

// DNS reconciles the `tailscale` layer (invariants §8). It is a scheduler with the stack's
// own guard; unreadable inputs and held guards change nothing.
type DNS struct {
	Service *Service
	Layers  Layers
}

// Tick moves the layer one step towards what tailscaled reports.
func (d *DNS) Tick(ctx context.Context) (Action, error) {
	layers, err := d.Layers.Status()
	if err != nil || layers.Pending || layers.Corrupt != "" {
		return Keep, err // a failed stack change waits for its explicit retry
	}
	if held, err := d.Service.busy(); err != nil || held {
		return Keep, err
	}
	st, err := d.Service.Status(ctx)
	if err != nil {
		return Keep, nil // a broken read is not a logged-out tailnet
	}
	rec, err := d.Layers.Load()
	if err != nil {
		return Keep, err
	}
	want, known := Want(st)
	action := Decide(want, known, current(rec))
	switch action {
	case PushLayer:
		err = d.Layers.Push(ctx, *want)
	case PopLayer:
		err = d.Layers.Pop(ctx, dnslayer.Tailscale)
	}
	return action, err
}

func current(rec *dnslayer.Record) *dnslayer.Layer {
	if rec == nil {
		return nil
	}
	for i := range rec.Layers {
		if rec.Layers[i].Name == dnslayer.Tailscale {
			return &rec.Layers[i]
		}
	}
	return nil
}
