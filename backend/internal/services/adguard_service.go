package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const (
	adguardBinary         = "/opt/AdGuardHome/AdGuardHome"
	adguardInitd          = "/etc/init.d/adguardhome"
	adguardAPIBase        = "http://127.0.0.1:3000"
	adguardDefaultDNSPort = 5353
)

// AdGuardChecker abstracts filesystem/process checks for testability.
type AdGuardChecker interface {
	// FileExists returns true if path exists and is a regular file.
	FileExists(path string) bool
	// RunCommand executes a command and returns combined output and error.
	RunCommand(name string, args ...string) (string, error)
	// HTTPGet performs an HTTP GET and returns the body bytes.
	HTTPGet(url string) ([]byte, error)
	// ReadFile reads the contents of a file.
	ReadFile(path string) ([]byte, error)
	// WriteFile writes contents to a file.
	WriteFile(path string, data []byte, perm os.FileMode) error
}

// RealAdGuardChecker performs real OS operations.
type RealAdGuardChecker struct {
	client *http.Client
}

// NewRealAdGuardChecker creates a checker that talks to the real system.
func NewRealAdGuardChecker() *RealAdGuardChecker {
	return &RealAdGuardChecker{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (r *RealAdGuardChecker) FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (r *RealAdGuardChecker) RunCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (r *RealAdGuardChecker) HTTPGet(url string) ([]byte, error) {
	resp, err := r.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func (r *RealAdGuardChecker) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (r *RealAdGuardChecker) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// AdGuardService provides status and statistics for AdGuard Home.
type AdGuardService struct {
	checker AdGuardChecker
}

// NewAdGuardService creates a new AdGuardService with a real checker.
func NewAdGuardService() *AdGuardService {
	return &AdGuardService{checker: NewRealAdGuardChecker()}
}

// NewAdGuardServiceWithChecker creates a new AdGuardService with a custom checker (for tests).
func NewAdGuardServiceWithChecker(c AdGuardChecker) *AdGuardService {
	return &AdGuardService{checker: c}
}

// IsInstalled returns true when the AdGuard Home binary exists on disk.
func (s *AdGuardService) IsInstalled() bool {
	return s.checker.FileExists(adguardBinary)
}

// IsRunning returns true when the adguardhome service is currently active.
func (s *AdGuardService) IsRunning() bool {
	if !s.checker.FileExists(adguardInitd) {
		return false
	}
	// On OpenWRT, `/etc/init.d/<svc> status` exits 0 if running.
	_, err := s.checker.RunCommand(adguardInitd, "status")
	return err == nil
}

// Version returns the installed AdGuard Home version string or empty.
func (s *AdGuardService) Version() string {
	if !s.IsInstalled() {
		return ""
	}
	out, err := s.checker.RunCommand(adguardBinary, "--version")
	if err != nil {
		return ""
	}
	// Output looks like "AdGuard Home, version v0.107.54"
	if idx := strings.Index(out, "version "); idx >= 0 {
		return strings.TrimSpace(out[idx+len("version "):])
	}
	return out
}

// adguardStatsResponse matches the JSON from /control/stats.
type adguardStatsResponse struct {
	NumDNSQueries           int64   `json:"num_dns_queries"`
	NumBlockedFiltering     int64   `json:"num_blocked_filtering"`
	NumReplacedSafebrowsing int64   `json:"num_replaced_safebrowsing"`
	NumReplacedParental     int64   `json:"num_replaced_parental"`
	AvgProcessingTime       float64 `json:"avg_processing_time"`
}

// adguardStatusResponse matches the JSON from /control/status.
type adguardStatusResponse struct {
	ProtectionEnabled bool   `json:"protection_enabled"`
	Version           string `json:"version"`
	Running           bool   `json:"running"`
}

// GetStatus returns a combined AdGuardStatus with stats and protection state.
func (s *AdGuardService) GetStatus() (models.AdGuardStatus, error) {
	var result models.AdGuardStatus

	// Fetch protection status.
	statusBody, err := s.checker.HTTPGet(adguardAPIBase + "/control/status")
	if err != nil {
		return result, fmt.Errorf("failed to reach AdGuard API: %w", err)
	}
	var statusResp adguardStatusResponse
	if err := json.Unmarshal(statusBody, &statusResp); err != nil {
		return result, fmt.Errorf("failed to parse status response: %w", err)
	}
	result.Enabled = statusResp.ProtectionEnabled

	// Fetch statistics.
	statsBody, err := s.checker.HTTPGet(adguardAPIBase + "/control/stats")
	if err != nil {
		return result, fmt.Errorf("failed to fetch stats: %w", err)
	}
	var statsResp adguardStatsResponse
	if err := json.Unmarshal(statsBody, &statsResp); err != nil {
		return result, fmt.Errorf("failed to parse stats response: %w", err)
	}

	result.TotalQueries = statsResp.NumDNSQueries
	blocked := statsResp.NumBlockedFiltering + statsResp.NumReplacedSafebrowsing + statsResp.NumReplacedParental
	result.BlockedQueries = blocked
	if result.TotalQueries > 0 {
		result.BlockPercentage = float64(blocked) / float64(result.TotalQueries) * 100.0
	}
	// avg_processing_time is in seconds; convert to ms.
	result.AvgResponseMS = statsResp.AvgProcessingTime * 1000.0

	return result, nil
}

// adguardDNSInfoResponse matches the JSON from /control/dns_info.
type adguardDNSInfoResponse struct {
	Port int `json:"port"`
}

// getDNSPort returns the DNS port AdGuard listens on, or the default.
func (s *AdGuardService) getDNSPort() int {
	body, err := s.checker.HTTPGet(adguardAPIBase + "/control/dns_info")
	if err != nil {
		return adguardDefaultDNSPort
	}
	var info adguardDNSInfoResponse
	if err := json.Unmarshal(body, &info); err != nil || info.Port == 0 {
		return adguardDefaultDNSPort
	}
	return info.Port
}

// dnsmasqServerEntry returns the dnsmasq server value for a given port.
func dnsmasqServerEntry(port int) string {
	return fmt.Sprintf("127.0.0.1#%d", port)
}

// GetDNSStatus checks whether dnsmasq is configured to forward to AdGuard.
func (s *AdGuardService) GetDNSStatus() (models.AdGuardDNSStatus, error) {
	port := s.getDNSPort()
	entry := dnsmasqServerEntry(port)

	out, err := s.checker.RunCommand("uci", "get", "dhcp.@dnsmasq[0].server")
	if err != nil {
		// No server option set means not enabled.
		return models.AdGuardDNSStatus{Enabled: false, DNSPort: port}, nil
	}

	enabled := strings.Contains(out, entry)
	return models.AdGuardDNSStatus{Enabled: enabled, DNSPort: port}, nil
}

// SetDNS enables or disables dnsmasq forwarding to AdGuard Home.
func (s *AdGuardService) SetDNS(enabled bool) error {
	port := s.getDNSPort()
	entry := dnsmasqServerEntry(port)

	if enabled {
		// Remove any existing server list, then add AdGuard entry.
		// Ignore error if option doesn't exist yet.
		_, _ = s.checker.RunCommand("uci", "delete", "dhcp.@dnsmasq[0].server")
		if _, err := s.checker.RunCommand("uci", "add_list", fmt.Sprintf("dhcp.@dnsmasq[0].server=%s", entry)); err != nil {
			return fmt.Errorf("failed to set dnsmasq server: %w", err)
		}
		if _, err := s.checker.RunCommand("uci", "set", "dhcp.@dnsmasq[0].noresolv=1"); err != nil {
			return fmt.Errorf("failed to set noresolv: %w", err)
		}
	} else {
		_, _ = s.checker.RunCommand("uci", "delete", "dhcp.@dnsmasq[0].server")
		if _, err := s.checker.RunCommand("uci", "set", "dhcp.@dnsmasq[0].noresolv=0"); err != nil {
			return fmt.Errorf("failed to unset noresolv: %w", err)
		}
	}

	if _, err := s.checker.RunCommand("uci", "commit", "dhcp"); err != nil {
		return fmt.Errorf("failed to commit dhcp: %w", err)
	}
	if _, err := s.checker.RunCommand("/etc/init.d/dnsmasq", "restart"); err != nil {
		return fmt.Errorf("failed to restart dnsmasq: %w", err)
	}
	return nil
}

const adguardConfigPath = "/etc/adguardhome/adguardhome.yaml"

// GetConfig reads the AdGuard Home YAML configuration file.
func (s *AdGuardService) GetConfig() (string, error) {
	data, err := s.checker.ReadFile(adguardConfigPath)
	if err != nil {
		return "", fmt.Errorf("reading AdGuard config: %w", err)
	}
	return string(data), nil
}

// SetConfig writes the AdGuard Home YAML configuration and restarts the service.
func (s *AdGuardService) SetConfig(content string) error {
	if err := s.checker.WriteFile(adguardConfigPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing AdGuard config: %w", err)
	}
	if _, err := s.checker.RunCommand(adguardInitd, "restart"); err != nil {
		return fmt.Errorf("restarting AdGuard: %w", err)
	}
	return nil
}
