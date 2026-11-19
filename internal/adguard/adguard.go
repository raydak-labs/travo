// Package adguard supervises the OpenWrt `adguardhome` package (ADR 0006): presence and
// status, the guarded install after the first uplink, and the loopback migration of its config.
// OpenWrt owns the binary and conffile; Travo only installs, configures and restarts it.
package adguard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

const (
	// Package is the OpenWrt package and its init script name.
	Package = "adguardhome"
	// ConfigPath is the package's conffile (UCI adguardhome.config.config_file, device-facts §11).
	ConfigPath = "/etc/adguardhome/adguardhome.yaml"
	// InstallGuard covers the package install; it is cleared only by an explicit install retry.
	InstallGuard = "adguard-install-in-progress"
	// ConfigGuard covers the config swap; Recover finishes an interrupted one at startup.
	ConfigGuard = "adguard-config-in-progress"
	// NeedFree is the overlay space an install needs: the package's installed size (32 MiB,
	// testdata/device/axt1800-25.12.6/adguard/apk-info.txt) plus the 20 MB headroom (invariants §5).
	NeedFree = (32 + 20) << 20
	// ReadyTimeout bounds how long AGH may take to answer after a restart.
	ReadyTimeout = 20 * time.Second
)

var (
	ErrBusy    = errors.New("an AdGuard Home install or config change is in progress or was interrupted")
	ErrOffline = errors.New("no Internet uplink yet")
	ErrNoSpace = errors.New("not enough free flash for AdGuard Home")
)

// Status is the read-only view. A failed presence check is an error, never "absent" (invariants §13).
type Status struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version" doc:"Installed package version; empty when absent"`
	Running   bool   `json:"running" doc:"procd reports a running instance"`
	Loopback  bool   `json:"loopback" doc:"The config binds the admin UI and DNS to loopback only (ADR 0006)"`
	Busy      bool   `json:"busy" doc:"An install or config change is in progress or was interrupted"`
	DNSStatus
	TopologyStatus
	// ConfigError says why the config could not be read as the canonical YAML shape.
	ConfigError string `json:"config_error,omitempty"`
	// MigrationFailed: the last loopback migration failed and was rolled back (#164).
	MigrationFailed bool `json:"migration_failed" doc:"The last loopback migration failed and was rolled back; retried after an hour, on a config edit or on daemon restart"`
}

// Service is the only code that installs AGH or writes its config.
type Service struct {
	Packages pkgmgr.PackageManager
	Services device.Services
	Guards   guard.Dir
	// Config is the AGH YAML; Backup holds the previous file during a swap (state dir);
	// Marker records that the one automatic install attempt happened.
	Config, Backup, Marker string
	// FreeBytes reads free overlay space; Online is the real Internet probe.
	FreeBytes func() (uint64, error)
	Online    func(context.Context) bool
	// Bundle is the release's offline package (ADR 0020); a verified one replaces the
	// online gate and the package list refresh for the first install.
	Bundle Bundle
	// Ready returns nil once AGH's admin API answers on loopback; it waits until ctx ends.
	Ready func(context.Context) error
	// DNS owns the `adguard` layer; nil when the daemon has no DNS layer stack.
	DNS *DNS
	// Now is the clock for the migration back-off; nil means time.Now (monotonic).
	Now func() time.Time

	// Expert topology (ADR 0021). UCI, Txns or ProbeAt nil: the switch is unavailable.
	UCI  device.UCI
	Txns TopologyTxns
	// TopologyState records the mode's details (state dir); TopologyBackup keeps AGH's
	// YAML while a switch is in flight.
	TopologyState, TopologyBackup string
	// ProbeAt resolves a fresh name through addr (UDPProbe in production).
	ProbeAt func(addr string) func(context.Context) error
	// SettleWait bounds the switch's waits for dnsmasq and AGH; 0 = DefaultSettleWait.
	SettleWait time.Duration

	mu             sync.Mutex
	failed         atomic.Pointer[migrationFailure]
	expertFailures int
}

// Status never mutates.
func (s *Service) Status(ctx context.Context) (Status, error) {
	st := Status{MigrationFailed: s.migrationFailed()}
	busy, err := s.busy()
	if err != nil {
		return st, err
	}
	st.Busy = busy
	if st.TopologyStatus, _, err = s.topologyStatus(); err != nil {
		return st, err
	}
	if s.DNS != nil {
		if st.DNSStatus, err = s.DNS.Status(); err != nil {
			return st, err
		}
	}
	v, err := s.Packages.Version(ctx, Package)
	if errors.Is(err, pkgmgr.ErrNotInstalled) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	st.Installed, st.Version = true, v
	if st.Running, err = s.Services.ServiceRunning(ctx, Package); err != nil {
		return st, err
	}
	b, err := os.ReadFile(s.Config)
	if err != nil {
		st.ConfigError = err.Error()
		return st, nil
	}
	l, err := ParseListen(b)
	if err != nil {
		st.ConfigError = err.Error()
		return st, nil
	}
	st.Loopback = l.IsLoopback() || (st.Topology == TopologyExpert && l.HTTP == LoopbackHTTP)
	st.MigrationFailed = st.MigrationFailed && !st.Loopback
	return st, nil
}

func (s *Service) busy() (bool, error) {
	g, err := s.Guards.Stale()
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(g, func(x guard.Info) bool {
		return x.Name == InstallGuard || x.Name == ConfigGuard || x.Name == TopologyGuard
	}), nil
}

// Idle is ErrBusy while an install, config swap or topology switch holds its guard, and
// ErrExpertTopology in expert mode: AGH's upstream must stay dnsmasq there (DoH refuses).
func (s *Service) Idle() error {
	if err := s.idle(); err != nil {
		return err
	}
	return s.expertGuard()
}

func (s *Service) idle() error {
	busy, err := s.busy()
	if err == nil && busy {
		err = ErrBusy
	}
	return err
}

// Reconcile is the background tick: one automatic install after the first uplink, and the
// loopback migration of a config that still binds the LAN. Guards make it skip.
func (s *Service) Reconcile(ctx context.Context) error {
	st, err := s.Status(ctx)
	if err != nil || st.Busy || st.Topology == TopologyExpert {
		return err // the expert config is not loopback on purpose; never migrate it back
	}
	if !st.Installed {
		return s.Install(ctx, false)
	}
	if _, err := os.Stat(s.Config); errors.Is(err, os.ErrNotExist) {
		return s.SeedMissing(ctx)
	}
	if !st.Loopback && st.ConfigError == "" {
		return s.Migrate(ctx)
	}
	if st.Loopback {
		s.failed.Store(nil) // fixed by hand after a failed try
	}
	return nil
}

// ErrNoDNS means the daemon runs without a DNS layer stack (no default-DNS toggle).
var ErrNoDNS = errors.New("DNS layers unavailable")

// SetDefaultDNS is the user's "default DNS" toggle.
func (s *Service) SetDefaultDNS(ctx context.Context, on bool) error {
	if s.DNS == nil {
		return ErrNoDNS
	}
	if _, err := s.Packages.Version(ctx, Package); err != nil {
		return err
	}
	// In expert mode the layer would loop AGH -> dnsmasq -> AGH (ADR 0021).
	if err := s.expertGuard(); err != nil {
		return err
	}
	return s.DNS.SetDefault(ctx, on)
}

// SetEnabled is the user's on/off. Off pops the layer BEFORE stopping AGH, so LAN DNS never
// points at a stopped resolver; on clears that choice and Tick re-adds the layer once AGH resolves.
func (s *Service) SetEnabled(ctx context.Context, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.idle(); err != nil {
		return err
	}
	if _, err := s.Packages.Version(ctx, Package); err != nil {
		return err
	}
	actions := []string{"enable", "start"}
	if !on {
		// Stopping AGH while it owns :53 is a LAN-wide DNS outage.
		if err := s.expertGuard(); err != nil {
			return err
		}
		if s.DNS != nil {
			if err := s.DNS.SetDefault(ctx, false); err != nil {
				return err
			}
		}
		actions = []string{"stop", "disable"}
	}
	for _, a := range actions {
		if err := s.Services.ServiceAction(ctx, Package, a); err != nil {
			return fmt.Errorf("%s %s: %w", Package, a, err)
		}
	}
	if !on {
		return nil
	}
	if err := s.waitReady(ctx); err != nil {
		return fmt.Errorf("%s not answering: %w", Package, err)
	}
	if s.DNS != nil {
		return s.DNS.Allow()
	}
	return nil
}

// Install adds the package once. Automatic attempts (explicit=false) happen only online
// (or with a verified offline bundle),
// once per router (Marker). explicit is the user's retry: it clears InstallGuard by name.
func (s *Service) Install(ctx context.Context, explicit bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if explicit {
		if err := s.Guards.Clear(InstallGuard); err != nil {
			return err
		}
	}
	if err := s.idle(); err != nil {
		return err
	}
	if _, err := s.Packages.Version(ctx, Package); err == nil {
		return nil
	} else if !errors.Is(err, pkgmgr.ErrNotInstalled) {
		return err
	}
	if !explicit {
		if _, err := os.Stat(s.Marker); err == nil {
			return nil // the automatic attempt already ran; only the user retries
		}
	}
	bundled := s.bundled()
	if !bundled && !s.Online(ctx) {
		return ErrOffline
	}
	unlock, err := pkgmgr.Exclusive(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if free, err := s.FreeBytes(); err != nil {
		return fmt.Errorf("free flash: %w", err)
	} else if free < NeedFree {
		return fmt.Errorf("%w: %d MiB free, %d MiB needed", ErrNoSpace, free>>20, NeedFree>>20)
	}
	if err := s.Guards.AcquireExclusive(InstallGuard, guard.RefuseTerminal); err != nil {
		return err
	}
	release := func(err error) error { return errors.Join(err, s.Guards.Release(InstallGuard)) }
	if err := atomicfile.Write(s.Marker, []byte("1\n"), 0o600); err != nil {
		return release(fmt.Errorf("install marker: %w", err))
	}
	if bundled {
		slog.Info("adguard: installing from the offline bundle", "file", s.Bundle.Path)
	} else if err := s.Packages.Update(ctx); err != nil {
		slog.Warn("adguard: package list refresh failed; installing from cached lists", "err", err)
	}
	// The config exists before the package does: AGH must never start its first-run
	// wizard on 0.0.0.0:3000 (invariants §8).
	if err := s.prepareConfig(); err != nil {
		return release(err)
	}
	if err := s.installPackage(ctx, bundled); err != nil {
		return fmt.Errorf("install %s: %w (guard %s kept)", Package, err, InstallGuard)
	}
	for _, a := range []string{"enable", "restart"} {
		if err := s.Services.ServiceAction(ctx, Package, a); err != nil {
			return fmt.Errorf("%s %s: %w (guard %s kept)", Package, a, err, InstallGuard)
		}
	}
	if err := s.waitReady(ctx); err != nil {
		return fmt.Errorf("%s not answering after install: %w (guard %s kept)", Package, err, InstallGuard)
	}
	return s.Guards.Release(InstallGuard)
}

// SeedMissing gives a package that came without a config (preinstalled in the firmware
// image) the loopback seed and restarts it. Without a config AGH serves its first-run
// wizard on 0.0.0.0:3000 to every LAN client (invariants §8). Failure keeps the guard.
func (s *Service) SeedMissing(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.idle(); err != nil {
		return err
	}
	if _, err := os.Stat(s.Config); !errors.Is(err, os.ErrNotExist) {
		return err // appeared meanwhile, or unreadable: nothing to seed
	}
	if err := s.Guards.Acquire(ConfigGuard); err != nil {
		return err
	}
	err := s.prepareConfig()
	if err == nil {
		err = s.Services.ServiceAction(ctx, Package, "restart")
	}
	if err == nil {
		err = s.waitReady(ctx)
	}
	if err != nil {
		return fmt.Errorf("adguard: seed missing config: %w (guard %s kept)", err, ConfigGuard)
	}
	return s.Guards.Release(ConfigGuard)
}

// prepareConfig writes the seed, or migrates a config left over from an earlier install.
func (s *Service) prepareConfig() error {
	old, err := os.ReadFile(s.Config)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(s.Config), 0o700); err != nil {
			return err
		}
		return atomicfile.Write(s.Config, Seed, 0o600)
	}
	if err != nil {
		return err
	}
	out, changed, err := Loopback(old)
	if err != nil || !changed {
		return err
	}
	return atomicfile.Write(s.Config, out, 0o600)
}

// Recover finishes a swap a daemon crash interrupted: with a kept old file it is put back
// (the next Reconcile migrates again). Without one, either a seed was written but AGH not
// yet restarted, or a migration stopped before writing anything: restarting AGH and
// proving it answers is right for both, so a seeded config is never left unapplied while
// the old process serves its wizard. Restart or readiness failure keeps the guard.
func (s *Service) Recover(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.Guards.Stale()
	if err != nil || !slices.ContainsFunc(g, func(x guard.Info) bool { return x.Name == ConfigGuard }) {
		return err
	}
	if _, err := os.Stat(s.Backup); errors.Is(err, os.ErrNotExist) {
		if err := s.Services.ServiceAction(ctx, Package, "restart"); err != nil {
			return fmt.Errorf("adguard: restart after interrupted config write: %w (guard %s kept)", err, ConfigGuard)
		}
		if err := s.waitReady(ctx); err != nil {
			return fmt.Errorf("adguard: not answering after interrupted config write: %w (guard %s kept)", err, ConfigGuard)
		}
		return s.Guards.Release(ConfigGuard)
	}
	return s.restore(ctx, nil)
}

// restore puts the kept file back and restarts AGH. Any failure keeps the guard.
func (s *Service) restore(ctx context.Context, cause error) error {
	old, err := os.ReadFile(s.Backup)
	if err == nil {
		err = atomicfile.Write(s.Config, old, 0o600)
	}
	if err == nil {
		err = s.Services.ServiceAction(ctx, Package, "restart")
	}
	if err != nil {
		return errors.Join(cause, fmt.Errorf("adguard: restore old config: %w (guard %s kept)", err, ConfigGuard))
	}
	return errors.Join(cause, s.finishSwap())
}

func (s *Service) finishSwap() error {
	if err := os.Remove(s.Backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("adguard: remove kept config: %w (guard %s kept)", err, ConfigGuard)
	}
	return s.Guards.Release(ConfigGuard)
}

func (s *Service) waitReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, ReadyTimeout)
	defer cancel()
	return s.Ready(ctx)
}
