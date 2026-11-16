// Package mock is an in-memory Device with real UCI semantics:
// Set requires an existing section, SetType upserts, staged changes are visible
// until Revert, and Commit makes them permanent.
package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

type config struct{ sections map[string]*device.Section }

// Device implements device.Device.
type Device struct {
	Reloads            int
	ReloadErr          error
	Reboots            int
	RebootErr          error
	UplinkReconnects   int
	BootIDErr          error
	RotateBootOnReboot bool
	mu                 sync.Mutex
	bootID             string
	bootSequence       int
	committed          map[string]*config
	staged             map[string]*config
	ubus               map[string]json.RawMessage
	scanReplies        map[string]json.RawMessage
	staLinkIndex       int
	neighbours         json.RawMessage
	wirelessChanged    bool
	DefaultRoute       bool
	// RenewErr makes RenewLease fail; renews records the interfaces renewed.
	RenewErr error
	renews   []string
	// AccountsErr makes every Accounts call fail (shadow unreadable).
	AccountsErr error
	// ServiceErr makes every ServiceAction fail; ServiceActions records "name action".
	ServiceErr     error
	ServiceActions []string
	services       map[string]serviceState
	password       string
	passwordInit   bool
}

func New() *Device {
	return &Device{bootID: "mock-boot-0", committed: map[string]*config{}, staged: map[string]*config{}, ubus: map[string]json.RawMessage{}, scanReplies: map[string]json.RawMessage{}}
}

// SetUbusReply scripts the reply for object.method (use verbatim captured payloads).
func (d *Device) SetUbusReply(object, method string, reply json.RawMessage) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ubus[object+"."+method] = reply
}

func clone(c *config) *config {
	n := &config{sections: map[string]*device.Section{}}
	if c == nil {
		return n
	}
	for k, s := range c.sections {
		cp := device.Section{Name: s.Name, Type: s.Type, Options: map[string]string{}, Lists: map[string][]string{}}
		for ok, ov := range s.Options {
			cp.Options[ok] = ov
		}
		for lk, lv := range s.Lists {
			cp.Lists[lk] = append([]string(nil), lv...)
		}
		n.sections[k] = &cp
	}
	return n
}

// view returns the working copy: staged if present, else committed.
func (d *Device) view(cfg string) *config {
	if s, ok := d.staged[cfg]; ok {
		return s
	}
	return d.committed[cfg]
}

func (d *Device) work(cfg string) *config {
	if s, ok := d.staged[cfg]; ok {
		return s
	}
	d.staged[cfg] = clone(d.committed[cfg])
	return d.staged[cfg]
}

func (d *Device) Show(_ context.Context, cfg string) ([]device.Section, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	c := d.view(cfg)
	if c == nil {
		return nil, device.ErrNotFound
	}
	out := make([]device.Section, 0, len(c.sections))
	for _, s := range clone(c).sections {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (d *Device) Set(_ context.Context, cfg, sec, opt, val string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	w := d.work(cfg)
	s, ok := w.sections[sec]
	if !ok {
		return fmt.Errorf("%s.%s: %w", cfg, sec, device.ErrNotFound)
	}
	s.Options[opt] = val
	return nil
}

func (d *Device) SetWirelessKey(ctx context.Context, section, key string) error {
	return d.Set(ctx, "wireless", section, "key", key)
}

func (d *Device) SetType(_ context.Context, cfg, sec, typ string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	w := d.work(cfg)
	if s, ok := w.sections[sec]; ok {
		s.Type = typ
		return nil
	}
	w.sections[sec] = &device.Section{Name: sec, Type: typ, Options: map[string]string{}, Lists: map[string][]string{}}
	return nil
}

func (d *Device) AddToList(_ context.Context, cfg, sec, opt, val string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.work(cfg).sections[sec]
	if !ok {
		return fmt.Errorf("%s.%s: %w", cfg, sec, device.ErrNotFound)
	}
	s.Lists[opt] = append(s.Lists[opt], val)
	return nil
}

func (d *Device) DeleteOption(_ context.Context, cfg, sec, opt string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.work(cfg).sections[sec]
	if !ok {
		return fmt.Errorf("%s.%s: %w", cfg, sec, device.ErrNotFound)
	}
	delete(s.Options, opt)
	delete(s.Lists, opt)
	return nil
}

func (d *Device) DeleteSection(_ context.Context, cfg, sec string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	w := d.work(cfg)
	if _, ok := w.sections[sec]; !ok {
		return fmt.Errorf("%s.%s: %w", cfg, sec, device.ErrNotFound)
	}
	delete(w.sections, sec)
	return nil
}

func (d *Device) Commit(_ context.Context, cfg string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s, ok := d.staged[cfg]; ok {
		d.committed[cfg] = s
		d.wirelessChanged = d.wirelessChanged || cfg == "wireless"
		delete(d.staged, cfg)
	}
	return nil
}

// Revert only undoes uncommitted changes, like the real `uci revert`.
func (d *Device) Revert(_ context.Context, cfg string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.staged, cfg)
	return nil
}

func (d *Device) Pending(_ context.Context, cfg string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.staged[cfg]
	return ok, nil
}

func (d *Device) Call(_ context.Context, object, method string, _ any) (json.RawMessage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r, ok := d.ubus[object+"."+method]
	if !ok {
		return nil, fmt.Errorf("ubus %s.%s: %w", object, method, device.ErrNotFound)
	}
	return r, nil
}

func (d *Device) Reload(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Reloads++
	if d.ReloadErr == nil && d.wirelessChanged {
		if d.staLinkIndex > 0 {
			d.staLinkIndex++
		}
		d.wirelessChanged = false
	}
	return d.ReloadErr
}

func (d *Device) HasDefaultRoute(context.Context) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.DefaultRoute, nil
}

var _ device.Device = (*Device)(nil)
