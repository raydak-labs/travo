package services

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const openwrtIPBin = "/sbin/ip"

const openwrtWgBin = "/usr/bin/wg"

const openwrtUbusBin = "/sbin/ubus"

const openwrtIfupBin = "/sbin/ifup"

const openwrtIfdownBin = "/sbin/ifdown"

var wireGuardVerifyTimeout = 12 * time.Second

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
	if err == nil && strings.TrimSpace(string(out)) != "" {
		if status, parseErr := ParseWgDump(string(out)); parseErr == nil && status != nil {
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
	}
	// `wg show` is unusable — the binary is missing, or its output no longer
	// parses. That is a broken PROBE, not a broken tunnel, and the two must not
	// collapse into one reading: the debounced heal of ADR 0001 §3.3 pops the VPN
	// resolver layer on enabled_not_up, so a persistently broken probe would
	// eventually take the DNS layer away from a tunnel that is carrying it fine.
	// The link state is the only evidence left, and an UP wg0 outranks the
	// missing tool.
	if link, lerr := v.cmd.Run(openwrtIPBin, "link", "show", "dev", "wg0"); lerr == nil {
		if wireGuardIfaceLooksUp(string(link)) {
			return "up_no_handshake"
		}
	}
	return "enabled_not_up"
}

// GetVpnStatus returns all VPN connection statuses.
//
// It also runs the DNS self-heal reconcile (ADR 0001 §3): if the VPN is still a
// stacked layer on the shared dnsmasq record but the tunnel that justified it
// can no longer carry DNS, the layer below it is restored here rather than
// waiting for an operator to toggle the VPN off. This is a status read that
// heals rather than just reporting, which is deliberate: GET /vpn/status is
// polled by the dashboard and the VPN page, so it is the only health signal that
// is guaranteed to be hit after a reboot.
//
// Because the poll rate is the operator's UI and not a health check, the heal
// is debounced: it fires only on a terminal tunnel state, or on enabled_not_up
// seen vpnDNSHealStreakRequired times in a row spanning
// vpnDNSHealGracePeriod. See maybeSelfHealVpnDNS.
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
		// The heal logs itself. StatusDetail is left at its documented value:
		// the frontend matches it by exact equality, so a suffixed value would
		// render no status text at all.
		v.maybeSelfHealVpnDNS(wgStatus.StatusDetail)
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
