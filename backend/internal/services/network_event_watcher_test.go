package services

import (
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestNoopEventWatcher(t *testing.T) {
	w := NewNoopEventWatcher()
	go w.Start()

	select {
	case <-w.Ch():
		t.Fatal("NoopEventWatcher should never send")
	case <-time.After(50 * time.Millisecond):
		// pass
	}

	w.Stop() // must not block
}

func TestNetworkEventWatcher_EmitsOnEvent(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	networkSvc := NewNetworkService(u, ub)

	// fakeRunner feeds one watched event line then blocks forever
	lines := make(chan string, 1)
	lines <- `{ "network.interface": { "action": "ifup", "interface": "wwan" } }`

	w := newNetworkEventWatcherWithRunner(networkSvc, &chanRunner{lines: lines})
	go w.Start()
	defer w.Stop()

	select {
	case ns := <-w.Ch():
		_ = ns // we just need any result; mock ubus returns a valid empty status
	case <-time.After(2 * time.Second):
		t.Fatal("expected network_status event within 2s (including 300ms debounce)")
	}
}

// chanRunner is a fake subprocessRunner whose Lines() method reads from a channel.
type chanRunner struct {
	lines chan string
}

func (r *chanRunner) Lines(stopCh <-chan struct{}) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		for {
			select {
			case line, ok := <-r.lines:
				if !ok {
					return
				}
				out <- line
			case <-stopCh:
				return
			}
		}
	}()
	return out
}

// Stop() must be idempotent: the lifecycle teardown and a deferred test
// cleanup both call it, and a bare close() panicked on the second call.
func TestEventWatcher_StopIsIdempotent(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	networkSvc := NewNetworkService(u, ub)

	lines := make(chan string)
	w := newNetworkEventWatcherWithRunner(networkSvc, &chanRunner{lines: lines})
	go w.Start()
	w.Stop()
	w.Stop() // must not panic
}

// A second Start() must not launch a second `iw event` loop.
func TestNetworkEventWatcher_StartIsIdempotent(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	networkSvc := NewNetworkService(u, ub)

	lines := make(chan string)
	w := newNetworkEventWatcherWithRunner(networkSvc, &chanRunner{lines: lines})
	go w.Start()
	// Wait until the first loop is actually running (it emits an initial
	// snapshot before blocking), otherwise the second Start() could win the
	// start race and the test would be meaningless.
	select {
	case <-w.Ch():
	case <-time.After(2 * time.Second):
		t.Fatal("first Start() never emitted its initial snapshot")
	}
	// A second (blocking) Start must return immediately.
	done := make(chan struct{})
	go func() {
		w.Start()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second Start() did not return: duplicate watcher loop")
	}
	w.Stop()
}
