package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const maxAlerts = 50
const alertThresholdsFile = "/etc/travo/alert-thresholds.json"

// defaultAlertThresholds returns the default threshold values.
func defaultAlertThresholds() models.AlertThresholds {
	return models.AlertThresholds{
		StoragePercent: 90,
		CPUPercent:     90,
		MemoryPercent:  90,
	}
}

// GetAlertThresholds reads thresholds from the config file, returning defaults if absent.
func (a *AlertService) GetAlertThresholds() models.AlertThresholds {
	data, err := os.ReadFile(alertThresholdsFile)
	if err != nil {
		return defaultAlertThresholds()
	}
	var t models.AlertThresholds
	if err := json.Unmarshal(data, &t); err != nil {
		return defaultAlertThresholds()
	}
	return t
}

// SetAlertThresholds persists thresholds to the config file.
func (a *AlertService) SetAlertThresholds(t models.AlertThresholds) error {
	if err := os.MkdirAll(filepath.Dir(alertThresholdsFile), 0750); err != nil {
		return err
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(alertThresholdsFile, data, 0600)
}

// AlertChecker abstracts the system checks used by AlertService.
type AlertChecker interface {
	GetSystemStats() (models.SystemStats, error)
}

// CarrierChecker reads whether a network interface has a physical link.
type CarrierChecker interface {
	// IsCarrierUp returns true when the ethernet carrier is detected (cable plugged in).
	// Returns (false, nil) when the cable is unplugged, (false, err) when the state
	// cannot be determined (e.g. interface does not exist).
	IsCarrierUp(iface string) (bool, error)
}

// RealCarrierChecker reads carrier state from /sys/class/net/<iface>/carrier.
type RealCarrierChecker struct{}

// IsCarrierUp implements CarrierChecker using the Linux sysfs carrier file.
func (r *RealCarrierChecker) IsCarrierUp(iface string) (bool, error) {
	data, err := os.ReadFile("/sys/class/net/" + iface + "/carrier")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(data)) == "1", nil
}

// AlertService monitors system conditions and generates alerts.
type AlertService struct {
	checker        AlertChecker
	carrierChecker CarrierChecker
	mu             sync.RWMutex
	alerts         []models.Alert
	alertCh        chan models.Alert
	stopCh         chan struct{}
	CheckInterval  time.Duration

	// Track active conditions to avoid duplicate alerts
	activeConditions map[string]bool
}

// SetCarrierChecker enables ethernet carrier monitoring in the alert service.
// Pass nil to disable (default).
func (a *AlertService) SetCarrierChecker(cc CarrierChecker) {
	a.carrierChecker = cc
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

	thresholds := a.GetAlertThresholds()

	// Storage above threshold
	if stats.Storage.UsagePercent > thresholds.StoragePercent {
		a.raiseCondition("storage_low", fmt.Sprintf("Storage usage is above %.0f%%", thresholds.StoragePercent), "warning")
	} else {
		a.clearCondition("storage_low")
	}

	// CPU above threshold
	if stats.CPU.UsagePercent > thresholds.CPUPercent {
		a.raiseCondition("high_cpu", fmt.Sprintf("CPU usage is above %.0f%%", thresholds.CPUPercent), "warning")
	} else {
		a.clearCondition("high_cpu")
	}

	// Memory above threshold
	if stats.Memory.UsagePercent > thresholds.MemoryPercent {
		a.raiseCondition("high_memory", fmt.Sprintf("Memory usage is above %.0f%%", thresholds.MemoryPercent), "warning")
	} else {
		a.clearCondition("high_memory")
	}

	// Ethernet carrier (WAN cable plug/unplug)
	if a.carrierChecker != nil {
		up, err := a.carrierChecker.IsCarrierUp("eth0")
		if err == nil {
			if !up {
				a.raiseCondition("eth_unplugged", "WAN ethernet cable is disconnected", "warning")
			} else {
				a.clearCondition("eth_unplugged")
			}
		}
		// If err != nil the interface may not exist (e.g. USB-only WAN) — skip silently.
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

// Publish appends and broadcasts a one-off alert event.
func (a *AlertService) Publish(alertType, message, severity string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	alert := models.Alert{
		ID:        generateAlertID(),
		Type:      alertType,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now().UnixMilli(),
	}
	a.appendAlert(alert)

	select {
	case a.alertCh <- alert:
	default:
	}
}

func generateAlertID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
