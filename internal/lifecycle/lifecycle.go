// Package lifecycle registers every background worker so SIGTERM stops them all.
package lifecycle

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Worker runs until ctx is cancelled.
type Worker func(ctx context.Context)

// Group owns all background workers of the process.
type Group struct {
	mu      sync.Mutex
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	ctx     context.Context
	stopped atomic.Bool
}

func New(parent context.Context) *Group {
	ctx, cancel := context.WithCancel(parent)
	return &Group{ctx: ctx, cancel: cancel}
}

// Go starts a worker; panics are logged and do not kill the process.
func (g *Group) Go(name string, w Worker) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped.Load() {
		return
	}
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("worker panic", "worker", name, "panic", r)
			}
		}()
		w(g.ctx)
	}()
}

// Stopped reports whether Stop was called; writers check it before any UCI commit.
func (g *Group) Stopped() bool { return g.stopped.Load() }

// Stop cancels workers and waits for them.
func (g *Group) Stop() {
	g.mu.Lock()
	g.stopped.Store(true)
	g.mu.Unlock()
	g.cancel()
	g.wg.Wait()
}

// Every runs fn at once and then each interval until ctx ends.
func Every(interval time.Duration, fn func(ctx context.Context)) Worker {
	return func(ctx context.Context) {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			fn(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}
}
