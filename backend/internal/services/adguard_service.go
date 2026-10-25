package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	yaml "gopkg.in/yaml.v3"
)

const (
	adguardBinary          = "/opt/AdGuardHome/AdGuardHome"
	adguardInitd           = "/etc/init.d/adguardhome"
	adguardAPIBaseDefault  = "http://127.0.0.1:3000"
	adguardDefaultDNSPort  = 5353
	adguardYAMLPathUCI     = "/etc/adguardhome/adguardhome.yaml"
	adguardYAMLPathOpt     = "/opt/AdGuardHome/AdGuardHome.yaml"
	adguardBundledTemplate = "/etc/travo/adguardhome.yaml"
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
	// RemoveFile deletes a file. Returns nil when it is already gone.
	RemoveFile(path string) error
	// TCPProbe returns true if a TCP connection to addr (host:port) succeeds within timeout.
	TCPProbe(addr string, timeout time.Duration) bool
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
	out, err := execx.CombinedOutput(execx.Slow, name, args...)
	return strings.TrimSpace(string(out)), err
}

func (r *RealAdGuardChecker) HTTPGet(url string) ([]byte, error) {
	resp, err := r.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
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

func (r *RealAdGuardChecker) RemoveFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (r *RealAdGuardChecker) TCPProbe(addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// AdGuardService provides status and statistics for AdGuard Home.
//
// mu guards httpAPIBase, yamlDNSPort and yamlSourcePath. refreshEndpointsFromYAML
// rewrites all three on every status read, from five different callers, so
// without it two concurrent requests can interleave and leave the service
// pointing at a config path AdGuard is not reading.
type AdGuardService struct {
	checker        AdGuardChecker
	mu             sync.RWMutex
	httpAPIBase    string
	yamlDNSPort    int
	yamlSourcePath string
	// dnsStackPath is the shared dnsmasq resolver layer record. It is a field
	// rather than a bare constant so the stack lifecycle can be exercised
	// without touching /etc/trafo.
	dnsStackPath string
}

type adguardYAMLTop struct {
	BindHost string `yaml:"bind_host"`
	BindPort int    `yaml:"bind_port"`
	DNS      struct {
		Port int `yaml:"port"`
	} `yaml:"dns"`
}

func normalizeAdGuardBindHost(h string) string {
	h = strings.Trim(strings.TrimSpace(h), `"'`)
	switch h {
	case "", "0.0.0.0", "::", "[::]":
		return "127.0.0.1"
	default:
		return h
	}
}

// refreshEndpointsFromYAML re-reads the AdGuard YAML and publishes the derived
// endpoints. The values are computed into a local struct and published together
// under one write lock, so a reader never sees a base URL from one file with a
// port from another.
func (s *AdGuardService) refreshEndpointsFromYAML() {
	base, port, sourcePath := s.readEndpointsFromYAML()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.httpAPIBase = base
	s.yamlDNSPort = port
	s.yamlSourcePath = sourcePath
}

// readEndpointsFromYAML performs the file read and parsing with no locking. The
// returned values are a consistent triple.
func (s *AdGuardService) readEndpointsFromYAML() (base string, dnsPort int, sourcePath string) {
	var data []byte
	var usedPath string
	for _, p := range []string{adguardYAMLPathUCI, adguardYAMLPathOpt} {
		b, err := s.checker.ReadFile(p)
		if err == nil && len(b) > 0 {
			data, usedPath = b, p
			break
		}
	}
	if len(data) == 0 {
		return adguardAPIBaseDefault, 0, ""
	}
	var y adguardYAMLTop
	if err := yaml.Unmarshal(data, &y); err != nil {
		return adguardAPIBaseDefault, 0, usedPath
	}
	webPort := y.BindPort
	if webPort <= 0 {
		webPort = 3000
	}
	host := normalizeAdGuardBindHost(y.BindHost)
	sourcePath = usedPath
	if y.DNS.Port > 0 {
		dnsPort = y.DNS.Port
	}
	return fmt.Sprintf("http://%s:%d", host, webPort), dnsPort, sourcePath
}

// configPathLocked returns the YAML path AdGuard is reading, or the default.
func (s *AdGuardService) configPathLocked() string {
	if s.yamlSourcePath != "" {
		return s.yamlSourcePath
	}
	return adguardYAMLPathOpt
}

func (s *AdGuardService) apiBase() string {
	if s == nil {
		return adguardAPIBaseDefault
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.httpAPIBase != "" {
		return s.httpAPIBase
	}
	return adguardAPIBaseDefault
}

// NewAdGuardService creates a new AdGuardService with a real checker.
func NewAdGuardService() *AdGuardService {
	s := &AdGuardService{checker: NewRealAdGuardChecker(), dnsStackPath: dnsmasqLayerStackPath}
	s.refreshEndpointsFromYAML()
	return s
}

// NewAdGuardServiceWithChecker creates a new AdGuardService with a custom checker (for tests).
func NewAdGuardServiceWithChecker(c AdGuardChecker) *AdGuardService {
	s := &AdGuardService{checker: c, dnsStackPath: dnsmasqLayerStackPath}
	s.refreshEndpointsFromYAML()
	return s
}

// IsInstalled returns true when the AdGuard Home binary or init script exists,
// OR when the process is already running. This avoids reporting installed=false
// when AdGuard was installed via a non-standard path but is actively running.
func (s *AdGuardService) IsInstalled() bool {
	if s.checker.FileExists(adguardBinary) {
		return true
	}
	if s.checker.FileExists(adguardInitd) {
		return true
	}
	s.refreshEndpointsFromYAML()
	_, err := s.checker.HTTPGet(s.apiBase() + "/control/status")
	return err == nil
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
	if _, after, ok := strings.Cut(out, "version "); ok {
		return strings.TrimSpace(after)
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
	s.refreshEndpointsFromYAML()
	result.AdminURL = s.apiBase()
	s.mu.RLock()
	result.ConfigYAMLPath = s.yamlSourcePath
	s.mu.RUnlock()

	statusBody, err := s.checker.HTTPGet(s.apiBase() + "/control/status")
	if err != nil {
		return result, fmt.Errorf("failed to reach AdGuard API: %w", err)
	}
	var statusResp adguardStatusResponse
	if err := json.Unmarshal(statusBody, &statusResp); err != nil {
		return result, fmt.Errorf("failed to parse status response: %w", err)
	}
	result.Enabled = statusResp.ProtectionEnabled

	statsBody, err := s.checker.HTTPGet(s.apiBase() + "/control/stats")
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

// defaultAdGuardConfig is written on first install to give AdGuard sensible defaults:
// web UI on port 3000, DNS listener on 5353 (dnsmasq-forwarding mode), DoH upstreams.
const defaultAdGuardConfig = `bind_host: 0.0.0.0
bind_port: 3000
users: []
auth_attempts: 5
block_auth_min: 15
dns:
  bind_hosts:
    - 0.0.0.0
  port: 5353
  upstream_dns:
    - https://dns.cloudflare.com/dns-query
    - https://dns.google/dns-query
  bootstrap_dns:
    - 1.1.1.1
    - 8.8.8.8
  protection_enabled: true
  blocking_mode: default
filtering:
  enabled: true
  update_interval: 24
  filters:
    - enabled: true
      url: https://adguardteam.github.io/AdGuardSDNSFilter/Filters/filter.txt
      name: AdGuard DNS filter
      id: 1
log_file: ""
verbose: false
`

// AutoConfigure writes a default AdGuardHome.yaml (if the file doesn't already exist)
// and enables + starts the adguardhome service.
// Registered as ServiceManager's post-install hook for "adguardhome" (main.go), so
// it runs after a successful package install.
//
// It deliberately does NOT touch dnsmasq: forwarding is an operator decision made
// through SetDNS, which records the pre-any-layer resolver list in the shared
// dnsmasq layer record. An earlier comment here claimed it enabled forwarding;
// it never did.
func (s *AdGuardService) AutoConfigure() error {
	if !s.checker.FileExists(adguardYAMLPathUCI) && !s.checker.FileExists(adguardYAMLPathOpt) {
		_, _ = s.checker.RunCommand("mkdir", "-p", "/opt/AdGuardHome")
		// Prefer the bundled template shipped with the tarball; fall back to the
		// embedded constant so AutoConfigure works in non-tarball environments too.
		var configBytes []byte
		if bundled, err := s.checker.ReadFile(adguardBundledTemplate); err == nil {
			configBytes = bundled
		} else {
			configBytes = []byte(defaultAdGuardConfig)
		}
		if err := s.checker.WriteFile(adguardYAMLPathOpt, configBytes, 0600); err != nil {
			return fmt.Errorf("writing default AdGuard config: %w", err)
		}
	}

	if s.checker.FileExists(adguardInitd) {
		_, _ = s.checker.RunCommand(adguardInitd, "enable")
		if _, err := s.checker.RunCommand(adguardInitd, "start"); err != nil {
			return fmt.Errorf("starting AdGuard Home: %w", err)
		}
	}

	s.refreshEndpointsFromYAML()
	return nil
}

// GetConfig reads the AdGuard Home YAML configuration file.
func (s *AdGuardService) GetConfig() (string, error) {
	var lastErr error
	for _, p := range []string{adguardYAMLPathUCI, adguardYAMLPathOpt} {
		data, err := s.checker.ReadFile(p)
		if err == nil {
			return string(data), nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return "", fmt.Errorf("reading AdGuard config: %w", lastErr)
}

// adguardConfigVerifyTimeout bounds how long SetConfig waits for the DNS
// listener to come back after a restart before rolling the config back.
var adguardConfigVerifyTimeout = 8 * time.Second

// adguardConfigVerifyPollDuration is the interval between listener probes.
var adguardConfigVerifyPollDuration = 500 * time.Millisecond

// SetConfig writes the AdGuard Home YAML configuration and restarts the service.
//
// dnsmasq forwards every LAN query to AdGuard, so a broken document would kill
// name resolution for the whole network. Three things guard it:
//
//  1. the body must parse as a YAML mapping, or it is rejected untouched;
//  2. the current file is copied to a timestamped .bak before the write;
//  3. after the restart the DNS listener is polled; if it never comes up the
//     backup is restored and AdGuard restarted again. A checker that cannot
//     probe triggers the rollback rather than a silent success.
func (s *AdGuardService) SetConfig(content string) error {
	// Validate before anything is written: an unparseable document must never
	// reach the live config.
	var probe map[string]any
	if err := yaml.Unmarshal([]byte(content), &probe); err != nil {
		return fmt.Errorf("AdGuard config is not valid YAML: %w", err)
	}
	if probe == nil {
		return fmt.Errorf("AdGuard config is empty; refusing to replace the live configuration")
	}

	s.refreshEndpointsFromYAML()
	s.mu.RLock()
	path := s.configPathLocked()
	s.mu.RUnlock()

	// Back up the current config so a bad write can be undone.
	previous, readErr := s.checker.ReadFile(path)
	backupPath := adguardConfigBackupPath(path, time.Now())
	if readErr == nil {
		if err := s.checker.WriteFile(backupPath, previous, 0o600); err != nil {
			return fmt.Errorf("backing up AdGuard config: %w", err)
		}
	}

	if err := s.checker.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing AdGuard config: %w", err)
	}
	if _, err := s.checker.RunCommand(adguardInitd, "restart"); err != nil {
		s.rollbackAdGuardConfig(path, backupPath, previous, readErr)
		return fmt.Errorf("restarting AdGuard: %w", err)
	}
	s.refreshEndpointsFromYAML()

	if s.probeAdGuardDNSListener(s.getDNSPort()) {
		// The change is good, so the backup has nothing left to protect. Keeping
		// one per edit would accumulate unbounded small files on the overlayfs
		// NAND, and each one is a copy of a file that holds bcrypt hashes.
		_ = s.checker.RemoveFile(backupPath)
		return nil
	}
	deadline := time.Now().Add(adguardConfigVerifyTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(adguardConfigVerifyPollDuration)
		s.refreshEndpointsFromYAML()
		if s.probeAdGuardDNSListener(s.getDNSPort()) {
			_ = s.checker.RemoveFile(backupPath)
			return nil
		}
	}
	s.rollbackAdGuardConfig(path, backupPath, previous, readErr)
	return fmt.Errorf("AdGuard DNS listener did not come back after the config change;" +
		" the previous config was restored")
}

// adguardConfigBackupPath is the timestamped backup name for a config path.
func adguardConfigBackupPath(path string, now time.Time) string {
	return fmt.Sprintf("%s.%s.bak", path, now.UTC().Format("20060102T150405Z"))
}

// rollbackAdGuardConfig restores the pre-write config and restarts AdGuard. It
// reports nothing: the caller is already returning the original failure and a
// rollback failure must not replace it. The .bak is deliberately left on disk
// on this path — the operator has a broken config to inspect.
func (s *AdGuardService) rollbackAdGuardConfig(path, backupPath string,
	previous []byte, readErr error,
) {
	if readErr == nil {
		if err := s.checker.WriteFile(path, previous, 0600); err != nil {
			slog.Error("adguard: restoring config", "backup", backupPath, "err", err)
			return
		}
	}
	if _, err := s.checker.RunCommand(adguardInitd, "restart"); err != nil {
		slog.Error("adguard: restarting after config rollback", "err", err)
	}
	s.refreshEndpointsFromYAML()
}

// SetPassword hashes password with bcrypt (default cost) and writes it into the
// AdGuard Home YAML config under the first matching user, then restarts the service.
func (s *AdGuardService) SetPassword(username, password string) error {
	if password == "" {
		return fmt.Errorf("password must not be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	configStr, err := s.GetConfig()
	if err != nil {
		return fmt.Errorf("reading AdGuard config: %w", err)
	}

	// Unmarshal into a generic map so all unrecognised keys are preserved.
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(configStr), &doc); err != nil {
		return fmt.Errorf("parsing AdGuard config: %w", err)
	}

	users, _ := doc["users"].([]any)
	updated := false
	for _, u := range users {
		m, ok := u.(map[string]any)
		if !ok {
			continue
		}
		if m["name"] == username {
			m["password"] = string(hash)
			updated = true
			break
		}
	}
	if !updated {
		// User not found — append a new entry.
		users = append(users, map[string]any{
			"name":     username,
			"password": string(hash),
		})
		doc["users"] = users
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("serialising AdGuard config: %w", err)
	}
	return s.SetConfig(string(out))
}

// GetDNSMode returns the current DNS resolver configuration mode.
func (s *AdGuardService) GetDNSMode(dnsBypassed bool) models.DNSMode {
	result := models.DNSMode{DNSBypassed: dnsBypassed}

	if !s.IsInstalled() {
		result.Mode = "default"
		result.Description = "Using OpenWRT default DNS (dnsmasq with upstream from DHCP)"
		return result
	}

	running := s.IsRunning()
	result.AdGuardRunning = running

	if !running {
		result.Mode = "default"
		result.Description = "AdGuard Home installed but not running — using default DNS"
		return result
	}

	port := s.getDNSPort()
	if port == 53 {
		result.Mode = "adguard-direct"
		result.Description = "AdGuard Home listening directly on port 53"
		return result
	}

	// Check if dnsmasq forwards to AdGuard
	noresolv, err := s.checker.RunCommand("uci", "get", "dhcp.@dnsmasq[0].noresolv")
	if err == nil && strings.TrimSpace(noresolv) == "1" {
		result.Mode = "adguard-forwarding"
		result.Description = fmt.Sprintf("dnsmasq forwards DNS to AdGuard Home (port %d)", port)
		return result
	}

	result.Mode = "default"
	result.Description = "Using OpenWRT default DNS (dnsmasq with upstream from DHCP)"
	return result
}
