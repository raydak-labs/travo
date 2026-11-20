// Package uciwrite serialises writers of UCI configs. Staged changes are process-global
// per config, so concurrent writers of one config would destroy each other's deltas.
package uciwrite

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
)

var (
	// ErrReentrant means the caller already holds locks; nested locking is a bug.
	ErrReentrant = errors.New("uci locks are not reentrant")
	ErrBadConfig = errors.New("invalid uci config name")
	validConfig  = regexp.MustCompile(`^[a-z0-9_-]+$`)
	heldKey      = ctxKey{}
)

type ctxKey struct{}

// Locks holds one mutex per config name (not per service: firewall/network are shared).
type Locks struct {
	mu          sync.Mutex
	m           map[string]*sync.Mutex
	onContended atomic.Pointer[func(config string)]
}

// OnContended sets f to run before a caller blocks on a held config lock (nil clears it).
// Tests use it to know a reader is waiting instead of sleeping.
func (l *Locks) OnContended(f func(config string)) {
	if f == nil {
		l.onContended.Store(nil)
		return
	}
	l.onContended.Store(&f)
}

// Normalize validates, sorts and de-duplicates config names.
func Normalize(configs []string) ([]string, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("%w: empty set", ErrBadConfig)
	}
	out := slices.Clone(configs)
	for _, c := range out {
		if !validConfig.MatchString(c) {
			return nil, fmt.Errorf("%w: %q", ErrBadConfig, c)
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// Lock takes every config lock in global sorted order (no deadlock between writers of
// overlapping sets). The returned ctx marks the locks as held; locking again with it fails.
func (l *Locks) Lock(ctx context.Context, configs []string) (context.Context, func(), error) {
	if ctx.Value(heldKey) != nil {
		return ctx, nil, ErrReentrant
	}
	cs, err := Normalize(configs)
	if err != nil {
		return ctx, nil, err
	}
	ms := make([]*sync.Mutex, len(cs))
	l.mu.Lock()
	if l.m == nil {
		l.m = map[string]*sync.Mutex{}
	}
	for i, c := range cs {
		if l.m[c] == nil {
			l.m[c] = &sync.Mutex{}
		}
		ms[i] = l.m[c]
	}
	l.mu.Unlock()
	for i, m := range ms {
		if !m.TryLock() {
			if f := l.onContended.Load(); f != nil {
				(*f)(cs[i])
			}
			m.Lock()
		}
	}
	unlock := func() {
		for i := len(ms) - 1; i >= 0; i-- {
			ms[i].Unlock()
		}
	}
	return context.WithValue(ctx, heldKey, cs), unlock, nil
}

// Held returns the configs locked by ctx, or nil.
func Held(ctx context.Context) []string {
	cs, _ := ctx.Value(heldKey).([]string)
	return cs
}
