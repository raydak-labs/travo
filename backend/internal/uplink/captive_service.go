package uplink

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// captiveDNSGuardFeature names the guard that stores the original DNS config
// while the bypass is active.
const captiveDNSGuardFeature = "captive-dns"

// captiveDNSRestoreTimeout auto-restores DNS if bypass has been active too long.
const captiveDNSRestoreTimeout = 5 * time.Minute

// CaptiveService checks for captive portal detection.
type CaptiveService struct {
	prober   platform.HTTPProber
	uci      uci.UCI
	cmd      platform.CommandRunner
	mu       sync.Mutex
	dnsGuard guard.Guard
	// test overrides (empty in production)
	adguardAPIBaseOverride string
	resolvConfAutoOverride string
	// dnsStackPath is the shared dnsmasq resolver layer record this service
	// stacks its bypass on. Empty means "next to the guard file", which is
	// /etc/trafo in production (the same directory the record's own constant
	// names) and a temp dir in tests.
	dnsStackPath string
	// stopCh cancels the startup auto-restore goroutine so a shutdown does not
	// race with a DNS restore.
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
	// wan-level DNS (the peerdns=0 blocking path)
	PeerDNS string `json:"peerdns,omitempty"`
	DNS     string `json:"dns,omitempty"`
	// dnsmasq-level settings (the noresolv blocking path)
	DnsmasqNoResolv      string `json:"dnsmasq_noresolv,omitempty"`
	DnsmasqRebindProtect string `json:"dnsmasq_rebind_protection,omitempty"`
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
func NewCaptiveService(prober platform.HTTPProber) *CaptiveService {
	return &CaptiveService{prober: prober, dnsGuard: guard.New(captiveDNSGuardFeature), stopCh: make(chan struct{})}
}

// NewCaptiveServiceWithUCI creates a CaptiveService with UCI access for DNS bypass.
func NewCaptiveServiceWithUCI(prober platform.HTTPProber, u uci.UCI, cmd platform.CommandRunner) *CaptiveService {
	svc := newCaptiveServiceWithGuard(prober, u, cmd, guard.New(captiveDNSGuardFeature))
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
// in an explicit guard and without the startup auto-restore.
func NewCaptiveServiceWithGuard(prober platform.HTTPProber, u uci.UCI, cmd platform.CommandRunner, g guard.Guard) *CaptiveService {
	return newCaptiveServiceWithGuard(prober, u, cmd, g)
}

func newCaptiveServiceWithGuard(prober platform.HTTPProber, u uci.UCI, cmd platform.CommandRunner, g guard.Guard) *CaptiveService {
	return &CaptiveService{prober: prober, uci: u, cmd: cmd, dnsGuard: g, stopCh: make(chan struct{})}
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
	statusCode, _, redirectURL, err := c.prober.Do(platform.ConnectivityProbeURL)

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
				portalURL = platform.ConnectivityProbeURL
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
		fallback := platform.ConnectivityProbeURL
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
	return c.dnsGuard.Present()
}

// BypassDNS temporarily switches DNS to the DHCP-provided gateway DNS so the
// captive portal login page can be resolved.  It patches both dnsmasq (for any
// local consumers) and AdGuardHome (which is the actual port-53 resolver for
// LAN clients).  Original config is stored in the guard file for restoration.
func (c *CaptiveService) BypassDNS() error {
	return platform.MutateUCI(c.uci, []string{"network", "dhcp"}, func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.uci == nil {
			return nil // no UCI = mock mode, noop
		}

		// Already bypassed?
		if c.dnsGuard.Present() {
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
		if err := c.dnsGuard.Write(string(data)); err != nil {
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
					_ = c.dnsGuard.Clear()
					return err
				}
			}
			if err := c.dnsmasqLayers().EnableLayerAs(dnsLayerCaptive, []string{hotelDNS}, "0"); err != nil {
				if rebindProtect == "1" {
					_ = c.setDnsmasqOption("rebind_protection", "1")
					_ = c.commitDhcp()
				}
				_ = c.dnsGuard.Clear()
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
				_ = c.dnsGuard.Clear()
				return err
			}
		} else if rebindProtect == "1" {
			if err := c.setDnsmasqOption("rebind_protection", "0"); err != nil {
				_ = c.dnsGuard.Clear()
				return err
			}
			if err := c.commitDhcp(); err != nil {
				_ = c.dnsGuard.Clear()
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
				slog.Warn("captive: could not update AdGuardHome upstream", "err", err)
				// Non-fatal: dnsmasq changes are still in effect
			} else {
				slog.Info("captive: AdGuardHome upstream switched", "dns", hotelDNS)
			}
		}

		slog.Info("captive: DNS bypassed", "noresolv", noresolv, "dns", hotelDNS)
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
	return c.dnsGuard.Write(string(data))
}

// RestoreDNS restores the original DNS config from the guard file.
// The guard file is removed ONLY when every step succeeded: it is the sole
// record of the pre-bypass configuration, so deleting it after a partial
// failure would make the original state unrecoverable. The same rule covers a
// restore that cannot determine the pre-bypass state at all — a bypass that is
// still in force must never be reported as restored.
func (c *CaptiveService) RestoreDNS() error {
	return platform.MutateUCI(c.uci, []string{"network", "dhcp"}, func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.uci == nil {
			return nil
		}

		data, err := c.dnsGuard.Read()
		if err != nil {
			return nil // no guard file = nothing to restore
		}

		var backup dnsBackup
		if err := json.Unmarshal(data, &backup); err != nil {
			// Keep the file: it is the only copy of the pre-bypass state.
			return fmt.Errorf("captive: guard file %s is unreadable, DNS not restored (remove it manually once recovered): %w", c.dnsGuard.Path(), err)
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
			// No layer record to pop. Either the bypass never owned the
			// resolver list (wan peerdns, AdGuard upstreams), or it did and the
			// record is gone, in which case dnsmasq still forwards to the hotel
			// resolver and nothing here can say what to put back: refuse, keep
			// the guard for retry, and let the operator see it (ADR 0001 §4).
			if c.bypassOwnedResolvers(backup) {
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
				slog.Info("captive: AdGuardHome upstream restored", "upstream", backup.AdGuardUpstream)
			}
		}

		if c.cmd != nil {
			if _, err := c.cmd.Run("/etc/init.d/dnsmasq", "restart"); err != nil {
				fail("restarting dnsmasq: %w", err)
			}
		}

		if len(errs) > 0 {
			return fmt.Errorf("captive: DNS restore incomplete (%d step(s) failed); %s kept for retry: %w",
				len(errs), c.dnsGuard.Path(), errors.Join(errs...))
		}

		if err := c.dnsGuard.Clear(); err != nil {
			return fmt.Errorf("captive: DNS restored but could not remove %s: %w", c.dnsGuard.Path(), err)
		}

		slog.Info("captive: DNS restored")
		return nil
	})
}

// bypassOwnedResolvers reports whether this guard describes a bypass that put
// dnsmasq's resolver list under the `captive` layer. noresolv=1 without the
// flag is the crash window between writing the guard and pushing the layer,
// and is treated the same way.
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

// autoRestoreStaleBypass restores DNS if guard file is older than timeout.
func (c *CaptiveService) autoRestoreStaleBypass() {
	// wait for startup, but give up immediately on shutdown
	select {
	case <-time.After(10 * time.Second):
	case <-c.stopCh:
		return
	}
	data, err := c.dnsGuard.Read()
	if err != nil {
		return
	}
	var backup dnsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		// A corrupt guard file is the only record of the pre-bypass DNS config.
		// Keep it (ADR 0003) instead of deleting it, and let an operator decide.
		slog.Error("captive: guard is unreadable; left in place for manual recovery", "guard", c.dnsGuard.Path(), "err", err)
		return
	}
	age := time.Since(time.Unix(backup.Time, 0))
	if age > captiveDNSRestoreTimeout {
		slog.Info("captive: auto-restoring a stale DNS bypass", "age", age)
		_ = c.RestoreDNS()
	}
}

// refreshBypassTimestamp updates the guard file timestamp to prevent stale auto-restore.
// It takes c.mu (the same lock BypassDNS/RestoreDNS hold) and rewrites the file
// atomically, so it can never race a concurrent bypass/restore into producing a
// truncated, unreadable guard file.
func (c *CaptiveService) refreshBypassTimestamp() {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := c.dnsGuard.Read()
	if err != nil {
		return
	}
	var backup dnsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		slog.Warn("captive: cannot refresh bypass timestamp; guard unreadable", "guard", c.dnsGuard.Path(), "err", err)
		return
	}
	backup.Time = time.Now().Unix()
	newData, err := json.Marshal(backup)
	if err != nil {
		return
	}
	if err := c.dnsGuard.Write(string(newData)); err != nil {
		slog.Warn("captive: cannot refresh bypass timestamp", "err", err)
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
