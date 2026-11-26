package guard

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

// race starts n goroutines at once and counts the nil results of run.
func race(n int, run func(i int) error) (won int32, errs []error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := run(i); err == nil {
				atomic.AddInt32(&won, 1)
			} else {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	return won, errs
}

func TestConcurrentAcquireOfOneNameHasOneWinner(t *testing.T) {
	d := newDir(t)
	won, errs := race(16, func(int) error { return d.Acquire("a-in-progress") })
	if won != 1 {
		t.Fatalf("%d winners, want 1", won)
	}
	for _, err := range errs {
		if !errors.Is(err, ErrActive) {
			t.Fatalf("loser got %v, want ErrActive", err)
		}
	}
}

// Two operations that refuse each other's guards never both pass (check-then-write race).
func TestConcurrentExclusiveAcquiresOfDifferentNamesHaveOneWinner(t *testing.T) {
	for range 20 {
		d := newDir(t)
		names := []string{"a-in-progress", "b-in-progress", "c-in-progress", "d-in-progress"}
		won, errs := race(len(names), func(i int) error { return d.AcquireExclusive(names[i], RefuseAny) })
		if won != 1 {
			t.Fatalf("%d winners, want 1", won)
		}
		for _, err := range errs {
			if !errors.Is(err, ErrHeld) {
				t.Fatalf("loser got %v, want ErrHeld", err)
			}
		}
		if st, _ := d.Stale(); len(st) != 1 {
			t.Fatalf("guards on disk %v, want only the winner's", st)
		}
	}
}

func TestRefusedAcquireWritesNothing(t *testing.T) {
	d := newDir(t)
	_ = d.Acquire("firmware-upgrade-in-progress")
	if err := d.AcquireExclusive("catalog-package-in-progress", RefuseTerminal); !errors.Is(err, ErrTerminal) || !errors.Is(err, ErrActive) {
		t.Fatalf("got %v, want ErrTerminal wrapping ErrActive", err)
	}
	entries, _ := os.ReadDir(d.Path)
	if len(entries) != 1 {
		t.Fatalf("dir holds %d entries, want only the firmware guard (no temp left)", len(entries))
	}
}

func TestRefuseTerminalAllowsOtherGuards(t *testing.T) {
	if err := RefuseTerminal([]Info{{Name: "txn-in-progress"}, {Name: "catalog-package-in-progress"}}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"firmware-upgrade-in-progress", "factory-reset-in-progress", "reboot-in-progress", "update-in-progress"} {
		if !IsTerminal(n) || RefuseTerminal([]Info{{Name: n}}) == nil {
			t.Errorf("%s not terminal", n)
		}
	}
}

func TestCheckSeesHeldGuards(t *testing.T) {
	d := newDir(t)
	if err := d.Check(RefuseAny); err != nil {
		t.Fatalf("empty dir: %v", err)
	}
	_ = d.Acquire("a-in-progress")
	if err := d.Check(RefuseAny); !errors.Is(err, ErrHeld) {
		t.Fatalf("got %v", err)
	}
}
