package services

import (
	"sync"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// mockAlertChecker implements AlertChecker with configurable stats.
type mockAlertChecker struct {
	mu    sync.RWMutex
	stats models.SystemStats
	err   error
}

func (m *mockAlertChecker) GetSystemStats() (models.SystemStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats, m.err
}

func (m *mockAlertChecker) setStats(stats models.SystemStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats = stats
}

func TestAlertService_NoAlertsWhenHealthy(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 30},
			Memory:  models.MemoryStats{UsagePercent: 40},
			Storage: models.StorageStats{UsagePercent: 50},
		},
	}
	svc := NewAlertService(checker)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	time.Sleep(50 * time.Millisecond)
	svc.Stop()

	alerts := svc.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestAlertService_StorageLowAlert(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 10},
			Memory:  models.MemoryStats{UsagePercent: 40},
			Storage: models.StorageStats{UsagePercent: 95},
		},
	}
	svc := NewAlertService(checker)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	time.Sleep(50 * time.Millisecond)
	svc.Stop()

	alerts := svc.GetAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "storage_low" {
		t.Errorf("expected type storage_low, got %s", alerts[0].Type)
	}
	if alerts[0].Severity != "warning" {
		t.Errorf("expected severity warning, got %s", alerts[0].Severity)
	}
}

func TestAlertService_HighCPUAlert(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 95},
			Memory:  models.MemoryStats{UsagePercent: 40},
			Storage: models.StorageStats{UsagePercent: 50},
		},
	}
	svc := NewAlertService(checker)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	time.Sleep(50 * time.Millisecond)
	svc.Stop()

	alerts := svc.GetAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "high_cpu" {
		t.Errorf("expected type high_cpu, got %s", alerts[0].Type)
	}
}

func TestAlertService_HighMemoryAlert(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 10},
			Memory:  models.MemoryStats{UsagePercent: 95},
			Storage: models.StorageStats{UsagePercent: 50},
		},
	}
	svc := NewAlertService(checker)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	time.Sleep(50 * time.Millisecond)
	svc.Stop()

	alerts := svc.GetAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "high_memory" {
		t.Errorf("expected type high_memory, got %s", alerts[0].Type)
	}
}

func TestAlertService_NoDuplicateAlerts(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 95},
			Memory:  models.MemoryStats{UsagePercent: 40},
			Storage: models.StorageStats{UsagePercent: 50},
		},
	}
	svc := NewAlertService(checker)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	// Wait for multiple check cycles
	time.Sleep(80 * time.Millisecond)
	svc.Stop()

	alerts := svc.GetAlerts()
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert (no duplicates), got %d", len(alerts))
	}
}

func TestAlertService_AlertClearsAndReRaises(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 95},
			Memory:  models.MemoryStats{UsagePercent: 40},
			Storage: models.StorageStats{UsagePercent: 50},
		},
	}
	svc := NewAlertService(checker)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	time.Sleep(30 * time.Millisecond)

	// Condition clears
	checker.setStats(models.SystemStats{
		CPU:     models.CpuStats{UsagePercent: 30},
		Memory:  models.MemoryStats{UsagePercent: 40},
		Storage: models.StorageStats{UsagePercent: 50},
	})
	time.Sleep(30 * time.Millisecond)

	// Condition returns
	checker.setStats(models.SystemStats{
		CPU:     models.CpuStats{UsagePercent: 95},
		Memory:  models.MemoryStats{UsagePercent: 40},
		Storage: models.StorageStats{UsagePercent: 50},
	})
	time.Sleep(30 * time.Millisecond)
	svc.Stop()

	alerts := svc.GetAlerts()
	if len(alerts) < 2 {
		t.Errorf("expected at least 2 alerts (raised, cleared, re-raised), got %d", len(alerts))
	}
}

func TestAlertService_MultipleConditions(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 95},
			Memory:  models.MemoryStats{UsagePercent: 95},
			Storage: models.StorageStats{UsagePercent: 95},
		},
	}
	svc := NewAlertService(checker)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	time.Sleep(50 * time.Millisecond)
	svc.Stop()

	alerts := svc.GetAlerts()
	if len(alerts) != 3 {
		t.Errorf("expected 3 alerts, got %d", len(alerts))
	}
}

func TestAlertService_MaxAlerts(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 10},
			Memory:  models.MemoryStats{UsagePercent: 40},
			Storage: models.StorageStats{UsagePercent: 50},
		},
	}
	svc := NewAlertService(checker)

	// Manually fill with more than max alerts
	for i := 0; i < 60; i++ {
		svc.alerts = append(svc.alerts, models.Alert{
			ID:      "test",
			Type:    "test",
			Message: "test",
		})
	}

	// Append one more via the internal method
	svc.appendAlert(models.Alert{ID: "last", Type: "test"})

	if len(svc.alerts) != maxAlerts {
		t.Errorf("expected %d alerts, got %d", maxAlerts, len(svc.alerts))
	}
}

func TestAlertService_AlertChannel(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 95},
			Memory:  models.MemoryStats{UsagePercent: 40},
			Storage: models.StorageStats{UsagePercent: 50},
		},
	}
	svc := NewAlertService(checker)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()

	select {
	case alert := <-svc.AlertCh():
		if alert.Type != "high_cpu" {
			t.Errorf("expected high_cpu alert, got %s", alert.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timed out waiting for alert on channel")
	}

	svc.Stop()
}

// mockCarrierChecker implements CarrierChecker for testing.
type mockCarrierChecker struct {
	mu   sync.Mutex
	up   bool
	err  error
}

func (m *mockCarrierChecker) IsCarrierUp(iface string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.up, m.err
}

func (m *mockCarrierChecker) setState(up bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.up = up
}

func TestAlertService_EthUnpluggedAlert(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 10},
			Memory:  models.MemoryStats{UsagePercent: 10},
			Storage: models.StorageStats{UsagePercent: 10},
		},
	}
	carrier := &mockCarrierChecker{up: false}
	svc := NewAlertService(checker)
	svc.SetCarrierChecker(carrier)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	time.Sleep(50 * time.Millisecond)
	svc.Stop()

	alerts := svc.GetAlerts()
	found := false
	for _, a := range alerts {
		if a.Type == "eth_unplugged" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected eth_unplugged alert when carrier is down")
	}
}

func TestAlertService_EthPluggedInClearsAlert(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{
			CPU:     models.CpuStats{UsagePercent: 10},
			Memory:  models.MemoryStats{UsagePercent: 10},
			Storage: models.StorageStats{UsagePercent: 10},
		},
	}
	carrier := &mockCarrierChecker{up: false}
	svc := NewAlertService(checker)
	svc.SetCarrierChecker(carrier)
	svc.CheckInterval = 10 * time.Millisecond

	svc.Start()
	time.Sleep(30 * time.Millisecond)
	carrier.setState(true) // cable plugged in
	time.Sleep(30 * time.Millisecond)
	svc.Stop()

	// After plugging in, eth_unplugged should no longer be an active condition
	svc.mu.RLock()
	active := svc.activeConditions["eth_unplugged"]
	svc.mu.RUnlock()
	if active {
		t.Error("expected eth_unplugged condition to be cleared after carrier comes up")
	}
}

func TestAlertService_GetAlertsReturnsNewestFirst(t *testing.T) {
	checker := &mockAlertChecker{
		stats: models.SystemStats{},
	}
	svc := NewAlertService(checker)

	svc.alerts = []models.Alert{
		{ID: "first", Timestamp: 1000},
		{ID: "second", Timestamp: 2000},
		{ID: "third", Timestamp: 3000},
	}

	alerts := svc.GetAlerts()
	if alerts[0].ID != "third" {
		t.Errorf("expected newest first, got %s", alerts[0].ID)
	}
	if alerts[2].ID != "first" {
		t.Errorf("expected oldest last, got %s", alerts[2].ID)
	}
}
