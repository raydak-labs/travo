package txn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

// fileDevice makes Commit write the config file, like real uci does.
type fileDevice struct {
	*mock.Device
	cfgDir    string
	commitErr error
}

func (d *fileDevice) Commit(ctx context.Context, cfg string) error {
	if d.commitErr != nil {
		return d.commitErr
	}
	if p, _ := d.Device.Pending(ctx, cfg); p {
		if err := os.WriteFile(filepath.Join(d.cfgDir, cfg), []byte("changed\n"), 0o600); err != nil {
			return err
		}
	}
	return d.Device.Commit(ctx, cfg)
}

// shortSpawner rewrites the window argument so unit tests need not wait MinWindow.
type shortSpawner struct {
	ShellSpawner
	window time.Duration
	armArg string // overrides arm limit when set
}

func (s *shortSpawner) Start(script string, args ...string) (int, error) {
	args = append([]string(nil), args...)
	args[2] = fmt.Sprint(int(s.window / time.Second))
	if s.armArg != "" {
		args[3] = s.armArg
	}
	return s.ShellSpawner.Start(script, args...)
}

var ctx = context.Background()

// stepClock advances on every Now so waitOutcome terminates deterministically.
type stepClock struct{ n atomic.Int64 }

func (c *stepClock) Now() time.Time { return time.Unix(1700000000+c.n.Add(1), 0) }

// fastWindow: tests shrink the watchdog's seconds via a scripted window; Manager enforces
// MinWindow, so tests that need a quick timeout set Window after construction checks.
func setup(t *testing.T, window time.Duration) (*Manager, *fileDevice) {
	t.Helper()
	root := t.TempDir()
	cfgDir := filepath.Join(root, "config")
	_ = os.MkdirAll(cfgDir, 0o755)
	for _, c := range []string{"network", "wireless"} {
		_ = os.WriteFile(filepath.Join(cfgDir, c), []byte("original "+c+"\n"), 0o644)
	}
	d := &fileDevice{Device: mock.New(), cfgDir: cfgDir}
	for _, c := range []string{"network", "wireless"} {
		_ = d.Device.SetType(ctx, c, "s", "x")
		_ = d.Device.Commit(ctx, c)
	}
	var n atomic.Int64
	m := &Manager{
		Device: d, Locks: &uciwrite.Locks{},
		Guards:    guard.Dir{Path: filepath.Join(root, "guards"), Now: time.Now},
		Dir:       filepath.Join(root, "txn"),
		ConfigDir: cfgDir, Reload: []string{"true"}, UCI: "true",
		Spawner:     &shortSpawner{ShellSpawner: ShellSpawner{Shell: "/bin/sh", RunTimeout: 10 * time.Second}, window: window},
		Clock:       &stepClock{},
		NewID:       func() string { return fmt.Sprintf("%016x", n.Add(1)) },
		Window:      MinWindow,
		ArmLimit:    30 * time.Second,
		ApplyBudget: 5 * time.Second,
		Settle:      20 * time.Second,
	}
	return m, d
}

func stageSet(d *fileDevice, cfg string) func(context.Context) error {
	return func(ctx context.Context) error { return d.Device.Set(ctx, cfg, "s", "k", "v") }
}

func content(t *testing.T, m *Manager, cfg string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(m.ConfigDir, cfg))
	if errors.Is(err, os.ErrNotExist) {
		return "<absent>"
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func guarded(t *testing.T, m *Manager) bool {
	t.Helper()
	st, err := m.Guards.Stale()
	if err != nil {
		t.Fatal(err)
	}
	return len(st) > 0
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestConfirmKeepsChangeAndDisarmsWatchdog(t *testing.T) {
	t.Parallel() // each test owns its temp dirs and manager
	m, d := setup(t, 1*time.Second)
	st, err := m.Begin(ctx, "test", []string{"network"}, stageSet(d, "network"), nil)
	if err != nil || st.State != Pending {
		t.Fatal(st, err)
	}
	if content(t, m, "network") != "changed\n" || d.Reloads != 1 {
		t.Fatal("not applied")
	}
	if _, err := m.Confirm(st.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second) // past the window: watchdog must have exited without restoring
	if content(t, m, "network") != "changed\n" {
		t.Fatal("watchdog restored a confirmed transaction")
	}
	if guarded(t, m) {
		t.Fatal("guard left after confirm")
	}
}

func TestNoConfirmWatchdogRestores(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 1*time.Second)
	st, err := m.Begin(ctx, "test", []string{"network", "wireless"}, func(ctx context.Context) error {
		return errors.Join(stageSet(d, "network")(ctx), stageSet(d, "wireless")(ctx))
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The script restores configs one by one, then writes result: wait for result, not for one file.
	waitFor(t, func() bool { _, err := os.Stat(filepath.Join(m.Dir, st.ID, "result")); return err == nil })
	if content(t, m, "network") != "original network\n" || content(t, m, "wireless") != "original wireless\n" {
		t.Fatal("not restored")
	}
	if _, err := m.Confirm(st.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("late confirm must lose: %v", err)
	}
	if guarded(t, m) {
		t.Fatal("timeout restore left the guard: next change would be blocked until restart")
	}
	if _, err := m.Begin(ctx, "next", []string{"network"}, stageSet(d, "network"), nil); err != nil {
		t.Fatalf("second Begin after timeout restore: %v", err)
	}
}

func TestTimeoutRestoreIsReapedByNextBeginWithoutStatusCall(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 1*time.Second)
	st, _ := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil)
	waitFor(t, func() bool { _, err := os.Stat(filepath.Join(m.Dir, st.ID, "result")); return err == nil })
	if _, err := m.Begin(ctx, "next", []string{"network"}, stageSet(d, "network"), nil); err != nil {
		t.Fatal(err)
	}
}

func TestWindowStartsAfterArm(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 1*time.Second)
	slow := func(context.Context) error { time.Sleep(2 * time.Second); return nil } // longer than the window
	st, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), slow)
	if err != nil || st.State != Pending {
		t.Fatalf("slow probe ate the window: %+v %v", st, err)
	}
	if _, err := m.Confirm(st.ID); err != nil {
		t.Fatal("user did not get the window:", err)
	}
}

func TestArmLimitRestoresWhenApplyHangs(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 30*time.Second)
	m.Spawner.(*shortSpawner).armArg = "1" // the script's view; Manager validation uses ArmLimit
	hang := func(context.Context) error { time.Sleep(3 * time.Second); return nil }
	st, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), hang)
	if err == nil || st.State != RolledBack {
		t.Fatalf("must not report pending over a restored config: %+v %v", st, err)
	}
	if content(t, m, "network") != "original network\n" || guarded(t, m) {
		t.Fatal("not restored or guard left")
	}
}

func TestStopDuringBeginDoesNotCommit(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 30*time.Second)
	stop := false
	m.Stopped = func() bool { return stop }
	_, err := m.Begin(ctx, "t", []string{"network"}, func(ctx context.Context) error {
		stop = true // SIGTERM arrives while staging
		return stageSet(d, "network")(ctx)
	}, nil)
	if !errors.Is(err, ErrStopped) || content(t, m, "network") != "original network\n" {
		t.Fatal(err)
	}
}

func TestConfirmBeforeArmIsRefused(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 30*time.Second)
	st, _ := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil)
	_ = os.Remove(filepath.Join(m.Dir, st.ID, "armed")) // as during commit/reload/probe
	if _, err := m.Confirm(st.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("confirmed an unarmed change: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir, st.ID, "claim")); err == nil {
		t.Fatal("unarmed confirm wrote a claim and disarmed the watchdog")
	}
	_, _ = m.Rollback(st.ID)
}

func TestProbeFailureRollsBackAtOnce(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	st, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), func(context.Context) error {
		return errors.New("wlan1 carrier=false")
	})
	if !errors.Is(err, ErrProbeFailed) || st.State != RolledBack {
		t.Fatal(st, err)
	}
	if content(t, m, "network") != "original network\n" || guarded(t, m) {
		t.Fatal("not restored or guard left")
	}
	if _, err := os.Stat(filepath.Join(m.Dir, st.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("txn dir left")
	}
}

func TestCommitFailureRollsBack(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	d.commitErr = errors.New("disk full")
	st, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil)
	if err == nil || st.State != RolledBack {
		t.Fatal(st, err)
	}
}

func TestStageErrorRevertsAndCommitsNothing(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	_, err := m.Begin(ctx, "t", []string{"network"}, func(ctx context.Context) error {
		_ = stageSet(d, "network")(ctx)
		return errors.New("validation")
	}, nil)
	if err == nil || guarded(t, m) || d.Reloads != 0 {
		t.Fatal(err)
	}
	if p, _ := d.Pending(ctx, "network"); p {
		t.Fatal("staged delta left")
	}
}

func TestRefusals(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	if _, err := m.Begin(ctx, "t", []string{"network"}, func(context.Context) error { return nil }, nil); !errors.Is(err, ErrNoChanges) {
		t.Errorf("no changes: %v", err)
	}
	_ = d.Device.Set(ctx, "network", "s", "foreign", "1")
	if _, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil); err == nil {
		t.Error("committed foreign staged changes")
	}
	_ = d.Revert(ctx, "network")
	if _, err := m.Begin(ctx, "t", []string{"network"}, func(ctx context.Context) error {
		_, _, err := m.Locks.Lock(ctx, []string{"dhcp"})
		return err
	}, nil); !errors.Is(err, uciwrite.ErrReentrant) {
		t.Errorf("reentrant: %v", err)
	}
	m.Stopped = func() bool { return true }
	if _, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil); !errors.Is(err, ErrStopped) {
		t.Errorf("stopped: %v", err)
	}
	if guarded(t, m) {
		t.Error("guard left by a refusal")
	}
}

func TestSecondTransactionWhilePendingIsRefused(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	st, _ := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil)
	if _, err := m.Begin(ctx, "t", []string{"wireless"}, stageSet(d, "wireless"), nil); !errors.Is(err, ErrActive) {
		t.Fatal(err)
	}
	if _, err := m.Rollback(st.ID); err != nil {
		t.Fatal(err)
	}
	if content(t, m, "network") != "original network\n" || guarded(t, m) {
		t.Fatal("explicit rollback incomplete")
	}
}

func TestAbsentConfigIsDeletedOnRestore(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	_ = d.Device.SetType(ctx, "travo", "s", "x")
	_ = d.Device.Commit(ctx, "travo") // known to uci, but no file on disk yet
	st, err := m.Begin(ctx, "t", []string{"travo"}, stageSet(d, "travo"), nil)
	if err != nil || content(t, m, "travo") != "changed\n" {
		t.Fatal(err)
	}
	if _, err := m.Rollback(st.ID); err != nil {
		t.Fatal(err)
	}
	if content(t, m, "travo") != "<absent>" {
		t.Fatal("config that did not exist survived restore")
	}
}

func TestFailedRestoreKeepsGuardAndSaysSo(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	m.Reload = []string{"false"} // restore cannot complete
	st, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), func(context.Context) error { return errors.New("down") })
	if err == nil || st.State != RollbackFailed || !guarded(t, m) {
		t.Fatalf("must not claim rolled back: %+v %v", st, err)
	}
}

func TestBadIDsAreNotFound(t *testing.T) {
	t.Parallel()
	m, _ := setup(t, time.Second)
	for _, id := range []string{"", "../x", "0123456789abcdeg", "0123456789abcdef"} {
		if _, err := m.Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("%q: %v", id, err)
		}
	}
}

func TestRecoverRestoresWhenWatchdogIsGone(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	st, _ := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil)
	// simulate reboot: watchdog died, daemon restarts
	out, err := m.Recover(ctx, func(int, string) bool { return false })
	if err != nil || len(out) != 1 || out[0].State != RolledBack || out[0].ID != st.ID {
		t.Fatal(out, err)
	}
	if content(t, m, "network") != "original network\n" || guarded(t, m) {
		t.Fatal("recovery incomplete")
	}
}

func TestRecoverKeepsLiveWatchdogAndCleansConfirmed(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	st, _ := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil)
	out, err := m.Recover(ctx, func(int, string) bool { return true })
	if err != nil || len(out) != 1 || out[0].State != Pending || !guarded(t, m) {
		t.Fatal(out, err)
	}
	if content(t, m, "network") != "changed\n" {
		t.Fatal("recover restored under a live watchdog")
	}
	_ = os.WriteFile(filepath.Join(m.Dir, st.ID, "claim"), []byte("confirm\n"), 0o600) // confirmed, crash before cleanup
	if _, err := m.Recover(ctx, func(int, string) bool { return false }); err != nil || guarded(t, m) {
		t.Fatal(err)
	}
	if content(t, m, "network") != "changed\n" {
		t.Fatal("confirmed change restored")
	}
}

func TestRecoverDiscardsCrashBeforeWatchdog(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 30*time.Second)
	_ = m.Guards.Acquire(GuardName)
	dir := filepath.Join(m.Dir, "00000000000000aa")
	_ = os.MkdirAll(filepath.Join(dir, "snap"), 0o700)
	_ = os.WriteFile(filepath.Join(dir, "configs"), []byte("network\n"), 0o600) // no meta, no pid
	_ = stageSet(d, "network")(ctx)
	if _, err := m.Recover(ctx, func(int, string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	if guarded(t, m) {
		t.Fatal("crash before any commit left the guard stuck")
	}
	if p, _ := d.Pending(ctx, "network"); p {
		t.Fatal("staging not reverted")
	}
	if content(t, m, "network") != "original network\n" {
		t.Fatal("touched config")
	}
}

func TestRollbackWaitsForClaimedRestore(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 30*time.Second)
	st, _ := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil)
	_ = os.WriteFile(filepath.Join(m.Dir, st.ID, "claim"), []byte("restore\n"), 0o600) // watchdog claimed, still running
	m.Settle = 0
	got, err := m.Rollback(st.ID)
	if got.State != Restoring || err == nil || !guarded(t, m) {
		t.Fatalf("must report restoring, not failed: %+v %v", got, err)
	}
	_ = os.WriteFile(filepath.Join(m.Dir, st.ID, "result"), []byte("restored\n"), 0o600)
	if s, _ := m.Get(st.ID); s.State != RolledBack || guarded(t, m) {
		t.Fatalf("%+v", s)
	}
}

func TestBadTimingRejected(t *testing.T) {
	t.Parallel()
	for _, f := range []func(*Manager){
		func(m *Manager) { m.Window = time.Second },
		func(m *Manager) { m.ApplyBudget = 0 },
		func(m *Manager) { m.ArmLimit = m.ApplyBudget }, // no margin: commit could land after restore
	} {
		m, d := setup(t, time.Second)
		f(m)
		if _, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil); !errors.Is(err, ErrBadWindow) {
			t.Error(err)
		}
		if guarded(t, m) {
			t.Error("guard left")
		}
	}
}

func TestReapDoesNotReleaseGuardHeldByFailedRestore(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 30*time.Second)
	m.Reload = []string{"false"}
	failed, _ := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), func(context.Context) error { return errors.New("down") })
	if failed.State != RollbackFailed {
		t.Fatal(failed)
	}
	// a second, restored dir appears (e.g. after an operator guard clear)
	other := filepath.Join(m.Dir, "00000000000000ff")
	_ = os.MkdirAll(other, 0o700)
	_ = os.WriteFile(filepath.Join(other, "meta.json"), []byte(`{"id":"00000000000000ff","configs":["network"]}`), 0o600)
	_ = os.WriteFile(filepath.Join(other, "result"), []byte("restored\n"), 0o600)
	if s, _ := m.Get("00000000000000ff"); s.State != RolledBack {
		t.Fatal(s)
	}
	if !guarded(t, m) {
		t.Fatal("reaping a restored txn released the guard of a failed one")
	}
}
