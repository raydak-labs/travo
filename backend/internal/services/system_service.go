package services

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// StorageProvider abstracts filesystem storage stat retrieval.
type StorageProvider interface {
	GetRootStorage() (total, used, free int64, err error)
}

// RealStorageProvider reads actual filesystem stats via syscall.Statfs.
type RealStorageProvider struct{}

// GetRootStorage returns storage stats for the root filesystem.
func (r *RealStorageProvider) GetRootStorage() (int64, int64, int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, 0, 0, err
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	used := total - free
	return total, used, free, nil
}

// MockStorageProvider returns realistic mock storage stats.
type MockStorageProvider struct{}

// GetRootStorage returns mock storage stats.
func (m *MockStorageProvider) GetRootStorage() (int64, int64, int64, error) {
	// 256MB total, 96MB used, 160MB free — realistic for a travel router
	return 268435456, 100663296, 167772160, nil
}

// SystemService provides system information and statistics.
type SystemService struct {
	ubus    ubus.Ubus
	uci     uci.UCI
	storage StorageProvider
}

// NewSystemService creates a new SystemService.
func NewSystemService(ub ubus.Ubus, u uci.UCI, storage StorageProvider) *SystemService {
	return &SystemService{ubus: ub, uci: u, storage: storage}
}

// GetSystemInfo returns system identification information.
func (s *SystemService) GetSystemInfo() (models.SystemInfo, error) {
	board, err := s.ubus.Call("system", "board", nil)
	if err != nil {
		return models.SystemInfo{}, err
	}
	info, err := s.ubus.Call("system", "info", nil)
	if err != nil {
		return models.SystemInfo{}, err
	}

	hostname, _ := board["hostname"].(string)
	model, _ := board["model"].(string)
	kernel, _ := board["kernel"].(string)

	var fwVersion string
	if release, ok := board["release"].(map[string]interface{}); ok {
		fwVersion, _ = release["version"].(string)
	}

	var uptime int64
	if u, ok := info["uptime"].(float64); ok {
		uptime = int64(u)
	}

	return models.SystemInfo{
		Hostname:        hostname,
		Model:           model,
		FirmwareVersion: fwVersion,
		KernelVersion:   kernel,
		UptimeSeconds:   uptime,
	}, nil
}

// GetSystemStats returns current system statistics.
func (s *SystemService) GetSystemStats() (models.SystemStats, error) {
	info, err := s.ubus.Call("system", "info", nil)
	if err != nil {
		return models.SystemStats{}, err
	}

	var stats models.SystemStats

	// Memory
	if mem, ok := info["memory"].(map[string]interface{}); ok {
		total, _ := mem["total"].(float64)
		free, _ := mem["free"].(float64)
		cached, _ := mem["cached"].(float64)
		buffered, _ := mem["buffered"].(float64)

		stats.Memory = models.MemoryStats{
			TotalBytes:  int64(total),
			FreeBytes:   int64(free),
			CachedBytes: int64(cached + buffered),
			UsedBytes:   int64(total - free - cached - buffered),
		}
		if total > 0 {
			stats.Memory.UsagePercent = float64(stats.Memory.UsedBytes) / total * 100
		}
	}

	// CPU / Load
	cores := runtime.NumCPU()
	if load, ok := info["load"].([]interface{}); ok && len(load) >= 3 {
		l1, _ := load[0].(float64)
		l5, _ := load[1].(float64)
		l15, _ := load[2].(float64)

		loadAvg1 := l1 / 65536
		loadAvg5 := l5 / 65536
		loadAvg15 := l15 / 65536

		// CPU usage: min(loadAvg1 / numCPUs * 100, 100)
		usagePercent := math.Min(loadAvg1/float64(cores)*100, 100)

		stats.CPU = models.CpuStats{
			LoadAverage:  [3]float64{loadAvg1, loadAvg5, loadAvg15},
			UsagePercent: usagePercent,
			Cores:        cores,
		}
	}

	// Storage from provider
	if total, used, free, err := s.storage.GetRootStorage(); err == nil && total > 0 {
		stats.Storage = models.StorageStats{
			TotalBytes:   total,
			UsedBytes:    used,
			FreeBytes:    free,
			UsagePercent: float64(used) / float64(total) * 100,
		}
	}

	// Network interface counters
	stats.Network = readNetworkStats()

	return stats, nil
}

// readNetworkStats reads cumulative RX/TX byte counters from sysfs for key interfaces.
func readNetworkStats() []models.NetworkInterfaceStats {
	interfaces := []string{"br-lan", "wwan0", "wg0", "eth0"}
	var result []models.NetworkInterfaceStats
	for _, iface := range interfaces {
		rx, errRx := readSysfsCounter(iface, "rx_bytes")
		tx, errTx := readSysfsCounter(iface, "tx_bytes")
		if errRx != nil || errTx != nil {
			continue
		}
		result = append(result, models.NetworkInterfaceStats{
			Interface: iface,
			RxBytes:   rx,
			TxBytes:   tx,
		})
	}
	return result
}

// readSysfsCounter reads a single counter value from /sys/class/net/<iface>/statistics/<counter>.
func readSysfsCounter(iface, counter string) (int64, error) {
	path := filepath.Join("/sys/class/net", iface, "statistics", counter)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

// Reboot initiates a system reboot via ubus.
// The reboot call is async so the HTTP response returns before the system goes down.
func (s *SystemService) Reboot() error {
	go func() {
		time.Sleep(500 * time.Millisecond)
		_, _ = s.ubus.Call("system", "reboot", nil)
	}()
	return nil
}

// GetTimezone returns the current timezone configuration.
func (s *SystemService) GetTimezone() (models.TimezoneConfig, error) {
	opts, err := s.uci.GetAll("system", "system")
	if err != nil {
		return models.TimezoneConfig{}, err
	}
	return models.TimezoneConfig{
		Zonename: opts["zonename"],
		Timezone: opts["timezone"],
	}, nil
}

// SetTimezone updates the timezone configuration.
func (s *SystemService) SetTimezone(config models.TimezoneConfig) error {
	if err := s.uci.Set("system", "system", "zonename", config.Zonename); err != nil {
		return fmt.Errorf("setting zonename: %w", err)
	}
	if err := s.uci.Set("system", "system", "timezone", config.Timezone); err != nil {
		return fmt.Errorf("setting timezone: %w", err)
	}
	return s.uci.Commit("system")
}

// SetHostname changes the device hostname via UCI and applies it.
func (s *SystemService) SetHostname(hostname string) error {
	if err := s.uci.Set("system", "system", "hostname", hostname); err != nil {
		return err
	}
	return s.uci.Commit("system")
}

// GetLEDStatus returns the current stealth mode state by checking LED brightness.
func (s *SystemService) GetLEDStatus() models.LEDStatus {
	leds := listLEDs()
	allOff := len(leds) > 0
	for _, led := range leds {
		b, err := os.ReadFile(filepath.Join("/sys/class/leds", led, "brightness"))
		if err == nil && strings.TrimSpace(string(b)) != "0" {
			allOff = false
			break
		}
	}
	return models.LEDStatus{
		StealthMode: allOff,
		LEDCount:    len(leds),
	}
}

// SetLEDStealthMode turns all LEDs off (stealth) or restores them.
func (s *SystemService) SetLEDStealthMode(stealth bool) error {
	leds := listLEDs()
	for _, led := range leds {
		path := filepath.Join("/sys/class/leds", led, "brightness")
		val := "255"
		if stealth {
			val = "0"
		}
		if err := os.WriteFile(path, []byte(val), 0644); err != nil {
			return err
		}
	}
	return nil
}

func listLEDs() []string {
	entries, err := os.ReadDir("/sys/class/leds")
	if err != nil {
		return nil
	}
	var leds []string
	for _, e := range entries {
		leds = append(leds, e.Name())
	}
	return leds
}

// GetLogs retrieves system logs from logread.
// If service is non-empty, only lines containing that service name (case-insensitive) are returned.
// If level is non-empty, only lines at or above that severity are returned.
func (s *SystemService) GetLogs(service, level string) (models.LogResponse, error) {
	out, err := exec.Command("logread").CombinedOutput()
	if err != nil {
		return models.LogResponse{}, err
	}
	return parseLogOutput("syslog", string(out), service, level), nil
}

// GetKernelLogs retrieves kernel logs from dmesg.
func (s *SystemService) GetKernelLogs() (models.LogResponse, error) {
	out, err := exec.Command("dmesg").CombinedOutput()
	if err != nil {
		return models.LogResponse{}, err
	}
	return parseLogOutput("kernel", string(out), "", ""), nil
}

// CreateBackup generates a configuration backup archive and returns its path.
func (s *SystemService) CreateBackup() (string, error) {
	path := "/tmp/backup-" + strconv.FormatInt(time.Now().Unix(), 10) + ".tar.gz"
	out, err := exec.Command("sysupgrade", "-b", path).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("creating backup: %w: %s", err, string(out))
	}
	return path, nil
}

// RestoreBackup applies a configuration backup from the given file path.
func (s *SystemService) RestoreBackup(path string) error {
	out, err := exec.Command("sysupgrade", "-r", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("restoring backup: %w: %s", err, string(out))
	}
	return nil
}

// FactoryReset clears the overlay partition and reboots, restoring factory defaults.
func (s *SystemService) FactoryReset() error {
	cmd := exec.Command("firstboot", "-y")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("factory reset failed: %s: %w", string(out), err)
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = exec.Command("reboot").Run()
	}()
	return nil
}

// logLevelSeverity maps syslog level names to numeric severity (lower = more severe).
var logLevelSeverity = map[string]int{
	"emerg":   0,
	"alert":   1,
	"crit":    2,
	"err":     3,
	"warning": 4,
	"warn":    4,
	"notice":  5,
	"info":    6,
	"debug":   7,
}

// extractLevel extracts the syslog level from a log line.
// Syslog format: "Tue Mar 10 22:00:34 2026 kern.info kernel: ..."
// Returns the level string (e.g. "info", "err") or empty string if not found.
func extractLevel(line string) string {
	// Find facility.level pattern — appears after the timestamp (first 5 fields)
	parts := strings.Fields(line)
	if len(parts) < 6 {
		return ""
	}
	// The facility.level field is typically at index 5 (after: dow mon day time year)
	facLevel := parts[5]
	if idx := strings.IndexByte(facLevel, '.'); idx >= 0 && idx < len(facLevel)-1 {
		level := facLevel[idx+1:]
		if _, ok := logLevelSeverity[level]; ok {
			return level
		}
	}
	return ""
}

func parseLogOutput(source, output, service, level string) models.LogResponse {
	raw := strings.Split(strings.TrimSpace(output), "\n")
	lines := make([]models.LogEntry, 0, len(raw))
	serviceLower := strings.ToLower(service)

	// Resolve minimum severity threshold
	minSeverity := -1
	if level != "" {
		if sev, ok := logLevelSeverity[strings.ToLower(level)]; ok {
			minSeverity = sev
		}
	}

	for _, l := range raw {
		if l == "" {
			continue
		}
		if service != "" && !strings.Contains(strings.ToLower(l), serviceLower) {
			continue
		}
		entryLevel := extractLevel(l)
		if minSeverity >= 0 && entryLevel != "" {
			if sev, ok := logLevelSeverity[entryLevel]; ok && sev > minSeverity {
				continue
			}
		}
		// Normalize "warn" to "warning"
		if entryLevel == "warn" {
			entryLevel = "warning"
		}
		lines = append(lines, models.LogEntry{Line: l, Level: entryLevel})
	}
	return models.LogResponse{
		Source: source,
		Lines:  lines,
		Total:  len(lines),
	}
}
