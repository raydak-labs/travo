package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/guard"
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
	// Bsize is int64 on linux and uint32 on darwin; uint64 is a real
	// conversion on both, so neither build flags it as unnecessary.
	bsize := int64(uint64(stat.Bsize))
	total := int64(stat.Blocks) * bsize
	free := int64(stat.Bavail) * bsize
	used := total - free
	return total, used, free, nil
}

// Service provides system information and statistics.
type Service struct {
	ubus    ubus.Ubus
	uci     uci.UCI
	storage StorageProvider
	// guardDir holds the crash-guard files (ADR 0003); empty means guard.Dir().
	guardDir string
	// sshKeysFile overrides authorizedKeysFile (tests only).
	sshKeysFile string
	// toggleScriptPath overrides where the generated wireless toggle helper is
	// written (tests only); empty means the production /usr/libexec path.
	toggleScriptPath string
	// logTail replaces logread/dmesg (mock mode); nil runs the commands.
	logTail func(cmd string) (string, error)
}

// sshKeysPath returns the authorized_keys path in use.
func (s *Service) sshKeysPath() string {
	if s.sshKeysFile != "" {
		return s.sshKeysFile
	}
	return authorizedKeysFile
}

// New creates a new SystemService.
func New(ub ubus.Ubus, u uci.UCI, storage StorageProvider) *Service {
	return &Service{ubus: ub, uci: u, storage: storage}
}

// SetGuardDir overrides the directory used for crash-guard files (tests).
func (s *Service) SetGuardDir(dir string) { s.guardDir = dir }

// Crash guards. Each marks a live-state mutation that is unsafe to blindly
// retry after a power cut (ADR 0003).
const (
	restoreGuard         = "restore"
	firmwareUpgradeGuard = "firmware-upgrade"
	factoryResetGuard    = "factory-reset"
	systemConfigGuard    = "system-config"
)

func (s *Service) crashGuard(feature string) guard.Guard {
	return guard.In(s.guardDir, feature)
}

// LogStaleCrashGuards warns about guards left behind by an interrupted run so
// the operator can see (and clear) them after a power cut mid-flash.
func (s *Service) LogStaleCrashGuards() {
	for _, feature := range []string{restoreGuard, firmwareUpgradeGuard, factoryResetGuard, systemConfigGuard} {
		g := s.crashGuard(feature)
		data, err := g.Read()
		if err != nil {
			continue
		}
		slog.Warn("crash guard present: the previous operation did not finish; clear it manually if the system looks healthy", "path", g.Path(), "reason", strings.TrimSpace(string(data)))
	}
}

// GetSystemInfo returns system identification information.
func (s *Service) GetSystemInfo() (models.SystemInfo, error) {
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
func (s *Service) GetSystemStats() (models.SystemStats, error) {
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

// ReadNetworkStats returns only the per-interface byte counters, without the
// ubus call and storage probe GetSystemStats also does. The WebSocket hub uses
// it to keep the traffic history warm while no client is connected.
func (s *Service) ReadNetworkStats() []models.NetworkInterfaceStats {
	return readNetworkStats()
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

// sysfsNetRoot is the directory holding the per-interface counter directories.
// It is a variable only so tests can point it at a fixture tree: the device
// interfaces readNetworkStats looks for (br-lan, wwan0, wg0, eth0) do not exist
// on a CI or developer host, which would otherwise make any test of network
// sampling silently pass on empty input.
var sysfsNetRoot = "/sys/class/net"

// SetSysfsNetRootForTesting points interface-counter reads at root and returns
// a function restoring the previous value. Call it before starting any
// background sampler; like SetBcryptCostForTesting it is not lock-protected.
func SetSysfsNetRootForTesting(root string) (restore func()) {
	previous := sysfsNetRoot
	sysfsNetRoot = root
	return func() { sysfsNetRoot = previous }
}

// readSysfsCounter reads a single counter value from <sysfsNetRoot>/<iface>/statistics/<counter>.
func readSysfsCounter(iface, counter string) (int64, error) {
	path := filepath.Join(sysfsNetRoot, iface, "statistics", counter)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

// Reboot initiates a system reboot via ubus.
// The reboot call is async so the HTTP response returns before the system goes down.
func (s *Service) Reboot() error {
	go func() {
		time.Sleep(500 * time.Millisecond)
		_, _ = s.ubus.Call("system", "reboot", nil)
	}()
	return nil
}

// Shutdown initiates a system poweroff.
// The call is async so the HTTP response returns before the system goes down,
// and it goes through execx so a hung poweroff cannot leak a process.
func (s *Service) Shutdown() error {
	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := execx.Run(execx.Quick, "poweroff"); err != nil {
			slog.Error("poweroff failed", "err", err)
		}
	}()
	return nil
}

// findSystemSection returns the UCI section name for the system-type section.
// On real OpenWRT it's an anonymous section (e.g. cfg01e48a); in mocks it may be "system".
func (s *Service) findSystemSection() (string, map[string]string, error) {
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
func (s *Service) GetTimezone() (models.TimezoneConfig, error) {
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
func (s *Service) commitSystemConfig(what string) error {
	if err := s.crashGuard(systemConfigGuard).Write(what); err != nil {
		return err
	}
	if err := s.uci.Commit("system"); err != nil {
		return fmt.Errorf("committing system config for %s (crash guard %s kept, /etc/config/system may be incomplete): %w", what, s.crashGuard(systemConfigGuard).Path(), err)
	}
	_ = s.crashGuard(systemConfigGuard).Clear()
	return nil
}

// SetTimezone updates the timezone configuration in /etc/config/system.
// The change takes effect on the next reboot (see commitSystemConfig).
func (s *Service) SetTimezone(config models.TimezoneConfig) error {
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
func (s *Service) GetNTPConfig() (models.NTPConfig, error) {
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
func (s *Service) SetNTPConfig(config models.NTPConfig) error {
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
func (s *Service) SyncNTP() error {
	out, err := execx.CombinedOutput(execx.Slow, "ntpd", "-q", "-n", "-p", "pool.ntp.org")
	if err != nil {
		return fmt.Errorf("ntp sync failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetHostname changes the device hostname in /etc/config/system.
// The change takes effect on the next reboot (see commitSystemConfig).
func (s *Service) SetHostname(hostname string) error {
	section, _, err := s.findSystemSection()
	if err != nil {
		return fmt.Errorf("finding system section: %w", err)
	}
	if err := s.uci.Set("system", section, "hostname", hostname); err != nil {
		return err
	}
	return s.commitSystemConfig("hostname " + hostname)
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
		slog.Warn("command output truncated", "command", name, "kept_lines", maxLogLines, "dropped_lines", truncated)
	}
	return strings.Join(lines, "\n"), nil
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
// so requiring at least one regular FILE there rejects "here is a tar of
// whatever I liked" without having to enumerate every path a real overlay
// contains.
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

// FactoryReset erases the overlay partition and reboots, restoring factory
// defaults. Both steps are asynchronous: firstboot can take tens of seconds on
// a large overlay, and blocking here would pin the HTTP handler (and, with
// fasthttp, every other connection) for that long. Only the pre-flight
// failures are reported synchronously; the guard file stays on disk because the
// device reboots before anyone could clear it.
func (s *Service) FactoryReset() error {
	// Pre-flight only: a missing binary is a real, reportable failure. The
	// long-running firstboot itself happens in the background below.
	if _, err := exec.LookPath("firstboot"); err != nil {
		return fmt.Errorf("factory reset unavailable: %w", err)
	}
	if err := s.crashGuard(factoryResetGuard).Write("firstboot -y && reboot"); err != nil {
		return err
	}
	go func() {
		if err := execx.Run(execx.Package, "firstboot", "-y"); err != nil {
			slog.Error("firstboot failed; device not reset, crash guard kept", "err", err, "guard", s.crashGuard(factoryResetGuard).Path())
			return
		}
		if err := execx.Run(execx.Quick, "reboot"); err != nil {
			slog.Error("reboot after firstboot failed", "err", err)
		}
	}()
	return nil
}

const setupCompleteFlagPath = "/etc/travo/setup-complete"

// GetSetupComplete checks whether the first-run setup has been completed.
func (s *Service) GetSetupComplete() models.SetupStatus {
	_, err := os.Stat(setupCompleteFlagPath)
	return models.SetupStatus{Complete: err == nil}
}

// SetSetupComplete marks the first-run setup as completed by creating the flag file.
func (s *Service) SetSetupComplete() error {
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

const speedTestResultFile = "/tmp/openwrt-speed-test.json"

// RunSpeedTest runs a basic speed test using wget and writes results.
// On constrained hardware this performs a simple HTTP download measurement.
func (s *Service) RunSpeedTest() (models.SpeedTestResult, error) {
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
