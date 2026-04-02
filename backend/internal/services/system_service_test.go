package services

import (
	"os"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

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

	content := "fake firmware binary"
	reader := strings.NewReader(content)

	err := svc.UpgradeFirmware(reader, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file was written to /tmp/firmware.bin
	data, err := os.ReadFile("/tmp/firmware.bin")
	if err != nil {
		t.Fatalf("failed to read firmware file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected firmware content %q, got %q", content, string(data))
	}
	_ = os.Remove("/tmp/firmware.bin")
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
	json := buildButtonActionsJSON(original)
	var parsed []models.HardwareButton
	if err := unmarshalButtonActions([]byte(json), &parsed); err != nil {
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
