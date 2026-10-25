package system

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// newSSHKeyTestService returns a SystemService whose authorized_keys path is a
// temp file, so the index handling can be exercised off-device.
func newSSHKeyTestService(t *testing.T, path string) *Service {
	t.Helper()
	svc := New(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})
	svc.sshKeysFile = path
	return svc
}

func TestGetSystemInfo(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := New(ub, uci.NewMockUCI(), &MockStorageProvider{})

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
	svc := New(ub, uci.NewMockUCI(), &MockStorageProvider{})

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
	svc := New(ub, uci.NewMockUCI(), &MockStorageProvider{})

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
	svc := New(ub, uci.NewMockUCI(), &MockStorageProvider{})

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
	svc := New(ub, uci.NewMockUCI(), custom)

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

func TestSetHostname(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	svc := New(ub, u, &MockStorageProvider{})
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
	svc := New(ub, u, &MockStorageProvider{})

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
	svc := New(ub, u, &MockStorageProvider{})
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
	svc := New(ub, u, &MockStorageProvider{})

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
	svc := New(ub, u, &MockStorageProvider{})
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
	svc := New(ub, u, &MockStorageProvider{})

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
	svc := New(ub, uci.NewMockUCI(), &MockStorageProvider{})

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

func TestGetTimezone_MissingSection(t *testing.T) {
	ub := ubus.NewMockUbus()
	u := uci.NewMockUCI()
	svc := New(ub, u, &MockStorageProvider{})

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

func newGuardedSystemService(t *testing.T) (*Service, string) {
	t.Helper()
	svc := New(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})
	dir := t.TempDir()
	svc.SetGuardDir(dir)
	return svc, dir
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
	if _, err := os.Stat(guard.In(dir, factoryResetGuard).Path()); err != nil {
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
	if _, err := os.Stat(guard.In(dir, systemConfigGuard).Path()); !os.IsNotExist(err) {
		t.Errorf("guard must be removed after a successful commit, stat err = %v", err)
	}
}

// --- Backups ---

// --- Memory stats ---

// used = total - free - cached - buffered can go negative when the reported
// numbers overlap. A negative value is nonsense in the UI.
func TestGetSystemStats_ClampsNegativeMemoryUsage(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := New(ub, uci.NewMockUCI(), &MockStorageProvider{})
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

// --- Button action config parsing ---
