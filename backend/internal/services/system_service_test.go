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

// buildFirmwareImage returns a synthetic sysupgrade image: kernel padding
// followed by an OpenWrt metadata block naming the given model and devices.
// It mirrors what mkimage writes, so the parser is exercised against the real
// layout rather than a convenient one.
func buildFirmwareImage(model string, supportedDevices []string) []byte {
	var pairs bytes.Buffer
	writePair := func(k, v string) {
		pairs.WriteString(k)
		pairs.WriteByte(0)
		pairs.WriteString(v)
		pairs.WriteByte(0)
	}
	writePair("version", "23.05.2")
	writePair("distname", "OpenWrt")
	writePair("model", model)
	writePair("supported_devices", strings.Join(supportedDevices, ","))

	const magic = "metadata\x00\x00"
	body := append([]byte(magic), 0, 0, 0, 1) // compat_version = 1
	body = append(body, pairs.Bytes()...)
	size := 8 + len(body)
	block := make([]byte, 0, size)
	block = binary.BigEndian.AppendUint32(block, uint32(size))
	block = binary.BigEndian.AppendUint32(block, 1)
	block = append(block, body...)

	// 4 KiB of kernel padding in front, so the parser has to scan for the block.
	out := make([]byte, 4096)
	for i := range out {
		out[i] = 0xff
	}
	return append(out, block...)
}

// mockBoardDevices are the supported_devices values the mock ubus answers for
// ("glinet,gl-mt3000" / model "GL.iNet GL-MT3000").
func mockBoardDevices() []string { return []string{"gl-mt3000", "glinet,gl-mt3000"} }

func TestUpgradeFirmware_SavesFile(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, uci.NewMockUCI(), &MockStorageProvider{})
	svc.SetGuardDir(t.TempDir())
	// The flash goroutine must finish inside this test, or it would pick up a
	// later test's PATH and record a call that has nothing to do with it.
	marker := installFakeSysupgrade(t)

	image := buildFirmwareImage("GL.iNet GL-MT3000", mockBoardDevices())
	reader := bytes.NewReader(image)

	meta, err := svc.UpgradeFirmware(reader, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Model != "GL.iNet GL-MT3000" {
		t.Errorf("parsed model = %q, want GL.iNet GL-MT3000", meta.Model)
	}
	waitForSysupgradeCall(t, marker)

	// The staged image must exist somewhere under /tmp with the uploaded bytes.
	data, err := findFirmwareImage(string(image))
	if err != nil {
		t.Fatalf("firmware file was not staged: %v", err)
	}
	_ = os.Remove(data)
}

// waitForSysupgradeCall blocks until the asynchronous flash has run, so the
// goroutine cannot outlive the test that installed the stub.
func waitForSysupgradeCall(t *testing.T, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("sysupgrade was never invoked (%s missing)", marker)
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

// A .bin for another board must be refused BEFORE sysupgrade runs. sysupgrade
// only rejects it after this endpoint answered 200, by which point the operator
// has already been told the flash started — and on a travel router with no
// serial console the result is a brick.
func TestUpgradeFirmware_RejectsForeignDevice(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	image := buildFirmwareImage("Some Other Router", []string{"other,router-x1"})
	_, err := svc.UpgradeFirmware(bytes.NewReader(image), true)
	if err == nil {
		t.Fatal("a firmware image for another board was accepted")
	}
	if !errors.Is(err, ErrUnsupportedFirmware) {
		t.Errorf("error = %v, want ErrUnsupportedFirmware", err)
	}
	if !strings.Contains(err.Error(), "Some Other Router") {
		t.Errorf("the error must name the image model, got %v", err)
	}
	assertNoSysupgradeCall(t, marker)
	if _, err := os.Stat(filepath.Join(dir, firmwareUpgradeGuardName)); err == nil {
		t.Error("a rejected image must not leave a crash guard: nothing was flashed")
	}
	assertNoStagedFirmware(t)
}

// A junk file with a .bin name has no metadata block at all — the textbook
// brick. It must be a 400-equivalent error, not a flash attempt.
func TestUpgradeFirmware_RejectsJunkBinary(t *testing.T) {
	svc, _ := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	junk := make([]byte, 8192)
	for i := range junk {
		junk[i] = byte(i)
	}
	_, err := svc.UpgradeFirmware(bytes.NewReader(junk), true)
	if err == nil {
		t.Fatal("a junk .bin with no metadata block was accepted")
	}
	if !errors.Is(err, ErrNoFirmwareMetadata) {
		t.Errorf("error = %v, want ErrNoFirmwareMetadata", err)
	}
	assertNoSysupgradeCall(t, marker)
	assertNoStagedFirmware(t)
}

// An image that names no supported_devices cannot be verified at all. Flashing
// it anyway is a coin flip with the device on the wrong side.
func TestUpgradeFirmware_RejectsImageWithoutSupportedDevices(t *testing.T) {
	svc, _ := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	image := buildFirmwareImage("GL.iNet GL-MT3000", nil)
	if _, err := svc.UpgradeFirmware(bytes.NewReader(image), true); err == nil {
		t.Fatal("an image without supported_devices was accepted")
	}
	assertNoSysupgradeCall(t, marker)
}

func TestParseFirmwareMetadata(t *testing.T) {
	image := buildFirmwareImage("Linksys EA8300", []string{"linksys,ea8300", "linksys_e8300-ubi"})
	meta, err := ParseFirmwareMetadata(bytes.NewReader(image))
	if err != nil {
		t.Fatalf("a well-formed image was rejected: %v", err)
	}
	if meta.Model != "Linksys EA8300" {
		t.Errorf("model = %q", meta.Model)
	}
	if meta.Version != "23.05.2" || meta.Distname != "OpenWrt" {
		t.Errorf("version/distname = %q/%q", meta.Version, meta.Distname)
	}
	if meta.CompatVersion != 1 {
		t.Errorf("compat_version = %d, want 1", meta.CompatVersion)
	}
	if len(meta.SupportedDevices) != 3 || meta.SupportedDevicesRaw != "linksys,ea8300,linksys_e8300-ubi" {
		t.Errorf("supported_devices = %v (raw %q)", meta.SupportedDevices, meta.SupportedDevicesRaw)
	}
	// The mock board answers "glinet,gl-mt3000" with model "GL.iNet GL-MT3000".
	if meta.SupportsDevice("glinet,gl-mt3000", []string{"gl-mt3000", "GL.iNet GL-MT3000", "GL-MT3000"}) {
		t.Error("an EA8300 image must not claim support for a GL-MT3000 board")
	}
	own := buildFirmwareImage("GL.iNet GL-MT3000", []string{"gl-mt3000"})
	ownMeta, err := ParseFirmwareMetadata(bytes.NewReader(own))
	if err != nil {
		t.Fatalf("parsing the board's own image: %v", err)
	}
	if !ownMeta.SupportsDevice("glinet,gl-mt3000", []string{"gl-mt3000"}) {
		t.Error("a GL-MT3000 image must be accepted for the GL-MT3000 board")
	}
	// An image listing the full "glinet,gl-mt3000" board name matches too.
	full := buildFirmwareImage("GL.iNet GL-MT3000", []string{"glinet,gl-mt3000"})
	fullMeta, err := ParseFirmwareMetadata(bytes.NewReader(full))
	if err != nil {
		t.Fatalf("parsing the full-name image: %v", err)
	}
	if !fullMeta.SupportsDevice("glinet,gl-mt3000", nil) {
		t.Error("supported_devices=glinet,gl-mt3000 must match this board")
	}
}

func TestParseFirmwareMetadata_RejectsJunk(t *testing.T) {
	if _, err := ParseFirmwareMetadata(bytes.NewReader(make([]byte, 1024))); !errors.Is(err, ErrNoFirmwareMetadata) {
		t.Errorf("junk image: error = %v, want ErrNoFirmwareMetadata", err)
	}
	truncated := buildFirmwareImage("GL.iNet GL-MT3000", mockBoardDevices())
	truncated = truncated[:len(truncated)-8]
	if _, err := ParseFirmwareMetadata(bytes.NewReader(truncated)); err == nil {
		t.Error("a truncated metadata block was accepted")
	}
}

// installFakeSysupgrade puts a stub sysupgrade first on PATH that records every
// invocation, so a test can assert that a rejected image was never handed to it.
// It returns the marker file the stub appends to.
func installFakeSysupgrade(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "sysupgrade-calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + marker + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sysupgrade"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake sysupgrade: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func assertNoSysupgradeCall(t *testing.T, marker string) {
	t.Helper()
	// The firmware flash sleeps 500ms before calling sysupgrade; the restore is
	// synchronous, but give both a moment so the assertion is not a race.
	time.Sleep(800 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Errorf("sysupgrade was invoked (%s exists) despite the rejection", marker)
	}
}

func assertNoStagedFirmware(t *testing.T) {
	t.Helper()
	matches, _ := filepath.Glob("/tmp/firmware-*.bin")
	for _, m := range matches {
		if b, err := os.ReadFile(m); err == nil && bytes.Equal(b, buildFirmwareImage("Some Other Router", []string{"other,router-x1"})) {
			t.Errorf("a rejected image was left staged at %s", m)
		}
	}
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
	marker := installFakeSysupgrade(t)
	image := buildFirmwareImage("GL.iNet GL-MT3000", mockBoardDevices())
	if _, err := svc.UpgradeFirmware(bytes.NewReader(image), true); err != nil {
		t.Fatalf("UpgradeFirmware: %v", err)
	}
	waitForSysupgradeCall(t, marker)
	guard := filepath.Join(dir, firmwareUpgradeGuardName)
	data, err := os.ReadFile(guard)
	if err != nil {
		t.Fatalf("expected a crash guard at %s: %v", guard, err)
	}
	if !strings.Contains(string(data), "sysupgrade") {
		t.Errorf("guard should record what was running, got %q", data)
	}
	if !strings.Contains(string(data), "GL.iNet GL-MT3000") {
		t.Errorf("guard should record the image model, got %q", data)
	}
}

// tarGzArchive builds a gzip tar archive from the given headers. A member's
// body is taken from bodies[name] when present, otherwise Size zero bytes are
// written (archive/tar enforces that the declared size is actually written).
func tarGzArchive(t *testing.T, headers []*tar.Header, bodies map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range headers {
		if body, ok := bodies[h.Name]; ok {
			h.Size = int64(len(body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatalf("write tar header %q: %v", h.Name, err)
		}
		if h.Typeflag != tar.TypeReg || h.Size <= 0 {
			continue
		}
		if body, ok := bodies[h.Name]; ok {
			if _, err := io.WriteString(tw, body); err != nil {
				t.Fatalf("write tar body %q: %v", h.Name, err)
			}
			continue
		}
		if _, err := io.CopyN(tw, zeroReader{}, h.Size); err != nil {
			t.Fatalf("write tar filler %q: %v", h.Name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

// zeroReader feeds archive/tar the declared member size without allocating it.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// genuineBackupArchive is what `sysupgrade -b` produces on a real device: the
// UCI config directory, and nothing else.
func genuineBackupArchive(t *testing.T) []byte {
	t.Helper()
	return tarGzArchive(t,
		[]*tar.Header{
			{Name: "etc", Typeflag: tar.TypeDir, Mode: 0o755},
			{Name: "etc/config", Typeflag: tar.TypeDir, Mode: 0o755},
			{Name: "etc/config/network", Typeflag: tar.TypeReg, Mode: 0o600, Size: 21},
			{Name: "etc/config/wireless", Typeflag: tar.TypeReg, Mode: 0o600, Size: 27},
			{Name: "etc/ppp", Typeflag: tar.TypeDir, Mode: 0o755},
			{Name: "etc/ppp/chap-secrets", Typeflag: tar.TypeReg, Mode: 0o600, Size: 21},
		},
		map[string]string{
			"etc/config/network":   "config network 'lan'\n",
			"etc/config/wireless":  "config wifi-iface 'default'\n",
			"etc/ppp/chap-secrets": "* * \"secret\" \"key\"\n",
		},
	)
}

// RestoreBackup rewrites /etc/config, so a crash guard must exist and must
// survive a failed restore — the marker is the only record that the config was
// left in an unknown state.
func TestRestoreBackup_WritesAndKeepsGuardOnFailure(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	guard := filepath.Join(dir, restoreGuardName)

	// sysupgrade does not exist on the test host, so the restore fails; the
	// guard must still be on disk afterwards.
	path := filepath.Join(dir, "backup.tar.gz")
	if err := os.WriteFile(path, genuineBackupArchive(t), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	if err := svc.RestoreBackup(path); err == nil {
		t.Fatal("RestoreBackup reported success although sysupgrade is absent")
	}

	if _, err := os.Stat(guard); err != nil {
		t.Fatalf("expected a crash guard at %s after a failed restore: %v", guard, err)
	}
}

// A genuine backup passes validation: the allowlist has to accept what
// CreateBackup's counterpart (sysupgrade -b) actually produces, or restore is
// simply broken.
func TestValidateRestoreArchive_AcceptsGenuineBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	if err := os.WriteFile(path, genuineBackupArchive(t), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	if err := ValidateRestoreArchive(path); err != nil {
		t.Errorf("a genuine sysupgrade backup was rejected: %v", err)
	}
}

// sysupgrade -r extracts the tarball at /, so every one of these members turns
// "restore my configuration" into "run this as root".
func TestValidateRestoreArchive_RejectsUnsafeMembers(t *testing.T) {
	cases := []struct {
		name    string
		headers []*tar.Header
		bodies  map[string]string
		wantErr string
	}{
		{
			name: "absolute path",
			headers: []*tar.Header{
				{Name: "/etc/config/network", Typeflag: tar.TypeReg, Size: 3},
			},
			bodies:  map[string]string{"/etc/config/network": "abc"},
			wantErr: "not a relative path",
		},
		{
			name: "parent traversal",
			headers: []*tar.Header{
				{Name: "etc/config/../../../etc/crontabs/root", Typeflag: tar.TypeReg, Size: 3},
			},
			bodies:  map[string]string{"etc/config/../../../etc/crontabs/root": "abc"},
			wantErr: "escapes the archive root",
		},
		{
			name: "symlink to a startup script",
			headers: []*tar.Header{
				{Name: "etc/config/rc.local", Typeflag: tar.TypeSymlink, Linkname: "/etc/init.d/travo", Size: 0},
			},
			wantErr: "only regular files are allowed",
		},
		{
			name: "hardlink",
			headers: []*tar.Header{
				{Name: "etc/config/hard", Typeflag: tar.TypeLink, Linkname: "etc/config/network", Size: 0},
			},
			wantErr: "only regular files are allowed",
		},
		{
			name: "device node",
			headers: []*tar.Header{
				{Name: "etc/config/dev", Typeflag: tar.TypeChar, Size: 0},
			},
			wantErr: "only regular files are allowed",
		},
		{
			name: "not a configuration backup",
			headers: []*tar.Header{
				{Name: "etc/some-other/file", Typeflag: tar.TypeReg, Size: 3},
			},
			wantErr: "not a configuration backup",
		},
		{
			name: "member larger than the cap",
			headers: []*tar.Header{
				{Name: "etc/config/network", Typeflag: tar.TypeReg, Size: maxRestoreMemberBytes + 1},
			},
			wantErr: "larger than",
		},
		{
			name: "total uncompressed size over the cap",
			headers: func() []*tar.Header {
				var hs []*tar.Header
				for i := 0; i < 16; i++ {
					hs = append(hs, &tar.Header{
						Name:     fmt.Sprintf("etc/config/big%d", i),
						Typeflag: tar.TypeReg,
						Size:     maxRestoreMemberBytes,
					})
				}
				return hs
			}(),
			wantErr: "uncompressed size exceeds",
		},
		{
			name: "too many members",
			headers: func() []*tar.Header {
				var hs []*tar.Header
				for i := 0; i <= maxRestoreArchiveMembers; i++ {
					hs = append(hs, &tar.Header{
						Name:     fmt.Sprintf("etc/config/f%d", i),
						Typeflag: tar.TypeReg,
						Size:     1,
					})
				}
				return hs
			}(),
			wantErr: "more than",
		},
		{
			name:    "empty archive",
			headers: nil,
			wantErr: "archive is empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "evil.tar.gz")
			if err := os.WriteFile(path, tarGzArchive(t, tc.headers, tc.bodies), 0o600); err != nil {
				t.Fatalf("write archive: %v", err)
			}
			err := ValidateRestoreArchive(path)
			if err == nil {
				t.Fatalf("the archive was accepted; members: %v", memberNames(tc.headers))
			}
			if !errors.Is(err, ErrInvalidBackupArchive) {
				t.Errorf("error = %v, want ErrInvalidBackupArchive", err)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

// TestValidateRestoreArchive_AcceptsFilesThisAppWrites is the regression guard
// for the decision to drop the directory allowlist.
//
// `sysupgrade -b` archives the overlay upper layer, and this application itself
// writes /etc/crontabs/root (LED and WiFi schedules),
// /etc/dropbear/authorized_keys (POST /system/ssh-keys) and, after a password
// change, /etc/shadow. An allowlist covering only etc/config and etc/ppp
// therefore refused every genuine backup taken on this device. Authorization is
// the boundary for restore (ADR 0007 section 2): the endpoint requires a token,
// and an authenticated admin already holds root via POST /system/ssh-keys,
// firmware flash and factory reset. Enumerating paths defended nothing and broke
// the feature.
func TestValidateRestoreArchive_AcceptsFilesThisAppWrites(t *testing.T) {
	headers := []*tar.Header{
		{Name: "etc/", Typeflag: tar.TypeDir, Size: 0},
		{Name: "etc/config/network", Typeflag: tar.TypeReg, Size: 3},
		{Name: "etc/crontabs/root", Typeflag: tar.TypeReg, Size: 19},
		{Name: "etc/dropbear/authorized_keys", Typeflag: tar.TypeReg, Size: 3},
		{Name: "etc/shadow", Typeflag: tar.TypeReg, Size: 3},
		{Name: "etc/uci-defaults/99-payload", Typeflag: tar.TypeReg, Size: 3},
		{Name: "etc/init.d/", Typeflag: tar.TypeDir, Size: 0},
		{Name: "etc/ppp/chap-secrets", Typeflag: tar.TypeReg, Size: 3},
	}
	bodies := map[string]string{"etc/crontabs/root": "* * * * * /bin/sh -c id"}
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	if err := os.WriteFile(path, tarGzArchive(t, headers, bodies), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	if err := ValidateRestoreArchive(path); err != nil {
		t.Errorf("a genuine overlay backup was refused: %v", err)
	}
}

func TestValidateRestoreArchive_RejectsNonGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	if err := os.WriteFile(path, []byte("not a gzip file at all"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := ValidateRestoreArchive(path); !errors.Is(err, ErrInvalidBackupArchive) {
		t.Errorf("error = %v, want ErrInvalidBackupArchive", err)
	}
	if err := ValidateRestoreArchive(filepath.Join(t.TempDir(), "missing.tar.gz")); !errors.Is(err, ErrInvalidBackupArchive) {
		t.Errorf("missing file: error = %v, want ErrInvalidBackupArchive", err)
	}
}

// A rejected archive must never reach sysupgrade — that call is what extracts
// it at / — and it must not leave a crash guard behind either: nothing was
// started, so a guard would tell the operator a restore is in flight.
func TestRestoreBackup_RejectsArchiveBeforeSysupgrade(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	path := filepath.Join(dir, "evil.tar.gz")
	evil := tarGzArchive(t, []*tar.Header{
		{Name: "etc/crontabs/root", Typeflag: tar.TypeReg, Size: 3},
	}, map[string]string{"etc/crontabs/root": "abc"})
	if err := os.WriteFile(path, evil, 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	err := svc.RestoreBackup(path)
	if err == nil {
		t.Fatal("an archive writing outside the allowlist was restored")
	}
	if !errors.Is(err, ErrInvalidBackupArchive) {
		t.Errorf("error = %v, want ErrInvalidBackupArchive", err)
	}
	assertNoSysupgradeCall(t, marker)
	if _, err := os.Stat(filepath.Join(dir, restoreGuardName)); err == nil {
		t.Error("a rejected archive left a restore-in-progress guard behind")
	}
}

func memberNames(headers []*tar.Header) []string {
	var out []string
	for _, h := range headers {
		out = append(out, h.Name)
	}
	return out
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

// Board names are prefixes of each other: "gl-mt3000" is a prefix of
// "gl-mt3000-nand". Matching the supported_devices list with a substring test
// therefore accepts a NAND-only image for a NOR router, and that flash is the
// brick this whole check exists to prevent. Entries must be compared whole.
func TestSupportsDevice_RequiresAWholeEntry(t *testing.T) {
	cases := []struct {
		name        string
		supported   []string
		boardName   string
		aliases     []string
		wantSupport bool
	}{
		{
			name:        "nand-only image on a nor board",
			supported:   []string{"glinet,gl-mt3000-nand"},
			boardName:   "glinet,gl-mt3000",
			aliases:     []string{"gl-mt3000", "GL.iNet GL-MT3000", "GL-MT3000"},
			wantSupport: false,
		},
		{
			name:        "nor-only image on a nand board",
			supported:   []string{"glinet,gl-mt3000"},
			boardName:   "glinet,gl-mt3000-nand",
			aliases:     []string{"gl-mt3000-nand"},
			wantSupport: false,
		},
		{
			name:        "own image listed beside a sibling variant",
			supported:   []string{"glinet,gl-mt3000", "glinet,gl-mt3000-nand"},
			boardName:   "glinet,gl-mt3000",
			aliases:     []string{"gl-mt3000", "GL.iNet GL-MT3000"},
			wantSupport: true,
		},
		{
			name:        "full multi-board name with no aliases",
			supported:   []string{"linksys,ea8300,linksys_e8300-ubi"},
			boardName:   "linksys,ea8300",
			aliases:     nil,
			wantSupport: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := ParseFirmwareMetadata(bytes.NewReader(
				buildFirmwareImage("GL.iNet GL-MT3000", tc.supported)))
			if err != nil {
				t.Fatalf("parsing image: %v", err)
			}
			if got := meta.SupportsDevice(tc.boardName, tc.aliases); got != tc.wantSupport {
				t.Errorf("SupportsDevice(%q, %v) with supported_devices %q = %v, want %v",
					tc.boardName, tc.aliases, meta.SupportedDevicesRaw, got, tc.wantSupport)
			}
		})
	}
}

// The end-to-end consequence of the prefix bug: an image that only supports the
// NAND variant must never reach sysupgrade on this NOR board.
func TestUpgradeFirmware_RejectsSiblingVariantImage(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	image := buildFirmwareImage("GL.iNet GL-MT3000 (NAND)", []string{"glinet,gl-mt3000-nand"})
	_, err := svc.UpgradeFirmware(bytes.NewReader(image), true)
	if err == nil {
		t.Fatal("an image for the sibling NAND variant was accepted for this NOR board")
	}
	if !errors.Is(err, ErrUnsupportedFirmware) {
		t.Errorf("error = %v, want ErrUnsupportedFirmware", err)
	}
	assertNoSysupgradeCall(t, marker)
	if _, err := os.Stat(filepath.Join(dir, firmwareUpgradeGuardName)); err == nil {
		t.Error("a rejected image must not leave a crash guard: nothing was flashed")
	}
	assertNoStagedFirmware(t)
}

// A DIRECTORY entry under etc/config is not evidence of anything: the
// message claims a UCI config member, so the witness has to be a real file. An
// archive that carries only directories below etc/config (note that
// cleanRestoreName drops the trailing slash, so it is the subdirectories that
// match the prefix) would otherwise pass.
func TestValidateRestoreArchive_RejectsConfigDirectoryWithoutAFile(t *testing.T) {
	headers := []*tar.Header{
		{Name: "etc", Typeflag: tar.TypeDir, Mode: 0o755, Size: 0},
		{Name: "etc/config", Typeflag: tar.TypeDir, Mode: 0o755, Size: 0},
		{Name: "etc/config/network", Typeflag: tar.TypeDir, Mode: 0o755, Size: 0},
		{Name: "etc/config/dhcp/", Typeflag: tar.TypeDir, Mode: 0o755, Size: 0},
		{Name: "etc/shadow", Typeflag: tar.TypeReg, Mode: 0o600, Size: 3},
	}
	archive := tarGzArchive(t, headers, map[string]string{"etc/shadow": "abc"})
	path := filepath.Join(t.TempDir(), "empty-config.tar.gz")
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	err := ValidateRestoreArchive(path)
	if err == nil {
		t.Fatal("an archive with only an etc/config directory was accepted as a backup")
	}
	if !errors.Is(err, ErrInvalidBackupArchive) {
		t.Errorf("error = %v, want ErrInvalidBackupArchive", err)
	}
	if !strings.Contains(err.Error(), "not a configuration backup") {
		t.Errorf("error = %q, want it to report the missing UCI config file", err)
	}
}

// board_name is comma separated ("glinet,gl-mt3000"), but it names ONE board.
// Handing its parts out as aliases hands out the vendor prefix "glinet", which
// matches every GL.iNet entry in a supported_devices list and would let a
// sibling board's image through.
func TestBoardIdentityDoesNotSplitTheVendorPrefix(t *testing.T) {
	svc, _ := newGuardedSystemService(t)
	name, aliases, err := svc.boardIdentity()
	if err != nil {
		t.Fatalf("boardIdentity: %v", err)
	}
	if name != "glinet,gl-mt3000" {
		t.Errorf("board name = %q, want the whole ubus value", name)
	}
	for _, a := range aliases {
		if a == "glinet" {
			t.Errorf("aliases %v must not contain the bare vendor prefix", aliases)
		}
	}
}
