package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// Hub manages WebSocket connections and broadcasts system stats.
type Hub struct {
	clients           map[*websocket.Conn]bool
	mu                sync.RWMutex
	systemSvc         *services.SystemService
	stopCh            chan struct{}
	BroadcastInterval time.Duration
}

// NewHub creates a new WebSocket hub.
func NewHub(systemSvc *services.SystemService) *Hub {
	return &Hub{
		clients:           make(map[*websocket.Conn]bool),
		systemSvc:         systemSvc,
		stopCh:            make(chan struct{}),
		BroadcastInterval: 2 * time.Second,
	}
}

// Register adds a client connection to the hub.
func (h *Hub) Register(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = true
}

// Unregister removes a client connection from the hub.
func (h *Hub) Unregister(conn *websocket.Conn) {
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

// Broadcast sends data to all connected clients.
func (h *Hub) Broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
		}
	}
}

// Start begins the periodic stats broadcast loop.
func (h *Hub) Start() {
	go func() {
		ticker := time.NewTicker(h.BroadcastInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.broadcastStats()
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
