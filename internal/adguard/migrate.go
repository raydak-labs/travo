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

// Migrate moves an installed AGH to the loopback topology as a guarded file swap: keep the
// old file, write the new one, restart, prove the admin API answers, else restore and restart.
// (txn snapshots UCI only; this YAML is not UCI, see ADR 0006.)
func (s *Service) Migrate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.idle(); err != nil {
		return err
	}
	old, err := os.ReadFile(s.Config)
	if err != nil {
		return err
	}
	out, changed, err := Loopback(old)
	if err != nil || !changed || s.migrationHeldOff(old) {
		return err
	}
	err = s.swap(ctx, old, out)
	s.recordMigration(old, err)
	return err
}

func (s *Service) swap(ctx context.Context, old, out []byte) error {
	if err := s.Guards.Acquire(ConfigGuard); err != nil {
		return err
	}
	if err := atomicfile.Write(s.Backup, old, 0o600); err != nil {
		return errors.Join(fmt.Errorf("adguard: keep old config: %w", err), s.Guards.Release(ConfigGuard))
	}
	err := atomicfile.Write(s.Config, out, 0o600)
	if err == nil {
		err = s.Services.ServiceAction(ctx, Package, "restart")
	}
	if err == nil {
		err = s.waitReady(ctx)
	}
	if err != nil {
		return s.restore(ctx, fmt.Errorf("adguard: loopback migration: %w", err))
	}
	return s.finishSwap()
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

// migrationFailed reports a recorded failure, expired or not: the config still binds the LAN.
func (s *Service) migrationFailed() bool { return s.failed.Load() != nil }

// migrationHeldOff is true while the last failure is younger than MigrationBackoff and the
// config is byte-identical to the one that failed. An edited config is tried again at once.
func (s *Service) migrationHeldOff(config []byte) bool {
	f := s.failed.Load()
	return f != nil && f.config == sha256.Sum256(config) && s.now().Sub(f.at) < MigrationBackoff
}

func (s *Service) recordMigration(config []byte, err error) {
	if err == nil {
		s.failed.Store(nil)
		return
	}
	s.failed.Store(&migrationFailure{config: sha256.Sum256(config), at: s.now()})
}
