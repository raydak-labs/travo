package services

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// SpeedtestService manages speedtest CLI installation and execution.
type SpeedtestService struct{}

// NewSpeedtestService creates a new SpeedtestService.
func NewSpeedtestService() *SpeedtestService {
	return &SpeedtestService{}
}

// GetSpeedtestServiceStatus returns the current status of the speedtest CLI service.
func (s *SpeedtestService) GetSpeedtestServiceStatus() (models.SpeedtestService, error) {
	arch, err := detectArchitecture()
	if err != nil {
		return models.SpeedtestService{}, fmt.Errorf("detecting architecture: %w", err)
	}

	supported := isArchitectureSupported(arch)
	installed, version := s.checkInstallation()

	return models.SpeedtestService{
		Installed:      installed,
		Supported:      supported,
		Architecture:   arch,
		Version:        version,
		PackageName:    "speedtest",
		StorageSizeMB:  2,
	}, nil
}

// InstallSpeedtestCLI installs the speedtest CLI package using opkg.
func (s *SpeedtestService) InstallSpeedtestCLI() error {
	arch, err := detectArchitecture()
	if err != nil {
		return fmt.Errorf("detecting architecture: %w", err)
	}

	if !isArchitectureSupported(arch) {
		return fmt.Errorf("architecture %s is not supported by speedtest CLI", arch)
	}

	if installed, _ := s.checkInstallation(); installed {
		return fmt.Errorf("speedtest CLI is already installed")
	}

	out, err := exec.Command("opkg", "update").CombinedOutput()
	if err != nil {
		return fmt.Errorf("updating package lists: %w: %s", err, string(out))
	}

	out, err = exec.Command("opkg", "install", "speedtest").CombinedOutput()
	if err != nil {
		return fmt.Errorf("installing speedtest: %w: %s", err, string(out))
	}

	return nil
}

// UninstallSpeedtestCLI removes the speedtest CLI package.
func (s *SpeedtestService) UninstallSpeedtestCLI() error {
	if installed, _ := s.checkInstallation(); !installed {
		return fmt.Errorf("speedtest CLI is not installed")
	}

	out, err := exec.Command("opkg", "remove", "speedtest").CombinedOutput()
	if err != nil {
		return fmt.Errorf("removing speedtest: %w: %s", err, string(out))
	}

	return nil
}

// RunSpeedtestCLI executes the speedtest CLI and returns parsed results.
func (s *SpeedtestService) RunSpeedtestCLI() (models.SpeedTestResult, error) {
	if installed, _ := s.checkInstallation(); !installed {
		return models.SpeedTestResult{}, fmt.Errorf("speedtest CLI is not installed")
	}

	cmd := exec.Command("speedtest", "--format=json")
	out, err := cmd.Output()
	if err != nil {
		return models.SpeedTestResult{}, fmt.Errorf("running speedtest: %w", err)
	}

	return parseSpeedtestOutput(string(out))
}

// checkInstallation checks if speedtest CLI is installed and returns its version.
func (s *SpeedtestService) checkInstallation() (bool, string) {
	path, err := exec.LookPath("speedtest")
	if err != nil {
		return false, ""
	}

	cmd := exec.Command(path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return true, "unknown"
	}

	version := strings.TrimSpace(string(out))
	re := regexp.MustCompile(`\d+\.\d+\.\d+`)
	if match := re.FindString(version); match != "" {
		return true, match
	}

	return true, "unknown"
}

// detectArchitecture returns the system architecture using uname.
func detectArchitecture() (string, error) {
	cmd := exec.Command("uname", "-m")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("executing uname: %w", err)
	}
	arch := strings.TrimSpace(string(out))

	// Normalize architecture names
	archMap := map[string]string{
		"aarch64":     "aarch64",
		"armv7l":      "arm",
		"armv6l":      "arm",
		"x86_64":      "x86_64",
		"i686":        "i386",
		"i386":        "i386",
		"mips":        "mips",
		"mipsel":      "mipsel",
		"mips64":      "mips64",
		"mips64el":    "mips64el",
	}

	if normalized, ok := archMap[arch]; ok {
		return normalized, nil
	}

	return arch, nil
}

// isArchitectureSupported checks if the architecture supports speedtest CLI.
func isArchitectureSupported(arch string) bool {
	supportedArchs := map[string]bool{
		"aarch64":  true,
		"arm":      true,
		"x86_64":   true,
		"i386":     true,
		"mips":     true,
		"mipsel":   true,
		"mips64":   true,
		"mips64el": true,
	}

	return supportedArchs[arch]
}

// parseSpeedtestOutput parses JSON output from speedtest CLI.
func parseSpeedtestOutput(output string) (models.SpeedTestResult, error) {
	result := models.SpeedTestResult{}

	reDownload := regexp.MustCompile(`"download":\s*\{[^}]*"bandwidth":\s*([\d.]+)`)
	reUpload := regexp.MustCompile(`"upload":\s*\{[^}]*"bandwidth":\s*([\d.]+)`)
	rePing := regexp.MustCompile(`"ping":\s*\{[^}]*"latency":\s*([\d.]+)`)
	reServer := regexp.MustCompile(`"server":\s*\{[^}]*"name":\s*"([^"]+)"`)

	downloadMBps := 0.0
	uploadMBps := 0.0
	pingMs := 0.0
	server := "unknown"

	if match := reDownload.FindStringSubmatch(output); len(match) > 1 {
		bandwidth := parseFloat(match[1])
		downloadMBps = bandwidth / 125000 // Convert bits/s to Mbps
	}

	if match := reUpload.FindStringSubmatch(output); len(match) > 1 {
		bandwidth := parseFloat(match[1])
		uploadMBps = bandwidth / 125000
	}

	if match := rePing.FindStringSubmatch(output); len(match) > 1 {
		pingMs = parseFloat(match[1])
	}

	if match := reServer.FindStringSubmatch(output); len(match) > 1 {
		server = match[1]
	}

	result.DownloadMbps = downloadMBps
	result.UploadMbps = uploadMBps
	result.PingMs = pingMs
	result.Server = server

	return result, nil
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
