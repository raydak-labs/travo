package services

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

func TestGetSystemInfo(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, &MockStorageProvider{})

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
	svc := NewSystemService(ub, &MockStorageProvider{})

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
}

func TestGetSystemInfo_StorageNotHardcoded(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := NewSystemService(ub, &MockStorageProvider{})

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
	svc := NewSystemService(ub, &MockStorageProvider{})

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
	svc := NewSystemService(ub, custom)

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
