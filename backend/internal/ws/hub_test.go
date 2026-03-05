package ws

import (
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

func TestNewHub(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, &services.MockStorageProvider{})
	hub := NewHub(svc)

	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if hub.ClientCount() != 0 {
		t.Error("expected 0 clients initially")
	}
}

func TestHubStartStop(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, &services.MockStorageProvider{})
	hub := NewHub(svc)
	hub.BroadcastInterval = 10 * time.Millisecond

	hub.Start()
	time.Sleep(50 * time.Millisecond)
	hub.Stop()
	// No panic = success
}
