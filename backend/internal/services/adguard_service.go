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
	adguardBinary  = "/opt/AdGuardHome/AdGuardHome"
	adguardInitd   = "/etc/init.d/adguardhome"
	adguardAPIBase = "http://127.0.0.1:3000"
)

// AdGuardChecker abstracts filesystem/process checks for testability.
type AdGuardChecker interface {
	// FileExists returns true if path exists and is a regular file.
	FileExists(path string) bool
	// RunCommand executes a command and returns combined output and error.
	RunCommand(name string, args ...string) (string, error)
	// HTTPGet performs an HTTP GET and returns the body bytes.
	HTTPGet(url string) ([]byte, error)
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
