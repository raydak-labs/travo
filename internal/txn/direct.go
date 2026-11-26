package txn

import (
	"context"
	"errors"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

// Apply is the direct path for changes that cannot cut the caller off the router
// (e.g. dnsmasq server options): lock, refuse foreign staged changes, stage, commit, reload.
// No watchdog and no confirm. Connectivity changes (wireless, network, firewall) use Begin.
// On a stage or commit error the staged delta is reverted; a reload error leaves the
// committed change live and is returned so the caller keeps its own guard.
func (m *Manager) Apply(ctx context.Context, configs []string, stage func(ctx context.Context) error) error {
	cs, err := uciwrite.Normalize(configs)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if connectivity[c] {
			return fmt.Errorf("txn: %s can cut connectivity; use Begin", c)
		}
	}
	if m.stopped() {
		return ErrStopped
	}
	if err := m.Guards.Check(Conflicts(ctx)); err != nil {
		return err
	}
	lctx, unlock, err := m.Locks.Lock(ctx, cs)
	if err != nil {
		return err
	}
	defer unlock()
	if err := m.refuseForeign(lctx, cs); err != nil {
		return err
	}
	if err := stage(lctx); err != nil {
		return errors.Join(fmt.Errorf("txn: stage: %w", err), m.revertAll(lctx, cs))
	}
	for _, c := range cs {
		if m.stopped() {
			return errors.Join(ErrStopped, m.revertAll(lctx, cs))
		}
		if err := m.Device.Commit(lctx, c); err != nil {
			return errors.Join(fmt.Errorf("txn: commit %s: %w", c, err), m.revertAll(lctx, cs))
		}
	}
	if err := m.Device.Reload(lctx); err != nil {
		return fmt.Errorf("txn: reload: %w", err)
	}
	return nil
}

// connectivity lists configs whose change can strand the user; they need the watchdog.
var connectivity = map[string]bool{"wireless": true, "network": true, "firewall": true}
