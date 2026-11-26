package failover

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Interval of the reconciler. A rolled-back first apply restores the stock config while
// mwan3 runs; this bounds how long its catch-all rules can be live (device-facts §9).
const Interval = 5 * time.Second

// Reconcile keeps mwan3 running exactly while the user wants failover and the committed
// config is Travo's: it stops mwan3 after a rollback or a disable, and starts and
// boot-enables it once a confirmed apply is in place. It waits while a transaction awaits
// confirmation, and stays off while a failover guard is kept (the next enable retries).
func (s *Service) Reconcile(ctx context.Context) error {
	if !s.ready() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stale, err := s.Guards.Stale()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(stale, func(g guard.Info) bool {
		return g.Name == txn.GuardName || g.Name == InstallGuard || g.Name == ServiceGuard
	}) {
		return nil
	}
	if _, err := s.Packages.Version(ctx, Package); errors.Is(err, pkgmgr.ErrNotInstalled) {
		return nil
	} else if err != nil {
		return err
	}
	set, err := s.LoadSettings()
	if err != nil {
		return err
	}
	current, err := s.UCI.Show(ctx, Config)
	if err != nil {
		return err
	}
	running, err := s.Services.ServiceRunning(ctx, Package)
	if err != nil {
		return err
	}
	want := set.Enabled && Active(current)
	switch {
	case want && !running:
		err = s.serviceActions(ctx, "start", "enable")
		s.bootEnabled = err == nil
	case want && !s.bootEnabled:
		err = s.serviceActions(ctx, "enable")
		s.bootEnabled = err == nil
	case !want && running:
		err = s.serviceActions(ctx, "stop", "disable")
	}
	return err
}
