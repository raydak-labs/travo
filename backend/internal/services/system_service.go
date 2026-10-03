package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
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
	// toggleScriptPath overrides where the generated wireless toggle helper is
	// written (tests only); empty means the production /usr/libexec path.
	toggleScriptPath string
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
		const prod = crashGuardDir
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

// ledCrontabPath is busybox crond's per-user crontab. It is the ONLY file it
// reads (it is built with -c /etc/crontabs), which is also why the WiFi schedule
// targets the same file.
const ledCrontabPath = "/etc/crontabs/root"

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
//
// Shares crontabMu with the WiFi schedule: both rewrite /etc/crontabs/root
// while preserving the lines they do not own, so a concurrent pair would drop
// one another's entries.
func (s *SystemService) SetLEDSchedule(schedule models.LEDSchedule) error {
	crontabMu.Lock()
	defer crontabMu.Unlock()

	data, _ := os.ReadFile(ledCrontabPath)
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
	if err := writeFileAtomic(ledCrontabPath, []byte(strings.Join(lines, "\n")), 0600); err != nil {
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
//
// The archive is validated FIRST: `sysupgrade -r` extracts the tarball at /,
// so an unvalidated upload is an arbitrary-root-file-write primitive (any path
// under /etc/crontabs, /etc/init.d or /etc/uci-defaults becomes a root code
// execution path). Validation happens before the crash guard is written because
// a rejected archive has touched nothing, so it must leave no state behind.
//
// A guard is then written and only removed once sysupgrade returned
// successfully: `sysupgrade -r` rewrites /etc/config (ADR 0003).
func (s *SystemService) RestoreBackup(path string) error {
	if err := ValidateRestoreArchive(path); err != nil {
		return err
	}
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

// ErrInvalidBackupArchive marks an uploaded restore archive that must never
// reach sysupgrade. Handlers map it to 400: the operator uploaded something
// that is not a configuration backup.
var ErrInvalidBackupArchive = errors.New("invalid backup archive")

// Restore archive limits. A travel router has a ~128 MB tmpfs, so an upload is
// untrusted input that must not be able to fill it.
const (
	// maxRestoreArchiveMembers bounds the number of tar headers. A genuine
	// `sysupgrade -b` archive holds a few hundred files at most.
	maxRestoreArchiveMembers = 4096
	// maxRestoreArchiveBytes bounds the total UNCOMPRESSED size claimed by the
	// members, which is what a decompression bomb inflates.
	maxRestoreArchiveBytes = 64 << 20 // 64 MiB
	// maxRestoreMemberBytes bounds a single member.
	maxRestoreMemberBytes = 8 << 20 // 8 MiB
)

// restoreArchiveNeedsUCIConfig is the one structural requirement that
// distinguishes a configuration backup from an arbitrary tarball. A genuine
// `sysupgrade -b` archive always carries the UCI packages under etc/config,
// so requiring at least one member there rejects "here is a tar of whatever I
// liked" without having to enumerate every path a real overlay contains.
//
// There is deliberately no directory allowlist beyond that. An earlier version
// restricted members to etc/config and etc/ppp, on the reasoning that an
// archive able to write etc/crontabs/root is an execution primitive. That was
// defence for the wrong layer: restore is an authenticated-admin-only endpoint
// (ADR 0007 section 2), and that admin already holds root through
// POST /system/ssh-keys, firmware flash and factory reset. The allowlist bought
// no real protection while guaranteeing that every genuine backup taken on
// THIS device was refused, because the application itself writes
// /etc/crontabs/root (LED and WiFi schedules), /etc/dropbear/authorized_keys
// and, after a password change, /etc/shadow — all of which `sysupgrade -b`
// captures. Refusing those silently breaks restore for real users, and a
// partial restore is its own surprise: the operator believes their
// configuration came back and it did not.
const restoreArchiveNeedsUCIConfig = "etc/config/"

// ValidateRestoreArchive checks that path is a structurally safe gzip tar
// archive carrying at least one UCI config member.
//
// Rejected: absolute paths, ".." traversal, symlinks, hardlinks, device nodes,
// archives exceeding the size/member caps, and archives with nothing under
// etc/config (i.e. not a configuration backup).
//
// It does NOT police WHICH paths an administrator may write. Authorization is
// that boundary (ADR 0007 section 2), and this function exists to reject
// malformed and hostile archives — traversal, link substitution and
// decompression bombs — so a corrupted or third-party file cannot damage a
// device the operator is trying to recover.
func ValidateRestoreArchive(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBackupArchive, err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%w: not a gzip archive: %w", ErrInvalidBackupArchive, err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var total int64
	members := 0
	sawUCIConfig := false
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: unreadable tar stream: %w", ErrInvalidBackupArchive, err)
		}
		members++
		if members > maxRestoreArchiveMembers {
			return fmt.Errorf("%w: more than %d members", ErrInvalidBackupArchive, maxRestoreArchiveMembers)
		}
		if err := checkRestoreMember(hdr); err != nil {
			return err
		}
		if hdr.Size < 0 || hdr.Size > maxRestoreMemberBytes {
			return fmt.Errorf("%w: member %q is larger than %d bytes", ErrInvalidBackupArchive, hdr.Name, maxRestoreMemberBytes)
		}
		total += hdr.Size
		if total > maxRestoreArchiveBytes {
			return fmt.Errorf("%w: uncompressed size exceeds %d bytes", ErrInvalidBackupArchive, maxRestoreArchiveBytes)
		}
		if strings.HasPrefix(cleanRestoreName(hdr.Name), restoreArchiveNeedsUCIConfig) {
			sawUCIConfig = true
		}
	}
	if members == 0 {
		return fmt.Errorf("%w: archive is empty", ErrInvalidBackupArchive)
	}
	if !sawUCIConfig {
		return fmt.Errorf("%w: not a configuration backup: no member under %s",
			ErrInvalidBackupArchive, restoreArchiveNeedsUCIConfig)
	}
	return nil
}

// cleanRestoreName normalises a member path for the etc/config membership test.
func cleanRestoreName(name string) string {
	return path.Clean(strings.TrimPrefix(name, "/"))
}

// checkRestoreMember rejects one tar header that sysupgrade -r would extract
// unsafely — outside the archive root, or as anything other than a plain file.
func checkRestoreMember(hdr *tar.Header) error {
	name := hdr.Name
	if name == "" {
		return fmt.Errorf("%w: member with an empty name", ErrInvalidBackupArchive)
	}
	// sysupgrade -r extracts relative to /, so a leading slash writes outside
	// the config tree and "..", however it is spelled, walks out of it.
	if strings.HasPrefix(name, "/") || strings.Contains(name, `\`) {
		return fmt.Errorf("%w: member %q is not a relative path", ErrInvalidBackupArchive, name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("%w: member %q escapes the archive root", ErrInvalidBackupArchive, name)
	}
	// Only regular files and the directories that contain them: a symlink or
	// hard link member is a write-what-where primitive (tar replaces the link
	// target's contents, or re-points it), and device nodes have no business in
	// a config backup.
	switch hdr.Typeflag {
	case tar.TypeReg, tar.TypeDir:
	default:
		return fmt.Errorf("%w: member %q has type %q, only regular files are allowed",
			ErrInvalidBackupArchive, name, string(rune(hdr.Typeflag)))
	}
	return nil
}

// Firmware image validation. A `.bin` extension check is not validation: a
// junk file with the right name is the textbook way to brick a router, and a
// travel router has no serial console to recover from. sysupgrade does re-check
// the image, but it does so in the background AFTER this endpoint answered 200,
// so doing it here turns a brick into a 400.
var (
	// ErrInvalidFirmware marks an image that must not be flashed. Handlers map
	// it to 400.
	ErrInvalidFirmware = errors.New("invalid firmware image")
	// ErrNoFirmwareMetadata is the "this is not an OpenWrt sysupgrade image"
	// case: no metadata block was found in the header.
	ErrNoFirmwareMetadata = errors.New("no OpenWrt firmware metadata block")
	// ErrUnsupportedFirmware is a readable image for other hardware.
	ErrUnsupportedFirmware = errors.New("firmware does not support this device")
)

// FirmwareMetadata is the OpenWrt metadata block that follows the kernel image
// in a sysupgrade image. It is what `sysupgrade -l` and the build system read to
// decide which boards an image may be flashed onto.
type FirmwareMetadata struct {
	// Model is the image's own model string, e.g. "Linksys EA8300".
	Model string
	// SupportedDevices is the image's supported_devices list, split on commas
	// for display. Note that OpenWrt board names themselves contain a comma
	// ("glinet,gl-mt3000"), so SupportedDevicesRaw is what device matching uses.
	SupportedDevices []string
	// SupportedDevicesRaw is the supported_devices value verbatim.
	SupportedDevicesRaw string
	// Version is the firmware version, e.g. "23.05.2".
	Version string
	// Distname is the distribution name, e.g. "OpenWrt".
	Distname string
	// CompatVersion is the metadata compat_version field.
	CompatVersion uint32
}

const (
	// firmwareMetadataMagic is the literal that opens the metadata payload,
	// NUL padded to 10 bytes.
	firmwareMetadataMagic = "metadata\x00\x00"
	// firmwareMetadataVersion is the only metadata block version mkimage emits.
	firmwareMetadataVersion = 1
	// maxFirmwareMetadataScan bounds how far into the image the metadata block is
	// searched. It sits directly after the (4-byte aligned) kernel image, so a
	// few MiB is generous; the bound keeps a junk upload from being read whole.
	maxFirmwareMetadataScan = 8 << 20
	// firmwareMetadataReadChunk is the streaming read size.
	firmwareMetadataReadChunk = 64 << 10
)

// ParseFirmwareMetadata reads the OpenWrt metadata block from the head of a
// sysupgrade image and returns it.
//
// Layout (as written by tools/mkimage and read by scripts/json_overview_image_info.py):
//
//	uint32 block size   (big endian, counts everything below including these 8 bytes)
//	uint32 version      (big endian, 1)
//	char   magic[10]    "metadata\0\0"
//	uint32 compat_version (big endian)
//	char   pairs[]      NUL-terminated key/value strings, key\0value\0…
func ParseFirmwareMetadata(r io.Reader) (FirmwareMetadata, error) {
	block, err := scanFirmwareMetadataBlock(r)
	if err != nil {
		return FirmwareMetadata{}, err
	}
	return parseFirmwareMetadataBlock(block)
}

// scanFirmwareMetadataBlock streams the head of the image and returns the
// complete metadata block, or ErrNoFirmwareMetadata.
func scanFirmwareMetadataBlock(r io.Reader) ([]byte, error) {
	magic := []byte(firmwareMetadataMagic)
	// Keep this much context around the magic so the 8-byte prefix and the
	// following fields are available even when the magic straddles a read.
	keep := len(magic) + 8 + 4
	buf := make([]byte, 0, firmwareMetadataReadChunk+keep)
	chunk := make([]byte, firmwareMetadataReadChunk)
	scanned := 0
	for scanned < maxFirmwareMetadataScan {
		n, err := r.Read(chunk)
		if n > 0 {
			scanned += n
			buf = append(buf, chunk[:n]...)
			if i := bytes.Index(buf, magic); i >= 8 {
				start := i - 8
				size := int(binary.BigEndian.Uint32(buf[start : start+4]))
				if size >= 8+len(magic)+4 && size <= maxFirmwareMetadataScan {
					need := start + size
					for len(buf) < need {
						n, rerr := r.Read(chunk)
						if n > 0 {
							buf = append(buf, chunk[:n]...)
							continue
						}
						if errors.Is(rerr, io.EOF) {
							break
						}
						if rerr != nil {
							return nil, rerr
						}
					}
					if len(buf) < need {
						return nil, fmt.Errorf("%w: truncated metadata block", ErrNoFirmwareMetadata)
					}
					return buf[start:need], nil
				}
			}
			// Keep the tail so a magic split across two reads is still found.
			if len(buf) > keep {
				buf = buf[len(buf)-keep:]
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: no \"metadata\" block in the first %d bytes", ErrNoFirmwareMetadata, maxFirmwareMetadataScan)
}

// parseFirmwareMetadataBlock decodes one metadata block.
func parseFirmwareMetadataBlock(block []byte) (FirmwareMetadata, error) {
	if len(block) < 8+len(firmwareMetadataMagic)+4 {
		return FirmwareMetadata{}, fmt.Errorf("%w: block is too short", ErrNoFirmwareMetadata)
	}
	if string(block[8:8+len(firmwareMetadataMagic)]) != firmwareMetadataMagic {
		return FirmwareMetadata{}, fmt.Errorf("%w: bad block magic", ErrNoFirmwareMetadata)
	}
	if version := binary.BigEndian.Uint32(block[4:8]); version != firmwareMetadataVersion {
		return FirmwareMetadata{}, fmt.Errorf("%w: unsupported metadata version %d", ErrNoFirmwareMetadata, version)
	}
	meta := FirmwareMetadata{CompatVersion: binary.BigEndian.Uint32(block[8+len(firmwareMetadataMagic):])}
	rest := block[8+len(firmwareMetadataMagic)+4:]
	for len(rest) > 0 {
		key, remainder, ok := cutNUL(rest)
		if !ok || key == "" {
			break
		}
		value, remainder, ok := cutNUL(remainder)
		if !ok {
			break
		}
		switch key {
		case "model":
			meta.Model = value
		case "version":
			meta.Version = value
		case "distname":
			meta.Distname = value
		case "supported_devices":
			meta.SupportedDevicesRaw = value
			for _, dev := range strings.Split(value, ",") {
				if dev = strings.TrimSpace(dev); dev != "" {
					meta.SupportedDevices = append(meta.SupportedDevices, dev)
				}
			}
		}
		rest = remainder
	}
	if meta.Model == "" {
		return FirmwareMetadata{}, fmt.Errorf("%w: metadata block has no model", ErrNoFirmwareMetadata)
	}
	return meta, nil
}

func cutNUL(b []byte) (value string, rest []byte, ok bool) {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		return "", nil, false
	}
	return string(b[:i]), b[i+1:], true
}

// SupportsDevice reports whether the image declares boardName (or any alias in
// aliases) as a supported device.
//
// OpenWrt board names contain a comma ("glinet,gl-mt3000") and the metadata
// list is itself comma separated, so the list cannot be split into board names
// without ambiguity. This matches the way OpenWrt's own platform_check_image
// does it: the board name must appear in the raw list. Names are compared
// case-insensitively with spaces/underscores folded to dashes, because
// "GL.iNet GL-MT3000", "gl-mt3000" and "glinet,gl-mt3000" all name one board.
func (m FirmwareMetadata) SupportsDevice(boardName string, aliases []string) bool {
	list := normaliseBoardName(m.SupportedDevicesRaw)
	if list == "" {
		return false
	}
	if strings.Contains(list, normaliseBoardName(boardName)) {
		return true
	}
	for _, a := range aliases {
		if n := normaliseBoardName(a); n != "" && strings.Contains(list, n) {
			return true
		}
	}
	return false
}

func normaliseBoardName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "-", "_", "-").Replace(s)
	return s
}

// boardIdentity returns the strings this device may legitimately be named by in
// an image's supported_devices list: the ubus board_name ("glinet,gl-mt3000"),
// each of its comma-separated parts, and the model ("GL.iNet GL-MT3000").
func (s *SystemService) boardIdentity() (string, []string, error) {
	board, err := s.ubus.Call("system", "board", nil)
	if err != nil {
		return "", nil, err
	}
	boardName, _ := board["board_name"].(string)
	model, _ := board["model"].(string)
	if boardName == "" && model == "" {
		return "", nil, fmt.Errorf("ubus system board returned no board identity")
	}
	primary := boardName
	if primary == "" {
		primary = model
	}
	var aliases []string
	if boardName != "" {
		aliases = append(aliases, strings.Split(boardName, ",")...)
	}
	if model != "" {
		aliases = append(aliases, model)
		// "GL.iNet GL-MT3000" → also try the trailing model token.
		if fields := strings.Fields(model); len(fields) > 1 {
			aliases = append(aliases, fields[len(fields)-1])
		}
	}
	return primary, aliases, nil
}

// ValidateFirmwareImage parses the OpenWrt metadata of an uploaded image and
// checks it against this board. It is exported so a handler can validate an
// upload before staging it.
func (s *SystemService) ValidateFirmwareImage(file io.Reader) (FirmwareMetadata, error) {
	meta, err := ParseFirmwareMetadata(file)
	if err != nil {
		return FirmwareMetadata{}, fmt.Errorf("%w: %w", ErrInvalidFirmware, err)
	}
	boardName, aliases, err := s.boardIdentity()
	if err != nil {
		return meta, fmt.Errorf("%w: cannot determine this board's identity, refusing to flash: %w", ErrInvalidFirmware, err)
	}
	if len(meta.SupportedDevices) == 0 {
		// Without the list nothing can be checked, and an unverifiable image on
		// a router with no serial console is a coin flip with the device on the
		// wrong side. sysupgrade would happily try.
		return meta, fmt.Errorf("%w: image %q declares no supported_devices, so it cannot be verified for %s",
			ErrInvalidFirmware, meta.Model, boardName)
	}
	if !meta.SupportsDevice(boardName, aliases) {
		return meta, fmt.Errorf("%w: %w: image %q supports %v, this device is %s",
			ErrInvalidFirmware, ErrUnsupportedFirmware, meta.Model, meta.SupportedDevicesRaw, boardName)
	}
	return meta, nil
}

// UpgradeFirmware saves the uploaded firmware image and flashes it via sysupgrade.
// If keepSettings is true, current configuration is preserved (-v flag).
// If keepSettings is false, settings are discarded (-n flag).
//
// The image is staged at a unique path, then its OpenWrt metadata is parsed and
// checked against this board BEFORE the crash guard is written and long before
// sysupgrade runs, so a junk or foreign image is a 400 and never reaches the
// flash. The parsed metadata is returned so the caller can tell the operator
// which image was flashed.
//
// The flash is asynchronous — it takes minutes and reboots the device — and
// runs under a crash guard so a power cut mid-write is visible after reboot.
func (s *SystemService) UpgradeFirmware(file io.Reader, keepSettings bool) (FirmwareMetadata, error) {
	firmwarePath, err := uniqueTempPath("/tmp", "firmware-*.bin")
	if err != nil {
		return FirmwareMetadata{}, err
	}
	out, err := os.Create(firmwarePath)
	if err != nil {
		return FirmwareMetadata{}, fmt.Errorf("creating firmware file: %w", err)
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = out.Close()
		_ = os.Remove(firmwarePath)
		return FirmwareMetadata{}, fmt.Errorf("saving firmware file: %w", err)
	}
	_ = out.Close()

	staged, err := os.Open(firmwarePath)
	if err != nil {
		_ = os.Remove(firmwarePath)
		return FirmwareMetadata{}, fmt.Errorf("re-reading firmware file: %w", err)
	}
	meta, err := s.ValidateFirmwareImage(staged)
	_ = staged.Close()
	if err != nil {
		_ = os.Remove(firmwarePath)
		return meta, err
	}

	var args []string
	if keepSettings {
		args = []string{"-v", firmwarePath}
	} else {
		args = []string{"-n", firmwarePath}
	}

	reason := "sysupgrade " + strings.Join(args, " ") + " (" + meta.Model + ")"
	if err := s.writeGuard(firmwareUpgradeGuardName, reason); err != nil {
		_ = os.Remove(firmwarePath)
		return meta, err
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

	return meta, nil
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
	if err := writeWirelessToggleScriptTo(s.toggleScriptPath); err != nil {
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
