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
	// Legacy wan-level settings (kept for backward compat)
	PeerDNS string `json:"peerdns,omitempty"`
	DNS     string `json:"dns,omitempty"`
	// Dnsmasq-level settings (the actual blocking mechanism)
	DnsmasqNoResolv      string   `json:"dnsmasq_noresolv,omitempty"`
	DnsmasqServers       []string `json:"dnsmasq_servers,omitempty"`
	DnsmasqRebindProtect string   `json:"dnsmasq_rebind_protection,omitempty"`
	Time                 int64    `json:"time"`
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
		// Probe failed (DNS error, timeout, connection refused, "operation not permitted").
		// This commonly happens when:
		// 1. Custom DNS (AdGuard) can't resolve because captive portal blocks upstream
		// 2. Captive portal firewall blocks all HTTP except to gateway
		// Try to detect which case and build a useful portal URL.
		gatewayURL := c.detectGatewayPortalURL()
		if c.CheckDNSBypassNeeded() || gatewayURL != "" {
			portalURL := gatewayURL
			if portalURL == "" {
				portalURL = captiveProbeURL
			}
			return models.CaptivePortalStatus{
				Detected:         true,
				PortalURL:        &portalURL,
				CanReachInternet: false,
			}, nil
		}
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
		statusCode == http.StatusSeeOther ||
		statusCode == http.StatusTemporaryRedirect {
		// Prefer gateway URL for auto-accept compatibility
		portalURL := redirectURL
		if gatewayURL := c.detectGatewayPortalURL(); gatewayURL != "" {
			portalURL = gatewayURL
		}
		return models.CaptivePortalStatus{
			Detected:         true,
			PortalURL:        &portalURL,
			CanReachInternet: false,
		}, nil
	}

	// 200 with content = likely captive portal login page
	if statusCode == http.StatusOK {
		fallback := captiveProbeURL
		// Prefer gateway URL if available — better for auto-accept
		if gatewayURL := c.detectGatewayPortalURL(); gatewayURL != "" {
			fallback = gatewayURL
		}
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

// BypassDNS temporarily disables custom DNS (AdGuard/noresolv) so that the
// captive portal's DNS can be resolved via the upstream DHCP-provided nameserver.
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

	// Read dnsmasq config — this is the real culprit when AdGuard is configured
	noresolv := c.getDnsmasqOption("noresolv")
	servers := c.getDnsmasqServers()
	rebindProtect := c.getDnsmasqOption("rebind_protection")

	// Also read wan config for completeness
	wanOpts, _ := c.uci.GetAll("network", "wan")
	wanPeerdns := wanOpts["peerdns"]
	wanDNS := wanOpts["dns"]

	// Check if there's anything to bypass
	needsBypass := (noresolv == "1" && len(servers) > 0) || (wanPeerdns == "0" && strings.TrimSpace(wanDNS) != "")
	if !needsBypass {
		return nil
	}

	// Save current state
	backup := dnsBackup{
		PeerDNS:              wanPeerdns,
		DNS:                  wanDNS,
		DnsmasqNoResolv:      noresolv,
		DnsmasqServers:       servers,
		DnsmasqRebindProtect: rebindProtect,
		Time:                 time.Now().Unix(),
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

	// Disable noresolv so dnsmasq reads /tmp/resolv.conf.d/resolv.conf.auto
	if noresolv == "1" {
		if err := c.setDnsmasqOption("noresolv", "0"); err != nil {
			_ = os.Remove(c.guardFile)
			return err
		}
		// Remove custom servers so dnsmasq uses upstream from resolv.conf
		_ = c.deleteDnsmasqOption("server")
	}

	// Disable rebind protection — captive portals use private IPs for hostnames
	if rebindProtect == "1" {
		_ = c.setDnsmasqOption("rebind_protection", "0")
	}

	if err := c.commitDhcp(); err != nil {
		_ = os.Remove(c.guardFile)
		return err
	}

	// Also fix wan peerdns if needed
	if wanPeerdns == "0" {
		_ = c.uci.Set("network", "wan", "peerdns", "1")
		_ = c.uci.Set("network", "wan", "dns", "")
		_ = c.uci.Commit("network")
	}

	// Restart dnsmasq to pick up new settings
	if c.cmd != nil {
		_, _ = c.cmd.Run("/etc/init.d/dnsmasq", "restart")
	}

	log.Printf("captive: DNS bypassed (noresolv=%s, servers=%v, peerdns=%s)", noresolv, servers, wanPeerdns)
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

	// Restore dnsmasq settings
	if backup.DnsmasqNoResolv == "1" {
		_ = c.setDnsmasqOption("noresolv", "1")
		for _, srv := range backup.DnsmasqServers {
			_ = c.addDnsmasqListItem("server", srv)
		}
	}
	if backup.DnsmasqRebindProtect == "1" {
		_ = c.setDnsmasqOption("rebind_protection", "1")
	}
	if backup.DnsmasqNoResolv == "1" || backup.DnsmasqRebindProtect == "1" {
		_ = c.commitDhcp()
	}

	// Restore wan settings
	if backup.PeerDNS != "" {
		_ = c.uci.Set("network", "wan", "peerdns", backup.PeerDNS)
	}
	if backup.DNS != "" {
		_ = c.uci.Set("network", "wan", "dns", backup.DNS)
	}
	if backup.PeerDNS != "" || backup.DNS != "" {
		_ = c.uci.Commit("network")
	}

	_ = os.Remove(c.guardFile)

	if c.cmd != nil {
		_, _ = c.cmd.Run("/etc/init.d/dnsmasq", "restart")
	}

	log.Printf("captive: DNS restored")
	return nil
}

// findDnsmasqSection returns the UCI section identifier for the first dnsmasq instance.
// On real OpenWRT this is typically "@dnsmasq[0]" (unnamed section).
func (c *CaptiveService) findDnsmasqSection() string {
	return "@dnsmasq[0]"
}

// getDnsmasqServers returns the list of server entries for the dnsmasq section
// by running `uci get dhcp.@dnsmasq[0].server` via CommandRunner.
func (c *CaptiveService) getDnsmasqServers() []string {
	if c.cmd == nil {
		return nil
	}
	out, err := c.cmd.Run("uci", "get", "dhcp.@dnsmasq[0].server")
	if err != nil {
		return nil
	}
	val := strings.TrimSpace(string(out))
	if val == "" {
		return nil
	}
	return strings.Fields(val)
}

// getDnsmasqOption reads a single dnsmasq option via uci get.
func (c *CaptiveService) getDnsmasqOption(option string) string {
	if c.cmd == nil {
		return ""
	}
	out, err := c.cmd.Run("uci", "get", "dhcp.@dnsmasq[0]."+option)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// setDnsmasqOption sets a dnsmasq option via uci set.
func (c *CaptiveService) setDnsmasqOption(option, value string) error {
	if c.cmd == nil {
		return nil
	}
	_, err := c.cmd.Run("uci", "set", "dhcp.@dnsmasq[0]."+option+"="+value)
	return err
}

// deleteDnsmasqOption deletes a dnsmasq option via uci delete.
func (c *CaptiveService) deleteDnsmasqOption(option string) error {
	if c.cmd == nil {
		return nil
	}
	_, _ = c.cmd.Run("uci", "delete", "dhcp.@dnsmasq[0]."+option)
	return nil
}

// addDnsmasqListItem appends a value to a dnsmasq list option.
func (c *CaptiveService) addDnsmasqListItem(option, value string) error {
	if c.cmd == nil {
		return nil
	}
	_, err := c.cmd.Run("uci", "add_list", "dhcp.@dnsmasq[0]."+option+"="+value)
	return err
}

// commitDhcp runs uci commit dhcp.
func (c *CaptiveService) commitDhcp() error {
	if c.cmd == nil {
		return nil
	}
	_, err := c.cmd.Run("uci", "commit", "dhcp")
	return err
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

// refreshBypassTimestamp updates the guard file timestamp to prevent stale auto-restore.
func (c *CaptiveService) refreshBypassTimestamp() {
	data, err := os.ReadFile(c.guardFile)
	if err != nil {
		return
	}
	var backup dnsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return
	}
	backup.Time = time.Now().Unix()
	newData, err := json.Marshal(backup)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.guardFile, newData, 0600)
}

// CheckDNSBypassNeeded returns true if custom DNS is configured that would
// block captive portal access (e.g. AdGuard with noresolv, or wan peerdns=0).
func (c *CaptiveService) CheckDNSBypassNeeded() bool {
	if c.uci == nil {
		return false
	}

	// Check dnsmasq noresolv + custom server (the main culprit with AdGuard)
	noresolv := c.getDnsmasqOption("noresolv")
	if noresolv == "1" {
		servers := c.getDnsmasqServers()
		if len(servers) > 0 {
			return true
		}
	}

	// Also check legacy wan peerdns=0
	opts, err := c.uci.GetAll("network", "wan")
	if err != nil {
		return false
	}
	return opts["peerdns"] == "0" && strings.TrimSpace(opts["dns"]) != ""
}

// detectGatewayPortalURL tries to detect a captive portal by probing the default
// gateway on HTTP. Many captive portals (hotel/airport) respond with a redirect
// when you hit the gateway IP directly.
func (c *CaptiveService) detectGatewayPortalURL() string {
	if c.cmd == nil {
		return ""
	}
	// Get the default gateway from `ip route`
	out, err := c.cmd.Run("ip", "route", "show", "default")
	if err != nil {
		return ""
	}
	// Parse "default via <IP> dev <iface> ..."
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 3 || fields[0] != "default" || fields[1] != "via" {
		return ""
	}
	gatewayIP := fields[2]
	if gatewayIP == "" {
		return ""
	}

	// Try to probe the gateway on HTTP (short timeout)
	gatewayURL := "http://" + gatewayIP + "/"
	statusCode, _, _, err := c.prober.Do(gatewayURL)
	if err != nil {
		return ""
	}

	// If it redirects or serves a page, the gateway itself is the portal entry point.
	// Return the gateway HTTP URL (not the redirect target) because:
	// 1. Users open this in their browser which follows redirects naturally
	// 2. HTTPS redirect targets often don't work when fetched directly
	// 3. Multi-step portals (MikroTik → external auth → back) need the browser flow
	if statusCode == http.StatusFound || statusCode == http.StatusMovedPermanently ||
		statusCode == http.StatusTemporaryRedirect || statusCode == http.StatusSeeOther ||
		statusCode == http.StatusOK {
		return gatewayURL
	}

	return ""
}
