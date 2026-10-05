package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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
const captiveDNSGuardFile = crashGuardDir + "/captive-dns-in-progress"

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
	// dnsStackPath is the shared dnsmasq resolver layer record this service
	// stacks its bypass on. Empty means "next to the guard file", which is
	// /etc/trafo in production (the same directory the record's own constant
	// names) and a temp dir in tests.
	dnsStackPath string
	// stopCh cancels the startup auto-restore goroutine so a shutdown does not
	// race with a DNS restore (the goroutine used to be untracked).
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
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
	// DnsmasqLayer records that this bypass pushed the `captive` layer onto the
	// shared dnsmasq resolver stack, which makes the layer RECORD the owner of
	// the pre-bypass resolver list. Without it a restore cannot tell a bypass
	// that never touched the list from one whose record has been lost, and the
	// second case restores nothing while the hotel resolver stays in force.
	DnsmasqLayer bool `json:"dnsmasq_layer,omitempty"`
	// AdGuardHome upstream DNS backup
	AdGuardUpstream  []string `json:"adguard_upstream,omitempty"`
	AdGuardBootstrap []string `json:"adguard_bootstrap,omitempty"`
	AdGuardFallback  []string `json:"adguard_fallback,omitempty"`
	Time             int64    `json:"time"`
}

// NewCaptiveService creates a new CaptiveService with the given HTTP prober.
func NewCaptiveService(prober HTTPProber) *CaptiveService {
	return &CaptiveService{prober: prober, guardFile: captiveDNSGuardFile, stopCh: make(chan struct{})}
}

// NewCaptiveServiceWithUCI creates a CaptiveService with UCI access for DNS bypass.
func NewCaptiveServiceWithUCI(prober HTTPProber, u uci.UCI, cmd CommandRunner) *CaptiveService {
	svc := newCaptiveServiceWithGuard(prober, u, cmd, captiveDNSGuardFile)
	// Auto-restore stale bypass on startup. Tracked by the app lifecycle: Stop()
	// must be able to keep this goroutine from restoring DNS after the process
	// has begun shutting down.
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		svc.autoRestoreStaleBypass()
	}()
	return svc
}

// NewCaptiveServiceWithGuard is NewCaptiveServiceWithUCI with the bypass state
// in an explicit file. The guard file is what marks a bypass active, so a test
// outside this package cannot exercise the restore path at all without choosing
// where that state lives — and on a development machine the production path is
// not writable.
func NewCaptiveServiceWithGuard(prober HTTPProber, u uci.UCI, cmd CommandRunner, guardFile string) *CaptiveService {
	return newCaptiveServiceWithGuard(prober, u, cmd, guardFile)
}

func newCaptiveServiceWithGuard(prober HTTPProber, u uci.UCI, cmd CommandRunner, guardFile string) *CaptiveService {
	return &CaptiveService{prober: prober, uci: u, cmd: cmd, guardFile: guardFile, stopCh: make(chan struct{})}
}

// Stop cancels the startup auto-restore goroutine and waits for it to finish.
// Safe to call multiple times.
func (c *CaptiveService) Stop() {
	c.stopOnce.Do(func() {
		if c.stopCh != nil {
			close(c.stopCh)
		}
	})
	c.wg.Wait()
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
	return mutateUCI(c.uci, []string{"network", "dhcp"}, func() error {
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
		//
		// If it is unknown we must abort: proceeding would clear noresolv and drop
		// the configured server list, leaving dnsmasq with no usable upstream at all
		// (a strictly worse state than the one we are trying to escape). Bypass is
		// recoverable, a resolver with zero upstreams is not.
		hotelDNS := c.readDHCPDNS()
		if hotelDNS == "" {
			return fmt.Errorf("captive: no DHCP-provided DNS in %s; refusing to bypass (would leave dnsmasq with no upstream)", c.resolvConfPath())
		}

		// Read current AdGuardHome upstream config so we can restore it.
		agUpstream, agBootstrap, agFallback := c.readAdGuardUpstream()

		// Save current state (including AdGuardHome config)
		//
		// noresolv is recorded because the bypass owns that flag on the layer
		// path: it is what the layer writes (as 0). The resolver LIST is not
		// recorded on any path that does not take it — on the peerdns / AdGuard
		// paths the bypass leaves dnsmasq's server list alone, so a restore that
		// "restores" it would delete entries nobody recorded.
		backup := dnsBackup{
			PeerDNS:              wanPeerdns,
			DNS:                  wanDNS,
			DnsmasqNoResolv:      noresolv,
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
		if err := c.writeGuardFile(data); err != nil {
			return err
		}

		// --- Push the bypass onto the shared dnsmasq layer stack (ADR 0001 §4) ---
		// The bypass is a layer, not a second writer of dhcp's server/noresolv.
		// A snapshot outside the stack cannot see what is already stacked under
		// it, so its restore replaces the VPN or AdGuard resolvers with whatever
		// the bypass happened to find — and the next EnableLayer, finding no
		// record, then adopts the bypass's own value as the "pre-any-layer"
		// base. The stack records the real state below, so both survive.
		//
		// noresolv=0 is the point of the bypass: the upstream network's own
		// resolver is wanted, and the layer must not keep cutting off the
		// resolv.conf fallback the way a forwarding layer does.
		//
		// Only a dnsmasq that is actually blocking needs a layer. When dnsmasq
		// has no custom resolvers (the AdGuard-only bypass) it already follows
		// resolv.conf, which IS the hotel resolver, so a layer would rewrite
		// working configuration to no effect.
		//
		// rebind protection is not a resolver option and has no layer: it is
		// written here, and the stack's commit carries it when there is one.
		if noresolv == "1" {
			if rebindProtect == "1" {
				if err := c.setDnsmasqOption("rebind_protection", "0"); err != nil {
					_ = os.Remove(c.guardFile)
					return err
				}
			}
			if err := c.dnsmasqLayers().EnableLayerAs(dnsLayerCaptive, []string{hotelDNS}, "0"); err != nil {
				if rebindProtect == "1" {
					_ = c.setDnsmasqOption("rebind_protection", "1")
					c.commitDhcp()
				}
				_ = os.Remove(c.guardFile)
				return err
			}
			backup.DnsmasqLayer = true
			// The guard is written BEFORE the layer is pushed (the record must not
			// exist without a restore target for it), so the flag has to be
			// written out again now that the layer is really stacked. Without it
			// a lost layer record would be indistinguishable from a bypass that
			// never owned the resolver list, and RestoreDNS would report success
			// while the hotel resolver is still in force.
			if err := c.rewriteGuard(backup); err != nil {
				_, _ = c.dnsmasqLayers().RemoveLayer(dnsLayerCaptive, false)
				_ = os.Remove(c.guardFile)
				return err
			}
		} else if rebindProtect == "1" {
			if err := c.setDnsmasqOption("rebind_protection", "0"); err != nil {
				_ = os.Remove(c.guardFile)
				return err
			}
			if err := c.commitDhcp(); err != nil {
				_ = os.Remove(c.guardFile)
				return err
			}
			if c.cmd != nil {
				_, _ = c.cmd.Run("/etc/init.d/dnsmasq", "restart")
			}
		}
		if wanPeerdns == "0" {
			// The dnsmasq bypass is already committed and applied at this point, so
			// the guard is the ONLY record of the pre-bypass configuration. Deleting
			// it on a failure here would leave RestoreDNS with nothing to restore and
			// strand LAN DNS on the hotel resolver. A stale guard only means the
			// restore runs on the next check, so keeping it is the safe direction.
			if err := c.uci.Set("network", "wan", "peerdns", "1"); err != nil {
				return err
			}
			if err := c.uci.Set("network", "wan", "dns", ""); err != nil {
				return err
			}
			if err := c.uci.Commit("network"); err != nil {
				return err
			}
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

		log.Printf("captive: DNS bypassed (noresolv=%s, hotelDNS=%s)", noresolv, hotelDNS)
		return nil
	})
}

// rewriteGuard re-writes an already-persisted guard file. BypassDNS writes it
// before it mutates anything, so a field that is only known afterwards (the
// `dnsmasq_layer` flag) is recorded by writing the whole backup again.
func (c *CaptiveService) rewriteGuard(backup dnsBackup) error {
	data, err := json.Marshal(backup)
	if err != nil {
		return err
	}
	return c.writeGuardFile(data)
}

// RestoreDNS restores the original DNS config from the guard file.
// The guard file is removed ONLY when every step succeeded: it is the sole
// record of the pre-bypass configuration, so deleting it after a partial
// failure would make the original state unrecoverable. The same rule covers a
// restore that cannot determine the pre-bypass state at all — a bypass that is
// still in force must never be reported as restored.
func (c *CaptiveService) RestoreDNS() error {
	return mutateUCI(c.uci, []string{"network", "dhcp"}, func() error {
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
			// Keep the file: it is the only copy of the pre-bypass state.
			return fmt.Errorf("captive: guard file %s is unreadable, DNS not restored (remove it manually once recovered): %w", c.guardFile, err)
		}

		var errs []error
		fail := func(format string, args ...any) {
			errs = append(errs, fmt.Errorf(format, args...))
		}

		// Restore dnsmasq settings — always restore noresolv regardless of servers
		needsDhcpCommit := false
		// dnsmasq's resolver options belong to the layer stack (ADR 0001 §4).
		// Popping the bypass layer puts back whatever was stacked under it — the
		// VPN layer, the AdGuard layer, or the pre-any-layer base — which a
		// snapshot in this guard file could never know.
		layers := c.dnsmasqLayers()
		applied, layerErr := layers.RemoveLayer(dnsLayerCaptive, false)
		stacked, stackedErr := layers.hasAnyLayer()
		switch {
		case layerErr != nil || stackedErr != nil:
			// An unreadable layer record means dnsmasq may be owned by something
			// this guard knows nothing about. Fail closed: leave dnsmasq alone
			// rather than write over an owner it cannot see.
			fail("reading the dnsmasq layer record: %w", errors.Join(layerErr, stackedErr))
		case !applied && !stacked:
			// A guard file written before the stack existed (ADR 0001 §3.2): no
			// layer record to pop, and the dnsmasq fields it actually carries are
			// the only record of what to put back. With another layer stacked the
			// bypass was never recorded there either, and applying these values
			// would overwrite the layer that owns dnsmasq now.
			//
			// Every action below is gated on the field being present: the guard
			// may describe a bypass (wan peerdns, AdGuard upstreams) that never
			// touched dnsmasq's resolver list, and deleting a list it does not
			// hold can never be undone.
			if c.bypassOwnedResolvers(backup) && backup.DnsmasqServers == nil {
				// The bypass pushed the hotel resolver onto the stack, the record
				// that owned the pre-bypass list is gone, and the guard never held
				// the list. dnsmasq is therefore still forwarding to the hotel
				// resolver and nothing here can prove it is not. Writing only
				// noresolv would leave all DNS pointed at the portal's resolver
				// and then report success, so refuse: leave dnsmasq alone, keep the
				// guard for retry, and let the operator see it (ADR 0001 §4).
				fail("the dnsmasq layer record %s is gone, so the pre-bypass resolvers are unknown; "+
					"the captive bypass is still in force and dnsmasq was left untouched", layers.path)
				break
			}
			if backup.DnsmasqNoResolv != "" {
				if err := c.setDnsmasqOption("noresolv", backup.DnsmasqNoResolv); err != nil {
					fail("restoring dnsmasq noresolv: %w", err)
				}
				needsDhcpCommit = true
			}
			if backup.DnsmasqServers != nil {
				if err := c.deleteDnsmasqOption("server"); err != nil {
					fail("clearing dnsmasq server list: %w", err)
				}
				// A staged `uci delete` that is never committed is a landmine: the
				// next unrelated writer of dhcp (a VPN toggle, an AdGuard apply)
				// flushes it and the operator's resolvers vanish with no error.
				needsDhcpCommit = true
				for _, srv := range backup.DnsmasqServers {
					if err := c.addDnsmasqListItem("server", srv); err != nil {
						fail("restoring dnsmasq server %s: %w", srv, err)
					}
					needsDhcpCommit = true
				}
			}
		}
		if backup.DnsmasqRebindProtect == "1" {
			if err := c.setDnsmasqOption("rebind_protection", "1"); err != nil {
				fail("restoring dnsmasq rebind_protection: %w", err)
			}
			needsDhcpCommit = true
		}
		if needsDhcpCommit {
			if err := c.commitDhcp(); err != nil {
				fail("committing dnsmasq config: %w", err)
			}
		}
		// Restore wan settings
		if backup.PeerDNS != "" {
			if err := c.uci.Set("network", "wan", "peerdns", backup.PeerDNS); err != nil {
				fail("restoring wan peerdns: %w", err)
			}
		}
		if backup.DNS != "" {
			if err := c.uci.Set("network", "wan", "dns", backup.DNS); err != nil {
				fail("restoring wan dns: %w", err)
			}
		}
		if backup.PeerDNS != "" || backup.DNS != "" {
			if err := c.uci.Commit("network"); err != nil {
				fail("committing network config: %w", err)
			}
		}

		// Restore AdGuardHome upstream DNS
		if len(backup.AdGuardUpstream) > 0 {
			if err := c.setAdGuardUpstream(backup.AdGuardUpstream, backup.AdGuardBootstrap, backup.AdGuardFallback); err != nil {
				// AdGuard is a best-effort extra; a failure here leaves the guard
				// file in place so the next attempt (or the auto-restore) retries it.
				fail("restoring AdGuardHome upstream: %w", err)
			} else {
				log.Printf("captive: AdGuardHome upstream restored to %v", backup.AdGuardUpstream)
			}
		}

		if c.cmd != nil {
			if _, err := c.cmd.Run("/etc/init.d/dnsmasq", "restart"); err != nil {
				fail("restarting dnsmasq: %w", err)
			}
		}

		if len(errs) > 0 {
			return fmt.Errorf("captive: DNS restore incomplete (%d step(s) failed); %s kept for retry: %w",
				len(errs), c.guardFile, errors.Join(errs...))
		}

		if err := os.Remove(c.guardFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("captive: DNS restored but could not remove %s: %w", c.guardFile, err)
		}

		log.Printf("captive: DNS restored")
		return nil
	})
}

// writeGuardFile writes the DNS backup atomically (temp file + rename).
// BypassDNS/RestoreDNS and refreshBypassTimestamp all target the same file
// under c.mu; a truncating O_TRUNC-style write would let a concurrent reader
// observe a half-written JSON document, which fails to unmarshal and used to
// delete the only record of the pre-bypass state.
func (c *CaptiveService) writeGuardFile(data []byte) error {
	dir := filepath.Dir(c.guardFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(c.guardFile)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		return err
	}
	return os.Rename(tmpName, c.guardFile)
}

// bypassOwnedResolvers reports whether this guard describes a bypass that put
// dnsmasq's resolver list under its own control — i.e. one that pushed the
// `captive` layer and therefore handed ownership of the list to the layer
// record. A guard written before the stack existed carries no flag, so a
// recorded noresolv=1 with no recorded list is the same shape and is treated
// the same way: conservatively, as a bypass whose resolvers are unaccounted
// for.
func (c *CaptiveService) bypassOwnedResolvers(backup dnsBackup) bool {
	return backup.DnsmasqLayer || backup.DnsmasqNoResolv == "1"
}

// resolvConfPath returns the path parsed for the DHCP-provided nameserver.
func (c *CaptiveService) resolvConfPath() string {
	if c.resolvConfAutoOverride != "" {
		return c.resolvConfAutoOverride
	}
	return resolvConfAuto
}

// readDHCPDNS parses the first nameserver line from resolv.conf.auto —
// this is the DNS server handed out by the upstream network via DHCP.
func (c *CaptiveService) readDHCPDNS() string {
	path := c.resolvConfPath()
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if after, ok := strings.CutPrefix(line, "nameserver "); ok {
			ip := strings.TrimSpace(after)
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

// dnsmasqLayers returns this service's view of the shared stack.
func (c *CaptiveService) dnsmasqLayers() *dnsmasqLayerStackFile {
	path := c.dnsStackPath
	if path == "" && c.guardFile != "" {
		// /etc/trafo in production: the same file the VPN and AdGuard paths use,
		// derived rather than hard-coded so a test service pointed at a temp
		// guard file cannot reach the real record.
		path = filepath.Join(filepath.Dir(c.guardFile), dnsmasqLayerStackFileName)
	}
	return &dnsmasqLayerStackFile{dns: commandRunnerDNS{cmd: c.cmd}, path: path}
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
// deleteDnsmasqOption removes a dnsmasq option. A missing option is not a
// failure: the AdGuard-only bypass path never creates a `server` list, and
// treating "Entry not found" as an error made RestoreDNS fail on every attempt,
// which kept the guard forever and left IsDNSBypassed stuck true.
func (c *CaptiveService) deleteDnsmasqOption(option string) error {
	if c.cmd == nil {
		return nil
	}
	out, err := c.cmd.Run("uci", "delete", "dhcp.@dnsmasq[0]."+option)
	if err != nil && strings.Contains(string(out)+err.Error(), "Entry not found") {
		return nil
	}
	return err
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
	// wait for startup, but give up immediately on shutdown
	select {
	case <-time.After(10 * time.Second):
	case <-c.stopCh:
		return
	}
	data, err := os.ReadFile(c.guardFile)
	if err != nil {
		return
	}
	var backup dnsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		// A corrupt guard file is the only record of the pre-bypass DNS config.
		// Keep it (ADR 0003) instead of deleting it, and let an operator decide.
		log.Printf("captive: guard file %s is unreadable (%v); leaving it in place for manual recovery", c.guardFile, err)
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
// It takes c.mu (the same lock BypassDNS/RestoreDNS hold) and rewrites the file
// atomically, so it can never race a concurrent bypass/restore into producing a
// truncated, unreadable guard file.
func (c *CaptiveService) refreshBypassTimestamp() {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.guardFile)
	if err != nil {
		return
	}
	var backup dnsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		log.Printf("captive: cannot refresh bypass timestamp: %s unreadable: %v", c.guardFile, err)
		return
	}
	backup.Time = time.Now().Unix()
	newData, err := json.Marshal(backup)
	if err != nil {
		return
	}
	if err := c.writeGuardFile(newData); err != nil {
		log.Printf("captive: cannot refresh bypass timestamp: %v", err)
	}
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
