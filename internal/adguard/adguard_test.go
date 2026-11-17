package adguard

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

type rig struct {
	svc  *Service
	dev  *mock.Device
	pkgs *pkgmgr.Mock
	dir  string
}

func newRig(t *testing.T) *rig {
	t.Helper()
	dir := t.TempDir()
	dev := mock.New()
	dev.SetService(Package, false, false)
	pkgs := pkgmgr.NewMock()
	svc := &Service{
		Packages: pkgs, Services: dev,
		Guards:    guard.Dir{Path: filepath.Join(dir, "guards"), Now: time.Now},
		Config:    filepath.Join(dir, "etc", "adguardhome", "adguardhome.yaml"),
		Backup:    filepath.Join(dir, "state", "adguardhome.yaml.old"),
		Marker:    filepath.Join(dir, "state", "adguard-install-attempted"),
		FreeBytes: func() (uint64, error) { return 70 << 20, nil },
		Online:    func(context.Context) bool { return true },
		Ready:     func(context.Context) error { return nil },
	}
	if err := os.MkdirAll(filepath.Join(dir, "state"), 0o700); err != nil {
		t.Fatal(err)
	}
	return &rig{svc: svc, dev: dev, pkgs: pkgs, dir: dir}
}

func (r *rig) installed(t *testing.T, yaml []byte) {
	t.Helper()
	if err := r.pkgs.Install(context.Background(), Package); err != nil {
		t.Fatal(err)
	}
	r.dev.SetService(Package, true, true)
	if err := os.MkdirAll(filepath.Dir(r.svc.Config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.svc.Config, yaml, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r *rig) guards(t *testing.T) []string {
	t.Helper()
	g, err := r.svc.Guards.Stale()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, x := range g {
		out = append(out, x.Name)
	}
	return out
}

func TestStatusAbsentPresentAndQueryFailure(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	st, err := r.svc.Status(ctx)
	if err != nil || st.Installed || st.Running {
		t.Fatalf("absent: %+v %v", st, err)
	}
	r.installed(t, readFixture(t))
	st, err = r.svc.Status(ctx)
	if err != nil || !st.Installed || !st.Running || st.Loopback || st.Version == "" {
		t.Fatalf("installed: %+v %v", st, err)
	}
	r.svc.Packages = failingPkgs{r.pkgs}
	if _, err := r.svc.Status(ctx); err == nil {
		t.Fatal("a failed presence check must be an error, not absent")
	}
}

type failingPkgs struct{ *pkgmgr.Mock }

func (failingPkgs) Version(context.Context, string) (string, error) {
	return "", errors.New("apk: database locked")
}

func TestAutoInstallSeedsLoopbackConfigBeforeThePackage(t *testing.T) {
	r := newRig(t)
	var seenAtInstall []byte
	r.svc.Packages = &recordingPkgs{Mock: r.pkgs, onInstall: func() { seenAtInstall, _ = os.ReadFile(r.svc.Config) }}
	if err := r.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(seenAtInstall, Seed) {
		t.Fatal("the loopback seed must exist before the package installs")
	}
	if !slices.Equal(r.dev.ServiceActions, []string{"adguardhome enable", "adguardhome restart"}) {
		t.Fatalf("actions = %v", r.dev.ServiceActions)
	}
	if g := r.guards(t); len(g) != 0 {
		t.Fatalf("guards left: %v", g)
	}
}

type recordingPkgs struct {
	*pkgmgr.Mock
	onInstall func()
}

func (p *recordingPkgs) Install(ctx context.Context, pkg string) error {
	p.onInstall()
	return p.Mock.Install(ctx, pkg)
}

func TestAutoInstallGates(t *testing.T) {
	ctx := context.Background()
	t.Run("offline", func(t *testing.T) {
		r := newRig(t)
		r.svc.Online = func(context.Context) bool { return false }
		if err := r.svc.Reconcile(ctx); !errors.Is(err, ErrOffline) {
			t.Fatal(err)
		}
	})
	t.Run("flash", func(t *testing.T) {
		r := newRig(t)
		r.svc.FreeBytes = func() (uint64, error) { return 40 << 20, nil }
		if err := r.svc.Reconcile(ctx); !errors.Is(err, ErrNoSpace) {
			t.Fatal(err)
		}
		if _, err := os.Stat(r.svc.Config); !os.IsNotExist(err) {
			t.Fatal("nothing may be written before the flash check passes")
		}
	})
	t.Run("one automatic attempt", func(t *testing.T) {
		r := newRig(t)
		r.svc.Packages = &recordingPkgs{Mock: r.pkgs, onInstall: func() {}}
		if err := os.WriteFile(r.svc.Marker, []byte("1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := r.svc.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := r.pkgs.Version(ctx, Package); err == nil {
			t.Fatal("auto install ran twice")
		}
		if err := r.svc.Install(ctx, true); err != nil {
			t.Fatal("the explicit retry ignores the marker:", err)
		}
	})
	t.Run("failed install keeps the guard; explicit retry clears it", func(t *testing.T) {
		r := newRig(t)
		r.svc.Packages = brokenInstall{r.pkgs}
		if err := r.svc.Reconcile(ctx); err == nil {
			t.Fatal("want install error")
		}
		if g := r.guards(t); !slices.Equal(g, []string{InstallGuard}) {
			t.Fatalf("guards = %v", g)
		}
		if err := r.svc.Reconcile(ctx); err != nil {
			t.Fatal("a guarded feature skips quietly:", err)
		}
		r.svc.Packages = r.pkgs
		if err := r.svc.Install(ctx, true); err != nil {
			t.Fatal(err)
		}
		if g := r.guards(t); len(g) != 0 {
			t.Fatalf("guards = %v", g)
		}
	})
}

type brokenInstall struct{ *pkgmgr.Mock }

func (brokenInstall) Install(context.Context, string) error { return errors.New("apk: fetch failed") }

func TestReconcileMigratesALanBoundConfig(t *testing.T) {
	r := newRig(t)
	r.installed(t, readFixture(t))
	if err := r.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(r.svc.Config)
	if !bytes.Equal(got, Seed) {
		t.Fatal("config not migrated to loopback")
	}
	if !slices.Equal(r.dev.ServiceActions, []string{"adguardhome restart"}) {
		t.Fatalf("actions = %v", r.dev.ServiceActions)
	}
	if _, err := os.Stat(r.svc.Backup); !os.IsNotExist(err) {
		t.Fatal("kept config must be removed after success")
	}
	if g := r.guards(t); len(g) != 0 {
		t.Fatalf("guards = %v", g)
	}
}

// A firmware image can ship the package without a config; AGH then runs its wizard on
// 0.0.0.0:3000 (seen on the AXT1800 after firstboot, 2026-11-17).
func TestReconcileSeedsAPreinstalledPackageWithoutConfig(t *testing.T) {
	r := newRig(t)
	r.installed(t, nil)
	if err := os.Remove(r.svc.Config); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(r.svc.Config)
	if !bytes.Equal(got, Seed) {
		t.Fatal("seed not written")
	}
	if !slices.Equal(r.dev.ServiceActions, []string{"adguardhome restart"}) {
		t.Fatalf("actions = %v", r.dev.ServiceActions)
	}
	if g := r.guards(t); len(g) != 0 {
		t.Fatalf("guards = %v", g)
	}
}

func TestSeedMissingKeepsGuardWhenAGHDoesNotAnswer(t *testing.T) {
	r := newRig(t)
	r.installed(t, nil)
	if err := os.Remove(r.svc.Config); err != nil {
		t.Fatal(err)
	}
	r.svc.Ready = func(context.Context) error { return errors.New("connection refused") }
	if err := r.svc.Reconcile(context.Background()); err == nil {
		t.Fatal("want an error")
	}
	if g := r.guards(t); len(g) != 1 {
		t.Fatalf("guards = %v, want the config guard kept", g)
	}
}

func TestMigrationRestoresWhenAGHDoesNotAnswer(t *testing.T) {
	r := newRig(t)
	old := readFixture(t)
	r.installed(t, old)
	r.svc.Ready = func(context.Context) error { return errors.New("connection refused") }
	err := r.svc.Migrate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("err = %v", err)
	}
	got, _ := os.ReadFile(r.svc.Config)
	if !bytes.Equal(got, old) {
		t.Fatal("old config not restored")
	}
	if !slices.Equal(r.dev.ServiceActions, []string{"adguardhome restart", "adguardhome restart"}) {
		t.Fatalf("actions = %v", r.dev.ServiceActions)
	}
	if g := r.guards(t); len(g) != 0 {
		t.Fatalf("a completed restore releases the guard: %v", g)
	}
}

func TestMigrationKeepsGuardWhenRestoreFails(t *testing.T) {
	r := newRig(t)
	r.installed(t, readFixture(t))
	r.dev.ServiceErr = errors.New("procd: timeout")
	if err := r.svc.Migrate(context.Background()); err == nil {
		t.Fatal("want error")
	}
	if g := r.guards(t); !slices.Equal(g, []string{ConfigGuard}) {
		t.Fatalf("guards = %v", g)
	}
	if err := r.svc.Migrate(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("a guarded migration must skip: %v", err)
	}
	// Next daemon start: Recover puts the kept file back and releases the guard.
	r.dev.ServiceErr = nil
	if err := r.svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(r.svc.Config)
	if !bytes.Equal(got, readFixture(t)) {
		t.Fatal("recover did not restore the kept config")
	}
	if g := r.guards(t); len(g) != 0 {
		t.Fatalf("guards = %v", g)
	}
}

// A daemon stop between writing the seed and restarting AGH leaves the guard without a
// kept file; Recover must apply the seed before it releases the guard (#169 review).
func TestRecoverRestartsAGHAfterInterruptedSeed(t *testing.T) {
	r := newRig(t)
	r.installed(t, Seed)
	if err := r.svc.Guards.Acquire(ConfigGuard); err != nil {
		t.Fatal(err)
	}
	r.svc.Ready = func(context.Context) error { return errors.New("connection refused") }
	if err := r.svc.Recover(context.Background()); err == nil {
		t.Fatal("want an error while AGH does not answer")
	}
	if g := r.guards(t); !slices.Equal(g, []string{ConfigGuard}) {
		t.Fatalf("guards = %v, want kept", g)
	}
	r.svc.Ready = func(context.Context) error { return nil }
	if err := r.svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.dev.ServiceActions, []string{"adguardhome restart", "adguardhome restart"}) {
		t.Fatalf("actions = %v", r.dev.ServiceActions)
	}
	if g := r.guards(t); len(g) != 0 {
		t.Fatalf("guards = %v", g)
	}
}

func TestMigrationRefusesUnknownShapeWithoutWriting(t *testing.T) {
	r := newRig(t)
	odd := []byte("http:\n  address: \"0.0.0.0:3000\"\n")
	r.installed(t, odd)
	if err := r.svc.Migrate(context.Background()); !errors.Is(err, ErrUnknownShape) {
		t.Fatal(err)
	}
	st, err := r.svc.Status(context.Background())
	if err != nil || st.ConfigError == "" || st.Loopback {
		t.Fatalf("%+v %v", st, err)
	}
	if err := r.svc.Reconcile(context.Background()); err != nil {
		t.Fatal("reconcile must not retry an unknown shape:", err)
	}
	if len(r.dev.ServiceActions) != 0 {
		t.Fatalf("actions = %v", r.dev.ServiceActions)
	}
}
