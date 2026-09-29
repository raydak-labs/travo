package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/execx"
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
	// guardDir holds the crash-guard files (ADR 0003). Empty means
	// /etc/trafo; tests point it at a temp dir.
	guardDir     string
	guardDirOnce sync.Once
	// guardResolved caches the resolved guard directory.
	guardResolved string
	// sshKeysFile overrides authorizedKeysFile (tests only).
	sshKeysFile string
}

// sshKeysPath returns the authorized_keys path in use.
func (s *SystemService) sshKeysPath() string {
	if s.sshKeysFile != "" {
		return s.sshKeysFile
	}
	return authorizedKeysFile
}

// NewSystemService creates a new SystemService.
func NewSystemService(ub ubus.Ubus, u uci.UCI, storage StorageProvider) *SystemService {
	return &SystemService{ubus: ub, uci: u, storage: storage}
}

// SetGuardDir overrides the directory used for crash-guard files (tests).
func (s *SystemService) SetGuardDir(dir string) {
	s.guardDir = dir
	s.guardResolved = ""
	s.guardDirOnce = sync.Once{}
}

// Crash-guard file names. Each marks a live-state mutation that is unsafe to
// blindly retry after a power cut; the guard is removed only once the whole
// operation succeeded (AGENTS.md, ADR 0003).
const (
	restoreGuardName         = "restore-in-progress"
	firmwareUpgradeGuardName = "firmware-upgrade-in-progress"
	factoryResetGuardName    = "factory-reset-in-progress"
	systemConfigGuardName    = "system-config-in-progress"
)

func (s *SystemService) guardDirOrDefault() string {
	if s.guardDir != "" {
		return s.guardDir
	}
	s.guardDirOnce.Do(func() {
		const prod = "/etc/trafo"
		mkErr := os.MkdirAll(prod, 0o750)
		if mkErr == nil {
			s.guardResolved = prod
			return
		}
		// Not running in the normal deployment (dev host, unit tests) or the
		// installation is broken. Do not skip the guard: fall back to a temp
		// directory and say so loudly.
		fallback := filepath.Join(os.TempDir(), "travo-guards")
		log.Printf("ERROR: %s is not writable (%v); crash guards are being written to %s instead. A device-side flash or reset is NOT protected by a durable marker.", prod, mkErr, fallback)
		_ = os.MkdirAll(fallback, 0o750)
		s.guardResolved = fallback
	})
	return s.guardResolved
}

// writeGuard records that a dangerous operation has started. name is the
// file name, e.g. "firmware-upgrade-in-progress".
func (s *SystemService) writeGuard(name, reason string) error {
	dir := s.guardDirOrDefault()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("crash guard: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(reason+"\n"), 0o600); err != nil {
		return fmt.Errorf("crash guard: write %s: %w", path, err)
	}
	return nil
}

// clearGuard removes the crash guard after a successful operation.
func (s *SystemService) clearGuard(name string) {
	_ = os.Remove(filepath.Join(s.guardDirOrDefault(), name))
}

// LogStaleCrashGuards warns about guards left behind by an interrupted run so
// the operator can see (and clear) them after a power cut mid-flash.
func (s *SystemService) LogStaleCrashGuards() {
	for _, name := range []string{restoreGuardName, firmwareUpgradeGuardName, factoryResetGuardName, systemConfigGuardName} {
		path := filepath.Join(s.guardDirOrDefault(), name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		log.Printf("WARNING: crash guard present: %s (%s). The previous operation did not finish; clear it manually if the system looks healthy.", path, strings.TrimSpace(string(data)))
	}
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
	if release, ok := board["release"].(map[string]any); ok {
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
	if mem, ok := info["memory"].(map[string]any); ok {
		total, _ := mem["total"].(float64)
		free, _ := mem["free"].(float64)
		cached, _ := mem["cached"].(float64)
		buffered, _ := mem["buffered"].(float64)

		// cached/buffered can overlap with what procd already counted as free,
		// which makes the subtraction go negative. A negative "used" is worse
		// than a slightly wrong one (it renders as a nonsensical percentage).
		used := int64(total - free - cached - buffered)
		if used < 0 {
			used = 0
		}
		if used > int64(total) {
			used = int64(total)
		}

		stats.Memory = models.MemoryStats{
			TotalBytes:  int64(total),
			FreeBytes:   int64(free),
			CachedBytes: int64(cached + buffered),
			UsedBytes:   used,
		}
		if total > 0 {
			stats.Memory.UsagePercent = float64(used) / total * 100
		}
	}

	// CPU / Load
	cores := runtime.NumCPU()
	if load, ok := info["load"].([]any); ok && len(load) >= 3 {
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
// The call is async so the HTTP response returns before the system goes down,
// and it goes through execx so a hung poweroff cannot leak a process.
func (s *SystemService) Shutdown() error {
	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := execx.Run(execx.Quick, "poweroff"); err != nil {
			log.Printf("ERROR: poweroff failed: %v", err)
		}
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

// commitSystemConfig commits /etc/config/system under a crash guard and
// reports honestly what the caller must tell the user.
//
// These settings (timezone, hostname, NTP servers) are committed to UCI but
// NOT applied: the running procd/sysntpd instances keep the old values until
// the next boot. We deliberately do not start an rpcd apply here. An apply
// without a browser in the loop has no rollback confirmation, and the
// SystemService has no rpcd session available; a reboot is the safe activation
// path and is the same contract the restore endpoint already advertises
// ("Configuration restored. Reboot to apply changes.").
//
// The crash guard is removed as soon as the commit succeeds, so what it really
// protects is the commit itself: a failure leaves a marker plus a half-written
// /etc/config/system, which is exactly the state an operator must be told about
// rather than silently overwritten by the next request.
func (s *SystemService) commitSystemConfig(what string) error {
	if err := s.writeGuard(systemConfigGuardName, what); err != nil {
		return err
	}
	if err := s.uci.Commit("system"); err != nil {
		return fmt.Errorf("committing system config for %s (crash guard %s kept, /etc/config/system may be incomplete): %w", what, systemConfigGuardName, err)
	}
	s.clearGuard(systemConfigGuardName)
	return nil
}

// SetTimezone updates the timezone configuration in /etc/config/system.
// The change takes effect on the next reboot (see commitSystemConfig).
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
	return s.commitSystemConfig("timezone " + config.Zonename)
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
// The change takes effect on the next reboot (see commitSystemConfig); use
// SyncNTP for an immediate one-shot resync.
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
	return s.commitSystemConfig("ntp " + strings.Join(config.Servers, " "))
}

// SyncNTP forces a one-shot NTP sync using ntpd.
func (s *SystemService) SyncNTP() error {
	out, err := execx.CombinedOutput(execx.Slow, "ntpd", "-q", "-n", "-p", "pool.ntp.org")
	if err != nil {
		return fmt.Errorf("ntp sync failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetHostname changes the device hostname in /etc/config/system.
// The change takes effect on the next reboot (see commitSystemConfig).
func (s *SystemService) SetHostname(hostname string) error {
	section, _, err := s.findSystemSection()
	if err != nil {
		return fmt.Errorf("finding system section: %w", err)
	}
	if err := s.uci.Set("system", section, "hostname", hostname); err != nil {
		return err
	}
	return s.commitSystemConfig("hostname " + hostname)
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
	for line := range strings.SplitSeq(string(data), "\n") {
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
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" || strings.Contains(line, ledCronTag) {
			continue
		}
		lines = append(lines, line)
	}
	if schedule.Enabled && schedule.OffTime != "" && schedule.OnTime != "" {
		// Times are formatted into a root crontab line, so they must be strict
		// HH:MM — a newline would inject a second attacker-controlled entry.
		if err := ValidateHHMM(schedule.OffTime); err != nil {
			return fmt.Errorf("off_time: %w", err)
		}
		if err := ValidateHHMM(schedule.OnTime); err != nil {
			return fmt.Errorf("on_time: %w", err)
		}
		offParts := strings.SplitN(schedule.OffTime, ":", 2)
		onParts := strings.SplitN(schedule.OnTime, ":", 2)
		ledScript := "for f in /sys/class/leds/*/brightness; do echo %s > $f; done"
		offLine := fmt.Sprintf("%s %s * * * %s %s", offParts[1], offParts[0], fmt.Sprintf(ledScript, "0"), ledCronTag+" brightness-off")
		onLine := fmt.Sprintf("%s %s * * * %s %s", onParts[1], onParts[0], fmt.Sprintf(ledScript, "255"), ledCronTag+" brightness-on")
		lines = append(lines, offLine, onLine)
	}
	lines = append(lines, "")
	if err := os.WriteFile("/etc/crontabs/root", []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return fmt.Errorf("writing crontab: %w", err)
	}
	_ = execx.Run(execx.Quick, "/etc/init.d/cron", "restart")
	return nil
}

// Bounds for log capture. logread/dmesg on a device in a log loop can be tens
// of megabytes; buffering all of it per request exhausts the router's RAM.
const (
	maxLogLines    = 2000
	maxLogLineRune = 1000
)

// streamLogTail runs a log-producing command and keeps only the tail, so peak
// memory stays bounded no matter how chatty the system is.
func streamLogTail(timeout time.Duration, name string, args ...string) (string, error) {
	lines := make([]string, 0, 256)
	truncated := 0
	err := execx.Stream(timeout, func(line string) {
		if len(line) > maxLogLineRune {
			line = line[:maxLogLineRune]
		}
		lines = append(lines, line)
		if len(lines) > maxLogLines {
			// Drop from the front: the newest entries are the useful ones.
			lines = lines[1:]
			truncated++
		}
	}, name, args...)
	if err != nil {
		return "", err
	}
	if truncated > 0 {
		log.Printf("WARNING: %s output truncated to the last %d lines (%d older lines dropped)", name, maxLogLines, truncated)
	}
	return strings.Join(lines, "\n"), nil
}

// GetLogs retrieves system logs from logread.
// If service is non-empty, only lines containing that service name (case-insensitive) are returned.
// If level is non-empty, only lines at or above that severity are returned.
func (s *SystemService) GetLogs(service, level string) (models.LogResponse, error) {
	out, err := streamLogTail(execx.Quick, "logread")
	if err != nil {
		return models.LogResponse{}, err
	}
	return parseLogOutput("syslog", out, service, level), nil
}

// GetKernelLogs retrieves kernel logs from dmesg.
func (s *SystemService) GetKernelLogs() (models.LogResponse, error) {
	out, err := streamLogTail(execx.Quick, "dmesg")
	if err != nil {
		return models.LogResponse{}, err
	}
	return parseLogOutput("kernel", out, "", ""), nil
}

// CreateBackup generates a configuration backup archive and returns its path.
// The path is unique per call: the handler removes exactly what it created, so
// two backups started in the same second must not share a filename.
func (s *SystemService) CreateBackup() (string, error) {
	path, err := uniqueTempPath("/tmp", "backup-*.tar.gz")
	if err != nil {
		return "", err
	}
	out, err := execx.CombinedOutput(execx.Slow, "sysupgrade", "-b", path)
	if err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("creating backup: %w: %s", err, string(out))
	}
	return path, nil
}

// uniqueTempPath reserves a name that does not yet exist and returns it.
func uniqueTempPath(dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("creating temp path: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	// sysupgrade -b wants to create the file itself.
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return name, nil
}

// RestoreBackup applies a configuration backup from the given file path.
// `sysupgrade -r` rewrites /etc/config, so a crash guard is written first and
// removed only after the restore returned successfully (ADR 0003).
func (s *SystemService) RestoreBackup(path string) error {
	if err := s.writeGuard(restoreGuardName, "restore backup "+path); err != nil {
		return err
	}
	out, err := execx.CombinedOutput(execx.Slow, "sysupgrade", "-r", path)
	if err != nil {
		// Guard stays: the restore may have left /etc/config half-written.
		return fmt.Errorf("restoring backup (guard %s kept): %w: %s", restoreGuardName, err, string(out))
	}
	s.clearGuard(restoreGuardName)
	return nil
}

// UpgradeFirmware saves the uploaded firmware image and flashes it via sysupgrade.
// If keepSettings is true, current configuration is preserved (-v flag).
// If keepSettings is false, settings are discarded (-n flag).
// The flash is asynchronous — it takes minutes and reboots the device — and
// runs under a crash guard so a power cut mid-write is visible after reboot.
func (s *SystemService) UpgradeFirmware(file io.Reader, keepSettings bool) error {
	firmwarePath, err := uniqueTempPath("/tmp", "firmware-*.bin")
	if err != nil {
		return err
	}
	out, err := os.Create(firmwarePath)
	if err != nil {
		return fmt.Errorf("creating firmware file: %w", err)
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = out.Close()
		_ = os.Remove(firmwarePath)
		return fmt.Errorf("saving firmware file: %w", err)
	}
	_ = out.Close()

	var args []string
	if keepSettings {
		args = []string{"-v", firmwarePath}
	} else {
		args = []string{"-n", firmwarePath}
	}

	reason := "sysupgrade " + strings.Join(args, " ")
	if err := s.writeGuard(firmwareUpgradeGuardName, reason); err != nil {
		_ = os.Remove(firmwarePath)
		return err
	}

	// Run sysupgrade asynchronously — the device will reboot. The guard is
	// intentionally NOT removed: the marker must survive the reboot so an
	// interrupted flash is discoverable.
	go func() {
		time.Sleep(500 * time.Millisecond)
		// Firmware writes to the MTD partition take minutes on a router.
		if err := execx.Run(execx.Package, "sysupgrade", args...); err != nil {
			log.Printf("ERROR: sysupgrade %s failed: %v (crash guard %s kept)", firmwarePath, err, firmwareUpgradeGuardName)
		}
	}()

	return nil
}

// FactoryReset erases the overlay partition and reboots, restoring factory
// defaults. Both steps are asynchronous: firstboot can take tens of seconds on
// a large overlay, and blocking here would pin the HTTP handler (and, with
// fasthttp, every other connection) for that long. Only the pre-flight
// failures are reported synchronously; the guard file stays on disk because the
// device reboots before anyone could clear it.
func (s *SystemService) FactoryReset() error {
	// Pre-flight only: a missing binary is a real, reportable failure. The
	// long-running firstboot itself happens in the background below.
	if _, err := exec.LookPath("firstboot"); err != nil {
		return fmt.Errorf("factory reset unavailable: %w", err)
	}
	if err := s.writeGuard(factoryResetGuardName, "firstboot -y && reboot"); err != nil {
		return err
	}
	go func() {
		if err := execx.Run(execx.Package, "firstboot", "-y"); err != nil {
			log.Printf("ERROR: firstboot failed: %v (crash guard %s kept; device not reset)", err, factoryResetGuardName)
			return
		}
		if err := execx.Run(execx.Quick, "reboot"); err != nil {
			log.Printf("ERROR: reboot after firstboot failed: %v", err)
		}
	}()
	return nil
}

const setupCompleteFlagPath = "/etc/travo/setup-complete"

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
	buttonActionsDir  = "/etc/travo"
	buttonActionsFile = "/etc/travo/button-actions.json"
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
	buttons, err := s.GetHardwareButtonsWithError()
	if err != nil {
		// The on-disk hotplug script still runs the previous actions, so the
		// user must not be shown "no buttons configured" without a hint.
		log.Printf("ERROR: reading %s: %v (the generated hotplug script still uses the last saved actions)", buttonActionsFile, err)
	}
	return buttons
}

// GetHardwareButtonsWithError is GetHardwareButtons with the config-parse
// error surfaced, so a caller that can report it does.
func (s *SystemService) GetHardwareButtonsWithError() ([]models.HardwareButton, error) {
	names := detectButtonNames()
	configured, err := s.loadButtonActions()
	if err != nil {
		return nil, err
	}
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
	return result, nil
}

// SetButtonActions saves button action config and regenerates the hotplug script.
func (s *SystemService) SetButtonActions(buttons []models.HardwareButton) error {
	// Validate actions and names. Names become `case` labels in a root hotplug
	// shell script, so anything outside the detected devicetree labels (and
	// outside [A-Za-z0-9_-]) must be rejected before the script is written.
	discovered := detectButtonNames()
	for _, b := range buttons {
		switch b.Action {
		case models.ButtonActionNone, models.ButtonActionVPNToggle,
			models.ButtonActionWifiToggle, models.ButtonActionLEDToggle,
			models.ButtonActionReboot:
		default:
			return fmt.Errorf("unknown action %q for button %q", b.Action, b.Name)
		}
		if err := ValidateButtonName(b.Name, discovered); err != nil {
			return err
		}
	}
	if err := writeWirelessToggleScript(); err != nil {
		return err
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

// loadButtonActions reads the button-action config from disk.
// A missing file is not an error (nothing configured yet); a malformed one is,
// because silently reporting "no buttons configured" while the hotplug script
// keeps firing the previous actions is a real misconfiguration.
func (s *SystemService) loadButtonActions() ([]models.HardwareButton, error) {
	data, err := os.ReadFile(buttonActionsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", buttonActionsFile, err)
	}
	var buttons []models.HardwareButton
	if err := json.Unmarshal(data, &buttons); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", buttonActionsFile, err)
	}
	return buttons, nil
}

func buildButtonActionsJSON(buttons []models.HardwareButton) string {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i, b := range buttons {
		fmt.Fprintf(&sb, "  {\"name\":%q,\"action\":%q}", b.Name, string(b.Action))
		if i < len(buttons)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("]\n")
	return sb.String()
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
		fmt.Fprintf(&sb, "  %s)\n", b.Name)
		switch b.Action {
		case models.ButtonActionVPNToggle:
			// Use netifd-managed interface control; wg-quick is often absent on OpenWrt.
			sb.WriteString("    if /sbin/ifstatus wg0 2>/dev/null | grep -q '\"up\": true'; then\n")
			sb.WriteString("      /sbin/ifdown wg0 2>/dev/null || true\n")
			sb.WriteString("    else\n")
			sb.WriteString("      /sbin/ifup wg0 2>/dev/null || true\n")
			sb.WriteString("    fi\n")
		case models.ButtonActionWifiToggle:
			// Never `wifi up`/`wifi down` from a script: delegate to the
			// generated toggle helper, which writes UCI and applies via rpcd.
			sb.WriteString("    if iwinfo 2>/dev/null | grep -q '^'; then\n")
			sb.WriteString("      " + wirelessToggleScriptPath + " down\n")
			sb.WriteString("    else\n")
			sb.WriteString("      " + wirelessToggleScriptPath + " up\n")
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

// logLevelPattern finds the severity token of a syslog line wherever it sits.
// Matching the token itself (instead of a fixed column) keeps level filtering
// working for every logread variant: with or without the "dow mon day time
// year" prefix, and with a trailing ':' on the token.
var logLevelPattern = regexp.MustCompile(`\b[a-z][a-z0-9_-]*\.(emerg|alert|crit|err|error|warning|warn|notice|info|debug)\b`)

// extractLevel extracts the syslog level from a log line, independently of the
// column the timestamp occupies. BusyBox logread emits
// "Tue Mar 10 22:00:34 2026 kern.info kernel: ..." but other producers (and
// dmesg) drop the weekday, and a fixed index then yields "" for every line —
// which silently disabled GET /logs?level=… filtering.
// Returns the level string (e.g. "info", "err") or empty string if not found.
func extractLevel(line string) string {
	m := logLevelPattern.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	level := m[1]
	// Aliases that map onto the canonical severity names.
	switch level {
	case "error":
		return "err"
	case "warn":
		return "warning"
	}
	return level
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
	data, err := os.ReadFile(s.sshKeysPath())
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
	// The key is appended verbatim to authorized_keys: a newline would add
	// extra authorized_keys lines (i.e. grant access to another key).
	if strings.ContainsAny(key, "\n\r") {
		return fmt.Errorf("key must be a single line")
	}
	if !sshKeyRe.MatchString(key) {
		return fmt.Errorf("invalid SSH public key: expected [type] [base64] [comment]")
	}
	if err := os.MkdirAll(filepath.Dir(s.sshKeysPath()), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.sshKeysPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintln(f, key)
	return err
}

// DeleteSSHKey removes the key at the given line index from authorized_keys.
// The index space is byte-for-byte the same one GetSSHKeys reports (both split
// the trimmed file content on "\n"), so the number a client saw always maps to
// the same line. Negative and out-of-range indexes are rejected instead of
// silently truncating the file.
func (s *SystemService) DeleteSSHKey(index int) error {
	path := s.sshKeysPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if index < 0 || index >= len(lines) {
		return fmt.Errorf("key index %d out of range (file has %d line(s))", index, len(lines))
	}
	lines = append(lines[:index], lines[index+1:]...)
	if len(lines) == 0 {
		return os.WriteFile(path, nil, 0600)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

const speedTestResultFile = "/tmp/openwrt-speed-test.json"

// RunSpeedTest runs a basic speed test using wget and writes results.
// On constrained hardware this performs a simple HTTP download measurement.
func (s *SystemService) RunSpeedTest() (models.SpeedTestResult, error) {
	result := models.SpeedTestResult{Server: "tele2.net (wget)"}

	// Measure download: fetch a 1MB test file and time it
	start := time.Now()
	_, err := execx.CombinedOutput(execx.Quick, "wget", "-O", "/dev/null", "--timeout=15",
		"--no-check-certificate",
		"http://speedtest.tele2.net/1MB.zip")
	elapsed := time.Since(start).Seconds()
	if err == nil && elapsed > 0 {
		result.DownloadMbps = (1024 * 1024 * 8) / elapsed / 1e6
	}

	// Measure ping to 8.8.8.8
	pingOut, err2 := execx.CombinedOutput(execx.Quick, "ping", "-c", "4", "-W", "3", "8.8.8.8")
	if err2 == nil {
		for line := range strings.SplitSeq(string(pingOut), "\n") {
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
