package catalog

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

type failingPkgs struct {
	*pkgmgr.Mock
	installErr, versionErr error
	keep                   string
}

func (f *failingPkgs) Install(ctx context.Context, p string) error {
	if f.installErr != nil {
		return f.installErr
	}
	return f.Mock.Install(ctx, p)
}

func (f *failingPkgs) Remove(ctx context.Context, p string) error {
	if p == f.keep {
		return nil // apk: "not removed due to" and exit 0
	}
	return f.Mock.Remove(ctx, p)
}

func (f *failingPkgs) Version(ctx context.Context, p string) (string, error) {
	if f.versionErr != nil {
		return "", f.versionErr
	}
	return f.Mock.Version(ctx, p)
}

func newService(t *testing.T) (*Service, *failingPkgs, *mock.Device) {
	t.Helper()
	dev := mock.New()
	pkgs := &failingPkgs{Mock: pkgmgr.NewMock()}
	s := &Service{Packages: pkgs, Services: dev, Worlds: dev,
		Guards:    guard.Dir{Path: t.TempDir(), Now: time.Now},
		FreeBytes: func() (uint64, error) { return 64 * MiB, nil },
		Online:    func(context.Context) bool { return true }}
	return s, pkgs, dev
}

func item(t *testing.T, s *Service, id string) Item {
	t.Helper()
	v, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range v.Entries {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("%s missing", id)
	return Item{}
}

func TestEntriesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Entries {
		if seen[e.ID] || len(e.Packages) == 0 || e.Size == 0 || (e.Daemon && e.InitScript == "") {
			t.Errorf("bad entry %+v", e)
		}
		seen[e.ID] = true
	}
}

func TestListReportsStateFromThePorts(t *testing.T) {
	s, pkgs, dev := newService(t)
	ctx := context.Background()
	_ = pkgs.Mock.Install(ctx, "sqm-scripts")
	_ = pkgs.Mock.Install(ctx, "vnstat2")
	dev.SetService("sqm", true, false)
	dev.SetService("vnstat", true, true)
	if it := item(t, s, "sqm"); !it.Installed || !it.BuiltIn || !it.Autostart || it.RunningSupported || it.LuCIPath == "" {
		t.Fatalf("sqm %+v", it)
	}
	if it := item(t, s, "vnstat"); !it.Installed || it.BuiltIn || !it.Autostart || !it.Running || it.Version == "" {
		t.Fatalf("vnstat %+v", it)
	}
	if it := item(t, s, "speedtest"); it.Installed || it.AutostartSupported || it.SizeBytes != 3*MiB {
		t.Fatalf("speedtest %+v", it)
	}
	pkgs.versionErr = errors.New("apk locked")
	if _, err := s.List(ctx); err == nil {
		t.Fatal("a failed presence check read as absent")
	}
}

func TestInstallIsGatedAndGuarded(t *testing.T) {
	s, pkgs, _ := newService(t)
	ctx := context.Background()
	s.Online = func(context.Context) bool { return false }
	if err := s.Install(ctx, "vnstat"); !errors.Is(err, ErrOffline) {
		t.Fatalf("offline: %v", err)
	}
	s.Online = func(context.Context) bool { return true }
	s.FreeBytes = func() (uint64, error) { return 40 * MiB, nil }
	if err := s.Install(ctx, "cloudflared"); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("no space: %v", err)
	}
	if err := s.Install(ctx, "nope"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("unknown: %v", err)
	}
	pkgs.installErr = errors.New("feed down")
	if err := s.Install(ctx, "vnstat"); err == nil {
		t.Fatal("failed install succeeded")
	}
	if v, _ := s.List(ctx); !v.Busy {
		t.Fatal("failed install released its guard")
	}
	if err := s.SetAutostart(ctx, "vnstat", true); !errors.Is(err, ErrBusy) {
		t.Fatalf("autostart while guarded: %v", err)
	}
	pkgs.installErr = nil
	if err := s.Install(ctx, "vnstat"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"vnstat2", "luci-app-vnstat2"} {
		if _, err := pkgs.Version(ctx, p); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	if v, _ := s.List(ctx); v.Busy {
		t.Fatal("retry kept the guard")
	}
}

func TestRemove(t *testing.T) {
	s, pkgs, _ := newService(t)
	ctx := context.Background()
	_ = pkgs.Mock.Install(ctx, "sqm-scripts")
	if err := s.Remove(ctx, "sqm"); !errors.Is(err, ErrBuiltIn) {
		t.Fatalf("built-in: %v", err)
	}
	if _, err := pkgs.Version(ctx, "sqm-scripts"); err != nil {
		t.Fatal("built-in package removed")
	}
	_ = s.Install(ctx, "watchcat")
	pkgs.keep = "watchcat"
	if err := s.Remove(ctx, "watchcat"); !errors.Is(err, ErrInUse) {
		t.Fatalf("in use: %v", err)
	}
	pkgs.keep = ""
	if err := s.Remove(ctx, "watchcat"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"watchcat", "luci-app-watchcat"} {
		if _, err := pkgs.Version(ctx, p); !errors.Is(err, pkgmgr.ErrNotInstalled) {
			t.Fatalf("%s still installed", p)
		}
	}
}

func TestSetAutostart(t *testing.T) {
	s, pkgs, dev := newService(t)
	ctx := context.Background()
	if err := s.SetAutostart(ctx, "speedtest", true); !errors.Is(err, ErrNoAutostart) {
		t.Fatalf("no init script: %v", err)
	}
	if err := s.SetAutostart(ctx, "watchcat", true); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("not installed: %v", err)
	}
	_ = pkgs.Mock.Install(ctx, "watchcat")
	dev.SetService("watchcat", false, false)
	if err := s.SetAutostart(ctx, "watchcat", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAutostart(ctx, "watchcat", false); err != nil {
		t.Fatal(err)
	}
	want := []string{"watchcat enable", "watchcat start", "watchcat stop", "watchcat disable"}
	if !slices.Equal(dev.ServiceActions, want) {
		t.Fatalf("actions %v", dev.ServiceActions)
	}
}
