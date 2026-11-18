package usbtether

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

// InstallGuard covers a driver install (kernel modules, init scripts: invariants §13). It is
// kept on failure and cleared by name only by the user's next install request.
const InstallGuard = "usbtether-install-in-progress"

// usbmuxd pairs an iPhone so it exposes its tether link; its package ships a procd init
// script and a USB hotplug script (testdata/device/axt1800-25.12.6/usbtether/usbmuxd-init.txt).
const usbmuxd = "usbmuxd"

// NeedFree is the overlay space a driver set needs: its installed size with dependencies,
// rounded up (testdata/device/axt1800-25.12.6/usbtether/apk-packages.txt: Android ~0.1 MiB,
// iOS ~5.7 MiB with libopenssl3), plus the 20 MB headroom (invariants §5).
var NeedFree = map[Phone]uint64{Android: (1 + 20) << 20, IOS: (6 + 20) << 20}

var (
	ErrNoSpace      = errors.New("not enough free flash for the tethering drivers")
	ErrBusy         = errors.New("a USB tethering driver install is running")
	ErrUnknownPhone = errors.New("unknown phone type")
)

// InstallDrivers installs the driver set of one phone type through the PackageManager port
// under InstallGuard. The request is the explicit retry: it clears a kept guard by name.
// It reports whether anything was installed.
func (s *Service) InstallDrivers(ctx context.Context, phone Phone) (bool, error) {
	pkgs, ok := Drivers[phone]
	if !ok {
		return false, ErrUnknownPhone
	}
	if s.Packages == nil || s.Guards.Path == "" || s.FreeBytes == nil {
		return false, ErrUnavailable
	}
	if !s.installing.TryLock() {
		return false, ErrBusy
	}
	defer s.installing.Unlock()
	if err := s.Guards.Clear(InstallGuard); err != nil {
		return false, err
	}
	missing, err := s.missing(ctx, pkgs)
	if err != nil {
		return false, err
	}
	// iOS always (re)runs the usbmuxd setup: a retry after a failed `start` finds every
	// package present and must still finish it under the guard.
	setup := phone == IOS
	if len(missing) == 0 && !setup {
		return false, nil
	}
	if len(missing) > 0 {
		if free, err := s.FreeBytes(); err != nil {
			return false, fmt.Errorf("free flash: %w", err)
		} else if free < NeedFree[phone] {
			return false, fmt.Errorf("%w: %d MiB free, %d MiB needed", ErrNoSpace, free>>20, NeedFree[phone]>>20)
		}
	}
	if err := s.Guards.Acquire(InstallGuard); err != nil {
		return false, fmt.Errorf("usbtether guard: %w", err) // never degrade to unguarded
	}
	if len(missing) > 0 {
		if err := s.Packages.Update(ctx); err != nil {
			slog.Warn("usbtether: package list refresh failed; installing from cached lists", "err", err)
		}
	}
	for _, pkg := range missing {
		if err := s.Packages.Install(ctx, pkg); err != nil {
			return false, fmt.Errorf("install %s: %w (guard %s kept)", pkg, err, InstallGuard)
		}
	}
	if setup {
		if err := s.startUsbmuxd(ctx); err != nil {
			return false, fmt.Errorf("%s: %w (guard %s kept)", usbmuxd, err, InstallGuard)
		}
	}
	return len(missing) > 0, s.Guards.Release(InstallGuard)
}

// missing lists the absent packages. A failed query is not "absent" (invariants §13).
func (s *Service) missing(ctx context.Context, pkgs []string) ([]string, error) {
	var out []string
	for _, pkg := range pkgs {
		_, err := s.Packages.Version(ctx, pkg)
		switch {
		case err == nil:
		case errors.Is(err, pkgmgr.ErrNotInstalled):
			out = append(out, pkg)
		default:
			return nil, fmt.Errorf("query %s: %w", pkg, err)
		}
	}
	return out, nil
}

func (s *Service) startUsbmuxd(ctx context.Context) error {
	if s.Services == nil {
		return ErrUnavailable
	}
	for _, action := range []string{"enable", "start"} {
		if err := s.Services.ServiceAction(ctx, usbmuxd, action); err != nil {
			return fmt.Errorf("%s: %w", action, err)
		}
	}
	return nil
}

// Busy reports a kept install guard: the drivers stay as they are until the user retries.
func (s *Service) Busy() (bool, error) {
	stale, err := s.Guards.Stale()
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(stale, func(g guard.Info) bool { return g.Name == InstallGuard }), nil
}
