package services

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

type SpeedtestService struct{}

func NewSpeedtestService() *SpeedtestService {
	return &SpeedtestService{}
}

func (s *SpeedtestService) GetSpeedtestServiceStatus() (models.SpeedtestService, error) {
	arch, err := detectArchitecture()
	if err != nil {
		return models.SpeedtestService{}, fmt.Errorf("detecting architecture: %w", err)
	}
	supported := isArchitectureSupported(arch)
	installed, version := s.checkInstallation()
	return models.SpeedtestService{
		Installed: installed, Supported: supported, Architecture: arch,
		Version: version, PackageName: "speedtest", StorageSizeMB: 2,
	}, nil
}

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
	if out, err := exec.Command("opkg", "update").CombinedOutput(); err != nil {
		return fmt.Errorf("updating package lists: %w: %s", err, string(out))
	}
	if out, err := exec.Command("opkg", "install", "speedtest").CombinedOutput(); err != nil {
		return fmt.Errorf("installing speedtest: %w: %s", err, string(out))
	}
	return nil
}

func (s *SpeedtestService) UninstallSpeedtestCLI() error {
	if installed, _ := s.checkInstallation(); !installed {
		return fmt.Errorf("speedtest CLI is not installed")
	}
	if out, err := exec.Command("opkg", "remove", "speedtest").CombinedOutput(); err != nil {
		return fmt.Errorf("removing speedtest: %w: %s", err, string(out))
	}
	return nil
}

func (s *SpeedtestService) RunSpeedtestCLI() (models.SpeedTestResult, error) {
	if installed, _ := s.checkInstallation(); !installed {
		return models.SpeedTestResult{}, fmt.Errorf("speedtest CLI is not installed")
	}
	out, err := exec.Command("speedtest", "--format=json").Output()
	if err != nil {
		return models.SpeedTestResult{}, fmt.Errorf("running speedtest: %w", err)
	}
	return parseSpeedtestOutput(string(out))
}

func (s *SpeedtestService) checkInstallation() (bool, string) {
	path, err := exec.LookPath("speedtest")
	if err != nil {
		return false, ""
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return true, "unknown"
	}
	re := regexp.MustCompile(`\d+\.\d+\.\d+`)
	if match := re.FindString(string(out)); match != "" {
		return true, match
	}
	return true, "unknown"
}

func detectArchitecture() (string, error) {
	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return "", fmt.Errorf("executing uname: %w", err)
	}
	arch := strings.TrimSpace(string(out))
	archMap := map[string]string{
		"aarch64": "aarch64", "armv7l": "arm", "armv6l": "arm",
		"x86_64": "x86_64", "i686": "i386", "i386": "i386",
		"mips": "mips", "mipsel": "mipsel", "mips64": "mips64", "mips64el": "mips64el",
	}
	if normalized, ok := archMap[arch]; ok {
		return normalized, nil
	}
	return arch, nil
}

func isArchitectureSupported(arch string) bool {
	return map[string]bool{
		"aarch64": true, "arm": true, "x86_64": true, "i386": true,
		"mips": true, "mipsel": true, "mips64": true, "mips64el": true,
	}[arch]
}

func parseSpeedtestOutput(output string) (models.SpeedTestResult, error) {
	result := models.SpeedTestResult{}
	downloadMBps := 0.0
	uploadMBps := 0.0
	pingMs := 0.0
	server := "unknown"

	reDownload := regexp.MustCompile(`"download":\s*\{[^}]*"bandwidth":\s*([\d.]+)`)
	reUpload := regexp.MustCompile(`"upload":\s*\{[^}]*"bandwidth":\s*([\d.]+)`)
	rePing := regexp.MustCompile(`"ping":\s*\{[^}]*"latency":\s*([\d.]+)`)
	reServer := regexp.MustCompile(`"server":\s*\{[^}]*"name":\s*"([^"]+)"`)

	if match := reDownload.FindStringSubmatch(output); len(match) > 1 {
		downloadMBps = parseFloat(match[1]) / 125000
	}
	if match := reUpload.FindStringSubmatch(output); len(match) > 1 {
		uploadMBps = parseFloat(match[1]) / 125000
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