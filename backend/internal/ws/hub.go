package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// writeTimeout bounds how long a single client write may stall the broadcast
// loop — a client on dead Wi-Fi otherwise blocks it indefinitely.
const writeTimeout = 5 * time.Second

// Conn is the subset of *websocket.Conn the hub needs; an interface so tests
// can exercise broadcast behavior without real network connections.
type Conn interface {
	WriteMessage(messageType int, data []byte) error
	SetWriteDeadline(t time.Time) error
	Close() error
}

// Hub manages WebSocket connections and broadcasts system stats.
type Hub struct {
	clients           map[Conn]bool
	mu                sync.RWMutex
	systemSvc         *services.SystemService
	alertSvc          *services.AlertService
	networkStatusCh   <-chan models.NetworkStatus
	stopCh            chan struct{}
	BroadcastInterval time.Duration
}

// NewHub creates a new WebSocket hub.
func NewHub(
	systemSvc *services.SystemService,
	alertSvc *services.AlertService,
	networkStatusCh <-chan models.NetworkStatus,
) *Hub {
	return &Hub{
		clients:           make(map[Conn]bool),
		systemSvc:         systemSvc,
		alertSvc:          alertSvc,
		networkStatusCh:   networkStatusCh,
		stopCh:            make(chan struct{}),
		BroadcastInterval: 2 * time.Second,
	}
}

// Register adds a client connection to the hub.
func (h *Hub) Register(conn Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = true
}

// Unregister removes a client connection from the hub.
func (h *Hub) Unregister(conn Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, conn)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends data to all connected clients. Writes happen outside the
// client-map lock so a slow or dead client never blocks Register/Unregister;
// failed connections are closed and removed.
func (h *Hub) Broadcast(data []byte) {
	h.mu.RLock()
	conns := make([]Conn, 0, len(h.clients))
	for conn := range h.clients {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()

	var failed []Conn
	for _, conn := range conns {
		_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			failed = append(failed, conn)
		}
	}

	for _, conn := range failed {
		_ = conn.Close()
		h.Unregister(conn)
	}
}

// Start begins the periodic stats broadcast loop and alert/network forwarding.
func (h *Hub) Start() {
	go func() {
		ticker := time.NewTicker(h.BroadcastInterval)
		defer ticker.Stop()

		var alertCh <-chan models.Alert
		if h.alertSvc != nil {
			alertCh = h.alertSvc.AlertCh()
		}

		var networkStatusCh <-chan models.NetworkStatus
		if h.networkStatusCh != nil {
			networkStatusCh = h.networkStatusCh
		}

		for {
			select {
			case <-ticker.C:
				h.broadcastStats()
			case alert, ok := <-alertCh:
				if ok {
					h.broadcastAlert(alert)
				}
			case ns, ok := <-networkStatusCh:
				if ok {
					h.broadcastNetworkStatus(ns)
				}
			case <-h.stopCh:
				return
			}
		}
	}()
}

// Stop stops the broadcast loop.
func (h *Hub) Stop() {
	close(h.stopCh)
}

func (h *Hub) broadcastStats() {
	if h.ClientCount() == 0 {
		return
	}
	stats, err := h.systemSvc.GetSystemStats()
	if err != nil {
		return
	}
	msg := map[string]interface{}{
		"type": "system_stats",
		"data": stats,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.Broadcast(data)
}

func (h *Hub) broadcastNetworkStatus(ns models.NetworkStatus) {
	if h.ClientCount() == 0 {
		return
	}
	msg := map[string]interface{}{
		"type": "network_status",
		"data": ns,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.Broadcast(data)
}

func (h *Hub) broadcastAlert(alert models.Alert) {
	msg := map[string]interface{}{
		"type": "alert",
		"data": alert,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.Broadcast(data)
}
