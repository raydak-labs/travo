package services

import (
	"testing"
)

func TestNewSpeedtestService(t *testing.T) {
	svc := NewSpeedtestService()
	if svc == nil {
		t.Fatal("NewSpeedtestService returned nil")
	}
}

func TestGetSpeedtestServiceStatus(t *testing.T) {
	svc := NewSpeedtestService()
	status, err := svc.GetSpeedtestServiceStatus()
	if err != nil {
		t.Fatalf("GetSpeedtestServiceStatus failed: %v", err)
	}
	if status.PackageName != "speedtest" {
		t.Errorf("PackageName = %s, want speedtest", status.PackageName)
	}
	if status.StorageSizeMB < 1 || status.StorageSizeMB > 10 {
		t.Errorf("StorageSizeMB out of range: %d", status.StorageSizeMB)
	}
}

func TestCheckInstallation(t *testing.T) {
	svc := NewSpeedtestService()
	installed, version := svc.checkInstallation()
	if installed && version == "" {
		t.Error("Version should not be empty when installed")
	}
	if !installed && version != "" {
		t.Error("Version should be empty when not installed")
	}
}

func TestDetectArchitecture(t *testing.T) {
	arch, err := detectArchitecture()
	if err != nil {
		t.Fatalf("detectArchitecture failed: %v", err)
	}
	if arch == "" {
		t.Error("Architecture should not be empty")
	}
}

func TestIsArchitectureSupported(t *testing.T) {
	tests := []struct {
		arch      string
		supported bool
	}{
		{"aarch64", true}, {"arm", true}, {"x86_64": true}, {"i386": true},
		{"mips", true}, {"mipsel", true}, {"mips64", true}, {"mips64el": true},
		{"riscv64", false}, {"ppc64le", false},
	}
	for _, tt := range tests {
		if got := isArchitectureSupported(tt.arch); got != tt.supported {
			t.Errorf("isArchitectureSupported(%s) = %v, want %v", tt.arch, got, tt.supported)
		}
	}
}

func TestParseSpeedtestOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"valid", `{"type":"result","download":{"bandwidth":50000000},"upload":{"bandwidth":10000000},"ping":{"latency":15.5},"server":{"name":"Test"}}`},
		{"empty", ""},
		{"invalid", "not json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSpeedtestOutput(tt.input)
			if tt.name == "valid" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.name == "valid" && result.DownloadMbps < 300 || result.DownloadMbps > 500 {
				t.Errorf("DownloadMbps unexpected: %f", result.DownloadMbps)
			}
		})
	}
}

func TestparseFloat(t *testing.T) {
	tests := []struct{ input string; want float64 }{
		{"123.456", 123.456}, {"0", 0}, {"-5.5", -5.5}, {"invalid", 0},
	}
	for _, tt := range tests {
		if got := parseFloat(tt.input); got != tt.want {
			t.Errorf("parseFloat(%s) = %f, want %f", tt.input, got, tt.want)
		}
	}
}