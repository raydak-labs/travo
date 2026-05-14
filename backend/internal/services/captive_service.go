package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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
			Timeout: 3 * time.Second,
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
	// test overrides (empty in production)
	adguardAPIBaseOverride string
	resolvConfAutoOverride string
}

// adguardAPIBase is the local AdGuardHome HTTP API endpoint.
const adguardAPIBase = "http://127.0.0.1:3000"

// resolvConfAuto is the path where dnsmasq/odhcp6c writes DHCP-provided DNS.
const resolvConfAuto = "/tmp/resolv.conf.d/resolv.conf.auto"

// dnsBackup holds the original DNS settings for restoration.
type dnsBackup struct {
	// Legacy wan-level settings (kept for backward compat)
	PeerDNS string `json:"peerdns,omitempty"`
	DNS     string `json:"dns,omitempty"`
	// Dnsmasq-level settings (the actual blocking mechanism)
	DnsmasqNoResolv      string   `json:"dnsmasq_noresolv,omitempty"`
	DnsmasqServers       []string `json:"dnsmasq_servers,omitempty"`
	DnsmasqRebindProtect string   `json:"dnsmasq_rebind_protection,omitempty"`
	// AdGuardHome upstream DNS backup
	AdGuardUpstream  []string `json:"adguard_upstream,omitempty"`
	AdGuardBootstrap []string `json:"adguard_bootstrap,omitempty"`
	AdGuardFallback  []string `json:"adguard_fallback,omitempty"`
	Time             int64    `json:"time"`
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

// isUpstreamConnected returns true if the device has an active default route,
// indicating it is connected to an upstream network. In test/mock mode (cmd == nil)
// it always returns true to avoid breaking tests.
func (c *CaptiveService) isUpstreamConnected() bool {
	if c.cmd == nil {
		return true // test mode: assume connected
	}
	out, err := c.cmd.Run("ip", "route", "show", "default")
	if err != nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(string(out)), "default via")
}

// IsUpstreamConnected is the exported wrapper around isUpstreamConnected.
func (c *CaptiveService) IsUpstreamConnected() bool {
	return c.isUpstreamConnected()
}

// CheckCaptivePortal probes for captive portals by making an HTTP request
// to a known endpoint and checking for redirects or unexpected responses.
func (c *CaptiveService) CheckCaptivePortal() (models.CaptivePortalStatus, error) {
	statusCode, _, redirectURL, err := c.prober.Do(captiveProbeURL)

	if err != nil {
		// If there's no upstream connection at all, avoid false-positive portal detection.
		if !c.isUpstreamConnected() {
			return models.CaptivePortalStatus{
				Detected:         false,
				CanReachInternet: false,
			}, nil
		}
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

	// 204 No Content = internet works fine — fast path, no gateway probe needed
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

// BypassDNS temporarily switches DNS to the DHCP-provided gateway DNS so the
// captive portal login page can be resolved.  It patches both dnsmasq (for any
// local consumers) and AdGuardHome (which is the actual port-53 resolver for
// LAN clients).  Original config is stored in the guard file for restoration.
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

	// Read dnsmasq config
	noresolv := c.getDnsmasqOption("noresolv")
	servers := c.getDnsmasqServers()
	rebindProtect := c.getDnsmasqOption("rebind_protection")

	// Read wan config for completeness
	wanOpts, _ := c.uci.GetAll("network", "wan")
	wanPeerdns := wanOpts["peerdns"]
	wanDNS := wanOpts["dns"]

	// Determine whether anything actually blocks portal DNS resolution.
	// Either dnsmasq noresolv, legacy wan peerdns=0, or AdGuardHome using
	// encrypted DoH/DoT upstreams (which bypass hotel DNS hijacking).
	agEncrypted := c.isAdGuardUsingEncryptedDNS()
	needsBypass := noresolv == "1" || (wanPeerdns == "0" && strings.TrimSpace(wanDNS) != "") || agEncrypted
	if !needsBypass {
		return nil
	}

	// Get the DHCP-provided upstream DNS from the WAN interface.
	// This is what the captive portal network expects us to use.
	hotelDNS := c.readDHCPDNS()
	if hotelDNS == "" {
		log.Printf("captive: could not read DHCP DNS from resolv.conf.auto")
	}

	// Read current AdGuardHome upstream config so we can restore it.
	agUpstream, agBootstrap, agFallback := c.readAdGuardUpstream()

	// Save current state (including AdGuardHome config)
	backup := dnsBackup{
		PeerDNS:              wanPeerdns,
		DNS:                  wanDNS,
		DnsmasqNoResolv:      noresolv,
		DnsmasqServers:       servers,
		DnsmasqRebindProtect: rebindProtect,
		AdGuardUpstream:      agUpstream,
		AdGuardBootstrap:     agBootstrap,
		AdGuardFallback:      agFallback,
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

	// --- Patch dnsmasq (only needed when noresolv=1 or rebind protection) ---
	needsDnsmasqCommit := false
	if noresolv == "1" {
		if err := c.setDnsmasqOption("noresolv", "0"); err != nil {
			_ = os.Remove(c.guardFile)
			return err
		}
		_ = c.deleteDnsmasqOption("server")
		// Add hotel DNS as explicit dnsmasq upstream (belt-and-suspenders)
		if hotelDNS != "" {
			_ = c.addDnsmasqListItem("server", hotelDNS)
		}
		needsDnsmasqCommit = true
	}
	if rebindProtect == "1" {
		_ = c.setDnsmasqOption("rebind_protection", "0")
		needsDnsmasqCommit = true
	}
	if needsDnsmasqCommit {
		if err := c.commitDhcp(); err != nil {
			_ = os.Remove(c.guardFile)
			return err
		}
		if c.cmd != nil {
			_, _ = c.cmd.Run("/etc/init.d/dnsmasq", "restart")
		}
	}
	if wanPeerdns == "0" {
		_ = c.uci.Set("network", "wan", "peerdns", "1")
		_ = c.uci.Set("network", "wan", "dns", "")
		_ = c.uci.Commit("network")
	}

	// --- Patch AdGuardHome (this is the actual port-53 resolver) ---
	// Switch its upstream from DoH/DoT to the plain hotel DNS so that
	// captive portal hostnames (which resolve to private IPs) are resolved.
	if hotelDNS != "" {
		if err := c.setAdGuardUpstream([]string{hotelDNS}, nil, nil); err != nil {
			log.Printf("captive: warning — could not update AdGuardHome upstream: %v", err)
			// Non-fatal: dnsmasq changes are still in effect
		} else {
			log.Printf("captive: AdGuardHome upstream switched to %s", hotelDNS)
		}
	}

	log.Printf("captive: DNS bypassed (noresolv=%s, servers=%v, hotelDNS=%s)", noresolv, servers, hotelDNS)
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

	// Restore dnsmasq settings — always restore noresolv regardless of servers
	needsDhcpCommit := false
	if backup.DnsmasqNoResolv != "" {
		_ = c.setDnsmasqOption("noresolv", backup.DnsmasqNoResolv)
		needsDhcpCommit = true
	}
	// Delete current servers first, then re-add original ones
	_ = c.deleteDnsmasqOption("server")
	for _, srv := range backup.DnsmasqServers {
		_ = c.addDnsmasqListItem("server", srv)
		needsDhcpCommit = true
	}
	if backup.DnsmasqRebindProtect == "1" {
		_ = c.setDnsmasqOption("rebind_protection", "1")
		needsDhcpCommit = true
	}
	if needsDhcpCommit {
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

	// Restore AdGuardHome upstream DNS
	if len(backup.AdGuardUpstream) > 0 {
		if err := c.setAdGuardUpstream(backup.AdGuardUpstream, backup.AdGuardBootstrap, backup.AdGuardFallback); err != nil {
			log.Printf("captive: warning — could not restore AdGuardHome upstream: %v", err)
		} else {
			log.Printf("captive: AdGuardHome upstream restored to %v", backup.AdGuardUpstream)
		}
	}

	_ = os.Remove(c.guardFile)

	if c.cmd != nil {
		_, _ = c.cmd.Run("/etc/init.d/dnsmasq", "restart")
	}

	log.Printf("captive: DNS restored")
	return nil
}

// readDHCPDNS parses the first nameserver line from resolv.conf.auto —
// this is the DNS server handed out by the upstream network via DHCP.
func (c *CaptiveService) readDHCPDNS() string {
	path := resolvConfAuto
	if c.resolvConfAutoOverride != "" {
		path = c.resolvConfAutoOverride
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "nameserver ") {
			ip := strings.TrimSpace(strings.TrimPrefix(line, "nameserver "))
			if ip != "" {
				return ip
			}
		}
	}
	return ""
}

// adguardDNSConfig is the JSON body for POST /control/dns_config.
type adguardDNSConfig struct {
	UpstreamDNS  []string `json:"upstream_dns"`
	BootstrapDNS []string `json:"bootstrap_dns,omitempty"`
	FallbackDNS  []string `json:"fallback_dns,omitempty"`
}

// adguardDNSInfoResp is the relevant subset of GET /control/dns_info.
type adguardDNSInfoResp struct {
	UpstreamDNS  []string `json:"upstream_dns"`
	BootstrapDNS []string `json:"bootstrap_dns"`
	FallbackDNS  []string `json:"fallback_dns"`
}

// isAdGuardUsingEncryptedDNS returns true when AdGuardHome is reachable and
// its upstream DNS entries use encrypted protocols (https://, tls://, quic://)
// that would prevent captive portal hostnames from resolving via hotel DNS.
// Returns false (non-blocking) when AdGuardHome is absent or unreachable.
func (c *CaptiveService) isAdGuardUsingEncryptedDNS() bool {
	upstream, _, _ := c.readAdGuardUpstream()
	for _, u := range upstream {
		if strings.HasPrefix(u, "https://") ||
			strings.HasPrefix(u, "tls://") ||
			strings.HasPrefix(u, "quic://") ||
			strings.HasPrefix(u, "sdns://") {
			return true
		}
	}
	return false
}

// agAPIBase returns the AdGuardHome API base URL, respecting test overrides.
func (c *CaptiveService) agAPIBase() string {
	if c.adguardAPIBaseOverride != "" {
		return c.adguardAPIBaseOverride
	}
	return adguardAPIBase
}

// readAdGuardUpstream fetches the current upstream DNS config from AdGuardHome.
// Returns empty slices if AdGuard is not running or not installed.
func (c *CaptiveService) readAdGuardUpstream() (upstream, bootstrap, fallback []string) {
	cl := &http.Client{Timeout: 2 * time.Second}
	resp, err := cl.Get(c.agAPIBase() + "/control/dns_info")
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	var info adguardDNSInfoResp
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return
	}
	return info.UpstreamDNS, info.BootstrapDNS, info.FallbackDNS
}

// setAdGuardUpstream calls AdGuardHome's /control/dns_config to update the upstream.
func (c *CaptiveService) setAdGuardUpstream(upstream, bootstrap, fallback []string) error {
	cfg := adguardDNSConfig{
		UpstreamDNS:  upstream,
		BootstrapDNS: bootstrap,
		FallbackDNS:  fallback,
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Post(c.agAPIBase()+"/control/dns_config", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AdGuardHome dns_config returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
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

	// noresolv=1 means dnsmasq ignores upstream DHCP DNS.
	noresolv := c.getDnsmasqOption("noresolv")
	if noresolv == "1" {
		return true
	}

	// Legacy wan peerdns=0 with static DNS.
	opts, err := c.uci.GetAll("network", "wan")
	if err == nil && opts["peerdns"] == "0" && strings.TrimSpace(opts["dns"]) != "" {
		return true
	}

	// AdGuardHome (if installed) uses encrypted DoH/DoT upstreams that bypass
	// hotel DNS hijacking — captive portal hostnames won't resolve.
	return c.isAdGuardUsingEncryptedDNS()
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

	// Use a short-timeout client for the gateway probe — the gateway is on LAN
	// so if it doesn't respond in 2s, it's not a captive portal gateway.
	gwClient := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	gatewayURL := "http://" + gatewayIP + "/"
	resp, err := gwClient.Get(gatewayURL)
	if err != nil {
		return ""
	}
	_ = resp.Body.Close()
	statusCode := resp.StatusCode

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
