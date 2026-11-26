package wifireconnect

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// An attempt racing another guarded operation: exactly one of them proceeds. A failing
// ifup keeps the reconnect guard, so a winning attempt still holds it when the other runs.
func TestAttemptRacingAnotherGuardedOperationHasOneWinner(t *testing.T) {
	for range 50 {
		h := newHarness(t)
		h.reconnect.err = errors.New("ifup failed")
		var otherErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; h.svc.attempt(context.Background(), record{}) }()
		go func() {
			defer wg.Done()
			<-start
			otherErr = h.guards.AcquireExclusive("interface-restart-in-progress", guard.RefuseAny)
		}()
		close(start)
		wg.Wait()
		if (h.reconnect.calls == 1) == (otherErr == nil) {
			t.Fatalf("attempts %d, other %v: want exactly one winner", h.reconnect.calls, otherErr)
		}
	}
}

func TestAttemptReleasesItsGuardWhenTheCounterCannotBePersisted(t *testing.T) {
	h := newHarness(t)
	h.svc.writeJSON = func(string, any, os.FileMode) error { return errors.New("flash full") }
	h.svc.attempt(context.Background(), record{})
	if active, _ := h.guards.Stale(); len(active) != 0 || h.reconnect.calls != 0 {
		t.Fatalf("guards %v, attempts %d", active, h.reconnect.calls)
	}
}
