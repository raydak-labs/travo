package services

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

const captiveProbeURL = "http://connectivitycheck.gstatic.com/generate_204"

// captiveDNSGuardFile stores original DNS config while bypass is active.
const captiveDNSGuardFile = "/etc/travo/captive-dns-in-progress"

// captiveDNSRestoreTimeout auto-restores DNS if bypass has been active too long.
const captiveDNSRestoreTimeout = 5 * time.Minute

// HTTPProber performs HTTP probes for captive portal detection.
type HTTPProber interface {
	// Do sends a GET request and returns status code, body, redirect URL (if any), and error.
	Do(url string) (statusCode int, body string, redirectURL string, err error)
}

// RealHTTPProber uses net/http with redirect checking disabled.
type RealHTTPProber struct {
	client *http.Client
}

// NewRealHTTPProber creates a prober with a 5-second timeout and no-redirect policy.
func NewRealHTTPProber() *RealHTTPProber {
	return &RealHTTPProber{
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Do performs an HTTP GET and returns status, body, redirect URL, and error.
func (p *RealHTTPProber) Do(url string) (int, string, string, error) {
	resp, err := p.client.Get(url)
	if err != nil {
		return 0, "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", "", err
	}

	var redirectURL string
	if loc := resp.Header.Get("Location"); loc != "" {
		redirectURL = loc
	}

	return resp.StatusCode, string(bodyBytes), redirectURL, nil
}

// MockHTTPProber returns preset responses for testing.
type MockHTTPProber struct {
	StatusCode  int
	Body        string
	RedirectURL string
	Err         error
}

// Do returns the preset mock response.
func (m *MockHTTPProber) Do(_ string) (int, string, string, error) {
	return m.StatusCode, m.Body, m.RedirectURL, m.Err
}

// CaptiveService checks for captive portal detection.
type CaptiveService struct {
	prober    HTTPProber
	uci       uci.UCI
	cmd       CommandRunner
	mu        sync.Mutex
	guardFile string
}

// dnsBackup holds the original DNS settings for restoration.
type dnsBackup struct {
	PeerDNS string `json:"peerdns"`
	DNS     string `json:"dns"`
	Time    int64  `json:"time"`
}

// NewCaptiveService creates a new CaptiveService with the given HTTP prober.
func NewCaptiveService(prober HTTPProber) *CaptiveService {
	return &CaptiveService{prober: prober, guardFile: captiveDNSGuardFile}
}

// NewCaptiveServiceWithUCI creates a CaptiveService with UCI access for DNS bypass.
func NewCaptiveServiceWithUCI(prober HTTPProber, u uci.UCI, cmd CommandRunner) *CaptiveService {
	svc := &CaptiveService{prober: prober, uci: u, cmd: cmd, guardFile: captiveDNSGuardFile}
	// Auto-restore stale bypass on startup
	go svc.autoRestoreStaleBypass()
	return svc
}

// CheckCaptivePortal probes for captive portals by making an HTTP request
// to a known endpoint and checking for redirects or unexpected responses.
func (c *CaptiveService) CheckCaptivePortal() (models.CaptivePortalStatus, error) {
	statusCode, _, redirectURL, err := c.prober.Do(captiveProbeURL)
	if err != nil {
		return models.CaptivePortalStatus{
			Detected:         false,
			CanReachInternet: false,
		}, nil
	}

	// 204 No Content = internet works fine
	if statusCode == http.StatusNoContent {
		return models.CaptivePortalStatus{
			Detected:         false,
			CanReachInternet: true,
		}, nil
	}

	// Redirect = captive portal
	if statusCode == http.StatusMovedPermanently ||
		statusCode == http.StatusFound ||
		statusCode == http.StatusTemporaryRedirect {
		portalURL := redirectURL
		return models.CaptivePortalStatus{
			Detected:         true,
			PortalURL:        &portalURL,
			CanReachInternet: false,
		}, nil
	}

	// 200 with content = likely captive portal login page
	if statusCode == http.StatusOK {
		fallback := captiveProbeURL
		return models.CaptivePortalStatus{
			Detected:         true,
			PortalURL:        &fallback,
			CanReachInternet: false,
		}, nil
	}

	// Anything else — assume no internet
	return models.CaptivePortalStatus{
		Detected:         false,
		CanReachInternet: false,
	}, nil
}

// IsDNSBypassed returns true if DNS bypass is currently active.
func (c *CaptiveService) IsDNSBypassed() bool {
	_, err := os.Stat(c.guardFile)
	return err == nil
}

// BypassDNS temporarily switches WAN to upstream DNS (peerdns=1) for captive portal access.
// Stores original config in guard file for later restoration.
func (c *CaptiveService) BypassDNS() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.uci == nil {
		return nil // no UCI = mock mode, noop
	}

	// Already bypassed?
	if _, err := os.Stat(c.guardFile); err == nil {
		return nil
	}

	// Read current DNS config
	opts, err := c.uci.GetAll("network", "wan")
	if err != nil {
		return err
	}

	// If already using upstream DNS, nothing to bypass
	if opts["peerdns"] != "0" {
		return nil
	}

	// Save current state
	backup := dnsBackup{
		PeerDNS: opts["peerdns"],
		DNS:     opts["dns"],
		Time:    time.Now().Unix(),
	}
	data, err := json.Marshal(backup)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.guardFile), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(c.guardFile, data, 0600); err != nil {
		return err
	}

	// Set to use upstream DNS
	if err := c.uci.Set("network", "wan", "peerdns", "1"); err != nil {
		_ = os.Remove(c.guardFile)
		return err
	}
	if err := c.uci.Set("network", "wan", "dns", ""); err != nil {
		_ = os.Remove(c.guardFile)
		return err
	}
	if err := c.uci.Commit("network"); err != nil {
		_ = os.Remove(c.guardFile)
		return err
	}

	// Restart dnsmasq to pick up new resolv.conf
	if c.cmd != nil {
		_, _ = c.cmd.Run("/etc/init.d/dnsmasq", "restart")
	}

	return nil
}

// RestoreDNS restores the original DNS config from the guard file.
func (c *CaptiveService) RestoreDNS() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.uci == nil {
		return nil
	}

	data, err := os.ReadFile(c.guardFile)
	if err != nil {
		return nil // no guard file = nothing to restore
	}

	var backup dnsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		_ = os.Remove(c.guardFile)
		return err
	}

	if err := c.uci.Set("network", "wan", "peerdns", backup.PeerDNS); err != nil {
		return err
	}
	dnsVal := backup.DNS
	if err := c.uci.Set("network", "wan", "dns", dnsVal); err != nil {
		return err
	}
	if err := c.uci.Commit("network"); err != nil {
		return err
	}

	_ = os.Remove(c.guardFile)

	if c.cmd != nil {
		_, _ = c.cmd.Run("/etc/init.d/dnsmasq", "restart")
	}

	return nil
}

// autoRestoreStaleBypass restores DNS if guard file is older than timeout.
func (c *CaptiveService) autoRestoreStaleBypass() {
	time.Sleep(10 * time.Second) // wait for startup
	data, err := os.ReadFile(c.guardFile)
	if err != nil {
		return
	}
	var backup dnsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		_ = os.Remove(c.guardFile)
		return
	}
	age := time.Since(time.Unix(backup.Time, 0))
	if age > captiveDNSRestoreTimeout {
		log.Printf("captive: auto-restoring DNS bypass (stale %v)", age)
		_ = c.RestoreDNS()
	}
}

// MaybeAutoRestoreDNS restores DNS if internet is now reachable and bypass is active.
func (c *CaptiveService) MaybeAutoRestoreDNS(canReachInternet bool) {
	if !canReachInternet || !c.IsDNSBypassed() {
		return
	}
	log.Printf("captive: internet reachable, auto-restoring DNS")
	_ = c.RestoreDNS()
}

// CheckDNSBypassNeeded returns true if custom DNS is configured (blocking portal access).
func (c *CaptiveService) CheckDNSBypassNeeded() bool {
	if c.uci == nil {
		return false
	}
	opts, err := c.uci.GetAll("network", "wan")
	if err != nil {
		return false
	}
	return opts["peerdns"] == "0" && strings.TrimSpace(opts["dns"]) != ""
}
