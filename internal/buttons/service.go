package buttons

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/hook"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

// Applier is txn.Manager.Apply: lock, refuse foreign staged changes, stage, commit, reload.
type Applier interface {
	Apply(ctx context.Context, configs []string, stage func(ctx context.Context) error) error
}

// Tunnel is the WireGuard tunnel's guarded transaction (internal/vpntunnel).
type Tunnel interface {
	Set(ctx context.Context, enabled bool) (txn.Status, error)
}

// Guest is the guest AP's guarded transaction (internal/wifi).
type Guest interface {
	Guest(context.Context) (wifi.GuestConfig, error)
	SetGuest(context.Context, wifi.GuestRequest) (txn.Status, error)
}

// Confirmer confirms a transaction whose probe proved the change (ADR 0027).
type Confirmer interface {
	Confirm(id string) (txn.Status, error)
}

// Guards reports held guards; any guard skips a switch action (invariants §1).
type Guards interface {
	Stale() ([]guard.Info, error)
}

// Service owns the button sections and the switch hook actions. Tunnel and Guest may be
// nil on a device without them; the action then fails with `failed` in the log.
type Service struct {
	UCI    device.UCI
	Keys   device.ButtonReader
	Apply  Applier
	Tunnel Tunnel
	Guest  Guest
	Txns   Confirmer
	Guards Guards
	// Now is the wall clock of recorded runs; nil is time.Now.
	Now func() time.Time

	mu     sync.Mutex // serialises Set; the hook server runs one action at a time
	runsMu sync.Mutex
	runs   map[string]Run
}

// Get lists the devicetree buttons with their actions and last runs.
func (s *Service) Get(ctx context.Context) (Status, error) {
	keys, err := s.Keys.Buttons(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("buttons: devicetree: %w", err)
	}
	secs, err := s.sections(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("buttons: uci: %w", err)
	}
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	return View(keys, Configured(secs), s.runs), nil
}

// Set stores the switch's action; none deletes its section. Nothing changes live: the
// action runs when the switch is slid.
func (s *Service) Set(ctx context.Context, node string, a Action) (Status, error) {
	if !a.Valid() {
		return Status{}, ErrInvalid
	}
	keys, err := s.Keys.Buttons(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("buttons: devicetree: %w", err)
	}
	if _, err := find(keys, node); err != nil {
		return Status{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	secs, err := s.sections(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("buttons: uci: %w", err)
	}
	current, ok := Configured(secs)[node]
	if (ok && current == a) || (!ok && a == None) {
		return s.Get(ctx)
	}
	name := SectionName(node)
	err = s.Apply.Apply(ctx, []string{config}, func(lctx context.Context) error {
		if a == None {
			return s.UCI.DeleteSection(lctx, config, name)
		}
		if err := s.UCI.SetType(lctx, config, name, sectionType); err != nil {
			return err
		}
		return errors.Join(s.UCI.Set(lctx, config, name, "node", node), s.UCI.Set(lctx, config, name, "action", string(a)))
	})
	if err != nil {
		return Status{}, err
	}
	return s.Get(ctx)
}

// Actions are the hook actions of every detected slide switch, one per position.
func (s *Service) Actions(ctx context.Context) (map[string]hook.Action, error) {
	keys, err := s.Keys.Buttons(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]hook.Action{}
	for _, k := range keys {
		if !Configurable(k) {
			continue
		}
		for _, pos := range []string{positionOn, positionOff} {
			out[ActionName(HotplugName(k.Code), pos)] = s.action(k.Node, pos)
		}
	}
	return out, nil
}

// action records the outcome and reports every outcome but applied, unchanged and
// no_action as an error code, so the hotplug script's logger line shows it.
func (s *Service) action(node, position string) hook.Action {
	return func(ctx context.Context) error {
		outcome := s.fire(ctx, node, position == positionOn)
		s.record(node, Run{At: s.now().Unix(), Position: position, Outcome: outcome})
		if outcome.ok() {
			return nil
		}
		return &hook.CodedError{Code: string(outcome), Err: errors.New("button " + node + " " + position + " not applied")}
	}
}

// sections reads the button sections; a missing travo-greenfield config has none.
func (s *Service) sections(ctx context.Context) ([]device.Section, error) {
	secs, err := s.UCI.Show(ctx, config)
	if errors.Is(err, device.ErrNotFound) {
		return nil, nil
	}
	return secs, err
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
