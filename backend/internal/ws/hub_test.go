package ws

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestNewHub(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{})
	alertSvc := services.NewAlertService(svc)
	hub := NewHub(svc, alertSvc, nil, nil)

	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if hub.ClientCount() != 0 {
		t.Error("expected 0 clients initially")
	}
}

func TestHubStartStop(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{})
	alertSvc := services.NewAlertService(svc)
	hub := NewHub(svc, alertSvc, nil, nil)
	hub.BroadcastInterval = 10 * time.Millisecond

	hub.Start()
	time.Sleep(50 * time.Millisecond)
	hub.Stop()
	// No panic = success
}

func TestHub_BroadcastsNetworkStatus(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{})
	alertSvc := services.NewAlertService(svc)

	nsCh := make(chan models.NetworkStatus, 1)
	hub := NewHub(svc, alertSvc, nsCh, nil)
	hub.BroadcastInterval = 10 * time.Millisecond

	hub.Start()
	defer hub.Stop()

	// No WebSocket clients connected — no panic expected even when channel receives.
	nsCh <- models.NetworkStatus{}
	time.Sleep(50 * time.Millisecond)
}

// fakeConn implements the conn interface used by the hub for testing.
type fakeConn struct {
	mu          sync.Mutex
	writeErr    error
	writeDelay  time.Duration
	deadlineSet bool
	closed      bool
	written     [][]byte
}

func (f *fakeConn) WriteMessage(messageType int, data []byte) error {
	if f.writeDelay > 0 {
		time.Sleep(f.writeDelay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	f.written = append(f.written, data)
	return nil
}

func (f *fakeConn) SetWriteDeadline(t time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deadlineSet = true
	return nil
}

func (f *fakeConn) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func TestHub_StopClosesRegisteredClients(t *testing.T) {
	hub := newTestHub()
	a, b := &fakeConn{}, &fakeConn{}
	hub.Register(a)
	hub.Register(b)

	hub.Stop()

	for i, c := range []*fakeConn{a, b} {
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if !closed {
			t.Errorf("client %d: expected Stop to close the connection", i)
		}
	}
	if got := hub.ClientCount(); got != 0 {
		t.Errorf("expected the client list to be empty after Stop, got %d", got)
	}
}

func TestHub_StopIsIdempotent(t *testing.T) {
	hub := newTestHub()
	hub.Start()
	conn := &fakeConn{}
	hub.Register(conn)

	hub.Stop()
	hub.Stop() // must not panic on a double close of the stop channel
	hub.Stop()

	// A client registering after Stop must not keep the hub alive, but the
	// broadcast loop is gone: no write happens.
	hub.Register(&fakeConn{})
	time.Sleep(30 * time.Millisecond)
	conn.mu.Lock()
	writes := len(conn.written)
	conn.mu.Unlock()
	if writes != 0 {
		t.Errorf("expected no broadcasts after Stop, got %d", writes)
	}
}

func newTestHub() *Hub {
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{})
	return NewHub(svc, services.NewAlertService(svc), nil, nil)
}

// The traffic history exists so a dashboard opening the page has ~10 minutes of
// history to paint. That is exactly the no-client case, so the broadcast loop
// must sample while ClientCount() == 0 (it used to return early, leaving the
// history permanently empty).
func TestHub_SamplesTrafficHistoryWithNoClients(t *testing.T) {
	root := fixtureSysfsNet(t)
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{})
	history := services.NewTrafficHistoryService(10)
	restore := services.SetSysfsNetRootForTesting(root)
	defer restore()

	hub := NewHub(svc, services.NewAlertService(svc), nil, history)
	hub.BroadcastInterval = 10 * time.Millisecond
	hub.Start()

	time.Sleep(60 * time.Millisecond)
	hub.Stop()
	// Let an already-selected tick finish before reading the fixture.
	time.Sleep(20 * time.Millisecond)

	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("precondition: expected no clients, got %d", got)
	}
	if got := len(history.History()); got == 0 {
		t.Error("expected traffic samples with zero WebSocket clients connected")
	}
}

// fixtureSysfsNet creates a temporary tree holding the interfaces
// readNetworkStats monitors and returns its root, to be handed to
// services.SetSysfsNetRootForTesting.
func fixtureSysfsNet(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	for _, iface := range []string{"br-lan", "wwan0", "wg0", "eth0"} {
		dir := filepath.Join(root, iface, "statistics")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for name, value := range map[string]string{"rx_bytes": "100", "tx_bytes": "200"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o644); err != nil {
				t.Fatalf("write %s/%s: %v", iface, name, err)
			}
		}
	}
	return root
}

// A hub wired without a history must keep working (nil history = no sampling).
func TestHub_NilTrafficHistoryDoesNotPanic(t *testing.T) {
	hub := newTestHub()
	hub.BroadcastInterval = 10 * time.Millisecond
	hub.Start()
	time.Sleep(30 * time.Millisecond)
	hub.Stop()
}

func TestHub_BroadcastRemovesDeadClients(t *testing.T) {
	hub := newTestHub()
	dead := &fakeConn{writeErr: errors.New("broken pipe")}
	alive := &fakeConn{}
	hub.Register(dead)
	hub.Register(alive)

	hub.Broadcast([]byte("hello"))

	if hub.ClientCount() != 1 {
		t.Errorf("expected dead client removed, count=%d", hub.ClientCount())
	}
	dead.mu.Lock()
	closed := dead.closed
	dead.mu.Unlock()
	if !closed {
		t.Error("expected dead client to be closed")
	}
	alive.mu.Lock()
	got := len(alive.written)
	alive.mu.Unlock()
	if got != 1 {
		t.Errorf("expected alive client to receive 1 message, got %d", got)
	}
}

func TestHub_BroadcastSetsWriteDeadline(t *testing.T) {
	hub := newTestHub()
	conn := &fakeConn{}
	hub.Register(conn)

	hub.Broadcast([]byte("hello"))

	conn.mu.Lock()
	deadlineSet := conn.deadlineSet
	conn.mu.Unlock()
	if !deadlineSet {
		t.Error("expected a write deadline to be set before writing")
	}
}

// A client stuck in a slow write must not block Register/Unregister — writes
// happen outside the client-map lock.
func TestHub_SlowClientDoesNotBlockRegister(t *testing.T) {
	hub := newTestHub()
	slow := &fakeConn{writeDelay: 300 * time.Millisecond}
	hub.Register(slow)

	done := make(chan struct{})
	go func() {
		hub.Broadcast([]byte("hello"))
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // let Broadcast enter the slow write

	registered := make(chan struct{})
	go func() {
		hub.Register(&fakeConn{})
		close(registered)
	}()

	select {
	case <-registered:
		// Register completed while the slow write was still in progress.
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Register blocked by a slow client write")
	}
	<-done
}
