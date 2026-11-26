package usbtether

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

type failingInstall struct {
	*pkgmgr.Mock
	fail string
}

func (f failingInstall) Install(ctx context.Context, pkg string) error {
	if pkg == f.fail {
		return errors.New("apk: network unreachable")
	}
	return f.Mock.Install(ctx, pkg)
}

func installService(t *testing.T, pm pkgmgr.PackageManager) (*Service, *guard.Dir) {
	t.Helper()
	d := router(t)
	g := guard.Dir{Path: filepath.Join(t.TempDir(), "guards"), Now: time.Now}
	s := service(d, pm)
	s.Guards, s.Services = g, d
	s.FreeBytes = func() (uint64, error) { return 64 << 20, nil }
	d.SetService(usbmuxd, false, false)
	return s, &g
}

func TestInstallAndroidDrivers(t *testing.T) {
	pm := pkgmgr.NewMock()
	s, g := installService(t, pm)
	did, err := s.InstallDrivers(context.Background(), Android)
	if err != nil || !did {
		t.Fatalf("install = %v, %v", did, err)
	}
	for _, p := range Drivers[Android] {
		if _, err := pm.Version(context.Background(), p); err != nil {
			t.Errorf("%s not installed", p)
		}
	}
	if stale, _ := g.Stale(); len(stale) != 0 {
		t.Fatalf("guard left behind: %v", stale)
	}
	if did, err := s.InstallDrivers(context.Background(), Android); err != nil || did {
		t.Fatalf("second install = %v, %v; want nothing to do", did, err)
	}
}

func TestInstallIOSStartsUsbmuxd(t *testing.T) {
	s, _ := installService(t, pkgmgr.NewMock())
	if _, err := s.InstallDrivers(context.Background(), IOS); err != nil {
		t.Fatal(err)
	}
	got := s.Services.(*mock.Device).ServiceActions
	if !slices.Equal(got, []string{"usbmuxd enable", "usbmuxd start"}) {
		t.Fatalf("service actions = %v", got)
	}
}

func TestInstallFailureKeepsGuardAndRetryClearsIt(t *testing.T) {
	pm := failingInstall{Mock: pkgmgr.NewMock(), fail: "kmod-usb-net-cdc-ncm"}
	s, g := installService(t, pm)
	if _, err := s.InstallDrivers(context.Background(), Android); err == nil {
		t.Fatal("want install error")
	}
	if blocked, err := s.Busy(); err != nil || !blocked {
		t.Fatalf("guard kept = %v, %v", blocked, err)
	}
	st, err := s.Status(context.Background())
	if err != nil || !st.InstallBlocked {
		t.Fatalf("status = %+v, %v", st, err)
	}
	s.Packages = pm.Mock // the explicit retry succeeds
	if _, err := s.InstallDrivers(context.Background(), Android); err != nil {
		t.Fatal(err)
	}
	if stale, _ := g.Stale(); len(stale) != 0 {
		t.Fatalf("retry left guard: %v", stale)
	}
}

func TestInstallRefusesUnknownPhoneAndFailedQuery(t *testing.T) {
	s, _ := installService(t, pkgmgr.NewMock())
	if _, err := s.InstallDrivers(context.Background(), Phone("nokia")); !errors.Is(err, ErrUnknownPhone) {
		t.Fatalf("err = %v", err)
	}
	s.Packages = brokenPM{}
	if did, err := s.InstallDrivers(context.Background(), Android); err == nil || did {
		t.Fatalf("failed query = %v, %v; a failed query is not absent", did, err)
	}
	if blocked, _ := s.Busy(); blocked {
		t.Fatal("guard written although nothing was installed")
	}
}

func TestInstallWhileRunningIsBusy(t *testing.T) {
	s, _ := installService(t, pkgmgr.NewMock())
	s.installing.Lock()
	defer s.installing.Unlock()
	if _, err := s.InstallDrivers(context.Background(), Android); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallRefusesLowFlashBeforeTheGuard(t *testing.T) {
	s, g := installService(t, pkgmgr.NewMock())
	s.FreeBytes = func() (uint64, error) { return 10 << 20, nil }
	if _, err := s.InstallDrivers(context.Background(), IOS); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("err = %v", err)
	}
	if stale, _ := g.Stale(); len(stale) != 0 {
		t.Fatalf("guard written: %v", stale)
	}
}

func TestIOSRetryFinishesUsbmuxdSetupAfterPackagesLanded(t *testing.T) {
	s, g := installService(t, pkgmgr.NewMock())
	d := s.Services.(*mock.Device)
	d.ServiceErr = errors.New("usbmuxd: start failed")
	if _, err := s.InstallDrivers(context.Background(), IOS); err == nil {
		t.Fatal("want service error")
	}
	if blocked, _ := s.Busy(); !blocked {
		t.Fatal("guard not kept")
	}
	d.ServiceErr = nil
	d.ServiceActions = nil
	did, err := s.InstallDrivers(context.Background(), IOS)
	if err != nil || did {
		t.Fatalf("retry = %v, %v; want setup only, no package installed", did, err)
	}
	if !slices.Equal(d.ServiceActions, []string{"usbmuxd enable", "usbmuxd start"}) {
		t.Fatalf("retry actions = %v", d.ServiceActions)
	}
	if stale, _ := g.Stale(); len(stale) != 0 {
		t.Fatalf("guard left: %v", stale)
	}
}

func TestFailedQueryOnRetryKeepsTheGuard(t *testing.T) {
	pm := failingInstall{Mock: pkgmgr.NewMock(), fail: "kmod-usb-net-cdc-ncm"}
	s, _ := installService(t, pm)
	if _, err := s.InstallDrivers(context.Background(), Android); err == nil {
		t.Fatal("want install error")
	}
	s.Packages = brokenPM{}
	if _, err := s.InstallDrivers(context.Background(), Android); err == nil {
		t.Fatal("want query error")
	}
	if blocked, err := s.Busy(); err != nil || !blocked {
		t.Fatalf("guard kept = %v, %v; a failed query must not clear it", blocked, err)
	}
}
