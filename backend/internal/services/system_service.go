package services

import (
	"encoding/json"
	"fmt"
	"io"
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

// Shutdown initiates a system poweroff.
// The call is async so the HTTP response returns before the system goes down.
func (s *SystemService) Shutdown() error {
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = exec.Command("poweroff").Run()
	}()
	return nil
}

// findSystemSection returns the UCI section name for the system-type section.
// On real OpenWRT it's an anonymous section (e.g. cfg01e48a); in mocks it may be "system".
func (s *SystemService) findSystemSection() (string, map[string]string, error) {
	// Try the named section first (works in mocks and some configs)
	if opts, err := s.uci.GetAll("system", "system"); err == nil {
		return "system", opts, nil
	}
	// Fall back to scanning for the anonymous system-type section
	sections, err := s.uci.GetSections("system")
	if err != nil {
		return "", nil, err
	}
	for name, opts := range sections {
		if opts[".type"] == "system" {
			return name, opts, nil
		}
	}
	return "", nil, fmt.Errorf("no system-type section found in UCI")
}

// GetTimezone returns the current timezone configuration.
func (s *SystemService) GetTimezone() (models.TimezoneConfig, error) {
	_, opts, err := s.findSystemSection()
	if err != nil {
		// Section may not exist — return empty default
		return models.TimezoneConfig{}, nil
	}
	return models.TimezoneConfig{
		Zonename: opts["zonename"],
		Timezone: opts["timezone"],
	}, nil
}

// SetTimezone updates the timezone configuration.
func (s *SystemService) SetTimezone(config models.TimezoneConfig) error {
	section, _, err := s.findSystemSection()
	if err != nil {
		return fmt.Errorf("finding system section: %w", err)
	}
	if err := s.uci.Set("system", section, "zonename", config.Zonename); err != nil {
		return fmt.Errorf("setting zonename: %w", err)
	}
	if err := s.uci.Set("system", section, "timezone", config.Timezone); err != nil {
		return fmt.Errorf("setting timezone: %w", err)
	}
	return s.uci.Commit("system")
}

// GetNTPConfig returns the NTP time synchronization configuration.
func (s *SystemService) GetNTPConfig() (models.NTPConfig, error) {
	opts, err := s.uci.GetAll("system", "ntp")
	if err != nil {
		// Section may not exist — return defaults
		return models.NTPConfig{
			Enabled: true,
			Servers: []string{"0.openwrt.pool.ntp.org", "1.openwrt.pool.ntp.org", "2.openwrt.pool.ntp.org", "3.openwrt.pool.ntp.org"},
		}, nil
	}
	config := models.NTPConfig{
		Enabled: opts["enabled"] != "0",
	}
	if srv, ok := opts["server"]; ok && srv != "" {
		config.Servers = strings.Split(srv, " ")
	}
	return config, nil
}

// SetNTPConfig updates the NTP time synchronization configuration.
func (s *SystemService) SetNTPConfig(config models.NTPConfig) error {
	enabled := "1"
	if !config.Enabled {
		enabled = "0"
	}
	if err := s.uci.Set("system", "ntp", "enabled", enabled); err != nil {
		return fmt.Errorf("setting ntp enabled: %w", err)
	}
	if err := s.uci.Set("system", "ntp", "server", strings.Join(config.Servers, " ")); err != nil {
		return fmt.Errorf("setting ntp servers: %w", err)
	}
	return s.uci.Commit("system")
}

// SyncNTP forces a one-shot NTP sync using ntpd.
func (s *SystemService) SyncNTP() error {
	out, err := exec.Command("ntpd", "-q", "-n", "-p", "pool.ntp.org").CombinedOutput()
	if err != nil {
		return fmt.Errorf("ntp sync failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetHostname changes the device hostname via UCI and applies it.
func (s *SystemService) SetHostname(hostname string) error {
	section, _, err := s.findSystemSection()
	if err != nil {
		return fmt.Errorf("finding system section: %w", err)
	}
	if err := s.uci.Set("system", section, "hostname", hostname); err != nil {
		return err
	}
	return s.uci.Commit("system")
}

// GetLEDStatus returns the current stealth mode state by checking LED brightness.
func (s *SystemService) GetLEDStatus() models.LEDStatus {
	leds := listLEDs()
	allOff := len(leds) > 0
	var ledInfos []models.LEDInfo
	for _, led := range leds {
		b, err := os.ReadFile(filepath.Join("/sys/class/leds", led, "brightness"))
		brightness := 0
		if err == nil {
			val := strings.TrimSpace(string(b))
			brightness, _ = strconv.Atoi(val)
			if val != "0" {
				allOff = false
			}
		}
		ledInfos = append(ledInfos, models.LEDInfo{
			Name:       led,
			Brightness: brightness,
		})
	}
	return models.LEDStatus{
		StealthMode: allOff,
		LEDCount:    len(leds),
		LEDs:        ledInfos,
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

const ledCronTag = "# openwrt-travel-gui-led-schedule"

// GetLEDSchedule reads the LED stealth schedule from crontab.
func (s *SystemService) GetLEDSchedule() models.LEDSchedule {
	data, err := os.ReadFile("/etc/crontabs/root")
	if err != nil {
		return models.LEDSchedule{}
	}
	schedule := models.LEDSchedule{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, ledCronTag) {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		minute, hour := parts[0], parts[1]
		timeStr := fmt.Sprintf("%s:%s", hour, minute)
		if strings.Contains(line, "brightness-off") {
			schedule.OffTime = timeStr
			schedule.Enabled = true
		} else if strings.Contains(line, "brightness-on") {
			schedule.OnTime = timeStr
			schedule.Enabled = true
		}
	}
	return schedule
}

// SetLEDSchedule writes or removes LED schedule cron entries.
func (s *SystemService) SetLEDSchedule(schedule models.LEDSchedule) error {
	data, _ := os.ReadFile("/etc/crontabs/root")
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.Contains(line, ledCronTag) {
			continue
		}
		lines = append(lines, line)
	}
	if schedule.Enabled && schedule.OffTime != "" && schedule.OnTime != "" {
		offParts := strings.SplitN(schedule.OffTime, ":", 2)
		onParts := strings.SplitN(schedule.OnTime, ":", 2)
		if len(offParts) == 2 && len(onParts) == 2 {
			ledScript := "for f in /sys/class/leds/*/brightness; do echo %s > $f; done"
			offLine := fmt.Sprintf("%s %s * * * %s %s", offParts[1], offParts[0], fmt.Sprintf(ledScript, "0"), ledCronTag+" brightness-off")
			onLine := fmt.Sprintf("%s %s * * * %s %s", onParts[1], onParts[0], fmt.Sprintf(ledScript, "255"), ledCronTag+" brightness-on")
			lines = append(lines, offLine, onLine)
		}
	}
	lines = append(lines, "")
	if err := os.WriteFile("/etc/crontabs/root", []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return fmt.Errorf("writing crontab: %w", err)
	}
	_ = exec.Command("/etc/init.d/cron", "restart").Run()
	return nil
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

// UpgradeFirmware saves the uploaded firmware image and flashes it via sysupgrade.
// If keepSettings is true, current configuration is preserved (-v flag).
// If keepSettings is false, settings are discarded (-n flag).
func (s *SystemService) UpgradeFirmware(file io.Reader, keepSettings bool) error {
	firmwarePath := "/tmp/firmware.bin"
	out, err := os.Create(firmwarePath)
	if err != nil {
		return fmt.Errorf("creating firmware file: %w", err)
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(firmwarePath)
		return fmt.Errorf("saving firmware file: %w", err)
	}
	out.Close()

	var args []string
	if keepSettings {
		args = []string{"-v", firmwarePath}
	} else {
		args = []string{"-n", firmwarePath}
	}

	// Run sysupgrade asynchronously — the device will reboot
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = exec.Command("sysupgrade", args...).Run()
	}()

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

const setupCompleteFlagPath = "/etc/openwrt-travel-gui/setup-complete"

// GetSetupComplete checks whether the first-run setup has been completed.
func (s *SystemService) GetSetupComplete() models.SetupStatus {
	_, err := os.Stat(setupCompleteFlagPath)
	return models.SetupStatus{Complete: err == nil}
}

// SetSetupComplete marks the first-run setup as completed by creating the flag file.
func (s *SystemService) SetSetupComplete() error {
	dir := filepath.Dir(setupCompleteFlagPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create setup dir: %w", err)
	}
	f, err := os.Create(setupCompleteFlagPath)
	if err != nil {
		return fmt.Errorf("create setup flag: %w", err)
	}
	return f.Close()
}

const (
	buttonActionsDir  = "/etc/openwrt-travel-gui"
	buttonActionsFile = "/etc/openwrt-travel-gui/button-actions.json"
	hotplugScript     = "/etc/hotplug.d/button/50-gui-button-actions"
	dtKeysDir         = "/sys/firmware/devicetree/base/keys"
)

// detectButtonNames returns the physical button names for this device.
// Primary source: devicetree /sys/firmware/devicetree/base/keys — each
// sub-directory is a physical key; its "label" property is what OpenWrt
// sets as $BUTTON in hotplug events.
// /etc/rc.button ships generic stock scripts for every common button type
// regardless of hardware, so it is intentionally not used.
func detectButtonNames() []string {
	entries, err := os.ReadDir(dtKeysDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		labelPath := filepath.Join(dtKeysDir, e.Name(), "label")
		raw, err := os.ReadFile(labelPath)
		if err != nil {
			continue
		}
		// Devicetree strings are NUL-terminated; strip the trailing NUL.
		label := strings.TrimRight(string(raw), "\x00")
		if label != "" {
			names = append(names, label)
		}
	}
	return names
}

// GetHardwareButtons returns the detected hardware buttons with their configured actions.
func (s *SystemService) GetHardwareButtons() []models.HardwareButton {
	names := detectButtonNames()
	// Merge with configured actions
	configured := s.loadButtonActions()
	actionMap := make(map[string]models.ButtonAction, len(configured))
	for _, b := range configured {
		actionMap[b.Name] = b.Action
	}
	result := make([]models.HardwareButton, 0, len(names))
	for _, name := range names {
		action := models.ButtonActionNone
		if a, ok := actionMap[name]; ok {
			action = a
		}
		result = append(result, models.HardwareButton{Name: name, Action: action})
	}
	return result
}

// SetButtonActions saves button action config and regenerates the hotplug script.
func (s *SystemService) SetButtonActions(buttons []models.HardwareButton) error {
	// Validate actions
	for _, b := range buttons {
		switch b.Action {
		case models.ButtonActionNone, models.ButtonActionVPNToggle,
			models.ButtonActionWifiToggle, models.ButtonActionLEDToggle,
			models.ButtonActionReboot:
		default:
			return fmt.Errorf("unknown action %q for button %q", b.Action, b.Name)
		}
	}
	if err := os.MkdirAll(buttonActionsDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	// Write JSON config
	data := buildButtonActionsJSON(buttons)
	if err := os.WriteFile(buttonActionsFile, []byte(data), 0o644); err != nil {
		return fmt.Errorf("write button-actions config: %w", err)
	}
	// Generate hotplug script
	script := buildButtonHotplugScript(buttons)
	if err := os.MkdirAll(filepath.Dir(hotplugScript), 0o755); err != nil {
		return fmt.Errorf("create hotplug dir: %w", err)
	}
	if err := os.WriteFile(hotplugScript, []byte(script), 0o755); err != nil {
		return fmt.Errorf("write hotplug script: %w", err)
	}
	return nil
}

func (s *SystemService) loadButtonActions() []models.HardwareButton {
	data, err := os.ReadFile(buttonActionsFile)
	if err != nil {
		return nil
	}
	var buttons []models.HardwareButton
	// Simple JSON parse without encoding/json to avoid import cycle — use encoding/json directly
	_ = unmarshalButtonActions(data, &buttons)
	return buttons
}

func buildButtonActionsJSON(buttons []models.HardwareButton) string {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i, b := range buttons {
		sb.WriteString(fmt.Sprintf("  {\"name\":%q,\"action\":%q}", b.Name, string(b.Action)))
		if i < len(buttons)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("]\n")
	return sb.String()
}

// unmarshalButtonActions is a minimal JSON parser for the button actions file.
// We use encoding/json via a local import to avoid a circular reference.
func unmarshalButtonActions(data []byte, out *[]models.HardwareButton) error {
	type raw struct {
		Name   string `json:"name"`
		Action string `json:"action"`
	}
	// Parse manually: find pairs of "name":"..." "action":"..."
	text := string(data)
	var result []models.HardwareButton
	for {
		ni := strings.Index(text, `"name":`)
		if ni < 0 {
			break
		}
		text = text[ni+len(`"name":`):]
		name := extractJSONString(text)
		ai := strings.Index(text, `"action":`)
		if ai < 0 {
			break
		}
		text = text[ai+len(`"action":`):]
		action := extractJSONString(text)
		result = append(result, models.HardwareButton{
			Name:   name,
			Action: models.ButtonAction(action),
		})
		// advance past the action value
		text = text[strings.Index(text, `"`)+1:]
		rest := strings.Index(text, `"`)
		if rest < 0 {
			break
		}
		text = text[rest+1:]
	}
	*out = result
	return nil
}

func extractJSONString(s string) string {
	start := strings.Index(s, `"`)
	if start < 0 {
		return ""
	}
	s = s[start+1:]
	end := strings.Index(s, `"`)
	if end < 0 {
		return ""
	}
	return s[:end]
}

func buildButtonHotplugScript(buttons []models.HardwareButton) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# Generated by openwrt-travel-gui — do not edit manually.\n")
	sb.WriteString("[ \"$ACTION\" = \"pressed\" ] || exit 0\n")
	sb.WriteString("case \"$BUTTON\" in\n")
	for _, b := range buttons {
		if b.Action == models.ButtonActionNone {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %s)\n", b.Name))
		switch b.Action {
		case models.ButtonActionVPNToggle:
			// Use netifd-managed interface control; wg-quick is often absent on OpenWrt.
			sb.WriteString("    if /sbin/ifstatus wg0 2>/dev/null | grep -q '\"up\": true'; then\n")
			sb.WriteString("      /sbin/ifdown wg0 2>/dev/null || true\n")
			sb.WriteString("    else\n")
			sb.WriteString("      /sbin/ifup wg0 2>/dev/null || true\n")
			sb.WriteString("    fi\n")
		case models.ButtonActionWifiToggle:
			sb.WriteString("    if iwinfo 2>/dev/null | grep -q '^'; then\n")
			sb.WriteString("      wifi down\n")
			sb.WriteString("    else\n")
			sb.WriteString("      wifi up\n")
			sb.WriteString("    fi\n")
		case models.ButtonActionLEDToggle:
			sb.WriteString("    for led in /sys/class/leds/*/brightness; do\n")
			sb.WriteString("      cur=$(cat \"$led\" 2>/dev/null)\n")
			sb.WriteString("      [ \"$cur\" = \"0\" ] && echo 1 > \"$led\" || echo 0 > \"$led\"\n")
			sb.WriteString("    done\n")
		case models.ButtonActionReboot:
			sb.WriteString("    reboot\n")
		}
		sb.WriteString("    ;;\n")
	}
	sb.WriteString("esac\n")
	return sb.String()
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

const authorizedKeysFile = "/etc/dropbear/authorized_keys"

// GetSSHKeys returns all public keys from the authorized_keys file.
func (s *SystemService) GetSSHKeys() (models.SSHKeysResponse, error) {
	data, err := os.ReadFile(authorizedKeysFile)
	if err != nil {
		if os.IsNotExist(err) {
			return models.SSHKeysResponse{Keys: []models.SSHKey{}}, nil
		}
		return models.SSHKeysResponse{}, err
	}
	var keys []models.SSHKey
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		comment := ""
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			comment = strings.Join(parts[2:], " ")
		}
		keys = append(keys, models.SSHKey{Index: i, Comment: comment, Key: line})
	}
	if keys == nil {
		keys = []models.SSHKey{}
	}
	return models.SSHKeysResponse{Keys: keys}, nil
}

// AddSSHKey appends a public key to the authorized_keys file.
func (s *SystemService) AddSSHKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(authorizedKeysFile), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(authorizedKeysFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, key)
	return err
}

// DeleteSSHKey removes the key at the given line index from authorized_keys.
func (s *SystemService) DeleteSSHKey(index int) error {
	data, err := os.ReadFile(authorizedKeysFile)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if index < 0 || index >= len(lines) {
		return fmt.Errorf("key index %d out of range", index)
	}
	lines = append(lines[:index], lines[index+1:]...)
	return os.WriteFile(authorizedKeysFile, []byte(strings.Join(lines, "\n")), 0600)
}

const speedTestResultFile = "/tmp/openwrt-speed-test.json"

// RunSpeedTest runs a basic speed test using wget and writes results.
// On constrained hardware this performs a simple HTTP download measurement.
func (s *SystemService) RunSpeedTest() (models.SpeedTestResult, error) {
	result := models.SpeedTestResult{Server: "tele2.net (wget)"}

	// Measure download: fetch a 1MB test file and time it
	start := time.Now()
	_, err := exec.Command("wget", "-O", "/dev/null", "--timeout=15",
		"--no-check-certificate",
		"http://speedtest.tele2.net/1MB.zip").CombinedOutput()
	elapsed := time.Since(start).Seconds()
	if err == nil && elapsed > 0 {
		result.DownloadMbps = (1024 * 1024 * 8) / elapsed / 1e6
	}

	// Measure ping to 8.8.8.8
	pingOut, err2 := exec.Command("ping", "-c", "4", "-W", "3", "8.8.8.8").CombinedOutput()
	if err2 == nil {
		for _, line := range strings.Split(string(pingOut), "\n") {
			if strings.Contains(line, "avg") {
				// "round-trip min/avg/max = X/Y/Z ms"
				parts := strings.Split(line, "/")
				if len(parts) >= 5 {
					if v, err3 := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64); err3 == nil {
						result.PingMs = v
					}
				}
			}
		}
	}

	data, _ := json.Marshal(result)
	_ = os.WriteFile(speedTestResultFile, data, 0600)
	return result, nil
}
