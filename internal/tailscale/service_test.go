package tailscale

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/runner"
)

type rig struct {
	svc  *Service
	dev  *mock.Device
	pkgs *pkgmgr.Mock
	cli  *Mock
}

func newRig(t *testing.T) *rig {
	t.Helper()
	dev := mock.New()
	dev.SetService(Package, false, false)
	for _, c := range []string{"network", "firewall"} {
		secs, err := cli.ParseShow(c, string(fixture(t, "../uci-show-"+c+".txt")))
		if err != nil {
			t.Fatal(err)
		}
		if err := dev.Load(context.Background(), c, secs); err != nil {
			t.Fatal(err)
		}
	}
	pkgs := pkgmgr.NewMock()
	ts := NewMock()
	svc := &Service{
		CLI: ts, Packages: pkgs, Services: dev, UCI: dev,
		Guards:    guard.Dir{Path: filepath.Join(t.TempDir(), "guards"), Now: time.Now},
		FreeBytes: func() (uint64, error) { return 60 << 20, nil },
		Online:    func(context.Context) bool { return true },
		Every:     time.Millisecond,
	}
	return &rig{svc: svc, dev: dev, pkgs: pkgs, cli: ts}
}

func (r *rig) installed(t *testing.T) {
	t.Helper()
	if err := r.pkgs.Install(context.Background(), Package); err != nil {
		t.Fatal(err)
	}
	r.dev.SetService(Package, true, true)
}

func (r *rig) held(t *testing.T, name string) bool {
	t.Helper()
	held, err := r.svc.guardHeld(name)
	if err != nil {
		t.Fatal(err)
	}
	return held
}

func TestStatusAbsentPackageIsNotAnError(t *testing.T) {
	r := newRig(t)
	st, err := r.svc.Status(context.Background())
	if err != nil || st.Installed || st.Running {
		t.Fatalf("%+v %v", st, err)
	}
	if st.ExitNodes == nil || st.Health == nil || st.Self.IPs == nil || st.Prefs.AdvertiseRoutes == nil {
		t.Fatalf("null collections: %+v", st)
	}
}

func TestInstallPinsDNSOffAndReleasesGuard(t *testing.T) {
	r := newRig(t)
	if err := r.svc.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.dev.ServiceActions, []string{"tailscale enable", "tailscale restart"}) {
		t.Fatalf("actions %v", r.dev.ServiceActions)
	}
	if len(r.cli.Calls) != 1 || !slices.Equal(r.cli.Calls[0], []string{"set", "--accept-dns=false"}) {
		t.Fatalf("calls %v", r.cli.Calls)
	}
	st, err := r.svc.Status(context.Background())
	if err != nil || !st.Installed || !st.Running || st.Busy || st.Prefs.AcceptDNS {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestInstallRefusesBeforeAnyWrite(t *testing.T) {
	for name, tc := range map[string]struct {
		mod  func(*Service)
		want error
	}{
		"offline":  {func(s *Service) { s.Online = func(context.Context) bool { return false } }, ErrOffline},
		"no space": {func(s *Service) { s.FreeBytes = func() (uint64, error) { return 40 << 20, nil } }, ErrNoSpace},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			tc.mod(r.svc)
			if err := r.svc.Install(context.Background()); !errors.Is(err, tc.want) {
				t.Fatalf("got %v", err)
			}
			if _, err := r.pkgs.Version(context.Background(), Package); !errors.Is(err, pkgmgr.ErrNotInstalled) {
				t.Fatal("package installed")
			}
			if r.held(t, InstallGuard) {
				t.Fatal("guard left behind")
			}
		})
	}
}

func TestInstallFailureKeepsGuardAndRetryClearsIt(t *testing.T) {
	r := newRig(t)
	r.dev.ServiceErr = errors.New("procd down")
	if err := r.svc.Install(context.Background()); err == nil || !r.held(t, InstallGuard) {
		t.Fatalf("err %v, guard held %v", err, r.held(t, InstallGuard))
	}
	st, _ := r.svc.Status(context.Background())
	if !st.Busy {
		t.Fatal("kept guard not reported busy")
	}
	r.dev.ServiceErr = nil
	if err := r.svc.Install(context.Background()); err != nil {
		t.Fatal(err) // already installed: the retry clears the guard and is done
	}
	if r.held(t, InstallGuard) {
		t.Fatal("retry kept the guard")
	}
}

func TestLoginReturnsAuthURLThenRuns(t *testing.T) {
	r := newRig(t)
	r.installed(t)
	st, err := r.svc.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateNeedsLogin || st.AuthURL == "" || r.held(t, ChangeGuard) {
		t.Fatalf("%+v", st)
	}
	up := r.cli.Calls[0]
	if up[0] != "up" || up[1] != "--reset" || !slices.Contains(up, "--accept-dns=false") {
		t.Fatalf("up %v", up)
	}
	st, _ = r.svc.Status(context.Background())
	if st.State != StateRunning || !st.MagicDNS.Enabled {
		t.Fatalf("%+v", st)
	}
	if st, err = r.svc.Logout(context.Background()); err != nil || st.State != StateNeedsLogin {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestLoginKeepsTravoPrefs(t *testing.T) {
	r := newRig(t)
	r.installed(t)
	if err := r.cli.Set(context.Background(), []string{"--ssh=true", "--advertise-routes=192.168.8.0/24"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	up := r.cli.Calls[len(r.cli.Calls)-1]
	if !slices.Contains(up, "--ssh=true") || !slices.Contains(up, "--advertise-routes=192.168.8.0/24") {
		t.Fatalf("up %v", up)
	}
}

func TestChangeNeedsPackageAndDaemon(t *testing.T) {
	r := newRig(t)
	if _, err := r.svc.Login(context.Background()); !errors.Is(err, pkgmgr.ErrNotInstalled) {
		t.Fatalf("got %v", err)
	}
	r.installed(t)
	r.dev.SetService(Package, true, false)
	if _, err := r.svc.Logout(context.Background()); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("got %v", err)
	}
	if len(r.cli.Calls) != 0 {
		t.Fatalf("calls %v", r.cli.Calls)
	}
}

type failingUp struct{ *Mock }

func (failingUp) Up(context.Context, []string) error { return errors.New("backend error") }

func TestFailedLoginKeepsGuardUntilRetry(t *testing.T) {
	r := newRig(t)
	r.installed(t)
	r.svc.CLI = failingUp{r.cli}
	if _, err := r.svc.Login(context.Background()); err == nil || !r.held(t, ChangeGuard) {
		t.Fatalf("err %v", err)
	}
	r.svc.CLI = r.cli
	if _, err := r.svc.Login(context.Background()); err != nil || r.held(t, ChangeGuard) {
		t.Fatalf("retry: %v", err)
	}
}

func TestInstallRefusedDuringChange(t *testing.T) {
	r := newRig(t)
	if err := r.svc.Guards.Acquire(ChangeGuard); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Install(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("got %v", err)
	}
}

func TestCommandMapsUpTimeout(t *testing.T) {
	var got []string
	run := func(_ context.Context, name string, args ...string) (string, int, error) {
		got = append([]string{name}, args...)
		return "", 1, &runner.CommandError{Name: name, Args: args, Err: errors.New("exit status 1"),
			Stderr: strings.TrimSpace(string(fixture(t, "up-timeout.err")))}
	}
	err := Command{Run: run}.Up(context.Background(), []string{"--reset"})
	if !errors.Is(err, ErrUpTimeout) {
		t.Fatalf("got %v", err)
	}
	if !slices.Equal(got, []string{"tailscale", "up", "--reset", "--timeout=10s"}) {
		t.Fatalf("argv %v", got)
	}
	other := func(_ context.Context, name string, args ...string) (string, int, error) {
		return "", 1, &runner.CommandError{Name: name, Args: args, Err: errors.New("exit status 1"), Stderr: string(fixture(t, "up-revert.txt"))}
	}
	if err := (Command{Run: other}).Up(context.Background(), nil); err == nil || errors.Is(err, ErrUpTimeout) {
		t.Fatalf("revert error mapped to %v", err)
	}
}
