// Package tailscale supervises the OpenWrt `tailscale` package (docs/adr/0017): the guarded
// install, status, login and logout, and the `tailscale` DNS layer for MagicDNS. OpenWrt owns
// the binaries, init script and state file; Travo drives them through the tailscale CLI.
package tailscale

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

const (
	// Package is the OpenWrt package and its init script name.
	Package = "tailscale"
	// InstallGuard covers the package install; only the explicit install retry clears it.
	InstallGuard = "tailscale-install-in-progress"
	// ChangeGuard covers login, logout and preference changes; the next explicit
	// user action clears it by name (its retry path). The DNS reconciler skips while held.
	ChangeGuard = "tailscale-change-in-progress"
	// NeedFree is the installed size (26 MiB, testdata/device/axt1800-25.12.6/tailscale/apk-info.txt)
	// plus the 20 MB headroom (invariants §5).
	NeedFree = (26 + 20) << 20
	// ReadyTimeout bounds how long tailscaled may take to answer after a restart.
	ReadyTimeout = 20 * time.Second
)

var (
	ErrBusy       = errors.New("a Tailscale install or change is in progress or was interrupted")
	ErrOffline    = errors.New("no Internet uplink yet")
	ErrNoSpace    = errors.New("not enough free flash for Tailscale")
	ErrNotRunning = errors.New("tailscaled is not running")
	// ErrFailover: mwan3 cannot manage its chains next to Tailscale's nftables rules (ADR 0018).
	ErrFailover = errors.New("failover is enabled; disable failover before installing or starting Tailscale")
)

// Status is the read-only view. A failed presence check is an error, never "absent" (invariants §13).
type Status struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version" doc:"Installed package version; empty when absent"`
	Running   bool   `json:"running" doc:"procd reports a running tailscaled"`
	Busy      bool   `json:"busy" doc:"An install or change is in progress or was interrupted"`
	Backend
	Settings Settings `json:"settings"`
	// FirewallInSync: the travo_ts zone and forwardings match the settings (docs/adr/0017).
	FirewallInSync bool `json:"firewall_in_sync"`
}

// Service is the only code that installs tailscale or changes its preferences.
type Service struct {
	CLI      CLI
	Packages pkgmgr.PackageManager
	Services device.Services
	Guards   guard.Dir
	// FreeBytes reads free overlay space; Online is the real Internet probe.
	FreeBytes func() (uint64, error)
	Online    func(context.Context) bool
	// Invalidate drops a cached Online result, so the probe after an exit-node change
	// runs through the new route (nil: Online is never cached).
	Invalidate func()
	// Every is the poll interval while waiting for tailscaled or the Internet probe.
	Every time.Duration
	// UCI reads network and firewall; Txns stages the firewall zone; Tunnel is the WireGuard
	// state for the exit-node conflict check (nil: no WireGuard on this device).
	UCI    device.UCI
	Txns   Transactions
	Tunnel Tunnel
	// ProbeWait bounds the Internet proof after selecting an exit node.
	ProbeWait time.Duration
	// Exclusive is shared with failover.Service; FailoverEnabled reads its saved choice
	// (both nil: no failover on this device). Install and Login refuse while failover is
	// enabled, checked under Exclusive (docs/adr/0018).
	Exclusive       *sync.Mutex
	FailoverEnabled func() (bool, error)

	mu sync.Mutex
}

// Status never mutates. The backend part is empty while the package or daemon is absent.
func (s *Service) Status(ctx context.Context) (Status, error) {
	st := Status{Backend: emptyBackend()}
	busy, err := s.busy()
	if err != nil {
		return st, err
	}
	st.Busy = busy
	v, err := s.Packages.Version(ctx, Package)
	if errors.Is(err, pkgmgr.ErrNotInstalled) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	st.Installed, st.Version = true, v
	if st.Running, err = s.Services.ServiceRunning(ctx, Package); err != nil || !st.Running {
		return st, err
	}
	if st.Backend, err = s.backend(ctx); err != nil {
		return st, err
	}
	lan, err := s.lanPrefix(ctx)
	if err != nil && !errors.Is(err, ErrNoLAN) {
		return st, err
	}
	st.Settings = SettingsOf(st.Backend, lan)
	st.FirewallInSync, err = s.firewallInSync(ctx, st.Settings.AdvertiseLAN)
	return st, err
}

func errNotInstalled() error { return fmt.Errorf("%s: %w", Package, pkgmgr.ErrNotInstalled) }

func emptyBackend() Backend {
	return Backend{Self: Node{IPs: []string{}}, ExitNodes: []ExitNode{}, Health: []string{},
		Prefs: Prefs{AdvertiseRoutes: []string{}}}
}

func (s *Service) backend(ctx context.Context) (Backend, error) {
	raw, err := s.CLI.Status(ctx)
	if err != nil {
		return Backend{}, fmt.Errorf("tailscale status: %w", err)
	}
	prefs, err := s.CLI.Prefs(ctx)
	if err != nil {
		return Backend{}, fmt.Errorf("tailscale prefs: %w", err)
	}
	return Parse(raw, prefs)
}

func (s *Service) busy() (bool, error) {
	g, err := s.Guards.Stale()
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(g, func(x guard.Info) bool { return x.Name == InstallGuard || x.Name == ChangeGuard }), nil
}

func (s *Service) guardHeld(name string) (bool, error) {
	g, err := s.Guards.Stale()
	return slices.ContainsFunc(g, func(x guard.Info) bool { return x.Name == name }), err
}

// Install adds the package on the user's request; there is no automatic install. It clears
// InstallGuard by name first (the explicit retry) and keeps it on any failure.
func (s *Service) Install(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.exclusive()()
	if err := s.failoverOff(); err != nil {
		return err
	}
	if err := s.Guards.Clear(InstallGuard); err != nil {
		return err
	}
	if held, err := s.guardHeld(ChangeGuard); err != nil || held {
		return errors.Join(err, busyIf(held))
	}
	if _, err := s.Packages.Version(ctx, Package); err == nil {
		return nil
	} else if !errors.Is(err, pkgmgr.ErrNotInstalled) {
		return err
	}
	if !s.Online(ctx) {
		return ErrOffline
	}
	release, err := pkgmgr.Exclusive(ctx)
	if err != nil {
		return err
	}
	defer release()
	if free, err := s.FreeBytes(); err != nil {
		return fmt.Errorf("free flash: %w", err)
	} else if free < NeedFree {
		return fmt.Errorf("%w: %d MiB free, %d MiB needed", ErrNoSpace, free>>20, NeedFree>>20)
	}
	if err := s.Guards.AcquireExclusive(InstallGuard, guard.RefuseTerminal); err != nil {
		return err
	}
	if err := s.Packages.Update(ctx); err != nil {
		slog.Warn("tailscale: package list refresh failed; installing from cached lists", "err", err)
	}
	if err := s.install(ctx); err != nil {
		return fmt.Errorf("%w (guard %s kept)", err, InstallGuard)
	}
	return s.Guards.Release(InstallGuard)
}

// install: the package's post-install already starts tailscaled (apk-add.txt). Restart makes
// the running instance explicit so DNS can be pinned off, then Tailscale is stopped and its
// boot link removed: it stays off until the user switches it on (SetRunning).
func (s *Service) install(ctx context.Context) error {
	if err := s.Packages.Install(ctx, Package); err != nil {
		return fmt.Errorf("install %s: %w", Package, err)
	}
	if err := s.Services.ServiceAction(ctx, Package, "restart"); err != nil {
		return fmt.Errorf("%s restart: %w", Package, err)
	}
	if err := s.waitReady(ctx); err != nil {
		return fmt.Errorf("tailscaled not answering after install: %w", err)
	}
	if err := s.CLI.Set(ctx, []string{"--accept-dns=false"}); err != nil {
		return err
	}
	return s.serviceActions(ctx, "stop", "disable")
}

func (s *Service) serviceActions(ctx context.Context, actions ...string) error {
	for _, a := range actions {
		if err := s.Services.ServiceAction(ctx, Package, a); err != nil {
			return fmt.Errorf("%s %s: %w", Package, a, err)
		}
	}
	return nil
}

// SetRunning switches tailscaled on (boot link, start, wait until it answers) or off (stop,
// no boot link). Off is not a logout: the node key and preferences stay in the state file, so
// switching on again rejoins the same tailnet. The DNS reconciler pops the MagicDNS layer
// while the daemon is stopped (Want).
func (s *Service) SetRunning(ctx context.Context, on bool) (Status, error) {
	err := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		defer s.exclusive()()
		if held, err := s.guardHeld(InstallGuard); err != nil || held {
			return errors.Join(err, busyIf(held))
		}
		if _, err := s.Packages.Version(ctx, Package); err != nil {
			return err
		}
		if on {
			if err := s.failoverOff(); err != nil {
				return err
			}
		}
		if err := s.Guards.Clear(ChangeGuard); err != nil {
			return err
		}
		if err := s.Guards.Acquire(ChangeGuard); err != nil {
			return err
		}
		if on {
			if err := s.serviceActions(ctx, "enable", "start"); err != nil {
				return fmt.Errorf("%w (guard %s kept)", err, ChangeGuard)
			}
			if err := s.waitReady(ctx); err != nil {
				return fmt.Errorf("tailscaled not answering: %w (guard %s kept)", err, ChangeGuard)
			}
		} else if err := s.serviceActions(ctx, "stop", "disable"); err != nil {
			return fmt.Errorf("%w (guard %s kept)", err, ChangeGuard)
		}
		return s.Guards.Release(ChangeGuard)
	}()
	if err != nil {
		return Status{}, err
	}
	return s.Status(ctx)
}

func (s *Service) waitReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, ReadyTimeout)
	defer cancel()
	every := s.every()
	for {
		_, err := s.CLI.Status(ctx)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (last: %v)", ctx.Err(), err)
		case <-time.After(every):
		}
	}
}

func (s *Service) every() time.Duration {
	if s.Every <= 0 {
		return 500 * time.Millisecond
	}
	return s.Every
}

// Login brings tailscaled up with Travo's prefs. Before login the CLI times out in
// NeedsLogin by design; the returned status then carries the auth URL.
func (s *Service) Login(ctx context.Context) (Status, error) {
	off := func(context.Context, Backend) error { return s.failoverOff() }
	err := s.changeChecked(ctx, off, func(ctx context.Context, b Backend) error {
		err := s.CLI.Up(ctx, LoginArgs(CurrentDesired(b)))
		if errors.Is(err, ErrUpTimeout) {
			return nil
		}
		return err
	})
	if err != nil {
		return Status{}, err
	}
	return s.Status(ctx)
}

// Logout signs the router out of the tailnet; the DNS reconciler pops the layer on its next tick.
func (s *Service) Logout(ctx context.Context) (Status, error) {
	err := s.change(ctx, func(ctx context.Context, _ Backend) error { return s.CLI.Logout(ctx) })
	if err != nil {
		return Status{}, err
	}
	return s.Status(ctx)
}

// change runs one user-initiated write under ChangeGuard. The user's action is the
// explicit retry, so a guard left by an earlier failure is cleared by name first.
func (s *Service) change(ctx context.Context, apply func(context.Context, Backend) error) error {
	return s.changeChecked(ctx, nil, apply)
}

// changeChecked runs check before the guard is taken: a refusal writes nothing and leaves
// no guard. An apply error wrapped in restored releases the guard (the old state is live).
func (s *Service) changeChecked(ctx context.Context, check, apply func(context.Context, Backend) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.exclusive()()
	if held, err := s.guardHeld(InstallGuard); err != nil || held {
		return errors.Join(err, busyIf(held))
	}
	b, err := s.ready(ctx)
	if err != nil {
		return err
	}
	if check != nil {
		if err := check(ctx, b); err != nil {
			return err
		}
	}
	if err := s.Guards.Clear(ChangeGuard); err != nil {
		return err
	}
	if err := s.Guards.Acquire(ChangeGuard); err != nil {
		return err
	}
	if err := apply(ctx, b); err != nil {
		var r restored
		if errors.As(err, &r) {
			return errors.Join(r.error, s.Guards.Release(ChangeGuard))
		}
		return fmt.Errorf("%w (guard %s kept)", err, ChangeGuard)
	}
	return s.Guards.Release(ChangeGuard)
}

// ready requires the package and a running daemon, and returns the parsed backend.
func (s *Service) ready(ctx context.Context) (Backend, error) {
	if _, err := s.Packages.Version(ctx, Package); err != nil {
		return Backend{}, err
	}
	running, err := s.Services.ServiceRunning(ctx, Package)
	if err != nil {
		return Backend{}, err
	}
	if !running {
		return Backend{}, ErrNotRunning
	}
	return s.backend(ctx)
}

// exclusive takes the lock shared with failover (a no-op without failover) and returns
// its release; every change holds it, so a failover enable never interleaves with one.
func (s *Service) exclusive() func() {
	if s.Exclusive == nil {
		return func() {}
	}
	s.Exclusive.Lock()
	return s.Exclusive.Unlock
}

// failoverOff refuses Tailscale while failover is enabled; callers hold exclusive().
func (s *Service) failoverOff() error {
	if s.FailoverEnabled == nil {
		return nil
	}
	on, err := s.FailoverEnabled()
	if err != nil {
		return fmt.Errorf("failover settings: %w", err)
	}
	if on {
		return ErrFailover
	}
	return nil
}

func busyIf(held bool) error {
	if held {
		return ErrBusy
	}
	return nil
}
