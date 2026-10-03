package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

const openwrtIPBin = "/sbin/ip"
const openwrtWgBin = "/usr/bin/wg"
const openwrtUbusBin = "/sbin/ubus"
const openwrtIfupBin = "/sbin/ifup"
const openwrtIfdownBin = "/sbin/ifdown"
const openwrtTailscaleBin = "/usr/sbin/tailscale"
const openwrtTailscaleBinAlt = "/usr/bin/tailscale"

var wireGuardVerifyTimeout = 12 * time.Second

// tailscaleInitScript is the procd service for the tailscale package.
const tailscaleInitScript = "/etc/init.d/tailscale"

// ErrTailscaleNotInstalled is returned when the tailscale package is absent, so
// none of its operations can run. Handlers map this to 503.
//
// Without it, every Tailscale mutation surfaced the raw exec error
// ("exec: \"tailscale\": executable file not found in $PATH") as a 500, which
// reads as a server fault and tells the operator nothing. Verified on the
// device: PUT /api/v1/vpn/tailscale/ssh returned
// 500 {"error":"exec: \"tailscale\": executable file not found in $PATH"}.
var ErrTailscaleNotInstalled = errors.New("tailscale is not installed on this router")

// TailscaleInstalled reports whether the tailscale package is present.
//
// Either artefact is enough: the binary is what the `tailscale` subcommands need,
// and the init script is what the start/stop toggle needs. A router can have one
// without the other mid-upgrade, so both are checked rather than assuming.
func TailscaleInstalled() bool {
	for _, p := range []string{openwrtTailscaleBin, openwrtTailscaleBinAlt, tailscaleInitScript} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// requireTailscale returns ErrTailscaleNotInstalled when the package is absent, so
// a Tailscale mutation fails with an actionable message instead of an exec error.
func requireTailscale() error {
	if !TailscaleInstalled() {
		return fmt.Errorf("%w: install the tailscale package to use Tailscale", ErrTailscaleNotInstalled)
	}
	return nil
}

func tailscaleBin() string {
	if _, err := os.Stat(openwrtTailscaleBin); err == nil {
		return openwrtTailscaleBin
	}
	if _, err := os.Stat(openwrtTailscaleBinAlt); err == nil {
		return openwrtTailscaleBinAlt
	}
	return "tailscale"
}

func (v *VpnService) validateWireGuardConfigForEnable() error {
	wgOpts, err := v.uci.GetAll("network", "wg0")
	if err != nil {
		return fmt.Errorf("wireguard is not configured (missing network.wg0)")
	}
	if strings.TrimSpace(wgOpts["private_key"]) == "" {
		return fmt.Errorf("wireguard config is incomplete: missing wg0 private_key")
	}

	peerOpts, err := v.uci.GetAll("network", "wg0_peer0")
	if err != nil {
		return fmt.Errorf("wireguard config is incomplete: missing peer section wg0_peer0")
	}
	if strings.TrimSpace(peerOpts["public_key"]) == "" {
		return fmt.Errorf("wireguard config is incomplete: missing peer public_key")
	}
	endpoint := strings.TrimSpace(v.combinePeerEndpointFromUCI("wg0_peer0"))
	if endpoint == "" {
		return fmt.Errorf("wireguard config is incomplete: missing peer endpoint")
	}
	host, portStr, splitErr := net.SplitHostPort(endpoint)
	if splitErr != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(portStr) == "" {
		return fmt.Errorf("wireguard config is invalid: endpoint must be host:port (got %q)", endpoint)
	}
	port, convErr := strconv.Atoi(portStr)
	if convErr != nil || port < 1 || port > 65535 {
		return fmt.Errorf("wireguard config is invalid: endpoint port must be 1-65535 (got %q)", portStr)
	}

	if strings.TrimSpace(peerOpts["allowed_ips"]) == "" {
		return fmt.Errorf("wireguard config is incomplete: missing peer allowed_ips")
	}
	if strings.TrimSpace(peerOpts["route_allowed_ips"]) != "1" {
		return fmt.Errorf("wireguard config is incomplete: peer route_allowed_ips must be 1")
	}
	return nil
}

// CommandRunner abstracts command execution for testability.
// VpnService provides VPN status and configuration.
type VpnService struct {
	uci          uci.UCI
	cmd          CommandRunner
	profilesPath string // Path to wireguard_profiles.json
	// guardFile is the crash guard path for VPN live-state changes. Empty
	// disables the guard (only the production constructor sets it).
	guardFile string
	// dnsSnapshotPath and legacyDnsSnapshotPath are fields rather than bare
	// constants so the snapshot lifecycle can be exercised against a temp dir.
	dnsSnapshotPath string
	// legacyDnsSnapshotPath is the pre-unification location, read-only.
	legacyDnsSnapshotPath string
}

// vpnDnsSnapshotPath holds the pre-VPN dnsmasq server/noresolv state. It is a
// state file, not a crash guard, so it keeps its own literal rather than being
// derived from crashGuardDir.
const vpnDnsSnapshotPath = "/etc/trafo/vpn-dns-snapshot.json"

// legacyVpnDnsSnapshotPath is where releases before the guard-directory
// unification wrote the snapshot. Read-only: a restore removes both.
const legacyVpnDnsSnapshotPath = "/etc/travo/vpn-dns-snapshot.json"

// NewVpnService creates a new VpnService with a real command runner.
func NewVpnService(u uci.UCI) *VpnService {
	return &VpnService{
		uci:                   u,
		cmd:                   &RealCommandRunner{},
		profilesPath:          "/etc/travo/wireguard_profiles.json",
		guardFile:             vpnGuardPath,
		dnsSnapshotPath:       vpnDnsSnapshotPath,
		legacyDnsSnapshotPath: legacyVpnDnsSnapshotPath,
	}
}

// NewVpnServiceWithRunner creates a new VpnService with a custom command runner (for tests).
func NewVpnServiceWithRunner(u uci.UCI, cmd CommandRunner) *VpnService {
	return &VpnService{
		uci:                   u,
		cmd:                   cmd,
		profilesPath:          "/etc/travo/wireguard_profiles.json",
		dnsSnapshotPath:       vpnDnsSnapshotPath,
		legacyDnsSnapshotPath: legacyVpnDnsSnapshotPath,
	}
}

// NewVpnServiceWithProfilesPath creates a VpnService with a custom profiles path (for tests).
func NewVpnServiceWithProfilesPath(u uci.UCI, cmd CommandRunner, profilesPath string) *VpnService {
	return &VpnService{
		uci:                   u,
		cmd:                   cmd,
		profilesPath:          profilesPath,
		dnsSnapshotPath:       vpnDnsSnapshotPath,
		legacyDnsSnapshotPath: legacyVpnDnsSnapshotPath,
	}
}

func wireGuardIfaceLooksUp(linkShowOutput string) bool {
	out := strings.TrimSpace(linkShowOutput)
	if out == "" || !strings.Contains(out, "wg0") {
		return false
	}
	if strings.Contains(strings.ToLower(out), "state down") {
		return false
	}
	return true
}

func (v *VpnService) reloadFirewall() {
	_, _ = v.cmd.Run("/etc/init.d/firewall", "reload")
}

func (v *VpnService) combinePeerEndpointFromUCI(section string) string {
	host, errH := v.uci.Get("network", section, "endpoint_host")
	port, errP := v.uci.Get("network", section, "endpoint_port")
	if errH == nil && errP == nil && host != "" && port != "" {
		return net.JoinHostPort(host, port)
	}
	if ep, err := v.uci.Get("network", section, "endpoint"); err == nil && ep != "" {
		return ep
	}
	if errH == nil && host != "" {
		if errP == nil && port != "" {
			return net.JoinHostPort(host, port)
		}
		return net.JoinHostPort(host, "51820")
	}
	return ""
}

func (v *VpnService) setWireGuardAddresses(address string) error {
	_ = v.uci.DeleteOption("network", "wg0", "addresses")
	addr := strings.TrimSpace(address)
	if addr == "" {
		return nil
	}
	for part := range strings.SplitSeq(addr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if err := v.uci.AddList("network", "wg0", "addresses", part); err != nil {
			return err
		}
	}
	return nil
}

func (v *VpnService) applyWireGuardPeerParsed(section string, peer WireguardParsedPeer) error {
	if err := v.ensureWireGuardInterface(); err != nil {
		return err
	}
	if err := v.ensureWireGuardPeer(section); err != nil {
		return err
	}
	_ = v.uci.DeleteOption("network", section, "endpoint")
	_ = v.uci.DeleteOption("network", section, "endpoint_host")
	_ = v.uci.DeleteOption("network", section, "endpoint_port")
	_ = v.uci.DeleteOption("network", section, "allowed_ips")
	_ = v.uci.Set("network", section, "public_key", peer.PublicKey)
	if peer.PresharedKey != "" {
		_ = v.uci.Set("network", section, "preshared_key", peer.PresharedKey)
	}
	host, port := SplitWireGuardEndpoint(peer.Endpoint)
	if host != "" {
		_ = v.uci.Set("network", section, "endpoint_host", host)
		_ = v.uci.Set("network", section, "endpoint_port", port)
	}
	_ = v.uci.Set("network", section, "route_allowed_ips", "1")
	if peer.PersistentKeepalive > 0 {
		_ = v.uci.Set("network", section, "persistent_keepalive", strconv.Itoa(peer.PersistentKeepalive))
	} else if peer.Endpoint != "" {
		_ = v.uci.Set("network", section, "persistent_keepalive", "25")
	}
	allowed := strings.TrimSpace(peer.AllowedIPs)
	if allowed == "" {
		allowed = "0.0.0.0/0,::/0"
	}
	for cidr := range strings.SplitSeq(allowed, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if err := v.uci.AddList("network", section, "allowed_ips", cidr); err != nil {
			return err
		}
	}
	return nil
}

// ensureWireGuardInterface normalizes the network.wg0 UCI section so it has the
// correct type (interface) and proto (wireguard). This must be called before
// writing WireGuard-specific options, because UCI Set on a missing or wrong-typed
// section silently fails on some OpenWrt builds.
func (v *VpnService) ensureWireGuardInterface() error {
	opts, err := v.uci.GetAll("network", "wg0")
	if err != nil {
		// Section doesn't exist: create it.
		if addErr := v.uci.AddSection("network", "wg0", "interface"); addErr != nil {
			return fmt.Errorf("creating network.wg0 section: %w", addErr)
		}
	} else if opts[".type"] != "interface" {
		// Wrong section type — delete and recreate is the safe approach, but
		// in practice just ensuring proto is set should be enough to overwrite.
		_ = opts
	}
	return v.uci.Set("network", "wg0", "proto", "wireguard")
}

// ensureWireGuardPeer normalizes a peer section. Peer sections must have type
// "wireguard_wg0" so netifd binds them to the wg0 interface.
func (v *VpnService) ensureWireGuardPeer(section string) error {
	if _, err := v.uci.GetAll("network", section); err != nil {
		if addErr := v.uci.AddSection("network", section, "wireguard_wg0"); addErr != nil {
			return fmt.Errorf("creating peer section %s: %w", section, addErr)
		}
	}
	return nil
}

// Settle delays after netifd operations that expose no observable "done"
// condition — the subsequent runtime probe is the actual readiness check.
const (
	wireGuardReloadSettle = 300 * time.Millisecond
	wireGuardRetrySettle  = 400 * time.Millisecond
)

// wireGuardRuntimeUp probes whether wg0 is live: a parseable `wg show dump`
// or an UP link state.
func (v *VpnService) wireGuardRuntimeUp() bool {
	if out, err := v.cmd.Run(openwrtWgBin, "show", "wg0", "dump"); err == nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			if st, perr := ParseWgDump(s); perr == nil && st != nil {
				return true
			}
		}
	}
	if out, err := v.cmd.Run(openwrtIPBin, "link", "show", "dev", "wg0"); err == nil {
		return wireGuardIfaceLooksUp(string(out))
	}
	return false
}

func (v *VpnService) applyAndVerifyWireGuard() error {
	_, _ = v.cmd.Run(openwrtUbusBin, "call", "network", "reload")
	time.Sleep(wireGuardReloadSettle)
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(wireGuardRetrySettle)
			_, _ = v.cmd.Run(openwrtUbusBin, "call", "network", "reload")
			time.Sleep(wireGuardReloadSettle)
		}
		_, _ = v.cmd.Run(openwrtIfupBin, "wg0")
		// Exit as soon as the interface is live instead of burning the
		// remaining fixed-count retries.
		if v.wireGuardRuntimeUp() {
			return nil
		}
	}
	return v.waitForWireGuardRuntime(wireGuardVerifyTimeout)
}

func (v *VpnService) hasKernelDefaultRoute() bool {
	out, err := v.cmd.Run(openwrtIPBin, "route", "show", "default")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func (v *VpnService) waitForKernelDefaultRoute(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if v.hasKernelDefaultRoute() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return v.hasKernelDefaultRoute()
}

func (v *VpnService) uplinksFromUbusDump() []string {
	out, err := v.cmd.Run(openwrtUbusBin, "-S", "call", "network.interface", "dump")
	if err != nil || len(out) == 0 {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}

	ifaces, ok := raw["interface"].([]any)
	if !ok {
		return nil
	}

	var res []string
	for _, v := range ifaces {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["interface"].(string)
		if name == "" || name == "wg0" || name == "lan" || name == "loopback" {
			continue
		}
		up, _ := m["up"].(bool)
		if !up {
			continue
		}
		routes, ok := m["route"].([]any)
		if !ok {
			continue
		}
		hasDefault := false
		for _, r := range routes {
			rm, ok := r.(map[string]any)
			if !ok {
				continue
			}
			target, _ := rm["target"].(string)
			maskF, _ := rm["mask"].(float64) // JSON numbers decode as float64
			if target == "0.0.0.0" && int(maskF) == 0 {
				hasDefault = true
				break
			}
		}
		if hasDefault {
			res = append(res, name)
		}
	}
	return res
}

func (v *VpnService) restoreDefaultRouteAfterWireGuardDisable() {
	// If the kernel already has a default route, don't touch uplinks.
	if v.hasKernelDefaultRoute() {
		return
	}

	// Prefer the interface(s) netifd reports as up with a default route. This is
	// the best signal for the “active uplink” across WAN/WWAN/USB tether modes.
	uplinks := v.uplinksFromUbusDump()
	if len(uplinks) == 0 {
		// Conservative fallback for common travel-router modes.
		uplinks = []string{"wwan", "wan"}
	}

	// Phase 1: try DHCP renew + reload (least disruptive).
	for _, ifname := range uplinks {
		_, _ = v.cmd.Run(openwrtUbusBin, "call", "network.interface."+ifname, "renew")
		_, _ = v.cmd.Run(openwrtUbusBin, "call", "network.interface."+ifname+"6", "renew")
	}
	_, _ = v.cmd.Run(openwrtUbusBin, "call", "network", "reload")
	// Allow netifd time to re-install routes.
	if v.waitForKernelDefaultRoute(2 * time.Second) {
		return
	}

	// Phase 2: force a down/up cycle + reload (more disruptive, but restores routes
	// when netifd believes interface is already up while kernel route state is broken).
	for _, ifname := range uplinks {
		_, _ = v.cmd.Run(openwrtUbusBin, "call", "network.interface."+ifname, "down")
		_, _ = v.cmd.Run(openwrtUbusBin, "call", "network.interface."+ifname, "up")
		_, _ = v.cmd.Run(openwrtUbusBin, "call", "network.interface."+ifname+"6", "down")
		_, _ = v.cmd.Run(openwrtUbusBin, "call", "network.interface."+ifname+"6", "up")
	}
	_, _ = v.cmd.Run(openwrtUbusBin, "call", "network", "reload")
	if v.waitForKernelDefaultRoute(5 * time.Second) {
		return
	}

	// Phase 3: netifd helper scripts (ifup/ifdown) as last resort. On some systems
	// this re-triggers proto handlers more reliably than ubus down/up alone.
	for _, ifname := range uplinks {
		_, _ = v.cmd.Run(openwrtIfdownBin, ifname)
		_, _ = v.cmd.Run(openwrtIfupBin, ifname)
	}
	_, _ = v.cmd.Run(openwrtUbusBin, "call", "network", "reload")
	_ = v.waitForKernelDefaultRoute(8 * time.Second)
}

func (v *VpnService) waitForWireGuardRuntime(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastIPOut string
	for time.Now().Before(deadline) {
		if out, err := v.cmd.Run(openwrtWgBin, "show", "wg0", "dump"); err == nil {
			s := strings.TrimSpace(string(out))
			if s != "" {
				if st, perr := ParseWgDump(s); perr == nil && st != nil {
					return nil
				}
			}
		}
		if out, err := v.cmd.Run(openwrtIPBin, "link", "show", "dev", "wg0"); err == nil {
			ls := string(out)
			if wireGuardIfaceLooksUp(ls) {
				return nil
			}
			lastIPOut = strings.TrimSpace(ls)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if lastIPOut != "" {
		return fmt.Errorf("wg0 did not become ready in time (last %s: %s)", openwrtIPBin, lastIPOut)
	}
	return fmt.Errorf("wg0 did not become ready in time (%s and %s did not succeed)", openwrtWgBin, openwrtIPBin)
}

// wgRuntimeState returns a fine-grained status detail string for the wg0 interface.
func (v *VpnService) wgRuntimeState(enabled bool) string {
	if !enabled {
		return "disabled"
	}
	out, err := v.cmd.Run(openwrtWgBin, "show", "wg0", "dump")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "enabled_not_up"
	}
	status, parseErr := ParseWgDump(string(out))
	if parseErr != nil || status == nil {
		return "enabled_not_up"
	}
	for _, peer := range status.Peers {
		if peer.LatestHandshake > 0 {
			return "connected"
		}
	}
	if len(status.Peers) > 0 {
		return "up_no_handshake"
	}
	return "configured"
}

// GetVpnStatus returns all VPN connection statuses.
//
// It also runs the DNS self-heal reconcile (ADR 0001 §3): if a VPN DNS
// forwarding snapshot is on disk but the tunnel that justified it is not
// connected, the pre-VPN dnsmasq state is restored here rather than waiting for
// an operator to toggle the VPN off. This is a status read that heals rather
// than just reporting, which is deliberate: GET /vpn/status is polled by the
// dashboard and the VPN page, so it is the only health signal that is guaranteed
// to be hit after a reboot.
func (v *VpnService) GetVpnStatus() ([]models.VpnStatus, error) {
	var statuses []models.VpnStatus

	// WireGuard — only include if configured
	opts, err := v.uci.GetAll("network", "wg0")
	if err == nil {
		// WireGuard is configured
		disabled := opts["disabled"]
		wgStatus := models.VpnStatus{Type: "wireguard",
			Enabled: disabled != "1"}
		wgStatus.StatusDetail = v.wgRuntimeState(wgStatus.Enabled)
		if v.maybeSelfHealVpnDNS(wgStatus.StatusDetail) {
			// StatusDetail is left at its documented value: the frontend matches
			// it by exact equality, so a suffixed value would render no status
			// text at all. The restore is logged instead.
			log.Printf("vpn: LAN DNS restored from snapshot; tunnel state %q",
				wgStatus.StatusDetail)
		}
		wgStatus.Connected = wgStatus.StatusDetail == "connected"
		if wgStatus.Enabled {
			wgStatus.Endpoint = v.combinePeerEndpointFromUCI("wg0_peer0")
		}
		statuses = append(statuses, wgStatus)
	}

	// Tailscale
	statuses = append(statuses, models.VpnStatus{
		Type:    "tailscale",
		Enabled: false,
	})

	return statuses, nil
}

// GetWireGuardStatus returns live WireGuard status by running `wg show wg0 dump`.
// The dump format is tab-separated:
// Line 1 (interface): private_key  public_key  listen_port  fwmark
// Line 2+ (peers): public_key  preshared_key  endpoint  allowed_ips  latest_handshake_epoch  transfer_rx  transfer_tx  persistent_keepalive
func (v *VpnService) GetWireGuardStatus() (*models.WireGuardStatus, error) {
	out, err := v.cmd.Run(openwrtWgBin, "show", "wg0", "dump")
	if err != nil {
		// wg show fails with exit status 1 when interface doesn't exist (tunnel not active)
		return &models.WireGuardStatus{Interface: "wg0", Peers: []models.WireGuardPeerStatus{}}, nil
	}
	return ParseWgDump(string(out))
}

// ParseWgDump parses the output of `wg show <iface> dump` into a WireGuardStatus.
func ParseWgDump(dump string) (*models.WireGuardStatus, error) {
	lines := strings.Split(strings.TrimSpace(dump), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, fmt.Errorf("empty wg dump output")
	}

	// Parse interface line
	ifFields := strings.Split(lines[0], "\t")
	if len(ifFields) < 3 {
		return nil, fmt.Errorf("invalid interface line: expected at least 3 fields, got %d", len(ifFields))
	}

	listenPort, _ := strconv.Atoi(ifFields[2])
	status := &models.WireGuardStatus{
		Interface:  "wg0",
		PublicKey:  ifFields[1],
		ListenPort: listenPort,
		Peers:      []models.WireGuardPeerStatus{},
	}

	// Parse peer lines
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			continue
		}
		handshake, _ := strconv.ParseInt(fields[4], 10, 64)
		rx, _ := strconv.ParseInt(fields[5], 10, 64)
		tx, _ := strconv.ParseInt(fields[6], 10, 64)

		peer := models.WireGuardPeerStatus{
			PublicKey:       fields[0],
			Endpoint:        fields[2],
			AllowedIPs:      fields[3],
			LatestHandshake: handshake,
			TransferRx:      rx,
			TransferTx:      tx,
		}
		status.Peers = append(status.Peers, peer)
	}

	return status, nil
}

// GetWireguardConfig returns the WireGuard configuration.
func (v *VpnService) GetWireguardConfig() (models.WireguardConfig, error) {
	opts, err := v.uci.GetAll("network", "wg0")
	if err != nil {
		// WireGuard not configured - return empty config (not an error)
		return models.WireguardConfig{}, nil
	}

	config := models.WireguardConfig{
		PrivateKey: opts["private_key"],
		Address:    opts["addresses"],
	}
	if dns, ok := opts["dns"]; ok && dns != "" {
		config.DNS = strings.Split(dns, " ")
	}

	// Get peer
	peerOpts, err := v.uci.GetAll("network", "wg0_peer0")
	if err == nil {
		peer := models.WireguardPeer{
			PublicKey: peerOpts["public_key"],
			Endpoint:  v.combinePeerEndpointFromUCI("wg0_peer0"),
		}
		if ips, ok := peerOpts["allowed_ips"]; ok && ips != "" {
			peer.AllowedIPs = strings.Split(ips, ",")
		}
		config.Peers = []models.WireguardPeer{peer}
	}

	return config, nil
}

// SetWireguardConfig is the entry point from the API. It takes the UCI transaction for every
// config the flow can touch, so a concurrent WiFi or DHCP write cannot land in
// the middle of a VPN toggle and revert half of it.
func (v *VpnService) SetWireguardConfig(config models.WireguardConfig) error {
	return mutateUCI(v.uci, vpnFlowConfigs, func() error {
		return v.SetWireguardConfigLocked(config)
	})
}

// SetWireguardConfig updates the WireGuard configuration.
func (v *VpnService) SetWireguardConfigLocked(config models.WireguardConfig) error {
	if err := v.ensureWireGuardInterface(); err != nil {
		return err
	}
	if config.PrivateKey != "" {
		_ = v.uci.Set("network", "wg0", "private_key", config.PrivateKey)
	}
	if config.Address != "" {
		if err := v.setWireGuardAddresses(config.Address); err != nil {
			return err
		}
	}
	if len(config.DNS) > 0 {
		_ = v.uci.Set("network", "wg0", "dns", strings.Join(config.DNS, " "))
	}
	if len(config.Peers) > 0 {
		p := config.Peers[0]
		parsed := WireguardParsedPeer{
			PublicKey:  p.PublicKey,
			Endpoint:   p.Endpoint,
			AllowedIPs: strings.Join(p.AllowedIPs, ","),
		}
		if p.PresharedKey != nil {
			parsed.PresharedKey = *p.PresharedKey
		}
		if err := v.applyWireGuardPeerParsed("wg0_peer0", parsed); err != nil {
			return err
		}
	}
	return v.uci.Commit("network")
}

// ToggleWireguard enables or disables WireGuard.
//
// Enable order matters: the tunnel is committed and verified FIRST, and only then
// are the dependent changes (firewall zone/forwarding, LAN DNS forwarding) applied.
// Those changes point LAN traffic at the tunnel, so committing them before the
// tunnel is up would leave DNS resolving through resolvers that are unreachable.
// If the tunnel does not come up, the dependent changes are never made and any
// partially applied change is torn down again.
// ToggleWireguard enables or disables the tunnel. It is the widest UCI writer in
// the service — network, firewall and dhcp — so the whole toggle is one
// transaction: a failure anywhere reverts all three together instead of leaving
// a half-applied tunnel whose firewall zone and resolver list disagree with its
// interface state.
func (v *VpnService) ToggleWireguard(enable bool) error {
	return mutateUCI(v.uci, vpnFlowConfigs, func() error {
		if enable {
			return v.enableWireguard()
		}
		return v.disableWireguard()
	})
}

// vpnGuardPath is the crash guard for VPN live-state changes (ADR 0003). It is
// set by the production constructor; test constructors leave it empty, which
// disables the guard for tests.
const vpnGuardPath = crashGuardDir + "/vpn-in-progress"

// writeVpnGuard creates the VPN crash guard before touching live state.
func (v *VpnService) writeVpnGuard() error {
	if v.guardFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(v.guardFile), 0750); err != nil {
		return fmt.Errorf("create vpn guard dir: %w", err)
	}
	if err := os.WriteFile(v.guardFile, []byte(time.Now().Format(time.RFC3339Nano)), 0600); err != nil {
		return fmt.Errorf("write vpn guard: %w", err)
	}
	return nil
}

// clearVpnGuard removes the crash guard after the operation completed.
func (v *VpnService) clearVpnGuard() {
	if v.guardFile == "" {
		return
	}
	_ = os.Remove(v.guardFile)
}

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
	if err := v.writeVpnGuard(); err != nil {
		return err
	}
	if err := v.uci.Set("network", "wg0", "disabled", "0"); err != nil {
		v.clearVpnGuard()
		return err
	}
	if err := v.uci.Commit("network"); err != nil {
		v.clearVpnGuard()
		return err
	}

	// Verify the tunnel before committing anything that depends on it.
	if err := v.applyAndVerifyWireGuard(); err != nil {
		if rbErr := v.rollbackWireguardEnable(prevDisabled); rbErr != nil {
			// The device is in an unknown state: keep the crash guard so no
			// other flow mutates it before a human looks.
			return fmt.Errorf("WireGuard enabled in UCI but tunnel failed to start: %w (rollback incomplete, crash guard %s kept: %v)", err, v.guardFile, rbErr)
		}
		v.clearVpnGuard()
		return fmt.Errorf("WireGuard enabled in UCI but tunnel failed to start: %w", err)
	}

	// The tunnel is verified up, so clearing the Tailscale exit node can no
	// longer strand a failed enable. It used to run first, which meant a failed
	// enable silently changed where the router's own traffic egresses with
	// nothing to put it back (rollbackWireguardEnable never restored it).
	_, _ = v.cmd.Run(tailscaleBin(), "set", "--exit-node=")

	if err := v.setupWireGuardFirewall(); err != nil {
		if rbErr := v.rollbackWireguardEnable(prevDisabled); rbErr != nil {
			return fmt.Errorf("setting up WireGuard firewall: %w (rollback incomplete, crash guard %s kept: %v)", err, v.guardFile, rbErr)
		}
		v.clearVpnGuard()
		return fmt.Errorf("setting up WireGuard firewall: %w", err)
	}

	if err := v.enableVpnDNSForwarding(); err != nil {
		// DNS forwarding was aborted on purpose (no snapshot could be taken),
		// so dnsmasq still has the pre-VPN resolvers. The tunnel is up and
		// verified; failing the toggle here would tear down a working VPN over
		// a DNS bookkeeping problem. Record it instead.
		log.Printf("vpn: tunnel up but LAN DNS left unchanged: %v", err)
	}
	v.clearVpnGuard()
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
	_, _ = v.cmd.Run(openwrtIfdownBin, "wg0")
	return errors.Join(errs...)
}

func (v *VpnService) disableWireguard() error {
	// The disable path bounces uplinks (ubus down/up, then ifdown/ifup as a last
	// resort). That is live state, so it needs the crash guard exactly like the
	// enable path (ADR 0003). It used to be written only by enableWireguard, so
	// an interrupted bounce left the uplink down with no marker at all.
	if err := v.writeVpnGuard(); err != nil {
		return err
	}
	// Every exit path tears the wg0 firewall plumbing down. Returning early on
	// the missing-default-route branch used to leave firewall.wg0_zone and
	// firewall.wg0_fwd committed across reboots, and the next enable reused
	// those stale sections.
	//
	// The guard is cleared inside this defer, after the teardown, so a device
	// that dies mid-teardown still has the marker. routeConfirmed is false on
	// every error return, which keeps the guard when the uplink bounce may not
	// have taken.
	routeConfirmed := false
	defer func() {
		if err := v.teardownWireGuardFirewall(); err != nil {
			log.Printf("vpn: tearing down WireGuard firewall: %v", err)
		}
		if routeConfirmed {
			v.clearVpnGuard()
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
	_, _ = v.cmd.Run(openwrtIfdownBin, "wg0")
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

type vpnDnsSnapshot struct {
	NoResolv string   `json:"noresolv"`
	Servers  []string `json:"servers"`
}

func (v *VpnService) readDnsmasqServers() []string {
	out, err := v.cmd.Run("uci", "get", "dhcp.@dnsmasq[0].server")
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil
	}
	// `uci get` returns a space-separated list for list options.
	return strings.Fields(s)
}

func (v *VpnService) readDnsmasqNoResolv() string {
	out, err := v.cmd.Run("uci", "get", "dhcp.@dnsmasq[0].noresolv")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (v *VpnService) writeVpnDnsSnapshot(snap vpnDnsSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(v.dnsSnapshotPath), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return os.WriteFile(v.dnsSnapshotPath, data, 0o600)
}

func (v *VpnService) loadVpnDnsSnapshot() (*vpnDnsSnapshot, error) {
	data, err := os.ReadFile(v.dnsSnapshotPath)
	if err != nil {
		// Fall back to the pre-unification location. A device upgraded while the
		// VPN was enabled has its snapshot under the old path; failing to read it
		// would leave dnsmasq with noresolv=1 forwarding to VPN resolvers that
		// are no longer reachable, i.e. LAN DNS stays broken after the tunnel
		// goes down.
		data, err = os.ReadFile(v.legacyDnsSnapshotPath)
		if err != nil {
			return nil, err
		}
	}
	var snap vpnDnsSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// removeVpnDnsSnapshot drops both snapshot locations. It is best-effort: the
// restore has already been written to dnsmasq at this point, and a leftover
// snapshot only costs one redundant restore on the next health reconcile.
func (v *VpnService) removeVpnDnsSnapshot() {
	_ = os.Remove(v.dnsSnapshotPath)
	_ = os.Remove(v.legacyDnsSnapshotPath)
}

// splitWireGuardDNSOption splits UCI network.wg0.dns. OpenWrt normally uses
// space-separated values; some imports (or hand-edited UCI) use commas.
func splitWireGuardDNSOption(dns string) []string {
	dns = strings.TrimSpace(dns)
	if dns == "" {
		return nil
	}
	dns = strings.ReplaceAll(dns, ",", " ")
	var out []string
	for s := range strings.FieldsSeq(dns) {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (v *VpnService) wgConfiguredDNSServers() []string {
	dns, err := v.uci.Get("network", "wg0", "dns")
	if err != nil {
		return nil
	}
	return splitWireGuardDNSOption(dns)
}

// enableVpnDNSForwarding points dnsmasq at the tunnel's resolvers. It reports
// failure but must never fail the toggle: the tunnel is already verified up
// before it runs, so aborting the whole enable over DNS bookkeeping would tear
// down a working VPN. Returning an error lets the caller record why LAN DNS was
// left alone.
//
// An existing readable snapshot is authoritative and is NEVER overwritten. The
// dashboard toggle and the VPN page are independent entry points with no
// idempotency guard, so a second enable while the tunnel is already up would
// otherwise snapshot the VPN's own resolvers, and the later disable would
// restore noresolv=1 pointing at 10.8.0.1 with the tunnel down.
func (v *VpnService) enableVpnDNSForwarding() error {
	vpnDNS := v.wgConfiguredDNSServers()
	if len(vpnDNS) == 0 {
		return nil
	}

	if _, err := v.loadVpnDnsSnapshot(); err != nil {
		// No usable snapshot: take one from the CURRENT dnsmasq state,
		// including AdGuard's 127.0.0.1#5353, so the disable path can put it
		// back. LAN DNS must forward to VPN DNS while the tunnel is up,
		// otherwise queries would still go to AdGuard only.
		snap := vpnDnsSnapshot{
			NoResolv: v.readDnsmasqNoResolv(),
			Servers:  v.readDnsmasqServers(),
		}
		if err := v.writeVpnDnsSnapshot(snap); err != nil {
			// Without a snapshot there is nothing to restore, so continuing
			// would set noresolv=1 against resolvers reachable only through
			// the tunnel: LAN DNS SERVFAILs once the tunnel goes down, and the
			// pre-VPN state is unrecoverable without hand-editing dhcp.
			return fmt.Errorf("snapshotting dnsmasq before VPN DNS forwarding: %w", err)
		}
	}

	// Apply: forward to VPN DNS only.
	_, _ = v.cmd.Run("uci", "delete", "dhcp.@dnsmasq[0].server")
	for _, s := range vpnDNS {
		_, _ = v.cmd.Run("uci", "add_list", fmt.Sprintf("dhcp.@dnsmasq[0].server=%s", s))
	}
	_, _ = v.cmd.Run("uci", "set", "dhcp.@dnsmasq[0].noresolv=1")
	_, _ = v.cmd.Run("uci", "commit", "dhcp")
	_, _ = v.cmd.Run("/etc/init.d/dnsmasq", "restart")
	return nil
}

// disableVpnDNSForwarding restores the dnsmasq snapshot taken when forwarding
// was enabled. Like enable, it is best-effort and reports nothing.
func (v *VpnService) disableVpnDNSForwarding() {
	snap, err := v.loadVpnDnsSnapshot()
	if err != nil {
		// Nothing to restore.
		return
	}

	// Restore previous servers/noresolv.
	_, _ = v.cmd.Run("uci", "delete", "dhcp.@dnsmasq[0].server")
	for _, s := range snap.Servers {
		_, _ = v.cmd.Run("uci", "add_list", fmt.Sprintf("dhcp.@dnsmasq[0].server=%s", s))
	}
	if snap.NoResolv != "" {
		_, _ = v.cmd.Run("uci", "set", "dhcp.@dnsmasq[0].noresolv="+snap.NoResolv)
	} else {
		_, _ = v.cmd.Run("uci", "set", "dhcp.@dnsmasq[0].noresolv=0")
	}
	_, _ = v.cmd.Run("uci", "commit", "dhcp")
	_, _ = v.cmd.Run("/etc/init.d/dnsmasq", "restart")
	v.removeVpnDnsSnapshot()
}

// maybeSelfHealVpnDNS restores the pre-VPN dnsmasq state when a forwarding
// snapshot exists but the tunnel that justified it can no longer carry DNS:
// wg0 is absent, up without a handshake, or the tunnel is disabled in UCI.
//
// This is the second entrypoint into disableVpnDNSForwarding. Without it, a
// dropped uplink or a reboot into an unreachable peer leaves dnsmasq at
// noresolv=1 pointing at resolvers only reachable through a down tunnel, and
// every LAN client SERVFAILs until the operator manually toggles the VPN off.
//
// tunnelDetail is the already-computed wgRuntimeState detail, so this costs no
// extra shell-out. Returns true when a restore happened.
func (v *VpnService) maybeSelfHealVpnDNS(tunnelDetail string) bool {
	if tunnelDetail == "connected" {
		return false
	}
	if _, err := v.loadVpnDnsSnapshot(); err != nil {
		// No snapshot: dnsmasq was never pointed at the tunnel.
		return false
	}
	log.Printf("vpn: tunnel state %q with a DNS snapshot present; restoring dnsmasq",
		tunnelDetail)
	// disableVpnDNSForwarding shells out to `uci` for dhcp, so it needs the same
	// lock every other dhcp writer holds (ADR 0010).
	if err := withConfigLocks([]string{"dhcp"}, func() error {
		v.disableVpnDNSForwarding()
		return nil
	}); err != nil {
		log.Printf("vpn: restoring pre-VPN dnsmasq: %v", err)
		return false
	}
	return true
}

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
	out, err := v.cmd.Run(openwrtIPBin, "link", "show", "dev", "wg0")
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
	routeOut, routeErr := v.cmd.Run(openwrtIPBin, "route", "show", "default")
	route6Out, _ := v.cmd.Run(openwrtIPBin, "-6", "route", "show", "default")
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

// tailscaleStatusJSON is the subset of `tailscale status --json` we parse.
type tailscaleStatusJSON struct {
	BackendState string   `json:"BackendState"`
	AuthURL      string   `json:"AuthURL"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Self         struct {
		DNSName      string   `json:"DNSName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
		Online       bool     `json:"Online"`
	} `json:"Self"`
	Peer map[string]struct {
		DNSName        string   `json:"DNSName"`
		OS             string   `json:"OS"`
		Online         bool     `json:"Online"`
		ExitNode       bool     `json:"ExitNode"`
		ExitNodeOption bool     `json:"ExitNodeOption"`
		TailscaleIPs   []string `json:"TailscaleIPs"`
		LastSeen       string   `json:"LastSeen"`
	} `json:"Peer"`
}

// isTailscaleInstalled checks whether the tailscale binary exists.
func (v *VpnService) isTailscaleInstalled() bool {
	if _, err := os.Stat(openwrtTailscaleBin); err == nil {
		return true
	}
	if _, err := os.Stat(openwrtTailscaleBinAlt); err == nil {
		return true
	}
	// Fall back to PATH check for non-OpenWrt/dev environments.
	_, err := v.cmd.Run("which", "tailscale")
	return err == nil
}

// isTailscaleRunning checks whether tailscaled is running.
func (v *VpnService) isTailscaleRunning() bool {
	_, err := v.cmd.Run("/etc/init.d/tailscale", "status")
	return err == nil
}

// GetTailscaleStatus returns Tailscale status, including peers when logged in.
func (v *VpnService) GetTailscaleStatus() (models.TailscaleStatus, error) {
	installed := v.isTailscaleInstalled()
	if !installed {
		return models.TailscaleStatus{
			Installed: false,
			Running:   false,
			LoggedIn:  false,
			Peers:     []models.TailscalePeer{},
		}, nil
	}

	running := v.isTailscaleRunning()
	if !running {
		return models.TailscaleStatus{
			Installed: true,
			Running:   false,
			LoggedIn:  false,
			Peers:     []models.TailscalePeer{},
		}, nil
	}

	raw, err := v.cmd.Run(tailscaleBin(), "status", "--json")
	if err != nil {
		return models.TailscaleStatus{Installed: true, Running: true, Peers: []models.TailscalePeer{}}, nil
	}

	var ts tailscaleStatusJSON
	if err := json.Unmarshal(raw, &ts); err != nil {
		return models.TailscaleStatus{Installed: true, Running: true, Peers: []models.TailscalePeer{}}, nil
	}

	loggedIn := ts.BackendState == "Running" || ts.BackendState == "Starting"

	var ip string
	if len(ts.Self.TailscaleIPs) > 0 {
		ip = ts.Self.TailscaleIPs[0]
	} else if len(ts.TailscaleIPs) > 0 {
		ip = ts.TailscaleIPs[0]
	}

	hostname := strings.TrimSuffix(ts.Self.DNSName, ".")
	if idx := strings.Index(hostname, "."); idx >= 0 {
		hostname = hostname[:idx]
	}

	// Build peers list.
	peers := make([]models.TailscalePeer, 0, len(ts.Peer))
	for _, p := range ts.Peer {
		var peerIP string
		if len(p.TailscaleIPs) > 0 {
			peerIP = p.TailscaleIPs[0]
		}
		peerHostname := strings.TrimSuffix(p.DNSName, ".")
		if idx := strings.Index(peerHostname, "."); idx >= 0 {
			peerHostname = peerHostname[:idx]
		}
		peers = append(peers, models.TailscalePeer{
			Hostname:       peerHostname,
			TailscaleIP:    peerIP,
			OS:             p.OS,
			Online:         p.Online,
			ExitNode:       p.ExitNode,
			ExitNodeOption: p.ExitNodeOption,
			LastSeen:       p.LastSeen,
		})
	}

	authURL := ""
	if ts.BackendState == "NeedsLogin" || ts.AuthURL != "" {
		authURL = ts.AuthURL
	}

	return models.TailscaleStatus{
		Installed: true,
		Running:   true,
		LoggedIn:  loggedIn,
		IPAddress: ip,
		Hostname:  hostname,
		Peers:     peers,
		AuthURL:   authURL,
	}, nil
}

// StartTailscaleAuth runs `tailscale up` and returns the auth URL if login is required.
func (v *VpnService) StartTailscaleAuth(authKey string) (string, error) {
	if err := requireTailscale(); err != nil {
		return "", err
	}
	args := []string{"up", "--accept-routes"}
	if authKey != "" {
		args = append(args, "--auth-key="+authKey)
	}
	out, _ := v.cmd.Run(tailscaleBin(), args...)
	combined := strings.TrimSpace(string(out))
	// Extract URL from output like "To authenticate, visit:\n\thttps://login.tailscale.com/..."
	for line := range strings.SplitSeq(combined, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://login.tailscale.com") || strings.HasPrefix(line, "https://tailscale.com") {
			return line, nil
		}
	}
	// Also check the status for an auth URL.
	status, _ := v.GetTailscaleStatus()
	return status.AuthURL, nil
}

// SetTailscaleExitNode sets or clears the Tailscale exit node.
// When a non-empty exit node is set, WireGuard is turned off first so only one
// full-tunnel-style path is active (see requirements: single active VPN policy).
func (v *VpnService) SetTailscaleExitNode(nodeIP string) error {
	if err := requireTailscale(); err != nil {
		return err
	}
	nodeIP = strings.TrimSpace(nodeIP)
	if nodeIP != "" {
		if opts, err := v.uci.GetAll("network", "wg0"); err == nil && opts["disabled"] != "1" {
			if err := v.ToggleWireguard(false); err != nil {
				return fmt.Errorf("disable WireGuard before using a Tailscale exit node: %w", err)
			}
		}
	}
	if nodeIP == "" {
		_, err := v.cmd.Run(tailscaleBin(), "set", "--exit-node=")
		return err
	}
	_, err := v.cmd.Run(tailscaleBin(), "set", "--exit-node="+nodeIP, "--exit-node-allow-lan-access=true")
	return err
}

// GetKillSwitch checks whether the VPN kill switch firewall rule exists.
func (v *VpnService) GetKillSwitch() (models.KillSwitchStatus, error) {
	opts, err := v.uci.GetAll("firewall", vpnKillSwitchSection)
	if err != nil {
		return models.KillSwitchStatus{Enabled: false}, nil
	}
	return models.KillSwitchStatus{
		Enabled: opts["src"] == "lan" && opts["dest"] == "wan" && opts["target"] == "REJECT",
	}, nil
}

// vpnKillSwitchSection is the firewall rule implementing the VPN kill switch.
const vpnKillSwitchSection = "vpn_killswitch"

// vpnKillSwitchOwnerOption marks which flow created the kill switch rule.
// "user" means the user configured it as a standalone policy, so a VPN toggle
// must not delete it; "vpn_toggle" means the VPN toggle owns it.
const (
	vpnKillSwitchOwnerOption = "travo_owner"
	vpnKillSwitchOwnerUser   = "user"
	vpnKillSwitchOwnerToggle = "vpn_toggle"
)

// SetKillSwitch enables or disables the VPN kill switch firewall rule.
// Every write error is propagated: a partial rule would be committed with the
// default target (ACCEPT), i.e. a "kill switch" that allows all traffic.
func (v *VpnService) SetKillSwitch(enabled bool) error {
	return mutateUCI(v.uci, vpnFirewallConfigs, func() error {
		if enabled {
			// Create the firewall rule that blocks LAN→WAN when VPN is down.
			if _, err := v.uci.GetAll("firewall", vpnKillSwitchSection); err != nil {
				if addErr := v.uci.AddSection("firewall", vpnKillSwitchSection, "rule"); addErr != nil {
					return fmt.Errorf("creating vpn kill switch rule: %w", addErr)
				}
			}
			settings := [][2]string{
				{"name", "VPN Kill Switch"},
				{"src", "lan"},
				{"dest", "wan"},
				{"target", "REJECT"},
				{vpnKillSwitchOwnerOption, vpnKillSwitchOwnerUser},
			}
			for _, kv := range settings {
				if err := v.uci.Set("firewall", vpnKillSwitchSection, kv[0], kv[1]); err != nil {
					return fmt.Errorf("set vpn kill switch %s: %w", kv[0], err)
				}
			}
		} else {
			// Remove the firewall rule; a missing section is not an error.
			if _, err := v.uci.GetAll("firewall", vpnKillSwitchSection); err == nil {
				if delErr := v.uci.DeleteSection("firewall", vpnKillSwitchSection); delErr != nil {
					return fmt.Errorf("removing vpn kill switch rule: %w", delErr)
				}
			}
		}
		if err := v.uci.Commit("firewall"); err != nil {
			return err
		}
		v.reloadFirewall()
		return nil
	})
}

// removeVPNOwnedKillSwitch deletes the kill switch rule only when the VPN toggle
// created it. A kill switch the user configured as a standalone policy is left
// untouched.
//
// It does NOT take a config lock: every caller is already inside the VPN
// transaction that holds `firewall`, and the config locks are not reentrant.
func (v *VpnService) removeVPNOwnedKillSwitch() error {
	{
		opts, err := v.uci.GetAll("firewall", vpnKillSwitchSection)
		if err != nil {
			// Nothing to remove.
			return nil
		}
		if opts[vpnKillSwitchOwnerOption] != vpnKillSwitchOwnerToggle {
			return nil
		}
		if err := v.uci.DeleteSection("firewall", vpnKillSwitchSection); err != nil {
			return fmt.Errorf("removing vpn kill switch rule: %w", err)
		}
		if err := v.uci.Commit("firewall"); err != nil {
			return fmt.Errorf("committing firewall: %w", err)
		}
		v.reloadFirewall()
		return nil
	}
}

// ImportWireguardConfig is the entry point from the API. It takes the UCI transaction for every
// config the flow can touch, so a concurrent WiFi or DHCP write cannot land in
// the middle of a VPN toggle and revert half of it.
func (v *VpnService) ImportWireguardConfig(confContent string) error {
	return mutateUCI(v.uci, vpnFlowConfigs, func() error {
		return v.ImportWireguardConfigLocked(confContent)
	})
}

// ImportWireguardConfig parses a .conf file, normalizes the UCI structure,
// applies the config, and verifies the tunnel comes up.
func (v *VpnService) ImportWireguardConfigLocked(confContent string) error {
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

// ToggleTailscale starts or stops the Tailscale daemon via init.d.
func (v *VpnService) ToggleTailscale(enable bool) error {
	if err := requireTailscale(); err != nil {
		return err
	}
	if enable {
		_, err := v.cmd.Run("/etc/init.d/tailscale", "start")
		return err
	}
	_, err := v.cmd.Run("/etc/init.d/tailscale", "stop")
	return err
}

// loadProfiles reads profiles from the JSON file.
func (v *VpnService) loadProfiles() ([]models.WireGuardProfile, error) {
	data, err := os.ReadFile(v.profilesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.WireGuardProfile{}, nil
		}
		return nil, fmt.Errorf("reading profiles file: %w", err)
	}
	var profiles []models.WireGuardProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parsing profiles file: %w", err)
	}
	return profiles, nil
}

// saveProfiles writes profiles to the JSON file.
func (v *VpnService) saveProfiles(profiles []models.WireGuardProfile) error {
	data, err := json.Marshal(profiles)
	if err != nil {
		return fmt.Errorf("marshaling profiles: %w", err)
	}
	dir := filepath.Dir(v.profilesPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating profiles directory: %w", err)
	}
	if err := os.WriteFile(v.profilesPath, data, 0o600); err != nil {
		return fmt.Errorf("writing profiles file: %w", err)
	}
	return nil
}

// generateProfileID creates a short random hex ID.
func generateProfileID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GetProfiles returns all saved WireGuard profiles.
func (v *VpnService) GetProfiles() ([]models.WireGuardProfile, error) {
	return v.loadProfiles()
}

// AddProfile saves a new WireGuard profile.
func (v *VpnService) AddProfile(name, config string) (*models.WireGuardProfile, error) {
	// Validate the config is parseable
	if _, err := ParseWireguardConfig(config); err != nil {
		return nil, fmt.Errorf("invalid WireGuard config: %w", err)
	}

	profiles, err := v.loadProfiles()
	if err != nil {
		return nil, err
	}

	profile := models.WireGuardProfile{
		ID:        generateProfileID(),
		Name:      name,
		Config:    config,
		Active:    false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	profiles = append(profiles, profile)

	if err := v.saveProfiles(profiles); err != nil {
		return nil, err
	}
	return &profile, nil
}

// ActivateProfile is the entry point from the API; see ActivateProfileLocked.
func (v *VpnService) ActivateProfile(id string) error {
	return mutateUCI(v.uci, vpnFlowConfigs, func() error {
		return v.ActivateProfileLocked(id)
	})
}

// DeleteProfile removes a profile by ID.
func (v *VpnService) DeleteProfile(id string) error {
	profiles, err := v.loadProfiles()
	if err != nil {
		return err
	}

	found := false
	filtered := make([]models.WireGuardProfile, 0, len(profiles))
	for _, p := range profiles {
		if p.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, p)
	}
	if !found {
		return fmt.Errorf("profile not found: %s", id)
	}

	return v.saveProfiles(filtered)
}

// ActivateProfileLocked loads a profile's config into UCI and marks it as
// active. It writes network (via the import)
// and firewall (the wg0 zone), so it runs inside the VPN transaction and calls
// the lock-free cores.
func (v *VpnService) ActivateProfileLocked(id string) error {
	profiles, err := v.loadProfiles()
	if err != nil {
		return err
	}

	var target *models.WireGuardProfile
	for i := range profiles {
		if profiles[i].ID == id {
			target = &profiles[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("profile not found: %s", id)
	}

	// Apply the config via the existing import logic
	if err := v.ImportWireguardConfigLocked(target.Config); err != nil {
		return fmt.Errorf("applying profile config: %w", err)
	}

	// Ensure firewall zone and forwarding rules exist.
	if err := v.setupWireGuardFirewall(); err != nil {
		return fmt.Errorf("setting up WireGuard firewall: %w", err)
	}

	// Mark only this profile as active
	for i := range profiles {
		profiles[i].Active = profiles[i].ID == id
	}

	return v.saveProfiles(profiles)
}

const wireGuardSpeedTestURL = "http://speedtest.tele2.net/1MB.zip"

// wireGuardIPv4Address returns the first IPv4 address assigned to wg0 (from `ip -4 -o addr show`).
func (v *VpnService) wireGuardIPv4Address() (string, error) {
	out, err := v.cmd.Run(openwrtIPBin, "-4", "-o", "addr", "show", "dev", "wg0")
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

	linkOut, err := v.cmd.Run(openwrtIPBin, "link", "show", "dev", "wg0")
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
	dnsmasqServers := v.readDnsmasqServers()
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

const splitTunnelPath = "/etc/travo/split-tunnel.json"

// GetSplitTunnel returns the current WireGuard split tunnel configuration.
func (v *VpnService) GetSplitTunnel() (models.SplitTunnelConfig, error) {
	data, err := os.ReadFile(splitTunnelPath)
	if err != nil {
		return models.SplitTunnelConfig{Mode: "all"}, nil
	}
	var cfg models.SplitTunnelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return models.SplitTunnelConfig{Mode: "all"}, nil
	}
	return cfg, nil
}

// SetSplitTunnel is the entry point from the API. It takes the UCI transaction for every
// config the flow can touch, so a concurrent WiFi or DHCP write cannot land in
// the middle of a VPN toggle and revert half of it.
func (v *VpnService) SetSplitTunnel(cfg models.SplitTunnelConfig) error {
	return mutateUCI(v.uci, vpnFlowConfigs, func() error {
		return v.SetSplitTunnelLocked(cfg)
	})
}

// SetSplitTunnel saves the split tunnel config and updates WireGuard allowed IPs in UCI.
// mode "all" = route everything through VPN (0.0.0.0/0,::/0)
// mode "custom" = only route the specified CIDR ranges (at least one is required;
//
//	an empty custom list must never silently become a full tunnel)
//
// Staged UCI deltas are reverted when any write fails, so a failed save cannot
// leave peers with a half-written allowed_ips list.
func (v *VpnService) SetSplitTunnelLocked(cfg models.SplitTunnelConfig) error {
	allowedParts, err := splitTunnelAllowedIPs(cfg)
	if err != nil {
		return err
	}

	sections, err := v.uci.GetSections("network")
	if err != nil {
		return err
	}
	// Snapshot the current values so a partial write can be reverted.
	var snapshots []splitTunnelPeerSnapshot
	for name, opts := range sections {
		if !strings.HasPrefix(opts[".type"], "wireguard_") {
			continue
		}
		var allowed []string
		for _, cidr := range strings.Split(opts["allowed_ips"], ",") {
			if trimmed := strings.TrimSpace(cidr); trimmed != "" {
				allowed = append(allowed, trimmed)
			}
		}
		snapshots = append(snapshots, splitTunnelPeerSnapshot{name: name, allowed: allowed})
	}
	slices.SortFunc(snapshots, func(a, b splitTunnelPeerSnapshot) int { return strings.Compare(a.name, b.name) })

	for _, snap := range snapshots {
		if err := v.setPeerAllowedIPs(snap.name, allowedParts); err != nil {
			if rbErr := v.restorePeerAllowedIPs(snapshots); rbErr != nil {
				return fmt.Errorf("updating split tunnel: %w (revert incomplete: %v)", err, rbErr)
			}
			return fmt.Errorf("updating split tunnel: %w", err)
		}
	}
	if err := v.uci.Commit("network"); err != nil {
		if rbErr := v.restorePeerAllowedIPs(snapshots); rbErr != nil {
			return fmt.Errorf("committing split tunnel: %w (revert incomplete: %v)", err, rbErr)
		}
		return fmt.Errorf("committing split tunnel: %w", err)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(splitTunnelPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(splitTunnelPath, data, 0o644)
}

// splitTunnelAllowedIPs validates the split tunnel config and returns the
// allowed_ips values to write to every WireGuard peer.
func splitTunnelAllowedIPs(cfg models.SplitTunnelConfig) ([]string, error) {
	switch cfg.Mode {
	case "all", "":
		return []string{"0.0.0.0/0", "::/0"}, nil
	case "custom":
		var routes []string
		for _, route := range cfg.Routes {
			if trimmed := strings.TrimSpace(route); trimmed != "" {
				routes = append(routes, trimmed)
			}
		}
		if len(routes) == 0 {
			return nil, errors.New("custom split tunnel requires at least one route")
		}
		return routes, nil
	default:
		return nil, fmt.Errorf("unknown split tunnel mode %q (expected \"all\" or \"custom\")", cfg.Mode)
	}
}

// setPeerAllowedIPs replaces a peer's allowed_ips with the given CIDR list.
func (v *VpnService) setPeerAllowedIPs(peer string, cidrs []string) error {
	_ = v.uci.DeleteOption("network", peer, "allowed_ips")
	for _, cidr := range cidrs {
		if err := v.uci.AddList("network", peer, "allowed_ips", cidr); err != nil {
			return fmt.Errorf("set allowed_ips %s: %w", cidr, err)
		}
	}
	return v.uci.Set("network", peer, "route_allowed_ips", "1")
}

// splitTunnelPeerSnapshot holds a peer's allowed_ips before a split tunnel change.
type splitTunnelPeerSnapshot struct {
	name    string
	allowed []string
}

// restorePeerAllowedIPs reverts peers to their pre-change allowed_ips values.
func (v *VpnService) restorePeerAllowedIPs(snapshots []splitTunnelPeerSnapshot) error {
	var errs []error
	for _, snap := range snapshots {
		if len(snap.allowed) == 0 {
			if err := v.uci.DeleteOption("network", snap.name, "allowed_ips"); err != nil {
				errs = append(errs, fmt.Errorf("clearing allowed_ips on %s: %w", snap.name, err))
			}
			continue
		}
		if err := v.setPeerAllowedIPs(snap.name, snap.allowed); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if err := v.uci.Commit("network"); err != nil {
		return fmt.Errorf("committing reverted split tunnel: %w", err)
	}
	return nil
}

// GetTailscaleSSHEnabled returns whether Tailscale SSH is enabled.
func (v *VpnService) GetTailscaleSSHEnabled() (bool, error) {
	out, err := v.cmd.Run(tailscaleBin(), "status", "--json")
	if err != nil {
		return false, nil // Tailscale not running
	}
	var status struct {
		Prefs struct {
			RunSSH bool `json:"RunSSH"`
		} `json:"Prefs"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return false, nil
	}
	return status.Prefs.RunSSH, nil
}

// SetTailscaleSSHEnabled enables or disables Tailscale SSH.
func (v *VpnService) SetTailscaleSSHEnabled(enabled bool) error {
	if err := requireTailscale(); err != nil {
		return err
	}
	arg := "--ssh=false"
	if enabled {
		arg = "--ssh"
	}
	_, err := v.cmd.Run(tailscaleBin(), "set", arg)
	return err
}
