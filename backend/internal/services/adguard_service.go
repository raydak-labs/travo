package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	yaml "gopkg.in/yaml.v3"
)

const (
	adguardBinary         = "/opt/AdGuardHome/AdGuardHome"
	adguardInitd          = "/etc/init.d/adguardhome"
	adguardAPIBaseDefault = "http://127.0.0.1:3000"
	adguardDefaultDNSPort = 5353
	adguardYAMLPathUCI    = "/etc/adguardhome/adguardhome.yaml"
	adguardYAMLPathOpt    = "/opt/AdGuardHome/AdGuardHome.yaml"
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

func (r *RealAdGuardChecker) TCPProbe(addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// AdGuardService provides status and statistics for AdGuard Home.
type AdGuardService struct {
	checker        AdGuardChecker
	httpAPIBase    string
	yamlDNSPort    int
	yamlSourcePath string
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

func (s *AdGuardService) refreshEndpointsFromYAML() {
	s.httpAPIBase = ""
	s.yamlDNSPort = 0
	s.yamlSourcePath = ""
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
		s.httpAPIBase = adguardAPIBaseDefault
		return
	}
	var y adguardYAMLTop
	if err := yaml.Unmarshal(data, &y); err != nil {
		s.httpAPIBase = adguardAPIBaseDefault
		s.yamlSourcePath = usedPath
		return
	}
	webPort := y.BindPort
	if webPort <= 0 {
		webPort = 3000
	}
	host := normalizeAdGuardBindHost(y.BindHost)
	s.httpAPIBase = fmt.Sprintf("http://%s:%d", host, webPort)
	s.yamlSourcePath = usedPath
	if y.DNS.Port > 0 {
		s.yamlDNSPort = y.DNS.Port
	}
}

func (s *AdGuardService) apiBase() string {
	if s != nil && s.httpAPIBase != "" {
		return s.httpAPIBase
	}
	return adguardAPIBaseDefault
}

// NewAdGuardService creates a new AdGuardService with a real checker.
func NewAdGuardService() *AdGuardService {
	s := &AdGuardService{checker: NewRealAdGuardChecker()}
	s.refreshEndpointsFromYAML()
	return s
}

// NewAdGuardServiceWithChecker creates a new AdGuardService with a custom checker (for tests).
func NewAdGuardServiceWithChecker(c AdGuardChecker) *AdGuardService {
	s := &AdGuardService{checker: c}
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
	s.refreshEndpointsFromYAML()
	result.AdminURL = s.apiBase()
	result.ConfigYAMLPath = s.yamlSourcePath

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

// adguardDNSInfoResponse matches the JSON from /control/dns_info.
type adguardDNSInfoResponse struct {
	Port int `json:"port"`
}

// getDNSPort returns the DNS port AdGuard listens on, or the default.
func (s *AdGuardService) getDNSPort() int {
	s.refreshEndpointsFromYAML()
	if s.yamlDNSPort > 0 {
		return s.yamlDNSPort
	}
	body, err := s.checker.HTTPGet(s.apiBase() + "/control/dns_info")
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

// probeAdGuardDNSListener returns true if the AdGuard DNS listener is accepting
// TCP connections on the given port.
func (s *AdGuardService) probeAdGuardDNSListener(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	return s.checker.TCPProbe(addr, 2*time.Second)
}

// probeResolver tries to resolve "example.com" via the local resolver and returns true on success.
func (s *AdGuardService) probeResolver(_ int) bool {
	_, err := s.checker.RunCommand("nslookup", "example.com", "127.0.0.1")
	return err == nil
}

// GetDNSStatus checks whether dnsmasq is configured to forward to AdGuard,
// and includes health checks for the DNS listener path.
func (s *AdGuardService) GetDNSStatus() (models.AdGuardDNSStatus, error) {
	port := s.getDNSPort()
	entry := dnsmasqServerEntry(port)

	out, err := s.checker.RunCommand("uci", "get", "dhcp.@dnsmasq[0].server")

	var forwardTarget string
	var enabled bool
	if err == nil {
		enabled = strings.Contains(out, entry)
		if enabled {
			forwardTarget = entry
		}
	}

	status := models.AdGuardDNSStatus{
		Enabled:              enabled,
		DNSPort:              port,
		DnsmasqForwardTarget: forwardTarget,
		AdguardListenerReady: s.probeAdGuardDNSListener(port),
	}
	if status.AdguardListenerReady {
		status.ResolverProbeOk = s.probeResolver(port)
	}
	return status, nil
}

// SetDNS enables or disables dnsmasq forwarding to AdGuard Home.
// When enabling, it first verifies that AdGuard is running and its DNS listener
// is reachable. If the pre-flight check fails, no dnsmasq changes are made and
// the error is returned (safe: DNS resolution is never left in a broken state).
func (s *AdGuardService) SetDNS(enabled bool) error {
	port := s.getDNSPort()
	entry := dnsmasqServerEntry(port)

	if enabled {
		// Pre-flight: AdGuard must be running.
		if !s.IsRunning() {
			return fmt.Errorf("AdGuard Home is not running — start it before enabling DNS forwarding")
		}
		// Pre-flight: DNS listener must be reachable.
		if !s.probeAdGuardDNSListener(port) {
			return fmt.Errorf("AdGuard Home DNS listener is not ready on 127.0.0.1:%d — verify AdGuard config", port)
		}

		// Apply dnsmasq changes.
		_, _ = s.checker.RunCommand("uci", "delete", "dhcp.@dnsmasq[0].server")
		if _, err := s.checker.RunCommand("uci", "add_list", fmt.Sprintf("dhcp.@dnsmasq[0].server=%s", entry)); err != nil {
			return fmt.Errorf("failed to set dnsmasq server: %w", err)
		}
		if _, err := s.checker.RunCommand("uci", "set", "dhcp.@dnsmasq[0].noresolv=1"); err != nil {
			// Rollback: delete the server entry we just added.
			_, _ = s.checker.RunCommand("uci", "delete", "dhcp.@dnsmasq[0].server")
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

// AutoConfigure writes a default AdGuardHome.yaml (if the file doesn't already exist),
// starts the adguardhome service, and enables dnsmasq forwarding to AdGuard.
// Called automatically after successful package install.
func (s *AdGuardService) AutoConfigure() error {
	if !s.checker.FileExists(adguardYAMLPathUCI) && !s.checker.FileExists(adguardYAMLPathOpt) {
		_, _ = s.checker.RunCommand("mkdir", "-p", "/opt/AdGuardHome")
		if err := s.checker.WriteFile(adguardYAMLPathOpt, []byte(defaultAdGuardConfig), 0600); err != nil {
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

// SetConfig writes the AdGuard Home YAML configuration and restarts the service.
func (s *AdGuardService) SetConfig(content string) error {
	s.refreshEndpointsFromYAML()
	path := adguardYAMLPathOpt
	if s.yamlSourcePath != "" {
		path = s.yamlSourcePath
	}
	if err := s.checker.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing AdGuard config: %w", err)
	}
	if _, err := s.checker.RunCommand(adguardInitd, "restart"); err != nil {
		return fmt.Errorf("restarting AdGuard: %w", err)
	}
	s.refreshEndpointsFromYAML()
	return nil
}
