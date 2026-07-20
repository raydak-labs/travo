package ws

import (
	"errors"
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
	hub := NewHub(svc, alertSvc, nil)

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
	hub := NewHub(svc, alertSvc, nil)
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
	hub := NewHub(svc, alertSvc, nsCh)
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

func newTestHub() *Hub {
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{})
	return NewHub(svc, services.NewAlertService(svc), nil)
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
