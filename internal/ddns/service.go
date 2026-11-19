package ddns

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

const (
	// InitScript is the package's procd service (testdata ddns/initd-ddns.txt).
	InitScript = "ddns"
	// InstallGuard covers the package install; only the explicit install retry clears it.
	InstallGuard = "ddns-install-in-progress"
	// NeedFree: ddns-scripts (71 KiB) and ddns-scripts-services (17 KiB, testdata
	// ddns/apk-info-size.txt), rounded up to 1 MiB, plus the 20 MB headroom (invariants §15).
	NeedFree = (1 + 20) << 20
	// ServicesDir holds one JSON file per provider (testdata ddns/services-default-ls.txt).
	ServicesDir = "/usr/share/ddns/default"
	// IPFile is where ddns-scripts stores the registered address for LuCI's status
	// (dynamic_dns_updater.sh IPFILE="$ddns_rundir/$SECTION_ID.ip", rundir /var/run/ddns).
	IPFile = "/var/run/ddns/" + Section + ".ip"
)

var (
	ErrUnavailable = errors.New("dynamic DNS is unavailable")
	ErrBusy        = errors.New("a ddns-scripts install did not finish; install again")
	ErrOffline     = errors.New("no Internet uplink yet")
	ErrNoSpace     = errors.New("not enough free flash for ddns-scripts")
)

// Applier is txn.Manager.Apply: ddns changes cannot cut the caller off, so no confirm.
type Applier interface {
	Apply(ctx context.Context, configs []string, stage func(context.Context) error) error
}

// Service reads and changes ddns.travo.
type Service struct {
	UCI       device.UCI
	Keys      device.DDNSPasswordWriter
	Packages  pkgmgr.PackageManager
	Services  device.Services
	Guards    guard.Dir
	Txns      Applier
	FreeBytes func() (uint64, error)
	Online    func(context.Context) bool
	// Root prefixes ServicesDir and IPFile; empty on the router, a temp dir in tests.
	Root string

	mu sync.Mutex
}

// Settings never mutates and never returns the token.
func (s *Service) Settings(ctx context.Context) (View, error) {
	if _, err := s.Packages.Version(ctx, Package); errors.Is(err, pkgmgr.ErrNotInstalled) {
		return View{Services: []string{}}, nil
	} else if err != nil {
		return View{}, ErrUnavailable
	}
	secs, err := s.show(ctx)
	if err != nil {
		return View{}, err
	}
	v, err := Read(secs)
	if err != nil {
		return View{}, err
	}
	v.Installed = true
	v.Services = s.providers()
	if b, err := os.ReadFile(s.Root + IPFile); err == nil {
		if a, err := netip.ParseAddr(strings.TrimSpace(string(b))); err == nil {
			v.RegisteredIP = a.String()
		}
	}
	return v, nil
}

// show reads the ddns config; a missing one (removed by hand, or the mock) reads as empty.
func (s *Service) show(ctx context.Context) ([]device.Section, error) {
	secs, err := s.UCI.Show(ctx, Config)
	if errors.Is(err, device.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	return secs, nil
}

func (s *Service) providers() []string {
	files, err := filepath.Glob(filepath.Join(s.Root+ServicesDir, "*.json"))
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, strings.TrimSuffix(filepath.Base(f), ".json"))
	}
	slices.Sort(out)
	return out
}

// Set validates in, applies it to ddns.travo and restarts the service so the section runs
// (or stops) now. The token goes through Keys, never argv or logs.
func (s *Service) Set(ctx context.Context, in Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Txns == nil || s.Keys == nil || s.Services == nil {
		return ErrUnavailable
	}
	if err := s.idle(); err != nil {
		return err
	}
	if _, err := s.Packages.Version(ctx, Package); err != nil {
		return err
	}
	if _, _, err := s.plan(ctx, in); err != nil {
		return err
	}
	err := s.Txns.Apply(ctx, []string{Config}, func(ctx context.Context) error {
		writes, token, err := s.plan(ctx, in)
		if err != nil {
			return err
		}
		return s.stage(ctx, writes, token, in.Password)
	})
	if err != nil {
		return err
	}
	return s.Services.ServiceAction(ctx, InitScript, "restart")
}

func (s *Service) plan(ctx context.Context, in Request) ([]Write, bool, error) {
	secs, err := s.show(ctx)
	if err != nil {
		return nil, false, err
	}
	cur, err := Read(secs)
	if err != nil {
		return nil, false, err
	}
	if err := Validate(in, s.providers(), cur.PasswordSet); err != nil {
		return nil, false, err
	}
	writes, token := Plan(secs, in)
	if len(writes) == 0 && !token {
		return nil, false, ErrNoChange
	}
	return writes, token, nil
}

func (s *Service) stage(ctx context.Context, writes []Write, token bool, password string) error {
	for _, w := range writes {
		var err error
		switch w.Op {
		case "type":
			err = s.UCI.SetType(ctx, Config, Section, w.Value)
		case "set":
			err = s.UCI.Set(ctx, Config, Section, w.Option, w.Value)
		default:
			if err = s.UCI.DeleteOption(ctx, Config, Section, w.Option); errors.Is(err, device.ErrNotFound) {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	if !token {
		return nil
	}
	return s.Keys.SetDDNSPassword(ctx, password)
}

func (s *Service) idle() error {
	g, err := s.Guards.Stale()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(g, func(x guard.Info) bool { return x.Name == InstallGuard }) {
		return ErrBusy
	}
	return nil
}

// Install is the user's explicit request; it clears a kept InstallGuard (its retry path).
// Online and free-flash gates run before the guard; a failed install keeps it.
func (s *Service) Install(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Guards.Clear(InstallGuard); err != nil {
		return err
	}
	if _, err := s.Packages.Version(ctx, Package); err == nil {
		return nil
	} else if !errors.Is(err, pkgmgr.ErrNotInstalled) {
		return err
	}
	if s.Online != nil && !s.Online(ctx) {
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
		slog.Warn("ddns: package list refresh failed; installing from cached lists", "err", err)
	}
	if err := s.Packages.Install(ctx, Package); err != nil {
		return fmt.Errorf("install %s: %w (guard %s kept)", Package, err, InstallGuard)
	}
	if s.Services != nil {
		if err := s.Services.ServiceAction(ctx, InitScript, "enable"); err != nil {
			return fmt.Errorf("%s enable: %w (guard %s kept)", InitScript, err, InstallGuard)
		}
	}
	return s.Guards.Release(InstallGuard)
}
