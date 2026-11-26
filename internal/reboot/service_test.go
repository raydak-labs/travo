package reboot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/factoryreset"
	"github.com/blackdark-org/travo-greenfield/internal/firmware"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/updateapply"
)

var errTest = errors.New("injected")

func testService(t *testing.T) (*Service, *mock.Device, guard.Dir, string, *time.Time) {
	t.Helper()
	root := t.TempDir()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	dev := mock.New()
	guards := guard.Dir{Path: filepath.Join(root, "guards"), Now: func() time.Time { return now }}
	recordPath := filepath.Join(root, "state", "reboot.json")
	svc := New(dev, guards, recordPath, func() time.Time { return now }, func() bool { return false })
	return svc, dev, guards, recordPath, &now
}

func guardExists(t *testing.T, guards guard.Dir, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(guards.Path, name))
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

func writeRecord(t *testing.T, path, bootID string, requested time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := atomicfile.WriteJSON(path, record{BootID: bootID, Requested: requested}, 0o600); err != nil {
		t.Fatal(err)
	}
}

func seedRequest(t *testing.T, guards guard.Dir, recordPath, bootID string, requested time.Time) {
	t.Helper()
	guards.Now = func() time.Time { return requested }
	if err := guards.Acquire(GuardName); err != nil {
		t.Fatal(err)
	}
	writeRecord(t, recordPath, bootID, requested)
}

func TestRebootSuccessWritesGuardAndRawRecord(t *testing.T) {
	svc, dev, guards, recordPath, now := testService(t)
	accepted, err := svc.Reboot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dev.Reboots != 1 {
		t.Fatalf("reboots = %d", dev.Reboots)
	}
	if !guardExists(t, guards, GuardName) {
		t.Fatal("guard missing after scheduling")
	}
	var got record
	if err := atomicfile.ReadJSON(recordPath, &got); err != nil {
		t.Fatal(err)
	}
	if got.BootID != "mock-boot-0" || !got.Requested.Equal(*now) {
		t.Fatalf("record = %+v", got)
	}
	sum := sha256.Sum256([]byte("mock-boot-0"))
	want := fmt.Sprintf("%x", sum)[:32]
	if accepted.BootID != want || accepted.BootID == got.BootID {
		t.Fatalf("accepted boot id = %q, want opaque %q", accepted.BootID, want)
	}
}

func TestRebootRepeatIsPending(t *testing.T) {
	svc, dev, _, _, _ := testService(t)
	if _, err := svc.Reboot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrPending) {
		t.Fatalf("second call: %v", err)
	}
	if dev.Reboots != 1 {
		t.Fatalf("reboots = %d", dev.Reboots)
	}
}

func TestRebootExcludedByOtherGuards(t *testing.T) {
	for _, tc := range []struct {
		name  string
		guard string
		want  error
	}{
		{name: "transaction", guard: txn.GuardName, want: ErrTxnPending},
		{name: "update", guard: updateapply.GuardName, want: ErrUpdateBusy},
		{name: "firmware upgrade", guard: firmware.GuardName, want: ErrTerminalBusy},
		{name: "factory reset", guard: factoryreset.GuardName, want: ErrTerminalBusy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, dev, guards, _, _ := testService(t)
			if err := guards.Acquire(tc.guard); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Reboot(context.Background()); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if guardExists(t, guards, GuardName) {
				t.Fatal("reboot guard left behind")
			}
			if dev.Reboots != 0 {
				t.Fatalf("reboots = %d", dev.Reboots)
			}
		})
	}
}

func TestRebootRechecksExclusionAfterAcquire(t *testing.T) {
	svc, dev, guards, _, now := testService(t)
	// Acquire writes its timestamp after the first stale-guard scan. Inject the
	// competing transaction at that exact point to exercise the second scan.
	svc.Guards.Now = func() time.Time {
		other := guard.Dir{Path: guards.Path, Now: func() time.Time { return *now }}
		if err := other.Acquire(txn.GuardName); err != nil {
			t.Fatal(err)
		}
		return *now
	}
	if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrTxnPending) {
		t.Fatalf("got %v", err)
	}
	if guardExists(t, guards, GuardName) {
		t.Fatal("reboot guard left behind")
	}
	if dev.Reboots != 0 {
		t.Fatalf("reboots = %d", dev.Reboots)
	}
}

func TestRebootGuardWriteFailure(t *testing.T) {
	svc, dev, guards, _, now := testService(t)
	// The first stale scan sees no guard directory. Sabotage it only when
	// Acquire serialises the guard so this exercises the write failure itself.
	svc.Guards.Now = func() time.Time {
		if err := os.RemoveAll(guards.Path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(guards.Path, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		return *now
	}
	if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrFailed) {
		t.Fatalf("got %v", err)
	}
	if dev.Reboots != 0 {
		t.Fatalf("reboots = %d", dev.Reboots)
	}
}

func TestRebootRecordWriteFailureReleasesGuard(t *testing.T) {
	svc, dev, guards, recordPath, _ := testService(t)
	state := filepath.Dir(recordPath)
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0o700) })
	if os.Geteuid() == 0 {
		// Root bypasses permission bits; renaming onto a directory still fails.
		svc.RecordPath = state
	}
	if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrFailed) {
		t.Fatalf("got %v", err)
	}
	if guardExists(t, guards, GuardName) {
		t.Fatal("guard retained after record failure")
	}
	if dev.Reboots != 0 {
		t.Fatalf("reboots = %d", dev.Reboots)
	}
}

func TestRebootSchedulingFailureCleansUpAndCanRetry(t *testing.T) {
	svc, dev, guards, recordPath, _ := testService(t)
	dev.RebootErr = errTest
	if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrFailed) {
		t.Fatalf("got %v", err)
	}
	if guardExists(t, guards, GuardName) {
		t.Fatal("guard retained after scheduling failure")
	}
	if _, err := os.Stat(recordPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("record retained after scheduling failure: %v", err)
	}
	dev.RebootErr = nil
	if _, err := svc.Reboot(context.Background()); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if dev.Reboots != 2 {
		t.Fatalf("reboots = %d", dev.Reboots)
	}
}

func TestRebootUnavailable(t *testing.T) {
	t.Run("boot id", func(t *testing.T) {
		svc, dev, guards, _, _ := testService(t)
		dev.BootIDErr = errTest
		if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("got %v", err)
		}
		if guardExists(t, guards, GuardName) || dev.Reboots != 0 {
			t.Fatal("unavailable request had side effects")
		}
	})
	t.Run("stopping", func(t *testing.T) {
		svc, dev, guards, _, _ := testService(t)
		svc.Stopped = func() bool { return true }
		if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("got %v", err)
		}
		if guardExists(t, guards, GuardName) || dev.Reboots != 0 {
			t.Fatal("unavailable request had side effects")
		}
	})
}

func TestRebootSameBootGuardGrace(t *testing.T) {
	t.Run("older retries", func(t *testing.T) {
		svc, dev, guards, recordPath, now := testService(t)
		seedRequest(t, guards, recordPath, "mock-boot-0", now.Add(-Grace-time.Second))
		guards.Now = func() time.Time { return *now }
		if _, err := svc.Reboot(context.Background()); err != nil {
			t.Fatal(err)
		}
		if dev.Reboots != 1 || !guardExists(t, guards, GuardName) {
			t.Fatalf("reboots = %d", dev.Reboots)
		}
	})
	t.Run("younger stays pending", func(t *testing.T) {
		svc, dev, guards, recordPath, now := testService(t)
		seedRequest(t, guards, recordPath, "mock-boot-0", now.Add(-Grace+time.Second))
		if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrPending) {
			t.Fatalf("got %v", err)
		}
		if dev.Reboots != 0 || !guardExists(t, guards, GuardName) {
			t.Fatal("young guard was cleared")
		}
	})
}

func TestRebootMissingRecordGuardGrace(t *testing.T) {
	t.Run("older retries", func(t *testing.T) {
		svc, dev, guards, recordPath, now := testService(t)
		guards.Now = func() time.Time { return now.Add(-Grace - time.Second) }
		if err := guards.Acquire(GuardName); err != nil {
			t.Fatal(err)
		}
		guards.Now = func() time.Time { return *now }
		if _, err := svc.Reboot(context.Background()); err != nil {
			t.Fatal(err)
		}
		if dev.Reboots != 1 || !guardExists(t, guards, GuardName) {
			t.Fatalf("reboots = %d", dev.Reboots)
		}
		var got record
		if err := atomicfile.ReadJSON(recordPath, &got); err != nil {
			t.Fatal(err)
		}
		if got.BootID != "mock-boot-0" {
			t.Fatalf("record boot id = %q", got.BootID)
		}
	})
	t.Run("younger stays pending", func(t *testing.T) {
		svc, dev, guards, _, now := testService(t)
		guards.Now = func() time.Time { return now.Add(-Grace + time.Second) }
		if err := guards.Acquire(GuardName); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Reboot(context.Background()); !errors.Is(err, ErrPending) {
			t.Fatalf("got %v", err)
		}
		if dev.Reboots != 0 || !guardExists(t, guards, GuardName) {
			t.Fatal("young guard without a record was cleared")
		}
	})
}

func TestRebootChangedBootCanRestartAgainWithoutSettle(t *testing.T) {
	svc, dev, guards, recordPath, _ := testService(t)
	dev.RotateBootOnReboot = true
	first, err := svc.Reboot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Reboot(context.Background())
	if err != nil {
		t.Fatalf("second restart after mock boot rotation: %v", err)
	}
	if first.BootID == second.BootID {
		t.Fatal("accepted boot id did not change")
	}
	if dev.Reboots != 2 || !guardExists(t, guards, GuardName) {
		t.Fatalf("reboots = %d", dev.Reboots)
	}
	var got record
	if err := atomicfile.ReadJSON(recordPath, &got); err != nil {
		t.Fatal(err)
	}
	if got.BootID != "mock-boot-1" {
		t.Fatalf("record boot id = %q, want mock-boot-1", got.BootID)
	}
}

func TestSettleChangedBootReleasesGuardAndRecord(t *testing.T) {
	svc, dev, guards, recordPath, now := testService(t)
	seedRequest(t, guards, recordPath, "old-boot", now.Add(-time.Minute))
	dev.SetBootID("new-boot")
	if err := svc.Settle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if guardExists(t, guards, GuardName) {
		t.Fatal("guard retained after new boot")
	}
	if _, err := os.Stat(recordPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("record retained: %v", err)
	}
}

func TestSettleSameBootKeepsGuardAndRecord(t *testing.T) {
	svc, _, guards, recordPath, now := testService(t)
	seedRequest(t, guards, recordPath, "mock-boot-0", now.Add(-time.Minute))
	if err := svc.Settle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !guardExists(t, guards, GuardName) {
		t.Fatal("same-boot guard released")
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatalf("same-boot record removed: %v", err)
	}
}

func TestSettleRemovesRecordWithoutGuard(t *testing.T) {
	svc, _, _, recordPath, now := testService(t)
	writeRecord(t, recordPath, "old-boot", now.Add(-time.Minute))
	if err := svc.Settle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(recordPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("record retained: %v", err)
	}
}

func TestSettleUnreadableBootKeepsGuard(t *testing.T) {
	svc, dev, guards, recordPath, now := testService(t)
	seedRequest(t, guards, recordPath, "old-boot", now.Add(-time.Minute))
	dev.BootIDErr = errTest
	if err := svc.Settle(context.Background()); err == nil {
		t.Fatal("expected boot id error")
	}
	if !guardExists(t, guards, GuardName) {
		t.Fatal("guard released without proof of a new boot")
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatalf("record removed: %v", err)
	}
}

func TestSettleUnknownBootKeepsGuardAndWarns(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{name: "missing record", setup: func(*testing.T, string) {}},
		{name: "unreadable record", setup: func(t *testing.T, path string) {
			t.Helper()
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, guards, recordPath, now := testService(t)
			guards.Now = func() time.Time { return now.Add(-time.Minute) }
			if err := guards.Acquire(GuardName); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, recordPath)
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			if err := svc.Settle(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !guardExists(t, guards, GuardName) {
				t.Fatal("guard released without a known boot")
			}
			if output := logs.String(); !strings.Contains(output, "level=WARN") || !strings.Contains(output, "unknown boot") {
				t.Fatalf("warning log = %q", output)
			}
		})
	}
}

func TestRebootConcurrentExactlyOneSchedules(t *testing.T) {
	svc, dev, _, _, _ := testService(t)
	const callers = 8
	var wg sync.WaitGroup
	results := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Reboot(context.Background())
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	succeeded, pending := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrPending):
			pending++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || pending != callers-1 || dev.Reboots != 1 {
		t.Fatalf("succeeded=%d pending=%d reboots=%d", succeeded, pending, dev.Reboots)
	}
}

func TestServiceBootIDIsOpaque(t *testing.T) {
	svc, _, _, _, _ := testService(t)
	got, err := svc.BootID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != OpaqueID("mock-boot-0") || got == "mock-boot-0" {
		t.Fatalf("boot id = %q", got)
	}
}
