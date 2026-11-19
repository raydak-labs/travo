package wifischedule

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// A scheduled toggle racing another guarded operation: never both. The fake begin takes
// the transaction guard with the policy Begin applies for the fire's ctx.
func TestFireRacingAnotherGuardedOperationHasOneWinner(t *testing.T) {
	for range 30 {
		s, _, aps, _, registered := newService(t)
		r := rule("0a0b0c0d")
		r.AcknowledgeLockout = true
		if _, err := s.Put(context.Background(), Rules{Rules: []Rule{r}}); err != nil {
			t.Fatal(err)
		}
		dir := guard.Dir{Path: filepath.Join(t.TempDir(), "guards"), Now: time.Now}
		s.Guards = dir
		aps.begin = func(ctx context.Context) error { return dir.AcquireExclusive(txn.GuardName, txn.Conflicts(ctx)) }
		aps.status = txn.Status{Meta: txn.Meta{ID: "abc"}, State: txn.Pending}
		var otherErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _ = registered["wifi-schedule-0a0b0c0d-off"](context.Background()) }()
		go func() {
			defer wg.Done()
			<-start
			otherErr = dir.AcquireExclusive("interface-restart-in-progress", guard.RefuseAny)
		}()
		close(start)
		wg.Wait()
		if applied := lastOutcome(t, s) == Applied; applied == (otherErr == nil) {
			t.Fatalf("outcome %s, other %v: want exactly one winner", lastOutcome(t, s), otherErr)
		}
	}
}
