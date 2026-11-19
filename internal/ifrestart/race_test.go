package ifrestart

import (
	"errors"
	"sync"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// A guarded operation racing the restart's guard check: exactly one of them proceeds.
func TestAcquireRacingAnotherGuardedOperationHasOneWinner(t *testing.T) {
	for range 50 {
		s, e := newEnv(t)
		var restartErr, otherErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; restartErr = s.acquire() }()
		go func() {
			defer wg.Done()
			<-start
			otherErr = e.guards.AcquireExclusive("firmware-upgrade-in-progress", guard.RefuseAny)
		}()
		close(start)
		wg.Wait()
		if (restartErr == nil) == (otherErr == nil) {
			t.Fatalf("restart %v, other %v: want exactly one winner", restartErr, otherErr)
		}
		if restartErr != nil && !errors.Is(restartErr, ErrBusy) {
			t.Fatalf("restart lost with %v, want ErrBusy", restartErr)
		}
	}
}
