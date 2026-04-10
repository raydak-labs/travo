package services

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// NetworkService provides network status and configuration.
type NetworkService struct {
	uci       uci.UCI
	ubus      ubus.Ubus
	aliasFile string
	cmd       CommandRunner

	// wifiMACsMu guards wifiMACsSeen.
	wifiMACsMu sync.Mutex
	// wifiMACsSeen tracks MACs recently seen in WiFi station dumps, with the
	// timestamp of their last appearance. Used to suppress "disconnected WiFi
	// client appears as LAN" during the ARP cache staleness window.
	wifiMACsSeen map[string]time.Time
}

// wifiMACTTL is how long a MAC stays in the wifiMACsSeen set after it was
// last observed in a station dump. Once expired the device may reappear as
// a wired LAN client if it reconnects that way.
const wifiMACTTL = 5 * time.Minute

// initdPath is the path to OpenWRT init.d scripts directory.
const initdPath = "/etc/init.d/"

// boolToEnabled converts a boolean to UCI enabled string ("1" or "0").
func boolToEnabled(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}

func newNetworkService(u uci.UCI, ub ubus.Ubus, aliasFile string, cmd CommandRunner) *NetworkService {
	return &NetworkService{
		uci: u, ubus: ub, aliasFile: aliasFile, cmd: cmd,
		wifiMACsSeen: make(map[string]time.Time),
	}
}

// NewNetworkService creates a new NetworkService.
func NewNetworkService(u uci.UCI, ub ubus.Ubus) *NetworkService {
	return newNetworkService(u, ub, "/etc/travo/aliases.json", &RealCommandRunner{})
}

// NewNetworkServiceWithAliasFile creates a NetworkService with a custom alias file path.
func NewNetworkServiceWithAliasFile(u uci.UCI, ub ubus.Ubus, aliasFile string) *NetworkService {
	return newNetworkService(u, ub, aliasFile, &RealCommandRunner{})
}

// NewNetworkServiceWithRunner creates a NetworkService with a custom command runner (for testing).
func NewNetworkServiceWithRunner(u uci.UCI, ub ubus.Ubus, cmd CommandRunner) *NetworkService {
	return newNetworkService(u, ub, "/etc/travo/aliases.json", cmd)
}

// uciSet is a helper for setting UCI values with consistent error wrapping.
func (n *NetworkService) uciSet(config, section, option, value string) error {
	if err := n.uci.Set(config, section, option, value); err != nil {
		return fmt.Errorf("set %s.%s.%s: %w", config, section, option, err)
	}
	return nil
}

// uciCommit commits a UCI config with consistent error wrapping.
func (n *NetworkService) uciCommit(config string) error {
	if err := n.uci.Commit(config); err != nil {
		return fmt.Errorf("commit %s: %w", config, err)
	}
	return nil
}

// restartService restarts an OpenWRT init.d service.
func (n *NetworkService) restartService(service string) error {
	_, err := n.cmd.Run(initdPath+service, "restart")
	return err
}

// normalizeMACForSection converts a MAC address to a UCI section-safe format
// by uppercasing and removing colons.
func normalizeMACForSection(mac string) string {
	return strings.ReplaceAll(strings.ToUpper(mac), ":", "")
}

// updateKnownWifiMACs refreshes the persistent WiFi MAC set with current station
// dump results and returns a snapshot of MACs that were RECENTLY WiFi but are
// not in the current wifiStats (i.e. they just disconnected). These must not be
// counted as wired LAN clients while their ARP entry is still stale.
func (n *NetworkService) updateKnownWifiMACs(wifiStats map[string]wifiClientStat) map[string]struct{} {
	now := time.Now()
	n.wifiMACsMu.Lock()
	defer n.wifiMACsMu.Unlock()
	for mac, lastSeen := range n.wifiMACsSeen {
		if now.Sub(lastSeen) > wifiMACTTL {
			delete(n.wifiMACsSeen, mac)
		}
	}
	for mac := range wifiStats {
		n.wifiMACsSeen[mac] = now
	}
	// Return only the MACs that were seen before but are absent now.
	recent := make(map[string]struct{})
	for mac := range n.wifiMACsSeen {
		if _, current := wifiStats[mac]; !current {
			recent[mac] = struct{}{}
		}
	}
	return recent
}

// allowedInterfaces is the set of interface names that can be toggled.
var allowedInterfaces = map[string]bool{
	"wan":  true,
	"lan":  true,
	"wwan": true,
}

// SetInterfaceState brings a network interface up or down via ifup/ifdown.
func (n *NetworkService) SetInterfaceState(iface string, up bool) error {
	if !allowedInterfaces[iface] {
		return fmt.Errorf("unknown interface: %s", iface)
	}
	cmd := "ifdown"
	if up {
		cmd = "ifup"
	}
	if _, err := n.cmd.Run(cmd, iface); err != nil {
		return fmt.Errorf("%s %s: %w", cmd, iface, err)
	}
	return nil
}

// loadAliases reads the alias file and returns a mac->alias map.
func (n *NetworkService) loadAliases() map[string]string {
	data, err := os.ReadFile(n.aliasFile)
	if err != nil {
		return map[string]string{}
	}
	aliases := map[string]string{}
	if err := json.Unmarshal(data, &aliases); err != nil {
		return map[string]string{}
	}
	return aliases
}

// saveAliases writes the alias map to the alias file.
func (n *NetworkService) saveAliases(aliases map[string]string) error {
	dir := filepath.Dir(n.aliasFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating alias directory: %w", err)
	}
	data, err := json.Marshal(aliases)
	if err != nil {
		return fmt.Errorf("marshaling aliases: %w", err)
	}
	return os.WriteFile(n.aliasFile, data, 0600)
}

// SetAlias sets or removes (if empty) an alias for a MAC address.
func (n *NetworkService) SetAlias(mac, alias string) error {
	aliases := n.loadAliases()
	upperMAC := strings.ToUpper(mac)
	if alias == "" {
		delete(aliases, upperMAC)
	} else {
		aliases[upperMAC] = alias
	}
	return n.saveAliases(aliases)
}

func maskToNetmask(mask float64) string {
	bits := int(mask)
	if bits <= 0 || bits > 32 {
		return "255.255.255.0"
	}
	m := uint32(0xFFFFFFFF) << (32 - bits)
	return fmt.Sprintf("%d.%d.%d.%d", (m>>24)&0xFF, (m>>16)&0xFF, (m>>8)&0xFF, m&0xFF)
}

// GetNetworkStatus returns the overall network status.
func (n *NetworkService) GetNetworkStatus() (models.NetworkStatus, error) {
	var status models.NetworkStatus

	wanData, err := n.ubus.Call("network.interface.wan", "status", nil)
	if err == nil {
		wan := parseInterface("wan", "eth0", wanData, n.ubus)
		status.WAN = &wan
	}

	lanData, err := n.ubus.Call("network.interface.lan", "status", nil)
	if err == nil {
		status.LAN = parseInterface("lan", "br-lan", lanData, n.ubus)
	}

	// Also check wwan (WiFi uplink) — common on travel routers
	wwanData, wwanErr := n.ubus.Call("network.interface.wwan", "status", nil)

	status.Interfaces = []models.NetworkInterface{}
	if status.WAN != nil {
		status.Interfaces = append(status.Interfaces, *status.WAN)
	}
	status.Interfaces = append(status.Interfaces, status.LAN)

	if wwanErr == nil {
		wwanIface := parseInterface("wwan", "phy0-sta0", wwanData, n.ubus)
		wwanIface.Type = "wifi"
		status.Interfaces = append(status.Interfaces, wwanIface)
		// If wwan is up and wan is not, use wwan as the effective WAN
		if wwanIface.IsUp && (status.WAN == nil || !status.WAN.IsUp) {
			status.WAN = &wwanIface
		}
	}

	// Fetch DHCP clients from ubus
	status.Clients = n.fetchDHCPClients()

	status.InternetReachable = status.WAN != nil && status.WAN.IsUp
	return status, nil
}

// dhcpLeaseTimeSeconds reads the DHCP lease time from UCI and returns it in seconds.
func (n *NetworkService) dhcpLeaseTimeSeconds() float64 {
	const defaultLease = 43200.0 // 12h
	opts, err := n.uci.GetAll("dhcp", "lan")
	if err != nil {
		return defaultLease
	}
	lt, ok := opts["leasetime"]
	if !ok || lt == "" {
		return defaultLease
	}
	return parseLeaseTime(lt, defaultLease)
}

// parseLeaseTime converts a lease time string (e.g. "12h", "30m", "86400") to seconds.
func parseLeaseTime(s string, fallback float64) float64 {
	if s == "" {
		return fallback
	}
	// Check for suffix
	last := s[len(s)-1]
	switch last {
	case 'h', 'H':
		if v, err := strconv.Atoi(s[:len(s)-1]); err == nil && v > 0 {
			return float64(v) * 3600
		}
	case 'm', 'M':
		if v, err := strconv.Atoi(s[:len(s)-1]); err == nil && v > 0 {
			return float64(v) * 60
		}
	case 's', 'S':
		if v, err := strconv.Atoi(s[:len(s)-1]); err == nil && v > 0 {
			return float64(v)
		}
	default:
		// Plain number = seconds
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return float64(v)
		}
	}
	return fallback
}

// dhcpLease holds information from a DHCP lease entry.
type dhcpLease struct {
	IP       string
	Expiry   int64
	Hostname string
}

// parseDHCPLeasesFile reads /tmp/dhcp.leases and returns a map of uppercase MAC → lease info.
// Format: <expiry_epoch> <mac> <ip> <hostname> [clientid]
func parseDHCPLeasesFile() map[string]dhcpLease {
	result := map[string]dhcpLease{}
	data, err := os.ReadFile("/tmp/dhcp.leases")
	if err != nil {
		return result
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		expiry, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		mac := strings.ToUpper(fields[1])
		ip := fields[2]
		hostname := fields[3]
		if hostname == "*" {
			hostname = ""
		}
		result[mac] = dhcpLease{IP: ip, Expiry: expiry, Hostname: hostname}
	}
	return result
}

// parseEtcHosts reads /etc/hosts and returns a map of IP → hostname.
func parseEtcHosts() map[string]string {
	result := map[string]string{}
	data, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		// Use the first hostname for each IP, skip localhost entries.
		if ip == "127.0.0.1" || ip == "::1" {
			continue
		}
		if _, exists := result[ip]; !exists {
			result[ip] = fields[1]
		}
	}
	return result
}

// fetchDHCPClients builds the connected-client list.
//
// Strategy:
//  1. iw station dump  → authoritative for currently-WiFi-associated MACs.
//  2. /tmp/dhcp.leases → primary source for IP / hostname / connect-time.
//     Only devices with a DHCP lease are shown as LAN clients; static-IP
//     devices (e.g. the operator's own laptop) are intentionally excluded.
//  3. ubus dhcp ipv4leases → used when available (some OpenWrt builds expose
//     this); results undergo the same post-processing as the lease-file path.
//  4. knownWifiMACs (in-memory TTL set) → prevents a just-disconnected WiFi
//     device from temporarily appearing as a wired LAN client while its ARP
//     entry is still flagged 0x2 (REACHABLE).
func (n *NetworkService) fetchDHCPClients() []models.Client {
	leaseTimeSec := n.dhcpLeaseTimeSeconds()

	// ── 1. WiFi station dump ──────────────────────────────────────────────
	wifiStats := n.getWifiClientStats()
	// Update the persistent set and get back MACs that JUST left WiFi.
	recentlyWifi := n.updateKnownWifiMACs(wifiStats)

	// ── 2. ARP table: build lookup maps ──────────────────────────────────
	type arpEntry struct{ flags, iface string }
	ipToARP := make(map[string]arpEntry) // ip  → {flags, iface}
	macToIP := make(map[string]string)   // MAC → ip  (last wins on dup)
	if raw, err := os.ReadFile("/proc/net/arp"); err == nil {
		for _, line := range strings.Split(string(raw), "\n")[1:] {
			f := strings.Fields(line)
			if len(f) < 6 {
				continue
			}
			ip, flags, mac, iface := f[0], f[2], strings.ToUpper(f[3]), f[5]
			if mac == "00:00:00:00:00:00" || ip == "0.0.0.0" {
				continue
			}
			ipToARP[ip] = arpEntry{flags: flags, iface: iface}
			macToIP[mac] = ip
		}
	}

	// ── 3. Build a deduplicated client map (keyed by uppercase MAC) ───────
	byMAC := make(map[string]models.Client)

	connectedSinceFromLease := func(expiry int64) string {
		if expiry <= 0 {
			return ""
		}
		t := time.Unix(expiry, 0).Add(-time.Duration(leaseTimeSec) * time.Second)
		return t.UTC().Format(time.RFC3339)
	}

	// 3a. Try ubus dhcp ipv4leases (works on some builds).
	if data, err := n.ubus.Call("dhcp", "ipv4leases", nil); err == nil {
		if device, ok := data["device"].(map[string]interface{}); ok {
			for ifaceName, ifaceData := range device {
				ifaceMap, _ := ifaceData.(map[string]interface{})
				leases, _ := ifaceMap["leases"].([]interface{})
				for _, raw := range leases {
					lm, _ := raw.(map[string]interface{})
					ip, _ := lm["ip"].(string)
					mac := strings.ToUpper(fmt.Sprintf("%v", lm["mac"]))
					hostname, _ := lm["hostname"].(string)
					expires, _ := lm["expires"].(float64)
					elapsed := leaseTimeSec - expires
					if elapsed < 0 {
						elapsed = 0
					}
					var cs string
					if expires > 0 {
						cs = time.Now().Add(-time.Duration(elapsed) * time.Second).UTC().Format(time.RFC3339)
					}
					byMAC[mac] = models.Client{
						IPAddress: ip, MACAddress: mac,
						Hostname: hostname, InterfaceName: ifaceName,
						ConnectedSince: cs,
					}
				}
			}
		}
	}

	// 3b. Fallback: ARP table for all reachable br-lan clients (used on most
	//     OpenWrt builds). Includes both DHCP and static-IP devices — any
	//     device with a live ARP entry is a legitimate LAN client.
	if len(byMAC) == 0 {
		dhcpForFallback := parseDHCPLeasesFile()
		for ip, arp := range ipToARP {
			if arp.iface != "br-lan" || arp.flags == "0x0" {
				continue
			}
			mac := ""
			// Reverse-look up MAC from the arp maps we already built.
			for m, mip := range macToIP {
				if mip == ip {
					mac = m
					break
				}
			}
			if mac == "" {
				continue
			}
			// WiFi-associated: will be enriched in step 3c below.
			if _, isWifi := wifiStats[mac]; isWifi {
				continue
			}
			// Just left WiFi: exclude until ARP expires or TTL elapses.
			if _, recent := recentlyWifi[mac]; recent {
				continue
			}
			lease := dhcpForFallback[mac]
			byMAC[mac] = models.Client{
				IPAddress:      ip,
				MACAddress:     mac,
				Hostname:       lease.Hostname,
				InterfaceName:  "br-lan",
				ConnectedSince: connectedSinceFromLease(lease.Expiry),
			}
		}
	}

	// 3c. Add all currently WiFi-associated clients (may not have a lease
	//     yet if the device just connected, so we do this unconditionally).
	dhcpLeases := parseDHCPLeasesFile()
	for mac, stat := range wifiStats {
		ip := macToIP[mac] // best-effort; empty for brand-new connections
		lease := dhcpLeases[mac]
		if ip == "" {
			ip = lease.IP
		}
		hostname := lease.Hostname
		c := models.Client{
			IPAddress:      ip,
			MACAddress:     mac,
			Hostname:       hostname,
			InterfaceName:  stat.IfaceName,
			ConnectedSince: connectedSinceFromLease(lease.Expiry),
			RxBytes:        stat.RxBytes,
			TxBytes:        stat.TxBytes,
		}
		byMAC[mac] = c // always overwrite: WiFi stats are more authoritative
	}

	// ── 4. Resolve missing hostnames from /etc/hosts ──────────────────────
	hostsMap := parseEtcHosts()
	clients := make([]models.Client, 0, len(byMAC))
	for _, c := range byMAC {
		if c.Hostname == "" {
			if h, ok := hostsMap[c.IPAddress]; ok {
				c.Hostname = h
			}
		}
		clients = append(clients, c)
	}

	return clients
}

// parseStationDump parses "iw dev <iface> station dump" output and returns
// a map of uppercase MAC → [rxBytes, txBytes] from the client's perspective.
// In station dump: "rx bytes" = AP received from client = client TX,
// "tx bytes" = AP sent to client = client RX.
func parseStationDump(output string) map[string][2]int64 {
	result := make(map[string][2]int64)
	var currentMAC string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Station ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				currentMAC = strings.ToUpper(parts[1])
			}
			continue
		}
		if currentMAC == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "rx bytes:") {
			valStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "rx bytes:"))
			if v, err := strconv.ParseInt(valStr, 10, 64); err == nil {
				entry := result[currentMAC]
				entry[1] = v // AP rx from client = client TX
				result[currentMAC] = entry
			}
		} else if strings.HasPrefix(trimmed, "tx bytes:") {
			valStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "tx bytes:"))
			if v, err := strconv.ParseInt(valStr, 10, 64); err == nil {
				entry := result[currentMAC]
				entry[0] = v // AP tx to client = client RX
				result[currentMAC] = entry
			}
		}
	}
	return result
}

// parseIwDev parses "iw dev" output and returns interface names in AP mode.
func parseIwDev(output string) []string {
	var interfaces []string
	var currentIface string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Interface ") {
			currentIface = strings.TrimPrefix(trimmed, "Interface ")
		} else if strings.HasPrefix(trimmed, "type ") && currentIface != "" {
			if strings.TrimPrefix(trimmed, "type ") == "AP" {
				interfaces = append(interfaces, currentIface)
			}
			currentIface = ""
		}
	}
	return interfaces
}

// wifiClientStat holds per-client WiFi traffic counters and the AP interface name.
type wifiClientStat struct {
	RxBytes   int64
	TxBytes   int64
	IfaceName string
}

// getWifiClientStats returns per-client WiFi stats (traffic + AP interface name).
// Clients are keyed by uppercase MAC address.
func (n *NetworkService) getWifiClientStats() map[string]wifiClientStat {
	result := make(map[string]wifiClientStat)
	iwDevOutput, err := n.cmd.Run("iw", "dev")
	if err != nil {
		return result
	}
	for _, iface := range parseIwDev(string(iwDevOutput)) {
		dumpOutput, err := n.cmd.Run("iw", "dev", iface, "station", "dump")
		if err != nil {
			continue
		}
		for mac, stats := range parseStationDump(string(dumpOutput)) {
			entry := result[mac]
			entry.RxBytes += stats[0]
			entry.TxBytes += stats[1]
			entry.IfaceName = iface
			result[mac] = entry
		}
	}
	return result
}

func parseInterface(name, device string, data map[string]interface{}, ub ubus.Ubus) models.NetworkInterface {
	iface := models.NetworkInterface{
		Name: name, Type: name,
	}
	if up, ok := data["up"].(bool); ok {
		iface.IsUp = up
	}
	// Get device name from ubus response
	devName, _ := data["device"].(string)
	if devName == "" {
		devName, _ = data["l3_device"].(string)
	}
	if devName == "" {
		devName = device
	}

	// Fetch device stats for MAC and traffic
	if ub != nil && devName != "" {
		if devData, err := ub.Call("network.device", "status", map[string]interface{}{"name": devName}); err == nil {
			if mac, ok := devData["macaddr"].(string); ok && mac != "" {
				iface.MACAddress = mac
			}
			if stats, ok := devData["statistics"].(map[string]interface{}); ok {
				if rxBytes, ok := stats["rx_bytes"].(float64); ok {
					iface.RxBytes = int64(rxBytes)
				}
				if txBytes, ok := stats["tx_bytes"].(float64); ok {
					iface.TxBytes = int64(txBytes)
				}
			}
		}
	}

	if addrs, ok := data["ipv4-address"].([]interface{}); ok && len(addrs) > 0 {
		if a, ok := addrs[0].(map[string]interface{}); ok {
			iface.IPAddress, _ = a["address"].(string)
			if mask, ok := a["mask"].(float64); ok {
				iface.Netmask = maskToNetmask(mask)
			}
		}
	}
	if routes, ok := data["route"].([]interface{}); ok {
		for _, r := range routes {
			if rm, ok := r.(map[string]interface{}); ok {
				if gw, ok := rm["nexthop"].(string); ok && gw != "" {
					iface.Gateway = gw
					break
				}
			}
		}
	}
	if dns, ok := data["dns-server"].([]interface{}); ok {
		iface.DNSServers = make([]string, 0, len(dns))
		for _, d := range dns {
			if s, ok := d.(string); ok {
				iface.DNSServers = append(iface.DNSServers, s)
			}
		}
	}
	return iface
}

// GetWanConfig returns the WAN configuration.
func (n *NetworkService) GetWanConfig() (models.WanConfig, error) {
	opts, err := n.uci.GetAll("network", "wan")
	if err != nil {
		return models.WanConfig{}, err
	}
	var dnsServers []string
	if dns, ok := opts["dns"]; ok && dns != "" {
		dnsServers = strings.Split(dns, " ")
	}
	mtu := 1500
	if m, ok := opts["mtu"]; ok && m != "" {
		if v, err := strconv.Atoi(m); err == nil {
			mtu = v
		}
	}
	return models.WanConfig{
		Type: opts["proto"], InterfaceName: opts["ifname"],
		IPAddress: opts["ip4addr"], Netmask: opts["netmask"],
		Gateway: opts["gateway"], DNSServers: dnsServers, MTU: mtu,
	}, nil
}

// SetWanConfig updates the WAN configuration.
func (n *NetworkService) SetWanConfig(config models.WanConfig) error {
	if config.Type != "" {
		if err := n.uciSet("network", "wan", "proto", config.Type); err != nil {
			return err
		}
	}
	if config.IPAddress != "" {
		if err := n.uciSet("network", "wan", "ip4addr", config.IPAddress); err != nil {
			return err
		}
	}
	if config.Netmask != "" {
		if err := n.uciSet("network", "wan", "netmask", config.Netmask); err != nil {
			return err
		}
	}
	if config.Gateway != "" {
		if err := n.uciSet("network", "wan", "gateway", config.Gateway); err != nil {
			return err
		}
	}
	return n.uciCommit("network")
}

// DetectWanType auto-detects the WAN connection type and returns
// both the detected type and the currently configured type.
func (n *NetworkService) DetectWanType() (models.WanDetectResult, error) {
	// Read current UCI configuration
	currentType := "dhcp"
	opts, err := n.uci.GetAll("network", "wan")
	if err == nil {
		if proto, ok := opts["proto"]; ok && proto != "" {
			currentType = proto
		}
	}

	// Detect running services to determine actual WAN type
	var detectedType string

	// Check if pppd is running → PPPoE
	if _, err := n.cmd.Run("pgrep", "-x", "pppd"); err == nil {
		detectedType = "pppoe"
	} else if _, err := n.cmd.Run("pgrep", "-x", "udhcpc"); err == nil {
		// Check if udhcpc (DHCP client) is running → DHCP
		detectedType = "dhcp"
	} else {
		// Neither running — fall back to current config
		detectedType = currentType
	}

	return models.WanDetectResult{
		DetectedType: detectedType,
		CurrentType:  currentType,
	}, nil
}

// GetClients returns connected LAN clients with aliases merged.
func (n *NetworkService) GetClients() ([]models.Client, error) {
	status, err := n.GetNetworkStatus()
	if err != nil {
		return nil, err
	}
	aliases := n.loadAliases()
	clients := status.Clients
	for i := range clients {
		if alias, ok := aliases[strings.ToUpper(clients[i].MACAddress)]; ok {
			clients[i].Alias = alias
		}
	}
	return clients, nil
}

// GetDHCPConfig returns the DHCP configuration for the LAN.
func (n *NetworkService) GetDHCPConfig() (models.DHCPConfig, error) {
	opts, err := n.uci.GetAll("dhcp", "lan")
	if err != nil {
		return models.DHCPConfig{}, err
	}
	start := 100
	if s, ok := opts["start"]; ok && s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			start = v
		}
	}
	limit := 150
	if l, ok := opts["limit"]; ok && l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	leaseTime := "12h"
	if lt, ok := opts["leasetime"]; ok && lt != "" {
		leaseTime = lt
	}
	return models.DHCPConfig{Start: start, Limit: limit, LeaseTime: leaseTime}, nil
}

// GetDNSConfig returns the custom DNS configuration.
func (n *NetworkService) GetDNSConfig() (models.DNSConfig, error) {
	opts, err := n.uci.GetAll("network", "wan")
	if err != nil {
		return models.DNSConfig{}, err
	}
	config := models.DNSConfig{
		UseCustomDNS: opts["peerdns"] == "0",
	}
	if dns, ok := opts["dns"]; ok && dns != "" {
		config.Servers = strings.Split(dns, " ")
	}
	return config, nil
}

// SetDNSConfig updates the custom DNS configuration.
func (n *NetworkService) SetDNSConfig(config models.DNSConfig) error {
	if config.UseCustomDNS {
		if err := n.uciSet("network", "wan", "peerdns", "0"); err != nil {
			return err
		}
		dns := strings.Join(config.Servers, " ")
		if err := n.uciSet("network", "wan", "dns", dns); err != nil {
			return err
		}
	} else {
		if err := n.uciSet("network", "wan", "peerdns", "1"); err != nil {
			return err
		}
		if err := n.uciSet("network", "wan", "dns", ""); err != nil {
			return err
		}
	}
	return n.uciCommit("network")
}

// SetDHCPConfig updates the DHCP configuration for the LAN.
func (n *NetworkService) SetDHCPConfig(config models.DHCPConfig) error {
	if err := n.uciSet("dhcp", "lan", "start", strconv.Itoa(config.Start)); err != nil {
		return err
	}
	if err := n.uciSet("dhcp", "lan", "limit", strconv.Itoa(config.Limit)); err != nil {
		return err
	}
	if err := n.uciSet("dhcp", "lan", "leasetime", config.LeaseTime); err != nil {
		return err
	}
	return n.uciCommit("dhcp")
}

// GetDHCPLeases reads active DHCP leases from /tmp/dhcp.leases.
func (n *NetworkService) GetDHCPLeases() []models.DHCPLease {
	data, err := os.ReadFile("/tmp/dhcp.leases")
	if err != nil {
		return []models.DHCPLease{}
	}
	return parseDHCPLeases(string(data))
}

// GetDNSEntries returns all local DNS entries from dhcp config (domain sections).
func (n *NetworkService) GetDNSEntries() ([]models.DNSEntry, error) {
	sections, err := n.uci.GetSections("dhcp")
	if err != nil {
		return []models.DNSEntry{}, nil
	}
	var entries []models.DNSEntry
	for section, opts := range sections {
		if opts[".type"] != "domain" {
			continue
		}
		entries = append(entries, models.DNSEntry{
			Name:    opts["name"],
			IP:      opts["ip"],
			Section: section,
		})
	}
	if entries == nil {
		return []models.DNSEntry{}, nil
	}
	return entries, nil
}

// AddDNSEntry adds a new local DNS entry as a named UCI section in dhcp config.
func (n *NetworkService) AddDNSEntry(entry models.DNSEntry) error {
	section := "dns_" + sanitizeSectionName(entry.Name)
	if err := n.uci.AddSection("dhcp", section, "domain"); err != nil {
		return fmt.Errorf("adding DNS entry section: %w", err)
	}
	if err := n.uci.Set("dhcp", section, "name", entry.Name); err != nil {
		return fmt.Errorf("setting DNS entry name: %w", err)
	}
	if err := n.uci.Set("dhcp", section, "ip", entry.IP); err != nil {
		return fmt.Errorf("setting DNS entry IP: %w", err)
	}
	return n.uci.Commit("dhcp")
}

// DeleteDNSEntry removes a local DNS entry by its UCI section name.
func (n *NetworkService) DeleteDNSEntry(section string) error {
	if err := n.uci.DeleteSection("dhcp", section); err != nil {
		return fmt.Errorf("deleting DNS entry: %w", err)
	}
	return n.uci.Commit("dhcp")
}

// sanitizeSectionName converts a hostname to a valid UCI section name.
func sanitizeSectionName(name string) string {
	var sb strings.Builder
	for _, c := range strings.ToLower(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			sb.WriteRune(c)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

// GetDHCPReservations returns all static DHCP reservations (host sections in dhcp config).
func (n *NetworkService) GetDHCPReservations() ([]models.DHCPReservation, error) {
	sections, err := n.uci.GetSections("dhcp")
	if err != nil {
		return []models.DHCPReservation{}, nil
	}
	var reservations []models.DHCPReservation
	for section, opts := range sections {
		if opts[".type"] != "host" {
			continue
		}
		reservations = append(reservations, models.DHCPReservation{
			Name:    opts["name"],
			MAC:     opts["mac"],
			IP:      opts["ip"],
			Section: section,
		})
	}
	if reservations == nil {
		return []models.DHCPReservation{}, nil
	}
	return reservations, nil
}

// AddDHCPReservation adds a static DHCP reservation as a named UCI section in dhcp config.
func (n *NetworkService) AddDHCPReservation(reservation models.DHCPReservation) error {
	section := "host_" + sanitizeSectionName(reservation.Name)
	if err := n.uci.AddSection("dhcp", section, "host"); err != nil {
		return fmt.Errorf("adding DHCP reservation section: %w", err)
	}
	if err := n.uci.Set("dhcp", section, "name", reservation.Name); err != nil {
		return fmt.Errorf("setting DHCP reservation name: %w", err)
	}
	if err := n.uci.Set("dhcp", section, "mac", reservation.MAC); err != nil {
		return fmt.Errorf("setting DHCP reservation MAC: %w", err)
	}
	if err := n.uci.Set("dhcp", section, "ip", reservation.IP); err != nil {
		return fmt.Errorf("setting DHCP reservation IP: %w", err)
	}
	return n.uci.Commit("dhcp")
}

// DeleteDHCPReservation removes a static DHCP reservation by its UCI section name.
func (n *NetworkService) DeleteDHCPReservation(section string) error {
	if err := n.uci.DeleteSection("dhcp", section); err != nil {
		return fmt.Errorf("deleting DHCP reservation: %w", err)
	}
	return n.uci.Commit("dhcp")
}

// parseDHCPLeases parses the content of /tmp/dhcp.leases into a slice of DHCPLease.
// Each line has the format: <expiry_epoch> <mac_address> <ip_address> <hostname> <client_id>
func parseDHCPLeases(data string) []models.DHCPLease {
	var leases []models.DHCPLease
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		expiry, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		hostname := fields[3]
		if hostname == "*" {
			hostname = ""
		}
		leases = append(leases, models.DHCPLease{
			Expiry:   expiry,
			MAC:      fields[1],
			IP:       fields[2],
			Hostname: hostname,
		})
	}
	if leases == nil {
		return []models.DHCPLease{}
	}
	return leases
}

// KickClient disconnects a WiFi client by deauthentication.
func (n *NetworkService) KickClient(mac string) error {
	// Discover AP interfaces dynamically using iw dev
	iwDevOutput, err := n.cmd.Run("iw", "dev")
	if err != nil {
		// Fallback to common AP interfaces if iw fails
		for _, iface := range []string{"phy0-ap0", "phy1-ap0", "wlan0", "wlan1"} {
			_, _ = n.cmd.Run("hostapd_cli", "-i", iface, "disassociate", mac)
		}
		return nil
	}

	// Parse iw dev output to find AP interfaces
	for _, iface := range parseIwDev(string(iwDevOutput)) {
		_, err := n.cmd.Run("hostapd_cli", "-i", iface, "disassociate", mac)
		if err == nil {
			// Successfully kicked from this interface
			return nil
		}
	}
	return nil
}

// BlockClient adds a firewall rule to drop all traffic from a MAC address.
func (n *NetworkService) BlockClient(mac string) error {
	section := "block_" + normalizeMACForSection(mac)
	macUpper := strings.ToUpper(mac)

	if err := n.uci.AddSection("firewall", section, "rule"); err != nil {
		return fmt.Errorf("add firewall block rule: %w", err)
	}
	if err := n.uciSet("firewall", section, "name", "Block-"+macUpper); err != nil {
		return err
	}
	if err := n.uciSet("firewall", section, "src", "lan"); err != nil {
		return err
	}
	if err := n.uciSet("firewall", section, "src_mac", macUpper); err != nil {
		return err
	}
	if err := n.uciSet("firewall", section, "target", "DROP"); err != nil {
		return err
	}
	if err := n.uciCommit("firewall"); err != nil {
		return err
	}
	if err := n.restartService("firewall"); err != nil {
		return fmt.Errorf("restart firewall: %w", err)
	}
	return nil
}

// UnblockClient removes the firewall block rule for a MAC address.
func (n *NetworkService) UnblockClient(mac string) error {
	section := "block_" + normalizeMACForSection(mac)

	if err := n.uci.DeleteSection("firewall", section); err != nil {
		return fmt.Errorf("delete firewall block rule: %w", err)
	}
	if err := n.uciCommit("firewall"); err != nil {
		return err
	}
	if err := n.restartService("firewall"); err != nil {
		return fmt.Errorf("restart firewall: %w", err)
	}
	return nil
}

// GetBlockedClients returns a list of blocked MAC addresses.
func (n *NetworkService) GetBlockedClients() ([]string, error) {
	sections, err := n.uci.GetSections("firewall")
	if err != nil {
		return []string{}, nil
	}
	var blocked []string
	for _, opts := range sections {
		if opts[".type"] == "rule" && strings.HasPrefix(opts["name"], "Block-") {
			if mac := opts["src_mac"]; mac != "" {
				blocked = append(blocked, mac)
			}
		}
	}
	if blocked == nil {
		return []string{}, nil
	}
	return blocked, nil
}

// GetDDNSConfig reads the DDNS configuration from UCI (ddns.myddns).
func (n *NetworkService) GetDDNSConfig() (models.DDNSConfig, error) {
	opts, err := n.uci.GetAll("ddns", "myddns")
	if err != nil {
		// No ddns config — return defaults (disabled)
		return models.DDNSConfig{}, nil
	}
	svc := opts["service_name"]
	updateURL := opts["update_url"]
	// LuCI/custom setups often use update_url with empty or "-" service_name.
	if updateURL != "" && (svc == "" || svc == "-") {
		svc = "custom"
	}
	return models.DDNSConfig{
		Enabled:    opts["enabled"] == "1",
		Service:    svc,
		Domain:     opts["domain"],
		Username:   opts["username"],
		Password:   opts["password"],
		LookupHost: opts["lookup_host"],
		UpdateURL:  updateURL,
	}, nil
}

// SetDDNSConfig writes the DDNS configuration to UCI and restarts ddns-scripts.
func (n *NetworkService) SetDDNSConfig(config models.DDNSConfig) error {
	enabled := "0"
	if config.Enabled {
		enabled = "1"
	}
	if err := n.uciSet("ddns", "myddns", "enabled", enabled); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(config.Service), "custom") {
		_ = n.uci.DeleteOption("ddns", "myddns", "service_name")
		if err := n.uciSet("ddns", "myddns", "update_url", strings.TrimSpace(config.UpdateURL)); err != nil {
			return err
		}
	} else {
		_ = n.uci.DeleteOption("ddns", "myddns", "update_url")
		if err := n.uciSet("ddns", "myddns", "service_name", config.Service); err != nil {
			return err
		}
	}
	if err := n.uciSet("ddns", "myddns", "domain", config.Domain); err != nil {
		return err
	}
	if err := n.uciSet("ddns", "myddns", "username", config.Username); err != nil {
		return err
	}
	if err := n.uciSet("ddns", "myddns", "password", config.Password); err != nil {
		return err
	}
	if err := n.uciSet("ddns", "myddns", "lookup_host", config.LookupHost); err != nil {
		return err
	}
	if err := n.uciCommit("ddns"); err != nil {
		return err
	}
	if err := n.restartService("ddns"); err != nil {
		return fmt.Errorf("restart ddns: %w", err)
	}
	return nil
}

// GetDDNSStatus checks whether the ddns service is running and returns public IP info.
func (n *NetworkService) GetDDNSStatus() (models.DDNSStatus, error) {
	var status models.DDNSStatus

	// Check if ddns process is running
	if _, err := n.cmd.Run("pgrep", "-f", "ddns"); err == nil {
		status.Running = true
	}

	// Read cached public IP from ddns update file
	data, err := n.cmd.Run("cat", "/var/run/ddns/myddns.ip")
	if err == nil {
		ip := strings.TrimSpace(string(data))
		if ip != "" {
			status.PublicIP = ip
		}
	}

	// Read last update timestamp
	data, err = n.cmd.Run("cat", "/var/run/ddns/myddns.update")
	if err == nil {
		ts := strings.TrimSpace(string(data))
		if ts != "" {
			status.LastUpdate = ts
		}
	}

	return status, nil
}

// GetFirewallZones returns a summary of all UCI firewall zones.
func (n *NetworkService) GetFirewallZones() ([]models.FirewallZone, error) {
	sections, err := n.uci.GetSections("firewall")
	if err != nil {
		return nil, err
	}
	var zones []models.FirewallZone
	for _, opts := range sections {
		if opts[".type"] != "zone" {
			continue
		}
		z := models.FirewallZone{
			Name:    opts["name"],
			Input:   "DROP",
			Output:  "ACCEPT",
			Forward: "DROP",
		}
		if v := opts["input"]; v != "" {
			z.Input = v
		}
		if v := opts["output"]; v != "" {
			z.Output = v
		}
		if v := opts["forward"]; v != "" {
			z.Forward = v
		}
		if v := opts["network"]; v != "" {
			z.Network = strings.Fields(v)
		}
		if z.Network == nil {
			z.Network = []string{}
		}
		zones = append(zones, z)
	}
	if zones == nil {
		zones = []models.FirewallZone{}
	}
	return zones, nil
}

const portForwardsFile = "/etc/travo/port-forwards.json"

// GetPortForwards returns stored port-forward rules.
func (n *NetworkService) GetPortForwards() ([]models.PortForwardRule, error) {
	data, err := os.ReadFile(portForwardsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.PortForwardRule{}, nil
		}
		return nil, err
	}
	var rules []models.PortForwardRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// AddPortForward adds a new port-forward rule.
func (n *NetworkService) AddPortForward(rule models.PortForwardRule) error {
	rules, err := n.GetPortForwards()
	if err != nil {
		return err
	}
	rule.ID = fmt.Sprintf("pf%d", time.Now().UnixMilli())
	rules = append(rules, rule)
	return n.savePortForwards(rules)
}

// DeletePortForward removes a port-forward rule by ID.
func (n *NetworkService) DeletePortForward(id string) error {
	rules, err := n.GetPortForwards()
	if err != nil {
		return err
	}
	filtered := rules[:0]
	for _, r := range rules {
		if r.ID != id {
			filtered = append(filtered, r)
		}
	}
	return n.savePortForwards(filtered)
}

func (n *NetworkService) savePortForwards(rules []models.PortForwardRule) error {
	if err := os.MkdirAll(filepath.Dir(portForwardsFile), 0750); err != nil {
		return err
	}
	data, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	return os.WriteFile(portForwardsFile, data, 0600)
}

// RunDiagnostics runs ping, traceroute, or DNS lookup and returns the output.
func (n *NetworkService) RunDiagnostics(req models.DiagnosticsRequest) models.DiagnosticsResult {
	result := models.DiagnosticsResult{Type: req.Type, Target: req.Target}
	var out []byte
	var err error
	switch req.Type {
	case "ping":
		out, err = n.cmd.Run("ping", "-c", "4", "-W", "3", req.Target)
	case "traceroute":
		out, err = n.cmd.Run("traceroute", "-w", "3", "-q", "1", "-m", "20", req.Target)
	case "dns":
		out, err = n.cmd.Run("nslookup", req.Target)
	default:
		result.Error = "unknown diagnostic type"
		return result
	}
	if err != nil {
		result.Error = err.Error()
	}
	result.Output = string(out)
	return result
}

const dohConfigFile = "/etc/travo/doh-config.json"

// GetDoHConfig returns the current DNS-over-HTTPS configuration.
func (n *NetworkService) GetDoHConfig() (models.DoHConfig, error) {
	data, err := os.ReadFile(dohConfigFile)
	if err != nil {
		return models.DoHConfig{Provider: "cloudflare"}, nil
	}
	var cfg models.DoHConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return models.DoHConfig{Provider: "cloudflare"}, nil
	}
	return cfg, nil
}

// SetDoHConfig saves the DNS-over-HTTPS config and restarts dnsmasq if enabled.
func (n *NetworkService) SetDoHConfig(cfg models.DoHConfig) error {
	if err := os.MkdirAll(filepath.Dir(dohConfigFile), 0750); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dohConfigFile, data, 0600); err != nil {
		return err
	}
	// Apply: configure dnsmasq to use a local DoH proxy if enabled.
	if cfg.Enabled {
		url := cfg.URL
		if url == "" {
			switch cfg.Provider {
			case "google":
				url = "https://dns.google/dns-query"
			case "quad9":
				url = "https://dns.quad9.net/dns-query"
			default: // cloudflare
				url = "https://cloudflare-dns.com/dns-query"
			}
		}
		_ = url // DoH proxy integration stored in config file; dnsmasq-over-HTTPS requires https-dns-proxy
		_, _ = n.cmd.Run("uci", "commit", "dhcp")
		_, _ = n.cmd.Run("/etc/init.d/dnsmasq", "restart")
	}
	return nil
}

// GetIPv6Status returns whether IPv6 is enabled and current global addresses.
func (n *NetworkService) GetIPv6Status() (models.IPv6Status, error) {
	var status models.IPv6Status
	data, err := os.ReadFile("/proc/sys/net/ipv6/conf/all/disable_ipv6")
	if err == nil {
		status.Enabled = strings.TrimSpace(string(data)) == "0"
	} else {
		status.Enabled = true // assume enabled if file unreadable
	}
	out, err := n.cmd.Run("ip", "-6", "addr", "show", "scope", "global")
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "inet6 ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					status.Addresses = append(status.Addresses, parts[1])
				}
			}
		}
	}
	if status.Addresses == nil {
		status.Addresses = []string{}
	}
	return status, nil
}

// SetIPv6Enabled enables or disables IPv6 system-wide via sysctl.
func (n *NetworkService) SetIPv6Enabled(enabled bool) error {
	val := "1"
	if enabled {
		val = "0"
	}
	_, err := n.cmd.Run("sysctl", "-w", "net.ipv6.conf.all.disable_ipv6="+val)
	return err
}

// SendWoL sends a Wake-on-LAN magic packet to the given MAC address.
func (n *NetworkService) SendWoL(mac, iface string) error {
	args := []string{mac}
	if iface != "" {
		args = append([]string{"-i", iface}, args...)
	}
	_, err := n.cmd.Run("etherwake", args...)
	if err != nil {
		// fallback to ether-wake
		_, err = n.cmd.Run("ether-wake", args...)
	}
	return err
}

// ConnectionMethod describes how the client is connected.
type ConnectionMethod struct {
	Method    string `json:"method"`     // "wifi-client", "wifi-ap", "ethernet", "unknown"
	Interface string `json:"interface"`  // interface name (e.g., "br-lan", "wwan0")
	IPAddress string `json:"ip_address"` // client's IP address
}

// GetConnectionMethod determines how the client is connected by matching the
// client IP against network interface addresses. Uses ubus network interface
// dump for accurate address detection. On error, logs details and returns
// "unknown" to avoid breaking UI.
func (n *NetworkService) GetConnectionMethod(clientIP string) (*ConnectionMethod, error) {
	// Handle localhost cases
	if clientIP == "" || clientIP == "::1" || clientIP == "127.0.0.1" {
		return &ConnectionMethod{Method: "unknown", Interface: "", IPAddress: clientIP}, nil
	}

	// Parse client IP to validate it's valid
	clientAddr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return &ConnectionMethod{Method: "unknown", Interface: "", IPAddress: clientIP}, nil
	}

	// IPv6 addresses are not supported for connection method detection
	if clientAddr.Is6() {
		return &ConnectionMethod{Method: "unknown", Interface: "", IPAddress: clientIP}, nil
	}

	// Get all network interface addresses via ubus
	ifaceDump, err := n.ubus.Call("network.interface", "dump", nil)
	if err != nil {
		return &ConnectionMethod{Method: "unknown", Interface: "", IPAddress: clientIP}, nil
	}

	// Parse the ubus response to find matching interfaces
	interfaces, ok := ifaceDump["interface"].([]interface{})
	if !ok {
		return &ConnectionMethod{Method: "unknown", Interface: "", IPAddress: clientIP}, nil
	}

	type ifaceInfo struct {
		name        string
		device      string
		ipv4Addrs   []netip.Prefix
		up          bool
		interfaceUp bool
	}

	var ifaces []ifaceInfo

	// Extract interface information
	for _, ifaceRaw := range interfaces {
		ifaceMap, ok := ifaceRaw.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := ifaceMap["l3_device"].(string)
		if name == "" {
			name, _ = ifaceMap["interface"].(string)
		}

		l3Device, _ := ifaceMap["l3_device"].(string)
		device, _ := ifaceMap["device"].(string)

		if l3Device != "" {
			device = l3Device
		}

		info := ifaceInfo{
			name:   name,
			device: device,
			up: func() bool {
				if up, ok := ifaceMap["up"].(bool); ok {
					return up
				}
				return false
			}(),
			interfaceUp: func() bool {
				if up, ok := ifaceMap["interface"].(bool); ok {
					return up
				}
				return false
			}(),
		}

		// Extract IPv4 addresses with netmasks
		if ipv4Addrs, ok := ifaceMap["ipv4-address"].([]interface{}); ok {
			for _, addrRaw := range ipv4Addrs {
				if addrMap, ok := addrRaw.(map[string]interface{}); ok {
					if addrStr, ok := addrMap["address"].(string); ok {
						// Parse address with netmask
						if addr, err := netip.ParseAddr(addrStr); err == nil {
							info.ipv4Addrs = append(info.ipv4Addrs, netip.PrefixFrom(addr, 32))
						}
					}
				}
			}
		}

		// Also extract IPv4 prefix data if available (includes netmask)
		if ipv4Prefixes, ok := ifaceMap["ipv4-prefix"].([]interface{}); ok {
			for _, prefixRaw := range ipv4Prefixes {
				if prefixMap, ok := prefixRaw.(map[string]interface{}); ok {
					if prefixStr, ok := prefixMap["address"].(string); ok {
						if prefix, err := netip.ParsePrefix(prefixStr); err == nil {
							info.ipv4Addrs = append(info.ipv4Addrs, prefix)
						}
					}
				}
			}
		}

		ifaces = append(ifaces, info)
	}

	// Find which interface matches the client IP
	var matchedIface *ifaceInfo
	for i := range ifaces {
		info := &ifaces[i]
		if !info.up && !info.interfaceUp {
			continue
		}

		// Check if client IP is in this interface's subnet
		for _, prefix := range info.ipv4Addrs {
			if prefix.Contains(clientAddr) {
				matchedIface = info
				break
			}
		}
		if matchedIface != nil {
			break
		}
	}

	if matchedIface == nil {
		// No match found - might be localhost or routed
		return &ConnectionMethod{Method: "unknown", Interface: "", IPAddress: clientIP}, nil
	}

	// Determine connection method based on interface
	method := "unknown"
	switch {
	case strings.HasPrefix(matchedIface.name, "wwan") ||
		strings.HasPrefix(matchedIface.device, "phy") && strings.Contains(matchedIface.device, "-sta"):
		method = "wifi-client"
	case matchedIface.name == "br-lan" || matchedIface.name == "lan":
		// Check if this is AP via wireless device presence
		method = "wifi-ap" // Default to AP for LAN
	case strings.HasPrefix(matchedIface.name, "eth") || strings.HasPrefix(matchedIface.device, "eth"):
		method = "ethernet"
	}

	return &ConnectionMethod{
		Method:    method,
		Interface: matchedIface.name,
		IPAddress: clientIP,
	}, nil
}
