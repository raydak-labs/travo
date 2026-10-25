package services

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// VpnService provides VPN status and configuration.
type VpnService struct {
	uci          uci.UCI
	cmd          platform.CommandRunner
	profilesPath string // Path to wireguard_profiles.json
	crashGuard   guard.Guard
	dnsStackPath string

	// dnsHealMu guards the self-heal debounce below. GET /vpn/status is polled
	// by several pages at once, so the counters are shared.
	dnsHealMu sync.Mutex
	// dnsHealStreak counts consecutive enabled_not_up readings and dnsHealSince
	// stamps the first of them, so a heal needs the tunnel down for a whole
	// grace period rather than for a single reading.
	dnsHealStreak int
	dnsHealSince  time.Time
}

// NewVpnService creates a new VpnService with a real command runner.
func NewVpnService(u uci.UCI) *VpnService {
	return &VpnService{
		uci:          u,
		cmd:          &platform.RealCommandRunner{},
		profilesPath: "/etc/travo/wireguard_profiles.json",
		crashGuard:   guard.New("vpn"),
		dnsStackPath: dnsmasqLayerStackPath,
	}
}

// NewVpnServiceWithRunner creates a new VpnService with a custom command runner (for tests).
func NewVpnServiceWithRunner(u uci.UCI, cmd platform.CommandRunner) *VpnService {
	return &VpnService{
		uci:          u,
		cmd:          cmd,
		profilesPath: "/etc/travo/wireguard_profiles.json",
		crashGuard:   guard.New("vpn"),
		dnsStackPath: dnsmasqLayerStackPath,
	}
}

// NewVpnServiceWithProfilesPath creates a VpnService with a custom profiles path (for tests).
func NewVpnServiceWithProfilesPath(u uci.UCI, cmd platform.CommandRunner, profilesPath string) *VpnService {
	return &VpnService{
		uci:          u,
		cmd:          cmd,
		profilesPath: profilesPath,
		crashGuard:   guard.New("vpn"),
		dnsStackPath: dnsmasqLayerStackPath,
	}
}

func (v *VpnService) reloadFirewall() {
	_, _ = v.cmd.Run("/etc/init.d/firewall", "reload")
}

// Settle delays after netifd operations that expose no observable "done"
// condition — the subsequent runtime probe is the actual readiness check.
const (
	wireGuardReloadSettle = 300 * time.Millisecond
	wireGuardRetrySettle  = 400 * time.Millisecond
)

func (v *VpnService) enableWireguard() error {
	if err := v.ensureWireGuardInterface(); err != nil {
		return fmt.Errorf("normalizing wg0 interface: %w", err)
	}
	if err := v.ensureWireGuardPeer("wg0_peer0"); err != nil {
		return fmt.Errorf("normalizing wg0 peer: %w", err)
	}
	if err := v.validateWireGuardConfigForEnable(); err != nil {
		return err
	}
	prevDisabled, _ := v.uci.Get("network", "wg0", "disabled")
	if err := v.crashGuard.Mark(); err != nil {
		return err
	}
	if err := v.uci.Set("network", "wg0", "disabled", "0"); err != nil {
		_ = v.crashGuard.Clear()
		return err
	}
	if err := v.uci.Commit("network"); err != nil {
		_ = v.crashGuard.Clear()
		return err
	}

	// Verify the tunnel before committing anything that depends on it.
	if err := v.applyAndVerifyWireGuard(); err != nil {
		if rbErr := v.rollbackWireguardEnable(prevDisabled); rbErr != nil {
			// The device is in an unknown state: keep the crash guard so no
			// other flow mutates it before a human looks.
			return fmt.Errorf("WireGuard enabled in UCI but tunnel failed to start: %w (rollback incomplete, crash guard %s kept: %w)", err, v.crashGuard.Path(), rbErr)
		}
		_ = v.crashGuard.Clear()
		return fmt.Errorf("WireGuard enabled in UCI but tunnel failed to start: %w", err)
	}

	// Clear the Tailscale exit node only once the tunnel is verified up, so a
	// failed enable never changes where the router's own traffic egresses.
	_, _ = v.cmd.Run(tailscaleBin(), "set", "--exit-node=")

	if err := v.setupWireGuardFirewall(); err != nil {
		if rbErr := v.rollbackWireguardEnable(prevDisabled); rbErr != nil {
			return fmt.Errorf("setting up WireGuard firewall: %w (rollback incomplete, crash guard %s kept: %w)", err, v.crashGuard.Path(), rbErr)
		}
		_ = v.crashGuard.Clear()
		return fmt.Errorf("setting up WireGuard firewall: %w", err)
	}

	if err := v.enableVpnDNSForwarding(); err != nil {
		// DNS forwarding was aborted on purpose (the dnsmasq layer record
		// could not be written), so dnsmasq still has the pre-VPN resolvers.
		// The tunnel is up and verified; failing the toggle here would tear
		// down a working VPN over a DNS bookkeeping problem. Record it.
		slog.Warn("vpn: tunnel up but LAN DNS left unchanged", "err", err)
	}
	_ = v.crashGuard.Clear()
	return nil
}

// rollbackWireguardEnable restores the pre-enable state: dependent firewall/DNS
// changes are reverted and wg0 is disabled again.
func (v *VpnService) rollbackWireguardEnable(prevDisabled string) error {
	var errs []error
	if err := v.teardownWireGuardFirewall(); err != nil {
		errs = append(errs, fmt.Errorf("tearing down WireGuard firewall: %w", err))
	}
	v.disableVpnDNSForwarding()
	disabled := strings.TrimSpace(prevDisabled)
	if disabled == "" {
		disabled = "1"
	}
	if err := v.uci.Set("network", "wg0", "disabled", disabled); err != nil {
		errs = append(errs, fmt.Errorf("restoring wg0 disabled=%s: %w", disabled, err))
	}
	if err := v.uci.Commit("network"); err != nil {
		errs = append(errs, fmt.Errorf("committing network: %w", err))
	}
	_, _ = v.cmd.Run(platform.IfdownBin, "wg0")
	return errors.Join(errs...)
}

func (v *VpnService) disableWireguard() error {
	// The disable path bounces uplinks (ubus down/up, then ifdown/ifup as a last
	// resort), so it needs the crash guard like the enable path (ADR 0003).
	if err := v.crashGuard.Mark(); err != nil {
		return err
	}
	// Every exit path tears the wg0 firewall plumbing down, so no stale
	// wg0_zone/wg0_fwd survives a reboot into the next enable.
	//
	// The guard is cleared inside this defer, after the teardown, so a device
	// that dies mid-teardown still has the marker. routeConfirmed is false on
	// every error return, which keeps the guard when the uplink bounce may not
	// have taken.
	routeConfirmed := false
	defer func() {
		if err := v.teardownWireGuardFirewall(); err != nil {
			slog.Warn("vpn: tearing down WireGuard firewall", "err", err)
		}
		if routeConfirmed {
			_ = v.crashGuard.Clear()
		}
	}()

	_ = v.uci.Set("network", "wg0", "disabled", "1")
	if err := v.uci.Commit("network"); err != nil {
		return err
	}
	// Bring down the interface and clean up firewall plumbing when disabling.
	// Only a kill switch this service owns is removed: a kill switch the user
	// set up as a standalone policy must survive a VPN toggle.
	_ = v.removeVPNOwnedKillSwitch()
	_, _ = v.cmd.Run(platform.IfdownBin, "wg0")
	v.disableVpnDNSForwarding()
	// Netifd-managed recovery: routes/DNS should be recomputed without wg0. On some
	// OpenWrt/netifd states, wg0 teardown can leave the kernel without any default
	// route even though the uplink interface still shows “up”. We recover by
	// restoring the uplink default route (renew first, then down/up if needed).
	_, _ = v.cmd.Run(openwrtUbusBin, "call", "network", "reload")
	time.Sleep(150 * time.Millisecond)
	v.restoreDefaultRouteAfterWireGuardDisable()
	// Rock-solid semantics: do not report success if the device has no default route.
	if !v.hasKernelDefaultRoute() {
		// Guard stays (routeConfirmed is false): the uplink bounce may not
		// have taken, and the operator needs the marker (ADR 0003).
		return fmt.Errorf("WireGuard disabled but no default route was restored; internet may be down")
	}
	routeConfirmed = true
	return nil
}

// The DNS self-heal debounce.
//
// GET /vpn/status is polled by the dashboard and the VPN page, so the heal runs
// on a timer the operator does not control. `wg show wg0 dump` failing once — a
// busy router, a network flap — reports enabled_not_up even though the tunnel
// is healthy, and healing on that single reading rewrote dnsmasq back to the
// pre-VPN resolvers AND dropped the snapshot, so nothing put VPN DNS back until
// the operator toggled the tunnel off and on again.
//
// The debounce therefore requires the down state to be observed
// vpnDNSHealStreakRequired times IN A ROW and to have persisted for at least
// vpnDNSHealGracePeriod. Any other reading — connected, configured,
// up_no_handshake, or disabled — resets the streak, so one healthy poll in
// between is enough to prove the failure was transient. These are variables so
// tests can compress the window instead of waiting it out.
var (
	vpnDNSHealStreakRequired = 3
	vpnDNSHealGracePeriod    = 2 * time.Minute
)

// Config sets for the VPN flows.
//
// vpnFirewallConfigs is the standalone firewall entry points (kill switch and the
// firewall helpers called cold).
//
// vpnFlowConfigs is what a full VPN toggle actually touches, and it is more than
// the `network` writes suggest:
//   - network:  the wg0 interface, peer, addresses and split-tunnel allowed IPs
//   - firewall: the wg0 zone/forwarding rule and the toggle-owned kill switch
//   - dhcp:     enableVpnDNSForwarding / disableVpnDNSForwarding rewrite
//     dnsmasq's server list and noresolv
//
// dhcp was the one nobody listed: those two helpers shell out to `uci` directly
// rather than going through the UCI interface, so they were invisible to every
// audit of "which configs does this write" and held no lock at all. A WiFi save
// committing `dhcp` while a VPN toggle was rewriting the resolver list is exactly
// the corruption these locks exist to prevent.
var (
	vpnFirewallConfigs = []string{"firewall"}
	vpnFlowConfigs     = []string{"dhcp", "firewall", "network"}
)

// setupWireGuardFirewall ensures the wg0 firewall zone and lan→wg0 forwarding rule
// exist in UCI and commits the firewall config. Called when activating a WireGuard profile.
// setupWireGuardFirewall ensures the wg0 firewall zone and lan->wg0 forwarding
// rule exist and commits. It does NOT take a config lock: every caller is inside
// the VPN transaction, which already holds `firewall`, and the config locks are
// not reentrant. TestVPNFlowsDoNotNestConfigLocks fails the build if that ever
// stops being true.
func (v *VpnService) setupWireGuardFirewall() error {
	{
		// Ensure the wg0 zone exists.
		if _, err := v.uci.GetAll("firewall", "wg0_zone"); err != nil {
			if addErr := v.uci.AddSection("firewall", "wg0_zone", "zone"); addErr != nil {
				return fmt.Errorf("creating wg0 firewall zone: %w", addErr)
			}
		}
		_ = v.uci.Set("firewall", "wg0_zone", "name", "wg0")
		_ = v.uci.Set("firewall", "wg0_zone", "network", "wg0")
		_ = v.uci.Set("firewall", "wg0_zone", "input", "DROP")
		_ = v.uci.Set("firewall", "wg0_zone", "output", "ACCEPT")
		_ = v.uci.Set("firewall", "wg0_zone", "forward", "DROP")
		_ = v.uci.Set("firewall", "wg0_zone", "masq", "1")
		_ = v.uci.Set("firewall", "wg0_zone", "mtu_fix", "1")

		// Ensure lan→wg0 forwarding exists.
		if _, err := v.uci.GetAll("firewall", "wg0_fwd"); err != nil {
			if addErr := v.uci.AddSection("firewall", "wg0_fwd", "forwarding"); addErr != nil {
				return fmt.Errorf("creating wg0 forwarding rule: %w", addErr)
			}
		}
		_ = v.uci.Set("firewall", "wg0_fwd", "src", "lan")
		_ = v.uci.Set("firewall", "wg0_fwd", "dest", "wg0")

		if err := v.uci.Commit("firewall"); err != nil {
			return err
		}
		v.reloadFirewall()
		return nil
	}
}

// teardownWireGuardFirewall removes the wg0 firewall zone and forwarding rule from UCI.
// Called when deactivating WireGuard. Errors are non-fatal (section may not exist).
// teardownWireGuardFirewall removes the wg0 zone and forwarding rule. It does
// NOT take a config lock; see setupWireGuardFirewall.
func (v *VpnService) teardownWireGuardFirewall() error {
	_ = v.uci.DeleteSection("firewall", "wg0_zone")
	_ = v.uci.DeleteSection("firewall", "wg0_fwd")
	if err := v.uci.Commit("firewall"); err != nil {
		return err
	}
	v.reloadFirewall()
	return nil
}

// VerifyWireGuard checks the health of the WireGuard tunnel:
// interface state, recent handshake, default route, and firewall plumbing.
func (v *VpnService) VerifyWireGuard() models.VPNVerifyResult {
	result := models.VPNVerifyResult{}

	// Check if wg0 interface is up.
	out, err := v.cmd.Run(platform.IPBin, "link", "show", "dev", "wg0")
	if err == nil && wireGuardIfaceLooksUp(string(out)) {
		result.InterfaceUp = true
	}

	// Check latest handshake from wg show dump.
	dumpOut, dumpErr := v.cmd.Run(openwrtWgBin, "show", "wg0", "dump")
	if dumpErr == nil {
		if status, err := ParseWgDump(string(dumpOut)); err == nil && status != nil {
			var latest int64
			for _, peer := range status.Peers {
				if peer.LatestHandshake > latest {
					latest = peer.LatestHandshake
				}
			}
			result.LatestHandshake = latest
			if latest > 0 && time.Now().Unix()-latest < 180 {
				result.HandshakeOk = true
			}
		}
	}

	// Check default route via wg0.
	routeOut, routeErr := v.cmd.Run(platform.IPBin, "route", "show", "default")
	route6Out, _ := v.cmd.Run(platform.IPBin, "-6", "route", "show", "default")
	combined := string(routeOut) + "\n" + string(route6Out)
	if routeErr == nil && strings.Contains(combined, "wg0") {
		result.RouteOk = true
	}

	// Check firewall plumbing in UCI.
	if opts, err := v.uci.GetAll("firewall", "wg0_zone"); err == nil && opts["name"] == "wg0" {
		result.FirewallZoneOk = true
	}
	if opts, err := v.uci.GetAll("firewall", "wg0_fwd"); err == nil && opts["src"] == "lan" && opts["dest"] == "wg0" {
		result.ForwardingOk = true
	}

	return result
}

// ImportWireguardConfig parses a .conf file, normalizes the UCI structure,
// applies the config, and verifies the tunnel comes up. It holds the UCI locks
// for every config the VPN flow touches (vpnFlowConfigs).
func (v *VpnService) ImportWireguardConfig(confContent string) error {
	return platform.MutateUCI(v.uci, vpnFlowConfigs, func() error {
		return v.importWireguardConfigLocked(confContent)
	})
}

// importWireguardConfigLocked is ImportWireguardConfig for a caller that holds the locks.
func (v *VpnService) importWireguardConfigLocked(confContent string) error {
	parsed, err := ParseWireguardConfig(confContent)
	if err != nil {
		return err
	}

	// Normalize wg0 UCI structure before writing values.
	if err := v.ensureWireGuardInterface(); err != nil {
		return fmt.Errorf("normalizing wg0 interface: %w", err)
	}

	_ = v.uci.Set("network", "wg0", "private_key", parsed.Interface.PrivateKey)
	if parsed.Interface.Address != "" {
		if err := v.setWireGuardAddresses(parsed.Interface.Address); err != nil {
			return err
		}
	}
	if parsed.Interface.DNS != "" {
		_ = v.uci.Set("network", "wg0", "dns", parsed.Interface.DNS)
	}
	if parsed.Interface.ListenPort > 0 {
		_ = v.uci.Set("network", "wg0", "listen_port", strconv.Itoa(parsed.Interface.ListenPort))
	}
	if parsed.Interface.MTU > 0 {
		_ = v.uci.Set("network", "wg0", "mtu", strconv.Itoa(parsed.Interface.MTU))
	}

	for i, peer := range parsed.Peers {
		section := fmt.Sprintf("wg0_peer%d", i)
		if err := v.applyWireGuardPeerParsed(section, peer); err != nil {
			return fmt.Errorf("applying peer section %s: %w", section, err)
		}
	}

	return v.uci.Commit("network")
}

const wireGuardSpeedTestURL = "http://speedtest.tele2.net/1MB.zip"

// wireGuardIPv4Address returns the first IPv4 address assigned to wg0 (from `ip -4 -o addr show`).
func (v *VpnService) wireGuardIPv4Address() (string, error) {
	out, err := v.cmd.Run(platform.IPBin, "-4", "-o", "addr", "show", "dev", "wg0")
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		for i := range fields {
			if fields[i] == "inet" && i+1 < len(fields) {
				addr := fields[i+1]
				if j := strings.Index(addr, "/"); j > 0 {
					return addr[:j], nil
				}
				return addr, nil
			}
		}
	}
	return "", fmt.Errorf("no inet address on wg0")
}

// RunWireGuardSpeedTest measures download throughput and ping latency with traffic bound to the
// WireGuard interface (wget --bind-address, ping -I wg0). Requires wg0 enabled and up with an IPv4.
func (v *VpnService) RunWireGuardSpeedTest() (models.SpeedTestResult, error) {
	result := models.SpeedTestResult{Server: "tele2.net via WireGuard (wget --bind-address)"}

	opts, err := v.uci.GetAll("network", "wg0")
	if err != nil {
		return result, fmt.Errorf("wireguard is not configured")
	}
	if opts["disabled"] == "1" {
		return result, fmt.Errorf("wireguard is disabled — enable the tunnel before running a VPN speed test")
	}

	linkOut, err := v.cmd.Run(platform.IPBin, "link", "show", "dev", "wg0")
	if err != nil || !wireGuardIfaceLooksUp(string(linkOut)) {
		return result, fmt.Errorf("wireguard interface wg0 is not up")
	}

	bindIP, err := v.wireGuardIPv4Address()
	if err != nil {
		return result, fmt.Errorf("could not read IPv4 on wg0: %w", err)
	}
	if bindIP == "" {
		return result, fmt.Errorf("no IPv4 address on wg0")
	}

	start := time.Now()
	_, werr := v.cmd.Run("wget", "-O", "/dev/null", "--timeout=15",
		"--no-check-certificate", "--bind-address="+bindIP,
		wireGuardSpeedTestURL)
	elapsed := time.Since(start).Seconds()
	if werr == nil && elapsed > 0 {
		result.DownloadMbps = (1024 * 1024 * 8) / elapsed / 1e6
	}

	pingOut, perr := v.cmd.Run("ping", "-I", "wg0", "-c", "4", "-W", "3", "8.8.8.8")
	if perr == nil {
		for line := range strings.SplitSeq(string(pingOut), "\n") {
			if strings.Contains(line, "avg") {
				parts := strings.Split(line, "/")
				if len(parts) >= 5 {
					if ms, err2 := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64); err2 == nil {
						result.PingMs = ms
					}
				}
			}
		}
	}

	return result, nil
}

// RunDNSLeakTest checks whether the router's effective DNS upstream (what dnsmasq
// uses for LAN clients) matches the VPN-configured DNS when WireGuard is active.
// On OpenWrt, /etc/resolv.conf usually lists only 127.0.0.1 (local dnsmasq) while
// actual upstreams are in dhcp.@dnsmasq[0].server — we merge those so the test is
// not a false positive when VPN DNS forwarding is applied.
func (v *VpnService) RunDNSLeakTest() models.DNSLeakResult {
	result := models.DNSLeakResult{}

	// 1. Effective upstream nameservers (resolv.conf + dnsmasq when resolv is loopback-only).
	servers, err := v.dnsmasqLayers().dns.getServers()
	if err != nil {
		servers = nil
	}
	dnsmasqServers := servers
	result.Nameservers = effectiveNameserversForMerge(readResolvConfNameservers(), dnsmasqServers)

	// 2. Check VPN status.
	if opts, err := v.uci.GetAll("network", "wg0"); err == nil {
		result.VPNActive = opts["disabled"] != "1"
	}

	// 3. Read VPN DNS servers from WireGuard UCI config.
	if dns, err := v.uci.Get("network", "wg0", "dns"); err == nil && dns != "" {
		result.VPNDNSServers = splitWireGuardDNSOption(dns)
	}

	// 4. Check for potential leak: VPN active but effective dnsmasq upstreams do not
	// include WireGuard DNS (e.g. still forwarding only to AdGuard).
	if result.VPNActive && len(result.VPNDNSServers) > 0 {
		vpnDNSSet := make(map[string]bool, len(result.VPNDNSServers))
		for _, s := range result.VPNDNSServers {
			vpnDNSSet[s] = true
		}
		leaking := true
		for _, ns := range result.Nameservers {
			if vpnDNSSet[ns] {
				leaking = false
				break
			}
		}
		result.PotentiallyLeaking = leaking
	}

	return result
}

func isLoopbackNameserver(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	return s == "127.0.0.1" || s == "::1" || s == "0:0:0:0:0:0:0:1"
}

// dnsmasqServerAddrToIP strips the #port suffix used by dnsmasq (e.g. 127.0.0.1#5353).
func dnsmasqServerAddrToIP(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "#"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// effectiveNameserversForMerge returns upstream DNS nameservers used for leak checks.
// When resolv.conf only lists the local dnsmasq stub (127.0.0.1 / ::1), upstream
// IPs come from dnsmasq server= list instead.
func effectiveNameserversForMerge(resolv []string, dnsmasqServers []string) []string {
	allLoopback := len(resolv) > 0
	for _, ns := range resolv {
		if !isLoopbackNameserver(ns) {
			allLoopback = false
			break
		}
	}
	if len(resolv) == 0 {
		allLoopback = true
	}
	if !allLoopback {
		return resolv
	}

	var out []string
	for _, s := range dnsmasqServers {
		ip := dnsmasqServerAddrToIP(s)
		if ip != "" {
			out = append(out, ip)
		}
	}
	if len(out) > 0 {
		return out
	}
	return resolv
}

// readResolvConfNameservers parses nameserver lines from /etc/resolv.conf.
// Overridable in tests (OpenWrt stub resolver vs dnsmasq upstream merge).
var readResolvConfNameservers = func() []string {
	return readResolvConfNameserversFromPath("/etc/resolv.conf")
}

// readResolvConfNameserversFromPath parses nameserver lines from the given file.
func readResolvConfNameserversFromPath(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var servers []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "nameserver "); ok {
			ns := strings.TrimSpace(after)
			if ns != "" {
				servers = append(servers, ns)
			}
		}
	}
	return servers
}
