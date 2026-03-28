package services

import (
	"bufio"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// EventWatcher is implemented by NetworkEventWatcher (real) and NoopEventWatcher (mock/test).
type EventWatcher interface {
	Start()
	Stop()
	Ch() <-chan models.NetworkStatus
}

// NoopEventWatcher satisfies EventWatcher but never emits anything.
// Used in mock mode and tests.
type NoopEventWatcher struct {
	ch     chan models.NetworkStatus
	stopCh chan struct{}
}

func NewNoopEventWatcher() *NoopEventWatcher {
	return &NoopEventWatcher{
		ch:     make(chan models.NetworkStatus),
		stopCh: make(chan struct{}),
	}
}

func (w *NoopEventWatcher) Start() {
	<-w.stopCh // block until Stop is called
}

func (w *NoopEventWatcher) Stop() {
	close(w.stopCh)
}

func (w *NoopEventWatcher) Ch() <-chan models.NetworkStatus {
	return w.ch
}

// subprocessRunner abstracts launching `ubus listen`. Replaced in tests by chanRunner.
type subprocessRunner interface {
	// Lines returns a channel of raw output lines.
	// It closes the channel when the subprocess exits or stopCh is closed.
	Lines(stopCh <-chan struct{}) <-chan string
}

// realRunner launches `ubus listen` and streams stdout lines.
type realRunner struct{}

func (r *realRunner) Lines(stopCh <-chan struct{}) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		cmd := exec.Command("ubus", "listen", "network.interface", "hostapd", "dhcp")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return
		}
		if err := cmd.Start(); err != nil {
			return
		}
		doneCh := make(chan struct{})
		go func() {
			defer close(doneCh)
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				select {
				case out <- scanner.Text():
				case <-stopCh:
					return
				}
			}
		}()
		select {
		case <-doneCh:
		case <-stopCh:
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	return out
}

// watchedPrefixes are the ubus namespaces we care about.
var watchedPrefixes = []string{
	`"network.interface"`,
	`"hostapd.`,
	`"dhcp"`,
}

func isWatched(line string) bool {
	for _, prefix := range watchedPrefixes {
		if strings.Contains(line, prefix) {
			return true
		}
	}
	return false
}

// NetworkEventWatcher watches ubus events and emits NetworkStatus snapshots on change.
type NetworkEventWatcher struct {
	networkSvc *NetworkService
	runner     subprocessRunner
	ch         chan models.NetworkStatus
	stopCh     chan struct{}
}

func NewNetworkEventWatcher(networkSvc *NetworkService) *NetworkEventWatcher {
	return newNetworkEventWatcherWithRunner(networkSvc, &realRunner{})
}

func newNetworkEventWatcherWithRunner(networkSvc *NetworkService, runner subprocessRunner) *NetworkEventWatcher {
	return &NetworkEventWatcher{
		networkSvc: networkSvc,
		runner:     runner,
		ch:         make(chan models.NetworkStatus, 1),
		stopCh:     make(chan struct{}),
	}
}

func (w *NetworkEventWatcher) Ch() <-chan models.NetworkStatus { return w.ch }
func (w *NetworkEventWatcher) Stop()                           { close(w.stopCh) }

func (w *NetworkEventWatcher) Start() {
	// Emit an initial snapshot so the first WebSocket client gets data immediately.
	w.emitSnapshot()

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		gotLine := false
		lines := w.runner.Lines(w.stopCh)
		var timer *time.Timer

	loop:
		for {
			select {
			case <-w.stopCh:
				if timer != nil {
					timer.Stop()
				}
				return
			case line, ok := <-lines:
				if !ok {
					break loop
				}
				gotLine = true
				if !isWatched(line) {
					continue
				}
				// Debounce: reset a 300 ms timer on every watched event.
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(300*time.Millisecond, func() {
					w.emitSnapshot()
				})
			}
		}

		log.Printf("NetworkEventWatcher: ubus listen exited, restarting in %s", backoff)
		select {
		case <-time.After(backoff):
		case <-w.stopCh:
			return
		}
		// Reset backoff only if the subprocess produced at least one line (healthy session).
		if gotLine {
			backoff = time.Second
		} else {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (w *NetworkEventWatcher) emitSnapshot() {
	ns, err := w.networkSvc.GetNetworkStatus()
	if err != nil {
		log.Printf("NetworkEventWatcher: GetNetworkStatus error: %v", err)
		return
	}
	// Non-blocking send. If the hub hasn't consumed the previous value, overwrite it.
	select {
	case w.ch <- ns:
	default:
		select {
		case <-w.ch:
		default:
		}
		select {
		case w.ch <- ns:
		default:
		}
	}
}

// Compile-time interface checks.
var _ EventWatcher = (*NetworkEventWatcher)(nil)
var _ EventWatcher = (*NoopEventWatcher)(nil)
