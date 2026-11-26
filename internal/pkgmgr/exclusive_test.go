package pkgmgr

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestExclusiveSerialisesConcurrentOperations(t *testing.T) {
	var running, peak int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := Exclusive(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			defer release()
			n := atomic.AddInt32(&running, 1)
			for {
				p := atomic.LoadInt32(&peak)
				if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt32(&running, -1)
		}()
	}
	wg.Wait()
	if peak != 1 {
		t.Fatalf("%d package operations ran at once", peak)
	}
}

func TestExclusiveGivesUpWithTheContext(t *testing.T) {
	release, err := Exclusive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := Exclusive(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}
