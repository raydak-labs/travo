package services

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const maxAlerts = 50

// AlertChecker abstracts the system checks used by AlertService.
type AlertChecker interface {
	GetSystemStats() (models.SystemStats, error)
}

// AlertService monitors system conditions and generates alerts.
type AlertService struct {
	checker       AlertChecker
	mu            sync.RWMutex
	alerts        []models.Alert
	alertCh       chan models.Alert
	stopCh        chan struct{}
	CheckInterval time.Duration

	// Track active conditions to avoid duplicate alerts
	activeConditions map[string]bool
}

// NewAlertService creates a new AlertService.
func NewAlertService(checker AlertChecker) *AlertService {
	return &AlertService{
		checker:          checker,
		alerts:           make([]models.Alert, 0),
		alertCh:          make(chan models.Alert, 16),
		stopCh:           make(chan struct{}),
		CheckInterval:    10 * time.Second,
		activeConditions: make(map[string]bool),
	}
}

// AlertCh returns the channel that emits new alerts.
func (a *AlertService) AlertCh() <-chan models.Alert {
	return a.alertCh
}

// GetAlerts returns the alert history (most recent first).
func (a *AlertService) GetAlerts() []models.Alert {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]models.Alert, len(a.alerts))
	copy(result, a.alerts)
	// Reverse so newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// Start begins the periodic alert checking loop.
func (a *AlertService) Start() {
	go func() {
		// Run an initial check immediately
		a.checkConditions()
		ticker := time.NewTicker(a.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.checkConditions()
			case <-a.stopCh:
				return
			}
		}
	}()
}

// Stop stops the alert checking loop.
func (a *AlertService) Stop() {
	close(a.stopCh)
}

func (a *AlertService) checkConditions() {
	stats, err := a.checker.GetSystemStats()
	if err != nil {
		return
	}

	// Storage > 90%
	if stats.Storage.UsagePercent > 90 {
		a.raiseCondition("storage_low", "Storage usage is above 90%", "warning")
	} else {
		a.clearCondition("storage_low")
	}

	// CPU > 90% sustained
	if stats.CPU.UsagePercent > 90 {
		a.raiseCondition("high_cpu", "CPU usage is above 90%", "warning")
	} else {
		a.clearCondition("high_cpu")
	}

	// Memory > 90%
	if stats.Memory.UsagePercent > 90 {
		a.raiseCondition("high_memory", "Memory usage is above 90%", "warning")
	} else {
		a.clearCondition("high_memory")
	}
}

func (a *AlertService) raiseCondition(alertType, message, severity string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.activeConditions[alertType] {
		return // Already active, don't duplicate
	}
	a.activeConditions[alertType] = true

	alert := models.Alert{
		ID:        generateAlertID(),
		Type:      alertType,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now().UnixMilli(),
	}
	a.appendAlert(alert)

	// Non-blocking send
	select {
	case a.alertCh <- alert:
	default:
	}
}

func (a *AlertService) clearCondition(alertType string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.activeConditions, alertType)
}

func (a *AlertService) appendAlert(alert models.Alert) {
	a.alerts = append(a.alerts, alert)
	if len(a.alerts) > maxAlerts {
		a.alerts = a.alerts[len(a.alerts)-maxAlerts:]
	}
}

func generateAlertID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
