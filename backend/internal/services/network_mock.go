package services

import (
	"sync"
	"sync/atomic"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// NoopEventWatcher satisfies EventWatcher but never emits anything.
// Used in mock mode and tests.
type NoopEventWatcher struct {
	ch       chan models.NetworkStatus
	stopCh   chan struct{}
	started  atomic.Bool
	stopOnce sync.Once
}

func NewNoopEventWatcher() *NoopEventWatcher {
	return &NoopEventWatcher{
		ch:     make(chan models.NetworkStatus),
		stopCh: make(chan struct{}),
	}
}

// Start blocks until Stop is called. A second call returns immediately instead
// of spawning a second loop.
func (w *NoopEventWatcher) Start() {
	if !w.started.CompareAndSwap(false, true) {
		return
	}
	<-w.stopCh
}

// Stop is safe to call multiple times.
func (w *NoopEventWatcher) Stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
}

func (w *NoopEventWatcher) Ch() <-chan models.NetworkStatus {
	return w.ch
}
