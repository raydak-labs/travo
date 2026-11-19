package catalog

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

// Guard covers a catalog install or remove; the next explicit install or remove clears it.
const Guard = "catalog-package-in-progress"

var (
	ErrUnknown      = errors.New("no such catalog service")
	ErrUnavailable  = errors.New("the services catalog is unavailable")
	ErrBusy         = errors.New("a catalog install or remove did not finish; run it again")
	ErrOffline      = errors.New("no Internet uplink yet")
	ErrNoSpace      = errors.New("not enough free flash")
	ErrBuiltIn      = errors.New("the package is part of the firmware image")
	ErrInUse        = errors.New("another installed package still needs it")
	ErrNotInstalled = errors.New("the service is not installed")
	ErrNoAutostart  = errors.New("the service has no init script")
)

// Services is the Device port part the catalog drives.
type Services interface {
	device.Services
	device.ServiceAutostart
}

// Worlds reads /etc/apk/world and /rom/etc/apk/world (os.ErrNotExist on opkg).
type Worlds interface {
	PackageWorlds(ctx context.Context) (world, rom []byte, err error)
}

// Item is one catalog entry and its state on the router.
type Item struct {
	ID       string   `json:"id"`
	Packages []string `json:"packages"`
	// Installed is true when the first package is installed; Version is its version.
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	// BuiltIn: the first package ships in the firmware image and cannot be removed.
	BuiltIn bool `json:"built_in"`
	// SizeBytes is the flash an install takes, dependencies included.
	SizeBytes          uint64 `json:"size_bytes"`
	AutostartSupported bool   `json:"autostart_supported"`
	Autostart          bool   `json:"autostart"`
	RunningSupported   bool   `json:"running_supported"`
	Running            bool   `json:"running"`
	LuCIPath           string `json:"luci_path,omitempty"`
}

// View is the whole catalog.
type View struct {
	Entries []Item `json:"entries"`
	// Busy: a catalog install or remove did not finish (its guard is kept).
	Busy bool `json:"busy"`
}

// Service installs, removes and switches catalog entries.
type Service struct {
	Packages  pkgmgr.PackageManager
	Services  Services
	Worlds    Worlds
	Guards    guard.Dir
	FreeBytes func() (uint64, error)
	Online    func(context.Context) bool

	mu sync.Mutex
}

// List never mutates. A failed presence check is an error, never "absent" (invariants §13).
func (s *Service) List(ctx context.Context) (View, error) {
	rom := s.romWorld(ctx)
	busy, err := s.busy()
	if err != nil {
		return View{}, err
	}
	out := View{Entries: make([]Item, 0, len(Entries)), Busy: busy}
	for _, e := range Entries {
		it, err := s.item(ctx, e, rom)
		if err != nil {
			return View{}, err
		}
		out.Entries = append(out.Entries, it)
	}
	return out, nil
}

func (s *Service) item(ctx context.Context, e Entry, rom map[string]bool) (Item, error) {
	it := Item{ID: e.ID, Packages: e.Packages, SizeBytes: e.Size, LuCIPath: e.LuCIPath,
		AutostartSupported: e.InitScript != "", RunningSupported: e.Daemon}
	v, err := s.Packages.Version(ctx, e.Packages[0])
	if errors.Is(err, pkgmgr.ErrNotInstalled) {
		return it, nil
	}
	if err != nil {
		return Item{}, fmt.Errorf("%s: %w", e.Packages[0], err)
	}
	it.Installed, it.Version, it.BuiltIn = true, v, rom[e.Packages[0]]
	if e.InitScript == "" {
		return it, nil
	}
	if it.Autostart, err = s.Services.ServiceEnabled(ctx, e.InitScript); err != nil {
		return Item{}, fmt.Errorf("%s enabled: %w", e.InitScript, err)
	}
	if e.Daemon {
		if it.Running, err = s.Services.ServiceRunning(ctx, e.InitScript); err != nil {
			return Item{}, fmt.Errorf("%s running: %w", e.InitScript, err)
		}
	}
	return it, nil
}

// romWorld lists the packages of the firmware image; empty when unreadable (opkg).
func (s *Service) romWorld(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	if s.Worlds == nil {
		return out
	}
	_, rom, err := s.Worlds.PackageWorlds(ctx)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("catalog: firmware package list", "err", err)
		}
		return out
	}
	sc := bufio.NewScanner(bytes.NewReader(rom))
	for sc.Scan() {
		if name := string(bytes.TrimSpace(sc.Bytes())); name != "" {
			out[name] = true
		}
	}
	return out
}

func (s *Service) busy() (bool, error) {
	g, err := s.Guards.Stale()
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(g, func(x guard.Info) bool { return x.Name == Guard }), nil
}

// Install is the user's explicit request; it clears a kept Guard (its retry path). The
// Internet and free-flash gates run before the guard; a failed install keeps it.
func (s *Service) Install(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := Find(id)
	if !ok {
		return ErrUnknown
	}
	if err := s.Guards.Clear(Guard); err != nil {
		return err
	}
	missing, err := s.missing(ctx, e)
	if err != nil || len(missing) == 0 {
		return err
	}
	if s.Online != nil && !s.Online(ctx) {
		return ErrOffline
	}
	if free, err := s.FreeBytes(); err != nil {
		return fmt.Errorf("free flash: %w", err)
	} else if need := e.Size + Headroom; free < need {
		return fmt.Errorf("%w: %d MiB free, %d MiB needed", ErrNoSpace, free/MiB, need/MiB)
	}
	if err := s.Guards.Acquire(Guard); err != nil {
		return err
	}
	if err := s.Packages.Update(ctx); err != nil {
		slog.Warn("catalog: package list refresh failed; installing from cached lists", "err", err)
	}
	for _, p := range missing {
		if err := s.Packages.Install(ctx, p); err != nil {
			return fmt.Errorf("install %s: %w (guard %s kept)", p, err, Guard)
		}
	}
	if e.OffAfterInstall {
		for _, a := range []string{"stop", "disable"} {
			if err := s.Services.ServiceAction(ctx, e.InitScript, a); err != nil {
				return fmt.Errorf("%s %s: %w (guard %s kept)", e.InitScript, a, err, Guard)
			}
		}
	}
	return s.Guards.Release(Guard)
}

func (s *Service) missing(ctx context.Context, e Entry) ([]string, error) {
	var out []string
	for _, p := range e.Packages {
		if _, err := s.Packages.Version(ctx, p); errors.Is(err, pkgmgr.ErrNotInstalled) {
			out = append(out, p)
		} else if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Remove deletes the entry's packages, LuCI app first. The package's prerm stops and
// disables its service. Packages of the firmware image are refused.
func (s *Service) Remove(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := Find(id)
	if !ok {
		return ErrUnknown
	}
	if err := s.Guards.Clear(Guard); err != nil {
		return err
	}
	rom := s.romWorld(ctx)
	if slices.ContainsFunc(e.Packages, func(p string) bool { return rom[p] }) {
		return ErrBuiltIn
	}
	if err := s.Guards.Acquire(Guard); err != nil {
		return err
	}
	for _, p := range slices.Backward(e.Packages) {
		if _, err := s.Packages.Version(ctx, p); errors.Is(err, pkgmgr.ErrNotInstalled) {
			continue
		}
		if err := s.Packages.Remove(ctx, p); err != nil {
			return fmt.Errorf("remove %s: %w (guard %s kept)", p, err, Guard)
		}
	}
	// apk keeps a package another one depends on and still exits 0.
	if _, err := s.Packages.Version(ctx, e.Packages[0]); err == nil {
		_ = s.Guards.Release(Guard)
		return ErrInUse
	} else if !errors.Is(err, pkgmgr.ErrNotInstalled) {
		return fmt.Errorf("%s: %w (guard %s kept)", e.Packages[0], err, Guard)
	}
	return s.Guards.Release(Guard)
}

// SetAutostart enables and starts, or stops and disables, the entry's init script.
func (s *Service) SetAutostart(ctx context.Context, id string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := Find(id)
	if !ok {
		return ErrUnknown
	}
	if e.InitScript == "" {
		return ErrNoAutostart
	}
	if busy, err := s.busy(); err != nil {
		return err
	} else if busy {
		return ErrBusy
	}
	if _, err := s.Packages.Version(ctx, e.Packages[0]); errors.Is(err, pkgmgr.ErrNotInstalled) {
		return ErrNotInstalled
	} else if err != nil {
		return err
	}
	actions := []string{"enable", "start"}
	if !on {
		actions = []string{"stop", "disable"}
	}
	for _, a := range actions {
		if err := s.Services.ServiceAction(ctx, e.InitScript, a); err != nil {
			return fmt.Errorf("%s %s: %w", e.InitScript, a, err)
		}
	}
	return nil
}
