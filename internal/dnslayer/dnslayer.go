// Package dnslayer owns the single ordered stack of DNS layers on dnsmasq (invariants §8).
// The core is pure: given base and active layers it computes the effective dnsmasq options.
package dnslayer

import (
	"errors"
	"fmt"
	"slices"
)

// Name identifies a layer. Precedence is fixed by name, never by push order,
// so the effective state never depends on history.
type Name string

const (
	Captive   Name = "captive"   // hotel DNS during a portal; must win over everything
	AdGuard   Name = "adguard"   // dnsmasq -> 127.0.0.1#5353
	VPN       Name = "vpn"       // tunnel resolvers
	Tailscale Name = "tailscale" // reserved; split-domain, additive
)

// precedence: higher wins for Replace layers.
var precedence = map[Name]int{Captive: 40, AdGuard: 30, VPN: 20, Tailscale: 10}

// additive is fixed per name so a caller cannot turn captive into an append-only layer.
var additive = map[Name]bool{Tailscale: true}

// Opt is an option that may be absent; absent restores by deleting the option.
type Opt struct {
	Set   bool   `json:"set"`
	Value string `json:"value,omitempty"`
}

// List is a list option that may be absent. Scalar records that base held
// `option server` rather than `list server`, so restore writes the same shape.
type List struct {
	Set    bool     `json:"set"`
	Values []string `json:"values,omitempty"`
	Scalar bool     `json:"scalar,omitempty"`
}

// State is the slice of dnsmasq config the stack owns.
type State struct {
	Server           List `json:"server"`
	NoResolv         Opt  `json:"noresolv"`
	RebindProtection Opt  `json:"rebind_protection"`
	// Order is the owned options' order in the section when read. Restore rewrites
	// options so base comes back in this order (#157). Empty: order unknown.
	Order []string `json:"order,omitempty"`
}

// Layer is what one feature wants. Replace layers override servers/noresolv (highest
// precedence wins). Additive layers (by name, see additive) append domain-scoped servers.
type Layer struct {
	Name     Name     `json:"name"`
	Servers  []string `json:"servers"`
	NoResolv bool     `json:"noresolv,omitempty"`
	// DisableRebind turns rebind protection off (captive: hotel portals answer with RFC1918).
	DisableRebind bool `json:"disable_rebind,omitempty"`
}

// Record is the persisted stack. Base is captured before the first layer.
type Record struct {
	Version int     `json:"version"`
	Base    State   `json:"base"`
	Layers  []Layer `json:"layers"`
	// Restoring marks an in-flight last pop: Resume must restore base, not re-apply layers.
	Restoring bool `json:"restoring,omitempty"`
}

const recordVersion = 1

var (
	ErrUnknownLayer = errors.New("unknown DNS layer")
	ErrNoServers    = errors.New("replace layer needs at least one server")
	ErrBadRecord    = errors.New("corrupt DNS layer record")
)

func validate(l Layer) error {
	if _, ok := precedence[l.Name]; !ok {
		return fmt.Errorf("%w: %q", ErrUnknownLayer, l.Name)
	}
	if len(l.Servers) == 0 {
		return fmt.Errorf("%s: %w", l.Name, ErrNoServers)
	}
	if additive[l.Name] && l.NoResolv {
		return fmt.Errorf("%s: additive layer cannot set noresolv", l.Name)
	}
	for _, s := range l.Servers {
		if s == "" {
			return fmt.Errorf("%s: empty server entry", l.Name)
		}
	}
	return nil
}

// Push adds or replaces a layer (idempotent). rec is nil when no layer is active;
// current is the live dnsmasq state, captured as base only on the first push.
func Push(rec *Record, current State, l Layer) (Record, error) {
	if err := validate(l); err != nil {
		return Record{}, err
	}
	var out Record
	if rec == nil {
		out = Record{Version: recordVersion, Base: current}
	} else {
		if err := Check(*rec); err != nil {
			return Record{}, err
		}
		out = clone(*rec)
		out.Restoring = false // live state is mid-restore; the saved base stays authoritative
	}
	out.Layers = slices.DeleteFunc(out.Layers, func(x Layer) bool { return x.Name == l.Name })
	out.Layers = append(out.Layers, l)
	sortLayers(out.Layers)
	return out, nil
}

// Pop removes only the named layer. It returns nil when the last layer left:
// the caller then marks the record Restoring, writes base, and deletes the record.
func Pop(rec *Record, name Name) (*Record, error) {
	if rec == nil {
		return nil, nil // nothing active: popping is a no-op, not an error
	}
	if err := Check(*rec); err != nil {
		return nil, err
	}
	out := clone(*rec)
	out.Layers = slices.DeleteFunc(out.Layers, func(x Layer) bool { return x.Name == name })
	if len(out.Layers) == 0 {
		return nil, nil
	}
	return &out, nil
}

// Effective computes the dnsmasq state to write. A nil record means base itself,
// which the caller supplies from the last record before deleting it.
func Effective(rec Record) State {
	st := cloneState(rec.Base)
	var top *Layer
	var extra []string
	for i := range rec.Layers {
		l := &rec.Layers[i]
		if additive[l.Name] {
			extra = append(extra, l.Servers...)
		} else if top == nil || precedence[l.Name] > precedence[top.Name] {
			top = l
		}
		if l.DisableRebind {
			st.RebindProtection = Opt{Set: true, Value: "0"}
		}
	}
	if top != nil {
		st.Server = List{Set: true, Values: slices.Clone(top.Servers)}
		st.NoResolv = Opt{Set: true, Value: "0"}
		if top.NoResolv {
			st.NoResolv.Value = "1"
		}
	}
	if len(extra) > 0 {
		st.Server = List{Set: true, Values: append(slices.Clone(st.Server.Values), extra...)}
	}
	if top != nil || len(extra) > 0 {
		st.Server.Scalar = false // layered state is always written as a list
	}
	return st
}

// Active reports whether a layer is on the stack. Reads never mutate.
func Active(rec *Record, name Name) bool {
	if rec == nil {
		return false
	}
	return slices.ContainsFunc(rec.Layers, func(x Layer) bool { return x.Name == name })
}

// Check rejects records a crash or hand edit could have produced.
func Check(r Record) error {
	if r.Version != recordVersion {
		return fmt.Errorf("%w: version %d", ErrBadRecord, r.Version)
	}
	if len(r.Layers) == 0 && !r.Restoring {
		return fmt.Errorf("%w: no layers (should have been deleted)", ErrBadRecord)
	}
	if len(r.Layers) != 0 && r.Restoring {
		return fmt.Errorf("%w: restoring with active layers", ErrBadRecord)
	}
	seen := map[Name]bool{}
	for _, l := range r.Layers {
		if err := validate(l); err != nil {
			return fmt.Errorf("%w: %v", ErrBadRecord, err)
		}
		if seen[l.Name] {
			return fmt.Errorf("%w: duplicate layer %q", ErrBadRecord, l.Name)
		}
		seen[l.Name] = true
	}
	return nil
}

func sortLayers(ls []Layer) {
	slices.SortFunc(ls, func(a, b Layer) int { return precedence[b.Name] - precedence[a.Name] })
}

func cloneState(s State) State {
	s.Server.Values = slices.Clone(s.Server.Values)
	s.Order = slices.Clone(s.Order)
	return s
}

func clone(r Record) Record {
	out := Record{Version: r.Version, Base: cloneState(r.Base), Layers: make([]Layer, len(r.Layers))}
	for i, l := range r.Layers {
		l.Servers = slices.Clone(l.Servers)
		out.Layers[i] = l
	}
	return out
}
