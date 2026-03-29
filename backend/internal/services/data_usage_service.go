package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const dataBudgetConfigPath = "/etc/travo/data_budget.json"

// interfaceLabels maps kernel interface names to human-readable labels.
var interfaceLabels = map[string]string{
	"eth0":  "Ethernet WAN",
	"eth1":  "Ethernet WAN",
	"wan":   "Ethernet WAN",
	"wwan0": "WiFi Uplink",
	"wwan":  "WiFi Uplink",
	"usb0":  "USB Tether",
	"usb1":  "USB Tether",
	"wg0":   "WireGuard VPN",
	"tun0":  "VPN Tunnel",
}

func interfaceLabel(name string) string {
	if label, ok := interfaceLabels[name]; ok {
		return label
	}
	return name
}

// vnstatRoot is the top-level JSON output of `vnstat --json`.
type vnstatRoot struct {
	Interfaces []vnstatInterface `json:"interfaces"`
}

type vnstatInterface struct {
	Name    string        `json:"name"`
	Traffic vnstatTraffic `json:"traffic"`
}

type vnstatTraffic struct {
	Total vnstatBytes   `json:"total"`
	Day   []vnstatDay   `json:"day"`
	Month []vnstatMonth `json:"month"`
}

type vnstatBytes struct {
	RX int64 `json:"rx"`
	TX int64 `json:"tx"`
}

type vnstatDay struct {
	Date struct {
		Year  int `json:"year"`
		Month int `json:"month"`
		Day   int `json:"day"`
	} `json:"date"`
	RX int64 `json:"rx"`
	TX int64 `json:"tx"`
}

type vnstatMonth struct {
	Date struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	} `json:"date"`
	RX int64 `json:"rx"`
	TX int64 `json:"tx"`
}

// DataUsageRunner abstracts command execution for testability.
type DataUsageRunner interface {
	RunJSON(args ...string) ([]byte, error)
	IsInstalled() bool
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
}

// RealDataUsageRunner runs the real vnstat binary.
type RealDataUsageRunner struct{}

func (r *RealDataUsageRunner) IsInstalled() bool {
	_, err := exec.LookPath("vnstat")
	return err == nil
}

func (r *RealDataUsageRunner) RunJSON(args ...string) ([]byte, error) {
	out, err := exec.Command("vnstat", args...).Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *RealDataUsageRunner) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (r *RealDataUsageRunner) WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll("/etc/travo", 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// DataUsageService provides vnstat-based traffic tracking.
type DataUsageService struct {
	runner DataUsageRunner
}

// NewDataUsageService creates a service backed by the real system.
func NewDataUsageService() *DataUsageService {
	return &DataUsageService{runner: &RealDataUsageRunner{}}
}

// NewDataUsageServiceWithRunner creates a service with an injected runner (for tests).
func NewDataUsageServiceWithRunner(r DataUsageRunner) *DataUsageService {
	return &DataUsageService{runner: r}
}

// GetStatus returns current data usage; available=false when vnstat is not installed.
func (s *DataUsageService) GetStatus() (models.DataUsageStatus, error) {
	if !s.runner.IsInstalled() {
		return models.DataUsageStatus{Available: false, Interfaces: []models.DataUsageInterface{}}, nil
	}

	raw, err := s.runner.RunJSON("--json")
	if err != nil {
		return models.DataUsageStatus{Available: false, Interfaces: []models.DataUsageInterface{}},
			fmt.Errorf("vnstat --json: %w", err)
	}

	var root vnstatRoot
	if err := json.Unmarshal(raw, &root); err != nil {
		return models.DataUsageStatus{}, fmt.Errorf("parse vnstat output: %w", err)
	}

	now := time.Now()
	ifaces := make([]models.DataUsageInterface, 0, len(root.Interfaces))
	for _, iface := range root.Interfaces {
		entry := models.DataUsageInterface{
			Name:  iface.Name,
			Label: interfaceLabel(iface.Name),
			Total: models.DataUsagePeriod{
				RXBytes: iface.Traffic.Total.RX,
				TXBytes: iface.Traffic.Total.TX,
			},
		}

		// Today's usage: find the day entry matching today.
		for _, d := range iface.Traffic.Day {
			if d.Date.Year == now.Year() && d.Date.Month == int(now.Month()) && d.Date.Day == now.Day() {
				entry.Today = models.DataUsagePeriod{RXBytes: d.RX, TXBytes: d.TX}
				break
			}
		}

		// This month's usage: find the month entry matching the current month.
		for _, m := range iface.Traffic.Month {
			if m.Date.Year == now.Year() && m.Date.Month == int(now.Month()) {
				entry.Month = models.DataUsagePeriod{RXBytes: m.RX, TXBytes: m.TX}
				break
			}
		}

		ifaces = append(ifaces, entry)
	}

	return models.DataUsageStatus{Available: true, Interfaces: ifaces}, nil
}

// ResetInterface removes and re-adds the vnstat database for an interface.
func (s *DataUsageService) ResetInterface(ifaceName string) error {
	if !s.runner.IsInstalled() {
		return fmt.Errorf("vnstat is not installed")
	}
	// vnstat2: --remove deletes the DB entry; --add re-creates it.
	if _, err := s.runner.RunJSON("--remove", "-i", ifaceName, "--force"); err != nil {
		return fmt.Errorf("vnstat remove %s: %w", ifaceName, err)
	}
	if _, err := s.runner.RunJSON("--add", "-i", ifaceName); err != nil {
		return fmt.Errorf("vnstat add %s: %w", ifaceName, err)
	}
	return nil
}

// GetBudget reads the data budget configuration from disk.
func (s *DataUsageService) GetBudget() (models.DataBudgetConfig, error) {
	data, err := s.runner.ReadFile(dataBudgetConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return models.DataBudgetConfig{Budgets: []models.DataBudget{}}, nil
		}
		return models.DataBudgetConfig{}, fmt.Errorf("reading budget config: %w", err)
	}
	var cfg models.DataBudgetConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return models.DataBudgetConfig{}, fmt.Errorf("parsing budget config: %w", err)
	}
	if cfg.Budgets == nil {
		cfg.Budgets = []models.DataBudget{}
	}
	return cfg, nil
}

// SetBudget writes the data budget configuration to disk.
func (s *DataUsageService) SetBudget(cfg models.DataBudgetConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling budget config: %w", err)
	}
	if err := s.runner.WriteFile(dataBudgetConfigPath, data, 0600); err != nil {
		return fmt.Errorf("writing budget config: %w", err)
	}
	return nil
}

// CheckBudgetAlerts checks current usage against budgets and returns alert messages
// for interfaces that have exceeded the warning threshold.
func (s *DataUsageService) CheckBudgetAlerts() []string {
	if !s.runner.IsInstalled() {
		return nil
	}
	cfg, err := s.GetBudget()
	if err != nil || len(cfg.Budgets) == 0 {
		return nil
	}
	status, err := s.GetStatus()
	if err != nil || !status.Available {
		return nil
	}

	var alerts []string
	for _, budget := range cfg.Budgets {
		if budget.MonthlyLimitBytes <= 0 {
			continue
		}
		for _, iface := range status.Interfaces {
			if iface.Name != budget.Interface {
				continue
			}
			monthlyUsed := iface.Month.RXBytes + iface.Month.TXBytes
			pct := float64(monthlyUsed) / float64(budget.MonthlyLimitBytes) * 100.0
			if pct >= budget.WarningThresholdPct {
				alerts = append(alerts, fmt.Sprintf(
					"Data budget warning: %s has used %.0f%% of monthly limit (%s used of %s)",
					iface.Label,
					pct,
					formatBytes(monthlyUsed),
					formatBytes(budget.MonthlyLimitBytes),
				))
			}
		}
	}
	return alerts
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// AutoConfigureVnstat adds WAN interfaces to vnstat monitoring after install.
func (s *DataUsageService) AutoConfigureVnstat() error {
	// Common WAN interface names to monitor.
	ifaces := []string{"eth0", "wwan0"}
	for _, iface := range ifaces {
		// --add is idempotent if interface already exists; ignore errors.
		_, _ = s.runner.RunJSON("--add", "-i", iface)
	}
	// Start vnstatd if available.
	out, _ := exec.Command("/etc/init.d/vnstat", "enable").CombinedOutput()
	_ = strings.TrimSpace(string(out))
	out, _ = exec.Command("/etc/init.d/vnstat", "start").CombinedOutput()
	_ = strings.TrimSpace(string(out))
	return nil
}
