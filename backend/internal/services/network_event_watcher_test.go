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
