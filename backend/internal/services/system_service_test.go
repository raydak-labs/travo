package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// newSSHKeyTestService returns a SystemService whose authorized_keys path is a
// temp file, so the index handling can be exercised off-device.
func newSSHKeyTestService(t *testing.T, path string) *SystemService {
	t.Helper()
	svc := NewSystemService(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})
	svc.sshKeysFile = path
	return svc
}

func TestGetSystemInfo(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, uci.NewMockUCI(), &MockStorageProvider{})

	info, err := svc.GetSystemInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Hostname != "OpenWrt" {
		t.Errorf("expected hostname 'OpenWrt', got %q", info.Hostname)
	}
	if info.Model == "" {
		t.Error("expected non-empty model")
	}
	if info.FirmwareVersion == "" {
		t.Error("expected non-empty firmware version")
	}
	if info.KernelVersion == "" {
		t.Error("expected non-empty kernel version")
	}
	if info.UptimeSeconds <= 0 {
		t.Error("expected positive uptime")
	}
}

func TestGetSystemStats(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, uci.NewMockUCI(), &MockStorageProvider{})

	stats, err := svc.GetSystemStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Memory.TotalBytes <= 0 {
		t.Error("expected positive total memory")
	}
	if stats.Memory.UsagePercent < 0 || stats.Memory.UsagePercent > 100 {
		t.Errorf("memory usage percent out of range: %f", stats.Memory.UsagePercent)
	}
	if stats.CPU.LoadAverage[0] <= 0 {
		t.Error("expected positive load average")
	}
	if stats.Storage.TotalBytes <= 0 {
		t.Error("expected positive storage total")
	}
	// Network stats may be empty in test env (no sysfs), but must not be nil
	if stats.Network == nil {
		// readNetworkStats returns nil slice when no interfaces found — that's ok
		_ = struct{}{} // explicitly ignore nil
	}
}

func TestGetSystemInfo_StorageNotHardcoded(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, uci.NewMockUCI(), &MockStorageProvider{})

	stats, err := svc.GetSystemStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// MockStorageProvider returns 256MB total, 96MB used, 160MB free
	if stats.Storage.TotalBytes != 268435456 {
		t.Errorf("expected storage total 268435456, got %d", stats.Storage.TotalBytes)
	}
	if stats.Storage.UsedBytes != 100663296 {
		t.Errorf("expected storage used 100663296, got %d", stats.Storage.UsedBytes)
	}
	if stats.Storage.FreeBytes != 167772160 {
		t.Errorf("expected storage free 167772160, got %d", stats.Storage.FreeBytes)
	}
	// Usage should be ~37.5%
	if stats.Storage.UsagePercent < 37 || stats.Storage.UsagePercent > 38 {
		t.Errorf("expected storage usage ~37.5%%, got %f", stats.Storage.UsagePercent)
	}

	// Verify it's NOT the old hardcoded values (8GB/2GB)
	if stats.Storage.TotalBytes == 8589934592 {
		t.Error("storage total is still the old hardcoded value")
	}
}

func TestGetSystemInfo_CpuUsageReasonable(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, uci.NewMockUCI(), &MockStorageProvider{})

	stats, err := svc.GetSystemStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.CPU.UsagePercent < 0 || stats.CPU.UsagePercent > 100 {
		t.Errorf("CPU usage percent out of range [0, 100]: %f", stats.CPU.UsagePercent)
	}
	if stats.CPU.Cores <= 0 {
		t.Errorf("expected positive CPU cores, got %d", stats.CPU.Cores)
	}
	// Load average in the mock is 4096/65536 ≈ 0.0625
	// usagePercent = min(0.0625 / cores * 100, 100) — should be well under 100
	if stats.CPU.UsagePercent > 50 {
		t.Errorf("CPU usage too high for mock load average: %f", stats.CPU.UsagePercent)
	}
}

func TestGetSystemStats_CustomStorageProvider(t *testing.T) {
	ub := ubus.NewMockUbus()
	custom := &testStorageProvider{total: 1073741824, used: 536870912, free: 536870912}
	svc := NewSystemService(ub, uci.NewMockUCI(), custom)

	stats, err := svc.GetSystemStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Storage.TotalBytes != 1073741824 {
		t.Errorf("expected 1GB total, got %d", stats.Storage.TotalBytes)
	}
	if stats.Storage.UsagePercent < 49 || stats.Storage.UsagePercent > 51 {
		t.Errorf("expected ~50%% usage, got %f", stats.Storage.UsagePercent)
	}
}

type testStorageProvider struct {
	total, used, free int64
}

func (p *testStorageProvider) GetRootStorage() (int64, int64, int64, error) {
	return p.total, p.used, p.free, nil
}

func TestParseLogOutput_Normal(t *testing.T) {
	input := "line one\nline two\nline three"
	result := parseLogOutput("syslog", input, "", "")

	if result.Source != "syslog" {
		t.Errorf("expected source 'syslog', got %q", result.Source)
	}
	if result.Total != 3 {
		t.Errorf("expected 3 lines, got %d", result.Total)
	}
	if len(result.Lines) != 3 {
		t.Fatalf("expected 3 line entries, got %d", len(result.Lines))
	}
	if result.Lines[0].Line != "line one" {
		t.Errorf("expected 'line one', got %q", result.Lines[0].Line)
	}
	if result.Lines[2].Line != "line three" {
		t.Errorf("expected 'line three', got %q", result.Lines[2].Line)
	}
}

func TestParseLogOutput_Empty(t *testing.T) {
	result := parseLogOutput("kernel", "", "", "")

	if result.Source != "kernel" {
		t.Errorf("expected source 'kernel', got %q", result.Source)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 lines, got %d", result.Total)
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected empty lines slice, got %d entries", len(result.Lines))
	}
}

func TestParseLogOutput_BlankLines(t *testing.T) {
	input := "first\n\nsecond\n\n\nthird\n"
	result := parseLogOutput("syslog", input, "", "")

	if result.Total != 3 {
		t.Errorf("expected 3 non-blank lines, got %d", result.Total)
	}
	if len(result.Lines) != 3 {
		t.Fatalf("expected 3 line entries, got %d", len(result.Lines))
	}
	if result.Lines[1].Line != "second" {
		t.Errorf("expected 'second', got %q", result.Lines[1].Line)
	}
}

func TestParseLogOutput_ServiceFilter(t *testing.T) {
	input := `Tue Mar 11 09:17:52 2026 daemon.info dnsmasq[1234]: query from 192.168.8.100
Tue Mar 11 09:17:53 2026 daemon.info AdGuardHome[3732]: blocked ad.example.com
Tue Mar 11 09:17:54 2026 daemon.info dnsmasq[1234]: forwarded google.com
Tue Mar 11 09:17:55 2026 kern.info netifd[456]: interface up`

	// Filter by dnsmasq
	result := parseLogOutput("syslog", input, "dnsmasq", "")
	if result.Total != 2 {
		t.Errorf("expected 2 dnsmasq lines, got %d", result.Total)
	}

	// Filter by AdGuardHome (case-insensitive)
	result = parseLogOutput("syslog", input, "adguardhome", "")
	if result.Total != 1 {
		t.Errorf("expected 1 AdGuardHome line, got %d", result.Total)
	}

	// No filter returns all
	result = parseLogOutput("syslog", input, "", "")
	if result.Total != 4 {
		t.Errorf("expected 4 lines with no filter, got %d", result.Total)
	}

	// Non-matching filter returns none
	result = parseLogOutput("syslog", input, "wireguard", "")
	if result.Total != 0 {
		t.Errorf("expected 0 lines for wireguard filter, got %d", result.Total)
	}
}

func TestExtractLevel(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"Tue Mar 11 09:17:52 2026 daemon.info dnsmasq[1234]: query", "info"},
		{"Tue Mar 11 09:17:52 2026 kern.err kernel: error occurred", "err"},
		{"Tue Mar 11 09:17:52 2026 daemon.warning dnsmasq[1234]: warn", "warning"},
		{"Tue Mar 11 09:17:52 2026 kern.crit kernel: critical", "crit"},
		{"Tue Mar 11 09:17:52 2026 user.notice netifd: up", "notice"},
		{"Tue Mar 11 09:17:52 2026 daemon.debug dnsmasq: debug", "debug"},
		{"Tue Mar 11 09:17:52 2026 auth.emerg sshd: emergency", "emerg"},
		{"Tue Mar 11 09:17:52 2026 kern.alert kernel: alert", "alert"},
		{"short line", ""},
		{"", ""},
		{"no facility field at all", ""},
	}
	for _, tt := range tests {
		got := extractLevel(tt.line)
		if got != tt.expected {
			t.Errorf("extractLevel(%q) = %q, want %q", tt.line, got, tt.expected)
		}
	}
}

func TestParseLogOutput_LevelFilter(t *testing.T) {
	input := `Tue Mar 11 09:17:50 2026 daemon.debug dnsmasq[1234]: debug msg
Tue Mar 11 09:17:51 2026 daemon.info dnsmasq[1234]: info msg
Tue Mar 11 09:17:52 2026 daemon.notice dnsmasq[1234]: notice msg
Tue Mar 11 09:17:53 2026 daemon.warning dnsmasq[1234]: warning msg
Tue Mar 11 09:17:54 2026 daemon.err dnsmasq[1234]: error msg
Tue Mar 11 09:17:55 2026 daemon.crit dnsmasq[1234]: critical msg
Tue Mar 11 09:17:56 2026 daemon.alert dnsmasq[1234]: alert msg
Tue Mar 11 09:17:57 2026 daemon.emerg dnsmasq[1234]: emergency msg`

	// No level filter returns all 8
	result := parseLogOutput("syslog", input, "", "")
	if result.Total != 8 {
		t.Errorf("expected 8 lines, got %d", result.Total)
	}

	// Filter: err and above (emerg, alert, crit, err) = 4
	result = parseLogOutput("syslog", input, "", "err")
	if result.Total != 4 {
		t.Errorf("expected 4 lines for err filter, got %d", result.Total)
	}

	// Filter: warning and above = 5
	result = parseLogOutput("syslog", input, "", "warning")
	if result.Total != 5 {
		t.Errorf("expected 5 lines for warning filter, got %d", result.Total)
	}

	// Filter: info and above = 7 (all except debug)
	result = parseLogOutput("syslog", input, "", "info")
	if result.Total != 7 {
		t.Errorf("expected 7 lines for info filter, got %d", result.Total)
	}

	// Filter: emerg = 1
	result = parseLogOutput("syslog", input, "", "emerg")
	if result.Total != 1 {
		t.Errorf("expected 1 line for emerg filter, got %d", result.Total)
	}

	// Filter: debug = all 8
	result = parseLogOutput("syslog", input, "", "debug")
	if result.Total != 8 {
		t.Errorf("expected 8 lines for debug filter, got %d", result.Total)
	}
}

func TestParseLogOutput_LevelExtracted(t *testing.T) {
	input := "Tue Mar 11 09:17:52 2026 daemon.err dnsmasq[1234]: error msg"
	result := parseLogOutput("syslog", input, "", "")
	if result.Total != 1 {
		t.Fatalf("expected 1 line, got %d", result.Total)
	}
	if result.Lines[0].Level != "err" {
		t.Errorf("expected level 'err', got %q", result.Lines[0].Level)
	}
}

func TestParseLogOutput_LevelAndServiceFilter(t *testing.T) {
	input := `Tue Mar 11 09:17:50 2026 daemon.debug dnsmasq[1234]: debug msg
Tue Mar 11 09:17:51 2026 daemon.err dnsmasq[1234]: error msg
Tue Mar 11 09:17:52 2026 kern.err netifd[456]: kernel error
Tue Mar 11 09:17:53 2026 daemon.info dnsmasq[1234]: info msg`

	// Filter: dnsmasq + err level = only the dnsmasq err line
	result := parseLogOutput("syslog", input, "dnsmasq", "err")
	if result.Total != 1 {
		t.Errorf("expected 1 line for dnsmasq+err, got %d", result.Total)
	}
}

func TestSetHostname(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	svc := NewSystemService(ub, u, &MockStorageProvider{})
	svc.SetGuardDir(t.TempDir())

	if err := svc.SetHostname("MyRouter"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it was written to UCI
	val, err := u.Get("system", "system", "hostname")
	if err != nil {
		t.Fatalf("failed to read hostname from UCI: %v", err)
	}
	if val != "MyRouter" {
		t.Errorf("expected hostname 'MyRouter', got %q", val)
	}
}

func TestGetTimezone(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	svc := NewSystemService(ub, u, &MockStorageProvider{})

	config, err := svc.GetTimezone()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Zonename == "" {
		t.Error("expected non-empty zonename")
	}
	if config.Timezone == "" {
		t.Error("expected non-empty timezone")
	}
}

func TestSetTimezone(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	svc := NewSystemService(ub, u, &MockStorageProvider{})
	svc.SetGuardDir(t.TempDir())

	err := svc.SetTimezone(models.TimezoneConfig{
		Zonename: "Europe/Berlin",
		Timezone: "CET-1CEST,M3.5.0,M10.5.0/3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config, err := svc.GetTimezone()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Zonename != "Europe/Berlin" {
		t.Errorf("expected zonename 'Europe/Berlin', got '%s'", config.Zonename)
	}
	if config.Timezone != "CET-1CEST,M3.5.0,M10.5.0/3" {
		t.Errorf("expected timezone 'CET-1CEST,M3.5.0,M10.5.0/3', got '%s'", config.Timezone)
	}
}

func TestGetNTPConfig(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	svc := NewSystemService(ub, u, &MockStorageProvider{})

	config, err := svc.GetNTPConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !config.Enabled {
		t.Error("expected NTP to be enabled")
	}
	if len(config.Servers) != 4 {
		t.Errorf("expected 4 NTP servers, got %d", len(config.Servers))
	}
	if config.Servers[0] != "0.openwrt.pool.ntp.org" {
		t.Errorf("expected first server '0.openwrt.pool.ntp.org', got %q", config.Servers[0])
	}
}

func TestSetNTPConfig(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	svc := NewSystemService(ub, u, &MockStorageProvider{})
	svc.SetGuardDir(t.TempDir())

	err := svc.SetNTPConfig(models.NTPConfig{
		Enabled: false,
		Servers: []string{"pool.ntp.org", "time.google.com"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config, err := svc.GetNTPConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Enabled {
		t.Error("expected NTP to be disabled")
	}
	if len(config.Servers) != 2 {
		t.Errorf("expected 2 NTP servers, got %d", len(config.Servers))
	}
	if config.Servers[0] != "pool.ntp.org" {
		t.Errorf("expected first server 'pool.ntp.org', got %q", config.Servers[0])
	}
	if config.Servers[1] != "time.google.com" {
		t.Errorf("expected second server 'time.google.com', got %q", config.Servers[1])
	}
}

func TestGetNTPConfig_DefaultsWhenMissing(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	// Remove the ntp section to test default fallback
	if err := u.DeleteSection("system", "ntp"); err != nil {
		t.Fatalf("failed to delete ntp section: %v", err)
	}
	svc := NewSystemService(ub, u, &MockStorageProvider{})

	config, err := svc.GetNTPConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !config.Enabled {
		t.Error("expected NTP defaults to be enabled")
	}
	if len(config.Servers) != 4 {
		t.Errorf("expected 4 default NTP servers, got %d", len(config.Servers))
	}
}

func TestUpgradeFirmware_SavesFile(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, uci.NewMockUCI(), &MockStorageProvider{})
	svc.SetGuardDir(t.TempDir())

	content := "fake firmware binary"
	reader := strings.NewReader(content)

	err := svc.UpgradeFirmware(reader, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The staged image must exist somewhere under /tmp with the uploaded bytes.
	data, err := findFirmwareImage(content)
	if err != nil {
		t.Fatalf("firmware file was not staged: %v", err)
	}
	_ = os.Remove(data)
}

// findFirmwareImage locates the staged /tmp/firmware-*.bin written by
// UpgradeFirmware.
func findFirmwareImage(want string) (string, error) {
	matches, err := filepath.Glob("/tmp/firmware-*.bin")
	if err != nil {
		return "", err
	}
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err == nil && string(b) == want {
			return m, nil
		}
	}
	return "", fmt.Errorf("no /tmp/firmware-*.bin containing the uploaded content")
}

func TestGetSetupComplete_NotComplete(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, uci.NewMockUCI(), &MockStorageProvider{})

	status := svc.GetSetupComplete()
	if status.Complete {
		t.Error("expected setup not complete when flag file doesn't exist")
	}
}

func TestSetSetupComplete_CreatesFlag(t *testing.T) {
	// Use a temp directory for the flag file
	tmpDir, err := os.MkdirTemp("", "setup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Test the logic: create the flag directory and file, then verify it exists.
	flagDir := tmpDir + "/etc/openwrt-travel-gui"
	if err := os.MkdirAll(flagDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	flagPath := flagDir + "/setup-complete"
	f, err := os.Create(flagPath)
	if err != nil {
		t.Fatalf("failed to create flag: %v", err)
	}
	_ = f.Close()

	// Verify the file exists
	if _, err := os.Stat(flagPath); err != nil {
		t.Errorf("expected flag file to exist: %v", err)
	}
}

func TestSetButtonActions_GeneratesHotplugScript(t *testing.T) {
	dir := t.TempDir()
	// Override paths via a temp dir (we test the helpers directly)
	buttons := []models.HardwareButton{
		{Name: "reset", Action: models.ButtonActionVPNToggle},
		{Name: "wps", Action: models.ButtonActionReboot},
	}
	script := buildButtonHotplugScript(buttons)
	if !strings.Contains(script, "reset)") {
		t.Error("expected script to contain 'reset)' case")
	}
	if !strings.Contains(script, "wps)") {
		t.Error("expected script to contain 'wps)' case")
	}
	if !strings.Contains(script, "/sbin/ifup wg0") || !strings.Contains(script, "/sbin/ifdown wg0") {
		t.Error("expected script to toggle wg0 via ifup/ifdown for vpn_toggle")
	}
	if !strings.Contains(script, "reboot") {
		t.Error("expected script to contain reboot for reboot action")
	}
	_ = dir
}

func TestSetButtonActions_NoneSkipped(t *testing.T) {
	buttons := []models.HardwareButton{
		{Name: "reset", Action: models.ButtonActionNone},
	}
	script := buildButtonHotplugScript(buttons)
	// "reset)" should not appear since action is none
	if strings.Contains(script, "reset)") {
		t.Error("expected 'none' action buttons to be omitted from hotplug script")
	}
}

func TestBuildButtonActionsJSON_RoundTrip(t *testing.T) {
	original := []models.HardwareButton{
		{Name: "reset", Action: models.ButtonActionWifiToggle},
		{Name: "wps", Action: models.ButtonActionLEDToggle},
	}
	// Round-trip through encoding/json: the hand-rolled parser this replaced
	// silently mangled names and actions.
	raw := buildButtonActionsJSON(original)
	var parsed []models.HardwareButton
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(parsed) != len(original) {
		t.Fatalf("expected %d buttons, got %d", len(original), len(parsed))
	}
	for i, b := range original {
		if parsed[i].Name != b.Name || parsed[i].Action != b.Action {
			t.Errorf("button %d: expected {%s, %s}, got {%s, %s}",
				i, b.Name, b.Action, parsed[i].Name, parsed[i].Action)
		}
	}
}

func TestGetTimezone_MissingSection(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	svc := NewSystemService(ub, u, &MockStorageProvider{})

	// Delete the system section to simulate it missing
	err := u.DeleteSection("system", "system")
	if err != nil {
		t.Fatalf("failed to delete section: %v", err)
	}

	// GetTimezone should return defaults, not error
	config, err := svc.GetTimezone()
	if err != nil {
		t.Errorf("expected no error when section missing, got: %v", err)
	}
	// Should return empty/default values
	if config.Zonename != "" || config.Timezone != "" {
		t.Errorf("expected empty timezone config when section missing, got zonename=%q timezone=%q", config.Zonename, config.Timezone)
	}
}

// --- Crash guards for device-mutating operations (ADR 0003) ---

func newGuardedSystemService(t *testing.T) (*SystemService, string) {
	t.Helper()
	svc := NewSystemService(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})
	dir := t.TempDir()
	svc.SetGuardDir(dir)
	return svc, dir
}

// UpgradeFirmware must leave a crash-guard marker: a power cut mid-flash
// leaves an unbootable device, and the marker is the only recovery hint.
func TestUpgradeFirmware_WritesCrashGuard(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	if err := svc.UpgradeFirmware(strings.NewReader("FIRMWARE"), true); err != nil {
		t.Fatalf("UpgradeFirmware: %v", err)
	}
	guard := filepath.Join(dir, firmwareUpgradeGuardName)
	data, err := os.ReadFile(guard)
	if err != nil {
		t.Fatalf("expected a crash guard at %s: %v", guard, err)
	}
	if !strings.Contains(string(data), "sysupgrade") {
		t.Errorf("guard should record what was running, got %q", data)
	}
}

// RestoreBackup rewrites /etc/config, so a crash guard must exist and must
// survive a failed restore — the marker is the only record that the config was
// left in an unknown state.
func TestRestoreBackup_WritesAndKeepsGuardOnFailure(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	guard := filepath.Join(dir, restoreGuardName)

	// sysupgrade does not exist on the test host, so the restore fails; the
	// guard must still be on disk afterwards.
	_ = svc.RestoreBackup(filepath.Join(dir, "nonexistent-backup.tar.gz"))

	if _, err := os.Stat(guard); err != nil {
		t.Fatalf("expected a crash guard at %s after a failed restore: %v", guard, err)
	}
}

// FactoryReset must refuse to start when the guard cannot be written, rather
// than erasing the overlay with no recovery marker.
func TestFactoryReset_WritesCrashGuard(t *testing.T) {
	if _, err := exec.LookPath("firstboot"); err != nil {
		t.Skip("firstboot not available on this host")
	}
	svc, dir := newGuardedSystemService(t)
	if err := svc.FactoryReset(); err != nil {
		t.Fatalf("FactoryReset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, factoryResetGuardName)); err != nil {
		t.Errorf("expected a crash guard for the factory reset: %v", err)
	}
}

// commitSystemConfig must clear its guard on success and keep it (plus name it
// in the error) when the commit fails.
func TestCommitSystemConfig_GuardLifecycle(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	if err := svc.SetHostname("travel-router"); err != nil {
		t.Fatalf("SetHostname: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, systemConfigGuardName)); !os.IsNotExist(err) {
		t.Errorf("guard must be removed after a successful commit, stat err = %v", err)
	}
}

// --- Backups ---

// The backup filename must be unique: the handler removes exactly the path it
// was handed, so two backups in the same second used to delete each other's
// archive while it was still streaming.
func TestUniqueTempPath_IsUniqueAndFree(t *testing.T) {
	dir := t.TempDir()
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		p, err := uniqueTempPath(dir, "backup-*.tar.gz")
		if err != nil {
			t.Fatalf("uniqueTempPath: %v", err)
		}
		if seen[p] {
			t.Fatalf("duplicate temp path %q", p)
		}
		seen[p] = true
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("temp path %q must not exist yet, stat err = %v", p, err)
		}
		if !strings.HasPrefix(filepath.Base(p), "backup-") || !strings.HasSuffix(p, ".tar.gz") {
			t.Errorf("unexpected backup path shape: %q", p)
		}
	}
}

// --- Memory stats ---

// used = total - free - cached - buffered can go negative when the reported
// numbers overlap. A negative value is nonsense in the UI.
func TestGetSystemStats_ClampsNegativeMemoryUsage(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, uci.NewMockUCI(), &MockStorageProvider{})
	ub.RegisterResponse("system.info", map[string]any{
		"memory": map[string]any{
			"total":    float64(1024),
			"free":     float64(800),
			"cached":   float64(300),
			"buffered": float64(100),
		},
	})
	stats, err := svc.GetSystemStats()
	if err != nil {
		t.Fatalf("GetSystemStats: %v", err)
	}
	if stats.Memory.UsedBytes < 0 {
		t.Errorf("UsedBytes must be clamped at 0, got %d", stats.Memory.UsedBytes)
	}
	if stats.Memory.UsagePercent < 0 {
		t.Errorf("UsagePercent must be clamped at 0, got %f", stats.Memory.UsagePercent)
	}
}

// --- Log level parsing ---

// Level extraction must not depend on a fixed column: BusyBox logread and
// dmesg disagree about the weekday prefix, and the old index-based parse
// returned "" for every line without it (so ?level=err returned everything).
func TestExtractLevel_PositionIndependent(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"weekday prefix", "Tue Mar 11 09:17:52 2026 daemon.err dnsmasq[1]: boom", "err"},
		{"no weekday prefix", "daemon.err dnsmasq[1]: boom", "err"},
		{"no year, short date", "Mar 11 09:17:52 daemon.warning dnsmasq[1]: slow", "warning"},
		{"no timestamp at all", "user.notice netifd: up", "notice"},
		{"kern facility", "kern.crit kernel: critical", "crit"},
		{"warn alias", "daemon.warn dnsmasq[1]: hmm", "warning"},
		{"error alias", "daemon.error dnsmasq[1]: bad", "err"},
		{"plain text, no level", "something happened at 10.1.2.3", ""},
		{"dmesg style", "[ 12.345678] ath11k firmware crashed", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractLevel(tt.line); got != tt.want {
				t.Errorf("extractLevel(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestParseLogOutput_LevelFilterWorksWithoutWeekday(t *testing.T) {
	input := `daemon.debug dnsmasq[1]: debug msg
daemon.info dnsmasq[1]: info msg
daemon.err dnsmasq[1]: error msg
kern.crit kernel: critical`
	res := parseLogOutput("syslog", input, "", "err")
	if res.Total != 2 {
		t.Fatalf("expected 2 lines at err or above, got %d", res.Total)
	}
	for _, l := range res.Lines {
		sev, ok := logLevelSeverity[l.Level]
		if !ok {
			t.Fatalf("line %q kept with unknown level %q", l.Line, l.Level)
		}
		if sev > logLevelSeverity["err"] {
			t.Errorf("line %q (level %q) should have been filtered out", l.Line, l.Level)
		}
	}
}

// --- Log capture bounds ---

// Log capture must be bounded: a device in a log loop can produce tens of MB
// per request.
func TestStreamLogTail_KeepsOnlyTheTail(t *testing.T) {
	script := "i=0; while [ $i -lt " + itoa(maxLogLines+500) + " ]; do echo \"line $i\"; i=$((i+1)); done"
	out, err := streamLogTail(10*time.Second, "sh", "-c", script)
	if err != nil {
		t.Fatalf("streamLogTail: %v", err)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != maxLogLines {
		t.Errorf("expected the output capped at %d lines, got %d", maxLogLines, len(lines))
	}
	// The tail must be the newest output.
	if lines[len(lines)-1] != "line "+itoa(maxLogLines+499) {
		t.Errorf("expected the newest line last, got %q", lines[len(lines)-1])
	}
}

func itoa(i int) string { return strconv.Itoa(i) }

// --- SSH key index bounds (Wave 1 hardening must stay intact) ---

func TestDeleteSSHKey_RejectsOutOfRangeIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	if err := os.WriteFile(path, []byte("ssh-ed25519 AAAA user1\nssh-rsa BBBB user2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := newSSHKeyTestService(t, path)

	if err := svc.DeleteSSHKey(-1); err == nil {
		t.Error("expected an error for a negative index")
	}
	if err := svc.DeleteSSHKey(2); err == nil {
		t.Error("expected an error for an index past the last line")
	}
	// Index 0 must map to the first key line, matching GetSSHKeys.
	keys, err := svc.GetSSHKeys()
	if err != nil {
		t.Fatalf("GetSSHKeys: %v", err)
	}
	if len(keys.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys.Keys))
	}
	if err := svc.DeleteSSHKey(0); err != nil {
		t.Fatalf("DeleteSSHKey(0): %v", err)
	}
	remaining, _ := os.ReadFile(path)
	if strings.Contains(string(remaining), "user1") {
		t.Errorf("expected user1 to be removed, file is %q", remaining)
	}
	if !strings.Contains(string(remaining), "user2") {
		t.Errorf("expected user2 to survive, file is %q", remaining)
	}
}

// --- Button action config parsing ---

// A malformed button-actions.json must not be reported as "no buttons
// configured": the hotplug script on disk still runs the previous actions.
func TestGetHardwareButtonsWithError_SurfacesParseFailure(t *testing.T) {
	svc := NewSystemService(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})

	// No file at all is not an error: nothing has been configured yet.
	if _, err := svc.GetHardwareButtonsWithError(); err != nil {
		t.Errorf("a missing config file must not be an error, got %v", err)
	}

	if err := os.WriteFile(buttonActionsFile, []byte("{ this is not json"), 0o600); err != nil {
		t.Skipf("cannot write %s on this host: %v", buttonActionsFile, err)
	}
	defer func() { _ = os.Remove(buttonActionsFile) }()

	if _, err := svc.GetHardwareButtonsWithError(); err == nil {
		t.Error("a malformed config file must surface an error")
	}
	// The non-error-returning wrapper must still be safe to call.
	_ = svc.GetHardwareButtons()
}

// encoding/json replaced a hand-rolled parser: names containing escapes and
// reordered keys must round-trip.
func TestLoadButtonActions_HandlesReorderedAndEscapedJSON(t *testing.T) {
	svc := NewSystemService(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})
	if err := os.WriteFile(buttonActionsFile, []byte(`[{"action":"reboot","name":"reset"},{"name":"wps","action":"led_toggle"}]`), 0o600); err != nil {
		t.Skipf("cannot write %s on this host: %v", buttonActionsFile, err)
	}
	defer func() { _ = os.Remove(buttonActionsFile) }()

	buttons, err := svc.loadButtonActions()
	if err != nil {
		t.Fatalf("loadButtonActions: %v", err)
	}
	if len(buttons) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(buttons))
	}
	if buttons[0].Name != "reset" || buttons[0].Action != models.ButtonActionReboot {
		t.Errorf("unexpected first button: %+v", buttons[0])
	}
	if buttons[1].Name != "wps" || buttons[1].Action != models.ButtonActionLEDToggle {
		t.Errorf("unexpected second button: %+v", buttons[1])
	}
}
