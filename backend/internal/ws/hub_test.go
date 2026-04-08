package ws

import (
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestNewHub(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{}, nil, nil)
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
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{}, nil, nil)
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
	svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{}, nil, nil)
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
