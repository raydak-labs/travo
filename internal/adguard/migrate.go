package adguard

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

// Migrate moves an installed AGH to the loopback topology and the memory caps (Limits) as a
// guarded file swap: keep the old file, write the new one, restart, prove the admin API
// answers, else restore and restart. (txn snapshots UCI only; this YAML is not UCI, see ADR 0006.)
func (s *Service) Migrate(ctx context.Context) error {
	_, err := s.migrate(ctx, true)
	return err
}

// migrate reports whether it restarted AGH. A caps-only change to a stopped AGH is written
// without a restart: restarting would start an AGH the user turned off, and the next start
// reads the file anyway.
func (s *Service) migrate(ctx context.Context, running bool) (restarted bool, _ error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.idle(); err != nil {
		return false, err
	}
	old, err := os.ReadFile(s.Config)
	if err != nil {
		return false, err
	}
	out, loopback, err := Loopback(old)
	if err != nil {
		return false, err
	}
	out, capped := Limits(out)
	if !loopback && !capped {
		s.failed.Store(nil) // nothing left to do, e.g. fixed by hand after a failed try
		return false, nil
	}
	if s.migrationHeldOff(old) {
		return false, nil
	}
	if !loopback && !running {
		return false, atomicfile.Write(s.Config, out, 0o600)
	}
	rolledBack, err := s.swap(ctx, old, out)
	s.recordMigration(old, rolledBack, err)
	return err == nil, err
}

// swap reports rolledBack only when a failed try fully restored the old config and released
// the guard; any other failure (nothing written yet, or cleanup failed) is not a "failed and
// undone" migration.
func (s *Service) swap(ctx context.Context, old, out []byte) (rolledBack bool, _ error) {
	if err := s.Guards.Acquire(ConfigGuard); err != nil {
		return false, err
	}
	if err := atomicfile.Write(s.Backup, old, 0o600); err != nil {
		return false, errors.Join(fmt.Errorf("adguard: keep old config: %w", err), s.Guards.Release(ConfigGuard))
	}
	err := atomicfile.Write(s.Config, out, 0o600)
	if err == nil {
		err = s.Services.ServiceAction(ctx, Package, "restart")
	}
	if err == nil {
		err = s.waitReady(ctx)
	}
	if err != nil {
		rerr := s.restore(ctx, nil)
		return rerr == nil, errors.Join(fmt.Errorf("adguard: loopback migration: %w", err), rerr)
	}
	return false, s.finishSwap()
}

// MigrationBackoff is how long a failed loopback migration is not retried while the
// config is unchanged. Each try restarts AGH twice (swap and restore), and every restart
// is a short DNS gap when AGH is the default DNS (#164).
const MigrationBackoff = time.Hour

// migrationFailure is memory-only on purpose: no flash write per failure (invariants §5),
// and a daemon restart is an acceptable retry point. at carries the monotonic reading.
type migrationFailure struct {
	config [sha256.Size]byte
	at     time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// migrationFailed reports a recorded failure, expired or not. Status hides it once the config
// is loopback (an operator may fix the YAML by hand); Reconcile then forgets it.
func (s *Service) migrationFailed() bool { return s.failed.Load() != nil }

// migrationHeldOff is true while the last failure is younger than MigrationBackoff and the
// config is byte-identical to the one that failed. An edited config is tried again at once.
func (s *Service) migrationHeldOff(config []byte) bool {
	f := s.failed.Load()
	return f != nil && f.config == sha256.Sum256(config) && s.now().Sub(f.at) < MigrationBackoff
}

// recordMigration keeps only the latest outcome: a completed rollback starts the back-off;
// success or any other failure (a kept guard shows as busy instead) clears an older record.
func (s *Service) recordMigration(config []byte, rolledBack bool, err error) {
	if err != nil && rolledBack {
		s.failed.Store(&migrationFailure{config: sha256.Sum256(config), at: s.now()})
		return
	}
	s.failed.Store(nil)
}
