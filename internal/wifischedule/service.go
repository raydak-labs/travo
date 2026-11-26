package wifischedule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/hook"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

var (
	// ErrUnavailable: the rule file or crontab cannot be read or written (503).
	ErrUnavailable = errors.New("WiFi schedule unavailable")
	// ErrAPNotFound: a rule names a section that is not an access point.
	ErrAPNotFound = errors.New("access point not found")
)

// Cron is the shared crontab writer (internal/crontab.Service, ADR 0024).
type Cron interface {
	Get(ctx context.Context, owner string) ([]string, error)
	Set(ctx context.Context, owner string, lines []string) error
}

// APs is the access point slice of the WiFi service: the existing guarded AP transaction.
type APs interface {
	AccessPoints(context.Context) ([]wifi.APConfig, error)
	UpdateAP(context.Context, wifi.APRequest) (txn.Status, error)
}

// Confirmer confirms a transaction whose probe proved the APs (ADR 0027).
type Confirmer interface {
	Confirm(id string) (txn.Status, error)
}

// Guards reports held guards; any guard skips a scheduled toggle (invariants §1).
type Guards interface {
	Stale() ([]guard.Info, error)
}

// Service owns the rule file, its crontab block and the hook actions the block calls.
type Service struct {
	Path     string // rule file in the state directory
	Cron     Cron
	Services device.Services // reports whether crond runs
	APs      APs
	Txns     Confirmer
	Guards   Guards
	// Register adds a hook action (hook.Server.Register); called once per rule id and action.
	Register func(name string, a hook.Action)
	// Now is the wall clock of recorded outcomes; nil is time.Now.
	Now func() time.Time

	mu         sync.Mutex
	runsMu     sync.Mutex
	runs       map[string]Run
	registered map[string]bool
	random     io.Reader
	writeJSON  func(string, any, os.FileMode) error
}

func (s *Service) load() (Rules, error) {
	var r Rules
	err := atomicfile.ReadJSON(s.Path, &r)
	if errors.Is(err, os.ErrNotExist) {
		return Rules{Rules: []Rule{}}, nil
	}
	if err == nil {
		err = r.Validate()
	}
	if err != nil {
		slog.Warn("wifi schedule unreadable: kept unchanged until fixed", "err", err)
		return Rules{}, ErrUnavailable
	}
	return r, nil
}

// Get returns the stored rules.
func (s *Service) Get(context.Context) (Rules, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// Put validates, checks lockout, rewrites the crontab block and then the rule file. A failed
// rule write restores the previous crontab, so cron never runs rules the file does not hold.
func (s *Service) Put(ctx context.Context, next Rules) (Rules, error) {
	if err := next.Validate(); err != nil {
		return Rules{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.load(); err != nil {
		return Rules{}, err
	}
	aps, err := s.APs.AccessPoints(ctx)
	if err != nil {
		return Rules{}, ErrUnavailable
	}
	if err := checkRules(aps, next); err != nil {
		return Rules{}, err
	}
	next, err = s.assignIDs(next)
	if err != nil {
		return Rules{}, ErrUnavailable
	}
	old, err := s.Cron.Get(ctx, Owner)
	if err != nil {
		return Rules{}, ErrUnavailable
	}
	s.registerActions(next)
	if err := s.Cron.Set(ctx, Owner, Lines(next)); err != nil {
		slog.Warn("wifi schedule: crontab", "err", err)
		return Rules{}, ErrUnavailable
	}
	write := s.writeJSON
	if write == nil {
		write = atomicfile.WriteJSON
	}
	if err := write(s.Path, next, 0o600); err != nil {
		if rerr := s.Cron.Set(ctx, Owner, old); rerr != nil {
			slog.Error("wifi schedule: crontab restore failed", "err", rerr)
		}
		return Rules{}, ErrUnavailable
	}
	return next, nil
}

// Sync registers the stored rules' hook actions and re-renders their crontab block (a
// restored backup, an operator edit or a sysupgrade may have changed the crontab). An
// unreadable rule file leaves the crontab alone.
func (s *Service) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.load()
	if err != nil {
		return err
	}
	s.registerActions(rules)
	return s.Cron.Set(ctx, Owner, Lines(rules))
}

// registerActions adds the on/off actions of every rule id not registered yet. Actions of
// a removed rule stay registered and answer `unchanged`: hook actions cannot be removed.
func (s *Service) registerActions(r Rules) {
	if s.registered == nil {
		s.registered = map[string]bool{}
	}
	for _, rule := range r.Rules {
		if s.registered[rule.ID] || s.Register == nil {
			continue
		}
		s.registered[rule.ID] = true
		for _, a := range []Action{Off, On} {
			s.Register(ActionName(rule.ID, a), s.action(rule.ID, a))
		}
	}
}

func (s *Service) assignIDs(r Rules) (Rules, error) {
	random := s.random
	if random == nil {
		random = rand.Reader
	}
	out := Rules{Rules: slices.Clone(r.Rules)}
	for i := range out.Rules {
		for out.Rules[i].ID == "" {
			b := make([]byte, 4)
			if _, err := io.ReadFull(random, b); err != nil {
				return Rules{}, err
			}
			id := hex.EncodeToString(b)
			if !slices.ContainsFunc(out.Rules, func(r Rule) bool { return r.ID == id }) {
				out.Rules[i].ID = id
			}
		}
	}
	return out, nil
}

// checkRules: every section is an AP, and an enabled rule that would leave no enabled
// non-guest AP needs acknowledge_lockout. The caller at fire time is unknown, so this is
// the fail-closed Unknown rule of invariants §2, judged against today's layout.
func checkRules(aps []wifi.APConfig, r Rules) error {
	for _, rule := range r.Rules {
		at := slices.IndexFunc(aps, func(a wifi.APConfig) bool { return a.Section == rule.Section })
		if at < 0 {
			return ErrAPNotFound
		}
		if !rule.Enabled || rule.AcknowledgeLockout {
			continue
		}
		if err := lockout.Check(lockout.Caller{Method: lockout.Unknown}, lockoutPlan(aps, rule.Section), false); err != nil {
			return err
		}
	}
	return nil
}

func lockoutPlan(aps []wifi.APConfig, off string) lockout.Plan {
	var p lockout.Plan
	for _, a := range aps {
		p.Before = append(p.Before, lockout.AP{Section: a.Section, Enabled: a.Enabled, Guest: a.Guest})
		p.After = append(p.After, lockout.AP{Section: a.Section, Enabled: a.Enabled && a.Section != off, Guest: a.Guest})
	}
	return p
}
