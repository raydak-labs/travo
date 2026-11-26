package catalog

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

// countingPkgs records how many package calls run at once across services.
type countingPkgs struct {
	*failingPkgs
	running, peak *int32
}

func (c countingPkgs) Install(ctx context.Context, p string) error {
	n := atomic.AddInt32(c.running, 1)
	defer atomic.AddInt32(c.running, -1)
	for {
		old := atomic.LoadInt32(c.peak)
		if n <= old || atomic.CompareAndSwapInt32(c.peak, old, n) {
			break
		}
	}
	time.Sleep(5 * time.Millisecond)
	return c.failingPkgs.Install(ctx, p)
}

// Two services (as tailscale and the catalog would be) installing at once run one after
// the other, each checking free flash only once it holds the package-op lock.
func TestConcurrentInstallsAreSerialised(t *testing.T) {
	var running, peak, checksWhileBusy int32
	var wg sync.WaitGroup
	for _, id := range []string{"vnstat", "cloudflared"} {
		s, pkgs, _ := newService(t)
		s.Packages = countingPkgs{pkgs, &running, &peak}
		s.FreeBytes = func() (uint64, error) {
			if atomic.LoadInt32(&running) > 0 {
				atomic.AddInt32(&checksWhileBusy, 1)
			}
			return 64 * MiB, nil
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Install(context.Background(), id); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if peak != 1 || checksWhileBusy != 0 {
		t.Fatalf("peak %d concurrent installs, %d free checks during another install", peak, checksWhileBusy)
	}
}

func TestInstallWaitsForTheRunningPackageOp(t *testing.T) {
	s, pkgs, _ := newService(t)
	release, err := pkgmgr.Exclusive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := s.Install(ctx, "vnstat"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
	release()
	if _, err := pkgs.Version(context.Background(), "vnstat2"); err == nil {
		t.Fatal("installed while another package operation held the lock")
	}
}

func TestInstallAndRemoveRefuseWhileFirmwareUpgradeIsPending(t *testing.T) {
	s, pkgs, _ := newService(t)
	if err := s.Guards.Acquire("firmware-upgrade-in-progress"); err != nil {
		t.Fatal(err)
	}
	if err := s.Install(context.Background(), "vnstat"); !errors.Is(err, guard.ErrTerminal) {
		t.Fatalf("install: %v", err)
	}
	if _, err := pkgs.Version(context.Background(), "vnstat2"); err == nil {
		t.Fatal("installed during a firmware upgrade")
	}
	if err := s.Remove(context.Background(), "vnstat"); !errors.Is(err, guard.ErrTerminal) {
		t.Fatalf("remove: %v", err)
	}
}
