package lifecycle

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestStopCancelsAndWaits(t *testing.T) {
	g := New(context.Background())
	var done atomic.Int32
	for i := 0; i < 3; i++ {
		g.Go("w", func(ctx context.Context) { <-ctx.Done(); done.Add(1) })
	}
	g.Stop()
	if done.Load() != 3 || !g.Stopped() {
		t.Fatalf("done=%d", done.Load())
	}
}

func TestPanicIsContainedAndLateWorkersRefused(t *testing.T) {
	g := New(context.Background())
	g.Go("boom", func(context.Context) { panic("x") })
	g.Stop()
	ran := false
	g.Go("late", func(context.Context) { ran = true })
	if ran {
		t.Fatal("worker started after stop")
	}
}
