package txn

import (
	"context"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Settled is a config read and whether a transaction overlapped it. Sections of an
// overlapped read may belong to a change that is unconfirmed or was rolled back.
type Settled struct {
	Sections                []device.Section
	NoTransactionDuringRead bool
	// NoTransactionSince reports that no transaction began or holds its guard since the read
	// started; call it right before acting on the read.
	NoTransactionSince func() (bool, error)
}

// ShowSettled reads config under its write lock, so it never sees another writer's staged
// delta: staging happens under that lock and ends in commit or revert before it is released.
// It waits for a transaction's apply phase (up to ApplyBudget); use plain Show for UI reads.
func (m *Manager) ShowSettled(ctx context.Context, config string) (Settled, error) {
	before := m.begun.Load()
	heldBefore, err := m.guardHeld()
	if err != nil {
		return Settled{}, err
	}
	if m.testHookAfterFirstCheck != nil {
		m.testHookAfterFirstCheck()
	}
	lctx, unlock, err := m.Locks.Lock(ctx, []string{config})
	if err != nil {
		return Settled{}, err
	}
	sections, err := m.Device.Show(lctx, config)
	unlock()
	if err != nil {
		return Settled{}, err
	}
	heldAfter, err := m.guardHeld()
	if err != nil {
		return Settled{}, err
	}
	untouched := !heldBefore && !heldAfter && m.begun.Load() == before
	since := func() (bool, error) {
		held, err := m.guardHeld()
		return untouched && !held && m.begun.Load() == before, err
	}
	return Settled{Sections: sections, NoTransactionDuringRead: untouched, NoTransactionSince: since}, nil
}

func (m *Manager) guardHeld() (bool, error) {
	held, err := m.Guards.Stale()
	for _, g := range held {
		if g.Name == GuardName {
			return true, err
		}
	}
	return false, err
}
