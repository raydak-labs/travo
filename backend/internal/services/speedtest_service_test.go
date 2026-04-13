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

	if status.Architecture == "" {
		t.Error("Architecture should not be empty")
	}

	if status.PackageName != "speedtest" {
		t.Errorf("PackageName should be 'speedtest', got %s", status.PackageName)
	}

	if status.StorageSizeMB < 1 || status.StorageSizeMB > 10 {
		t.Errorf("StorageSizeMB should be between 1 and 10, got %d", status.StorageSizeMB)
	}
}

func TestCheckInstallation(t *testing.T) {
	svc := NewSpeedtestService()

	installed, version := svc.checkInstallation()

	if installed && version == "" {
		t.Error("Version should not be empty when installed is true")
	}

	if !installed && version != "" {
		t.Error("Version should be empty when installed is false")
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

	validArches := map[string]bool{
		"aarch64": true, "arm": true, "x86_64": true,
		"i386": true, "mips": true, "mipsel": true,
		"mips64": true, "mips64el": true,
	}

	if !validArches[arch] {
		t.Logf("Warning: Unknown architecture detected: %s", arch)
	}
}

func TestIsArchitectureSupported(t *testing.T) {
	tests := []struct {
		arch      string
		supported bool
	}{
		{"aarch64", true},
		{"arm", true},
		{"x86_64", true},
		{"i386", true},
		{"mips", true},
		{"mipsel", true},
		{"mips64", true},
		{"mips64el": true},
		{"riscv64", false},
		{"ppc64le", false},
	}

	for _, tt := range tests {
		result := isArchitectureSupported(tt.arch)
		if result != tt.supported {
			t.Errorf("isArchitectureSupported(%s) = %v, want %v", tt.arch, result, tt.supported)
		}
	}
}

func TestParseSpeedtestOutput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid JSON",
			input:   `{"type":"result","download":{"bandwidth":50000000},"upload":{"bandwidth":10000000},"ping":{"latency":15.5},"server":{"name":"Test Server"}}`,
			wantErr: false,
		},
		{
			name:    "empty output",
			input:   "",
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   "not json",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSpeedtestOutput(tt.input)
			if !tt.wantErr {
				if err != nil {
					t.Errorf("parseSpeedtestOutput() unexpected error: %v", err)
				}
				return
			}

			if tt.name == "valid JSON" {
				if result.DownloadMbps < 300 || result.DownloadMbps > 500 {
					t.Errorf("DownloadMbps out of expected range: %f", result.DownloadMbps)
				}
				if result.UploadMbps < 50 || result.UploadMbps > 100 {
					t.Errorf("UploadMbps out of expected range: %f", result.UploadMbps)
				}
				if result.PingMs < 10 || result.PingMs > 20 {
					t.Errorf("PingMs out of expected range: %f", result.PingMs)
				}
				if result.Server != "Test Server" {
					t.Errorf("Server = %s, want Test Server", result.Server)
				}
			}
		})
	}
}

func TestparseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"123.456", 123.456},
		{"0", 0},
		{"-5.5", -5.5},
		{"invalid", 0},
	}

	for _, tt := range tests {
		got := parseFloat(tt.input)
		if got != tt.want {
			t.Errorf("parseFloat(%s) = %f, want %f", tt.input, got, tt.want)
		}
	}
}
