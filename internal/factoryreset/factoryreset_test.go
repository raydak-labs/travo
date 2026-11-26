package factoryreset

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// device records the order of events: the guard file must exist before the wipe is scheduled.
type device struct {
	guards  guard.Dir
	resets  int
	sawLock bool
	err     error
}

func (d *device) FactoryReset(context.Context) error {
	d.resets++
	_, err := os.Stat(filepath.Join(d.guards.Path, GuardName))
	d.sawLock = err == nil
	return d.err
}

func setup(t *testing.T) (*Service, *device) {
	t.Helper()
	g := guard.Dir{Path: t.TempDir(), Now: time.Now}
	d := &device{guards: g}
	return &Service{Device: d, Guards: g}, d
}

func held(t *testing.T, s *Service) []string {
	t.Helper()
	infos, err := s.Guards.(guard.Dir).Stale()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, i := range infos {
		names = append(names, i.Name)
	}
	return names
}

func TestResetWritesTheGuardFirstAndNeverRemovesIt(t *testing.T) {
	s, d := setup(t)
	if err := s.Reset(context.Background(), Phrase); err != nil {
		t.Fatal(err)
	}
	if d.resets != 1 || !d.sawLock {
		t.Fatalf("resets %d, guard present at wipe %v", d.resets, d.sawLock)
	}
	if got := held(t, s); len(got) != 1 || got[0] != GuardName {
		t.Fatalf("guards after success %v", got)
	}
	if err := s.Reset(context.Background(), Phrase); !errors.Is(err, ErrPending) || d.resets != 1 {
		t.Fatalf("second request: %v, resets %d", err, d.resets)
	}
}

func TestResetKeepsTheGuardWhenSchedulingFails(t *testing.T) {
	s, d := setup(t)
	d.err = errors.New("setsid: exit 1")
	if err := s.Reset(context.Background(), Phrase); !errors.Is(err, ErrFailed) {
		t.Fatalf("err %v", err)
	}
	if got := held(t, s); len(got) != 1 {
		t.Fatalf("guard removed after a failed schedule: %v", got)
	}
}

func TestResetRefusesWrongPhraseAndHeldGuards(t *testing.T) {
	s, d := setup(t)
	for _, p := range []string{"", "Factory Reset", "factory reset ", "yes"} {
		if err := s.Reset(context.Background(), p); !errors.Is(err, ErrConfirm) {
			t.Fatalf("%q: %v", p, err)
		}
	}
	if err := d.guards.Acquire("txn-in-progress"); err != nil {
		t.Fatal(err)
	}
	if err := s.Reset(context.Background(), Phrase); !errors.Is(err, ErrBusy) {
		t.Fatalf("held txn guard: %v", err)
	}
	if d.resets != 0 || len(held(t, s)) != 1 {
		t.Fatalf("resets %d guards %v", d.resets, held(t, s))
	}
}

// failingGuards makes the guard write fail: the reset must abort, never run unguarded.
type failingGuards struct{ guard.Dir }

func (failingGuards) AcquireExclusive(string, func([]guard.Info) error) error {
	return errors.New("read-only filesystem")
}

func TestGuardWriteFailureAborts(t *testing.T) {
	s, d := setup(t)
	s.Guards = failingGuards{guard.Dir{Path: t.TempDir(), Now: time.Now}}
	if err := s.Reset(context.Background(), Phrase); !errors.Is(err, ErrFailed) || d.resets != 0 {
		t.Fatalf("err %v resets %d", err, d.resets)
	}
}

// A reset racing another guarded operation: exactly one proceeds, and a refused reset
// leaves no factory-reset guard behind (nothing was wiped).
func TestResetRacingAnotherGuardedOperationHasOneWinner(t *testing.T) {
	for range 30 {
		s, d := setup(t)
		var resetErr, otherErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; resetErr = s.Reset(context.Background(), Phrase) }()
		go func() {
			defer wg.Done()
			<-start
			otherErr = d.guards.AcquireExclusive("reboot-in-progress", guard.RefuseAny)
		}()
		close(start)
		wg.Wait()
		if (resetErr == nil) == (otherErr == nil) {
			t.Fatalf("reset %v, other %v: want exactly one winner", resetErr, otherErr)
		}
		if resetErr != nil && (!errors.Is(resetErr, ErrBusy) || d.resets != 0 || slices.Contains(held(t, s), GuardName)) {
			t.Fatalf("refused reset: %v, resets %d, guards %v", resetErr, d.resets, held(t, s))
		}
	}
}

func TestStoppedDaemonRefuses(t *testing.T) {
	s, d := setup(t)
	s.Stopped = func() bool { return true }
	if err := s.Reset(context.Background(), Phrase); !errors.Is(err, ErrUnavailable) || d.resets != 0 {
		t.Fatalf("err %v", err)
	}
}
