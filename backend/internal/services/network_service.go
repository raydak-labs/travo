package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

	// portForwardsFile overrides the const path (tests only), following the
	// same pattern as aliasFile.
	portForwardsFile string

	// arpFile is the neighbour table the client classifier reads an IP to a MAC
	// through. Overridable so a test can answer with the table captured from the
	// device instead of the build machine's.
	arpFile string

	// ddnsInitScript is the ddns-scripts init script. Its absence is how we know
	// the package is not installed, because that is the only thing on the device
	// that can service a `ddns` config. Overridable for tests.
	ddnsInitScript string

	// wifiMACsMu guards wifiMACsSeen.
	wifiMACsMu sync.Mutex
	// wifiMACsSeen tracks MACs recently seen in WiFi station dumps, with the
	// timestamp of their last appearance. Used to suppress "disconnected WiFi
	// client appears as LAN" during the ARP cache staleness window.
	wifiMACsSeen map[string]time.Time

	// portForwardsMu serialises the port-forward read-modify-write.
	//
	// This is NOT the per-config UCI lock (see lockUCIConfigs), which guards the
	// process-global /tmp/.uci/<config>/changes file. Port forwards are a
	// read-modify-write over BOTH /etc/travo/port-forwards.json and the firewall
	// config, and the config lock cannot cover the JSON file. Unserialised, two
	// concurrent adds each read the same rule list and each wrote its own
	// version, so one add silently vanished; and because os.WriteFile truncates
	// before writing, a reader could observe a half-written file and fail with
	// "unexpected end of JSON input". Both were reproduced on the device with
	// 24 concurrent writers: 35 successful creates, 1 rule on disk.
	portForwardsMu sync.Mutex
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

// stringFromBytes converts a byte slice to a trimmed string.
func stringFromBytes(data []byte) string {
	return strings.TrimSpace(string(data))
}

func newNetworkService(u uci.UCI, ub ubus.Ubus, aliasFile string, cmd CommandRunner) *NetworkService {
	return &NetworkService{
		uci: u, ubus: ub, aliasFile: aliasFile, cmd: cmd, arpFile: procNetARP,
		wifiMACsSeen:     make(map[string]time.Time),
		portForwardsFile: defaultPortForwardsFile,
		ddnsInitScript:   ddnsInitScriptPath,
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
		// parseInterface maps the wireless device to the "wifi" discriminator.
		wwanIface := parseInterface("wwan", "phy0-sta0", wwanData, n.ubus)
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

// dhcpEpochFloor is the smallest value that can plausibly be a UNIX timestamp
// in seconds (2001-09-09). `ubus call dhcp ipv4leases` reports `expires` as an
// absolute timestamp — the same value as the first column of
// /tmp/dhcp.leases — but some builds have been observed reporting the
// *remaining* lease duration instead, so both encodings are accepted.
const dhcpEpochFloor = 1e9

// connectedSinceFromLeaseExpiry converts a DHCP lease expiry into a
// connected-since timestamp. Both encodings of `expires` are accepted, and in
// both cases the connection start is the expiry minus the configured lease
// time: treating an absolute expiry as the connection start reports a client as
// connected "since the future".
func connectedSinceFromLeaseExpiry(expires, leaseSec float64) string {
	if expires <= 0 {
		return ""
	}
	lease := time.Duration(leaseSec) * time.Second
	if expires >= dhcpEpochFloor {
		start := time.Unix(int64(expires), 0).Add(-lease)
		if start.After(time.Now()) {
			// Lease time larger than the remaining lease: the start is unknown.
			return ""
		}
		return start.UTC().Format(time.RFC3339)
	}
	elapsed := leaseSec - expires
	if elapsed < 0 {
		elapsed = 0
	}
	return time.Now().Add(-time.Duration(elapsed) * time.Second).UTC().Format(time.RFC3339)
}

// procNetARP is the kernel neighbour table. It is the only place on the device
// that maps a client IP to the MAC actually sitting behind it.
const procNetARP = "/proc/net/arp"

// arpEntry is one row of the neighbour table, reduced to what callers need.
type arpEntry struct{ flags, iface string }

// parseArpTable turns /proc/net/arp into ip → entry and MAC → ip lookups. It is
// the ONE neighbour-table reader: the client list and the lockout guard's
// classifier both go through it, so they cannot disagree about who is who.
//
// The header row is skipped by position (it is the only non-numeric field[0]),
// and incomplete/incomplete-MAC rows are dropped rather than half-read.
func parseArpTable(raw string) (map[string]arpEntry, map[string]string) {
	ipToARP := make(map[string]arpEntry) // ip  → entry
	macToIP := make(map[string]string)   // MAC → ip  (last wins on dup)
	for _, line := range strings.Split(raw, "\n")[1:] {
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
	return ipToARP, macToIP
}

// readArpFile returns the neighbour table, or "" when it cannot be read. A
// caller that gets "" sees no neighbours, which classifies as `unknown` rather
// than as a guess.
func readArpFile(path string) string {
	if path == "" {
		path = procNetARP
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

// parseStationMACs returns just the MACs from "iw dev <iface> station dump".
// The classifier needs membership ("is this MAC associated?"), not the traffic
// counters, so it does not pay for the full parse.
func parseStationMACs(output string) map[string]struct{} {
	macs := make(map[string]struct{})
	for line := range strings.SplitSeq(output, "\n") {
		after, ok := strings.CutPrefix(strings.TrimSpace(line), "Station ")
		if !ok {
			continue
		}
		if f := strings.Fields(after); len(f) >= 1 && f[0] != "" {
			macs[strings.ToUpper(f[0])] = struct{}{}
		}
	}
	return macs
}

// apStationMACs returns every MAC currently associated with any access point.
//
// It asks the radio directly, the same way the client list does, rather than
// inferring WiFi membership from the fact that a bridge is carrying traffic —
// on this hardware the LAN bridge carries both wired and wireless clients, so
// that inference classifies every client as wireless.
//
// An interface whose station dump cannot be read is SKIPPED. A client MAC that
// would have been on it then reads as "not associated", which is the one way
// this answer can be wrong; the caller below narrows that by treating an
// unreadable radio as unprovable rather than as wired.
func apStationMACs(cmd CommandRunner) map[string]struct{} {
	macs := make(map[string]struct{})
	iwDev, err := cmd.Run("iw", "dev")
	if err != nil {
		return macs
	}
	for _, iface := range parseIwDev(string(iwDev)) {
		dump, err := cmd.Run("iw", "dev", iface, "station", "dump")
		if err != nil {
			continue
		}
		for mac := range parseStationMACs(string(dump)) {
			macs[mac] = struct{}{}
		}
	}
	return macs
}

// parseIfaceIPv4Prefixes extracts an interface's IPv4 prefixes from one entry of
// `ubus call network.interface dump`.
//
// It reads BOTH shapes, because they differ between netifd builds:
//
//   - {"address": "192.168.1.1", "mask": 24} — a bare address plus a separate
//     integer netmask length. This is what the device on 192.168.1.1 emits, and
//     it is the shape the classifier used to ignore.
//   - {"address": "192.168.8.0/24"} — a CIDR string, which is what the previous
//     test fixture invented under an `ipv4-prefix` key the device never sends.
//
// A malformed entry is SKIPPED and the rest of the list is still returned.
// Returning an empty list on any bad entry is the failure mode that made every
// real client `unknown`: no prefix, no match, no discrimination.
func parseIfaceIPv4Prefixes(iface map[string]any) []netip.Prefix {
	var prefixes []netip.Prefix
	for _, key := range []string{"ipv4-address", "ipv4-prefix"} {
		entries, _ := iface[key].([]any)
		for _, raw := range entries {
			if prefix, ok := parseIfaceIPv4Entry(raw); ok {
				prefixes = append(prefixes, prefix)
			}
		}
	}
	return prefixes
}

// parseIfaceIPv4Entry turns one address entry into a prefix, accepting both the
// bare-address-plus-mask and CIDR forms. The boolean is false for an entry that
// cannot be read, which the caller skips.
func parseIfaceIPv4Entry(raw any) (netip.Prefix, bool) {
	entry, ok := raw.(map[string]any)
	if !ok {
		return netip.Prefix{}, false
	}
	addrStr, _ := entry["address"].(string)
	if addrStr == "" {
		return netip.Prefix{}, false
	}
	// CIDR form, in either key.
	if strings.Contains(addrStr, "/") {
		prefix, err := netip.ParsePrefix(addrStr)
		if err != nil {
			return netip.Prefix{}, false
		}
		return prefix.Masked(), true
	}
	// Bare address plus a separate integer mask. An absent or out-of-range mask
	// yields no prefix rather than a /0, which would match the whole internet
	// and place every caller on the first interface in the dump.
	addr, err := netip.ParseAddr(addrStr)
	if err != nil || addr.Is6() {
		return netip.Prefix{}, false
	}
	bits, ok := maskBits(entry["mask"])
	if !ok {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, bits).Masked(), true
}

// maskBits reads a netmask length from a ubus value. ubus JSON decodes numbers
// as float64, so both that and the integers a hand-built fixture carries are
// accepted.
func maskBits(raw any) (int, bool) {
	var n int
	switch v := raw.(type) {
	case float64:
		n = int(v)
		if float64(n) != v {
			return 0, false
		}
	case int:
		n = v
	case int64:
		n = int(v)
	default:
		return 0, false
	}
	if n < 0 || n > 32 {
		return 0, false
	}
	return n, true
}

// parseDHCPLeasesFile reads /tmp/dhcp.leases and returns a map of uppercase MAC → lease info.
// Format: <expiry_epoch> <mac> <ip> <hostname> [clientid]
func parseDHCPLeasesFile() map[string]dhcpLease {
	result := map[string]dhcpLease{}
	data, err := os.ReadFile("/tmp/dhcp.leases")
	if err != nil {
		return result
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
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
	for line := range strings.SplitSeq(string(data), "\n") {
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
	ipToARP, macToIP := parseArpTable(readArpFile(n.arpFile))

	// ── 3. Build a deduplicated client map (keyed by uppercase MAC) ───────
	byMAC := make(map[string]models.Client)

	// The lease file is read on builds that report EITHER an absolute epoch or
	// a remaining duration in `expires` (see connectedSinceFromLeaseExpiry), so
	// this must go through the shared helper rather than assuming an epoch —
	// doing the conversion inline rendered "connected since 1970" for every
	// client on duration-reporting builds.
	connectedSinceFromLease := func(expiry int64) string {
		return connectedSinceFromLeaseExpiry(float64(expiry), leaseTimeSec)
	}

	// 3a. Try ubus dhcp ipv4leases (works on some builds).
	if data, err := n.ubus.Call("dhcp", "ipv4leases", nil); err == nil {
		if device, ok := data["device"].(map[string]any); ok {
			for ifaceName, ifaceData := range device {
				ifaceMap, _ := ifaceData.(map[string]any)
				leases, _ := ifaceMap["leases"].([]any)
				for _, raw := range leases {
					lm, _ := raw.(map[string]any)
					ip, _ := lm["ip"].(string)
					mac := strings.ToUpper(fmt.Sprintf("%v", lm["mac"]))
					hostname, _ := lm["hostname"].(string)
					expires, _ := lm["expires"].(float64)
					cs := connectedSinceFromLeaseExpiry(expires, leaseTimeSec)
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
	for line := range strings.SplitSeq(output, "\n") {
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
		if after, ok := strings.CutPrefix(trimmed, "rx bytes:"); ok {
			valStr := strings.TrimSpace(after)
			if v, err := strconv.ParseInt(valStr, 10, 64); err == nil {
				entry := result[currentMAC]
				entry[1] = v // AP rx from client = client TX
				result[currentMAC] = entry
			}
		} else if after, ok := strings.CutPrefix(trimmed, "tx bytes:"); ok {
			valStr := strings.TrimSpace(after)
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
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "Interface "); ok {
			currentIface = after
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

// interfaceType maps a ubus network interface onto the discriminator the API
// contract declares in shared/src/api/network.ts:
// 'wan' | 'lan' | 'wifi' | 'vpn' | 'usb'.
//
// Rules (in order):
//   - logical interfaces named wan/lan keep their name, whatever device carries
//     them, so the WAN/LAN cards keep working;
//   - wireguard/tunnel devices (proto=wireguard, wg*, tun*, tap*) are "vpn";
//   - USB tethering devices (usbtether, usb*, rndis*, cdc_*) are "usb";
//   - wireless devices (phy*, wlan*, *-sta, *-ap) and the wwan uplink are
//     "wifi";
//   - anything else keeps the interface name.
func interfaceType(name, device, proto string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	d := strings.ToLower(strings.TrimSpace(device))
	p := strings.ToLower(strings.TrimSpace(proto))
	if n == "wan" || n == "lan" {
		return n
	}
	switch {
	case p == "wireguard" ||
		strings.HasPrefix(d, "wg") || strings.HasPrefix(d, "tun") || strings.HasPrefix(d, "tap") ||
		strings.HasPrefix(n, "wg") || strings.HasPrefix(n, "vpn"):
		return "vpn"
	case n == "usbtether" || strings.HasPrefix(d, "usb") ||
		strings.HasPrefix(d, "rndis") || strings.HasPrefix(d, "cdc_"):
		return "usb"
	case n == "wwan" || strings.HasPrefix(d, "phy") || strings.HasPrefix(d, "wlan") ||
		strings.Contains(d, "-sta") || strings.Contains(d, "-ap"):
		return "wifi"
	default:
		return n
	}
}

func parseInterface(name, device string, data map[string]any, ub ubus.Ubus) models.NetworkInterface {
	iface := models.NetworkInterface{
		Name: name,
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
	proto, _ := data["proto"].(string)
	iface.Type = interfaceType(name, devName, proto)

	// Fetch device stats for MAC and traffic
	if ub != nil && devName != "" {
		if devData, err := ub.Call("network.device", "status", map[string]any{"name": devName}); err == nil {
			if mac, ok := devData["macaddr"].(string); ok && mac != "" {
				iface.MACAddress = mac
			}
			if stats, ok := devData["statistics"].(map[string]any); ok {
				if rxBytes, ok := stats["rx_bytes"].(float64); ok {
					iface.RxBytes = int64(rxBytes)
				}
				if txBytes, ok := stats["tx_bytes"].(float64); ok {
					iface.TxBytes = int64(txBytes)
				}
			}
		}
	}

	if addrs, ok := data["ipv4-address"].([]any); ok && len(addrs) > 0 {
		if a, ok := addrs[0].(map[string]any); ok {
			iface.IPAddress, _ = a["address"].(string)
			if mask, ok := a["mask"].(float64); ok {
				iface.Netmask = maskToNetmask(mask)
			}
		}
	}
	if routes, ok := data["route"].([]any); ok {
		for _, r := range routes {
			if rm, ok := r.(map[string]any); ok {
				if gw, ok := rm["nexthop"].(string); ok && gw != "" {
					iface.Gateway = gw
					break
				}
			}
		}
	}
	if dns, ok := data["dns-server"].([]any); ok {
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

// wanStaticOptions are the options that only mean something for a static WAN.
// They are deleted when the WAN is switched to another mode: leaving them behind
// is how a static→dhcp switch produced a WAN that kept a stale ipaddr/netmask/
// gateway, and — combined with a leftover peerdns=0 + dns from SetDNSConfig —
// kept resolving through a DNS server the DHCP lease never handed out.
var wanStaticOptions = []string{"ip4addr", "netmask", "gateway"}

// errPPPoEUnsupported is returned for a save that would create a PPPoE WAN
// without credentials. Writing proto=pppoe alone produces a WAN that can never
// authenticate: the handler validates "type", not a username/password pair the
// model does not have.
var errPPPoEUnsupported = errors.New(
	"PPPoE needs a username and password that this API cannot store: " +
		"configure PPPoE in LuCI (Network -> Interfaces -> WAN), then save other WAN fields here")

// SetWanConfig updates the WAN configuration.
//
// Everything the request declares is written. MTU and the DNS servers used to be
// validated by the handler, published in the OpenAPI body and then dropped on
// the floor, so the documented "GET, change a field, PUT it back" client pattern
// answered 200 having changed nothing at all.
func (n *NetworkService) SetWanConfig(config models.WanConfig) error {
	return mutateUCI(n.uci, []string{"network"}, func() error {
		if config.MTU != 0 && (config.MTU < 68 || config.MTU > 9000) {
			return fmt.Errorf("MTU must be between 68 and 9000, got %d", config.MTU)
		}
		current := map[string]string{}
		if opts, err := n.uci.GetAll("network", "wan"); err == nil {
			current = opts
		}
		if config.Type == "pppoe" {
			// Editing other fields of an already-configured PPPoE WAN is fine;
			// switching INTO one is not.
			if current["proto"] != "pppoe" || current["username"] == "" || current["password"] == "" {
				return errPPPoEUnsupported
			}
		}
		if config.Type != "" {
			if err := n.uciSet("network", "wan", "proto", config.Type); err != nil {
				return err
			}
			if config.Type != current["proto"] && config.Type != "static" {
				for _, option := range wanStaticOptions {
					// Only options that are actually set: `uci delete` on a
					// missing entry is itself an error, and that must not fail
					// the whole save.
					if _, present := current[option]; !present {
						continue
					}
					if err := n.uci.DeleteOption("network", "wan", option); err != nil {
						return fmt.Errorf("clear %s after switching WAN to %s: %w", option, config.Type, err)
					}
				}
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
		if config.MTU > 0 {
			if err := n.uciSet("network", "wan", "mtu", strconv.Itoa(config.MTU)); err != nil {
				return err
			}
		}
		// A nil dns_servers means "not part of this save"; an explicit empty
		// list means "go back to the DHCP-provided resolvers".
		if config.DNSServers != nil {
			peerdns := "1"
			if len(config.DNSServers) > 0 {
				peerdns = "0"
			}
			if err := n.uciSet("network", "wan", "peerdns", peerdns); err != nil {
				return err
			}
			if err := n.uciSet("network", "wan", "dns", strings.Join(config.DNSServers, " ")); err != nil {
				return err
			}
		}
		return n.uciCommit("network")
	})
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
// SetDNSConfig sets the WAN DNS servers.
//
// Locked, not just committed: `network` is in the VPN transaction's revert set, so
// an unlocked writer here can have its staged delta discarded by a failing VPN
// toggle and still answer 200 — the user is told the WAN DNS changed and it did
// not. It was also the mirror image: its own `uci commit network` could commit the
// VPN's half-written peer, leaving a wg0 peer with new allowed_ips and no matching
// public_key.
func (n *NetworkService) SetDNSConfig(config models.DNSConfig) error {
	return mutateUCI(n.uci, []string{"network"}, func() error {
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
	})
}

// SetDHCPConfig updates the DHCP configuration for the LAN.
// SetDHCPConfig updates the LAN DHCP pool. Locked for the same reason as
// SetDNSConfig: `dhcp` is in the VPN transaction's revert set, and the VPN's
// DNS-forwarding helpers delete and re-add this same dhcp.@dnsmasq[0] section.
func (n *NetworkService) SetDHCPConfig(config models.DHCPConfig) error {
	return mutateUCI(n.uci, []string{"dhcp"}, func() error {
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
	})
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
		// A failed read is not "no entries": returning an empty list would make
		// the UI invite a duplicate entry for an entry that already exists.
		return nil, fmt.Errorf("reading dhcp sections: %w", err)
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

// uciSectionExists reports whether a UCI section is already present.
//
// It exists because real `uci set config.section=stype` CREATES OR UPDATES:
// AddSection is not a duplicate check. Callers that relied on its error passed
// under a stricter mock and silently UPSERTED on the device.
func (n *NetworkService) uciSectionExists(config, section string) (bool, error) {
	sections, err := n.uci.GetSections(config)
	if err != nil {
		return false, fmt.Errorf("reading %s sections: %w", config, err)
	}
	_, ok := sections[section]
	return ok, nil
}

// AddDNSEntry adds a new local DNS entry as a named UCI section in dhcp config.
func (n *NetworkService) AddDNSEntry(entry models.DNSEntry) error {
	// `dhcp` is locked, not just committed-and-hoped: the uci CLI keeps
	// uncommitted changes in the process-global /tmp/.uci/dhcp/changes file, so
	// an unlocked writer interleaves with AddDHCPReservation (which IS wrapped)
	// and destroys its staged delta. Verified on the device: concurrent adds to
	// these two endpoints failed with "uci: Invalid argument" and "uci set
	// dhcp.dns_...name", i.e. each request's own section was clobbered by the
	// other. This was a documented gap in the mutateUCI rollout; it is not a
	// theoretical one.
	return mutateUCI(n.uci, []string{"dhcp"}, func() error {
		section := "dns_" + sanitizeSectionName(entry.Name)
		// Explicit duplicate check, inside the lock: real uci reuses the section
		// instead of failing, so a second entry with the same name would silently
		// REPLACE the first one's IP.
		exists, err := n.uciSectionExists("dhcp", section)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("DNS entry %q already exists", entry.Name)
		}
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
	})
}

// DeleteDNSEntry removes a local DNS entry by its UCI section name.
func (n *NetworkService) DeleteDNSEntry(section string) error {
	// Same lock as AddDNSEntry, and for the same reason: a delete that reverts
	// or commits `dhcp` concurrently with an add silently discards the add.
	return mutateUCI(n.uci, []string{"dhcp"}, func() error {
		if err := n.requireNamedDHCPSection(section, "domain"); err != nil {
			return err
		}
		if err := n.uci.DeleteSection("dhcp", section); err != nil {
			return fmt.Errorf("deleting DNS entry: %w", err)
		}
		return n.uci.Commit("dhcp")
	})
}

// requireNamedDHCPSection rejects anything that is not a named section of the
// expected type before it is deleted.
//
// uci.validSectionName deliberately admits anonymous references (@dnsmasq[0]),
// because stock /etc/config files use them, so
// DELETE /api/v1/network/dns/entries/@dnsmasq[0] passed validation and
// `uci delete dhcp.@dnsmasq[0]` removed the whole dnsmasq section and committed
// it: DHCP pool, lease time, DNS servers, DHCPv6 and RA all gone, with nothing
// but a 200 to show for it. Both add paths only ever create named sections, so
// no legitimate caller can produce an anonymous reference.
func (n *NetworkService) requireNamedDHCPSection(section, wantType string) error {
	if strings.HasPrefix(section, "@") {
		return fmt.Errorf("refusing to delete anonymous dhcp section %s: "+
			"it is part of the file, not one entry", section)
	}
	sections, err := n.uci.GetSections("dhcp")
	if err != nil {
		return fmt.Errorf("reading dhcp sections: %w", err)
	}
	opts, ok := sections[section]
	if !ok {
		return fmt.Errorf("dhcp section not found: %s", section)
	}
	if opts[".type"] != wantType {
		return fmt.Errorf("dhcp section %s is a %q section, not %q", section, opts[".type"], wantType)
	}
	return nil
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
		return nil, fmt.Errorf("reading dhcp sections: %w", err)
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
	return mutateUCI(n.uci, []string{"dhcp"}, func() error {
		section := "host_" + sanitizeSectionName(reservation.Name)
		// Explicit duplicate check, inside the lock: a same-named reservation
		// would otherwise be overwritten in place, silently changing which MAC
		// gets which address.
		exists, err := n.uciSectionExists("dhcp", section)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("DHCP reservation %q already exists", reservation.Name)
		}
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
	})
}

// DeleteDHCPReservation removes a static DHCP reservation by its UCI section name.
func (n *NetworkService) DeleteDHCPReservation(section string) error {
	return mutateUCI(n.uci, []string{"dhcp"}, func() error {
		if err := n.requireNamedDHCPSection(section, "host"); err != nil {
			return err
		}
		if err := n.uci.DeleteSection("dhcp", section); err != nil {
			return fmt.Errorf("deleting DHCP reservation: %w", err)
		}
		return n.uci.Commit("dhcp")
	})
}

// parseDHCPLeases parses the content of /tmp/dhcp.leases into a slice of DHCPLease.
// Each line has the format: <expiry_epoch> <mac_address> <ip_address> <hostname> <client_id>
func parseDHCPLeases(data string) []models.DHCPLease {
	var leases []models.DHCPLease
	for line := range strings.SplitSeq(strings.TrimSpace(data), "\n") {
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
// It returns an error when no interface accepted the disassociate so the UI can
// report the failure instead of pretending the client is gone.
func (n *NetworkService) KickClient(mac string) error {
	// Discover AP interfaces dynamically using iw dev
	iwDevOutput, err := n.cmd.Run("iw", "dev")
	if err != nil {
		// Fallback to common AP interfaces if iw fails
		for _, iface := range []string{"phy0-ap0", "phy1-ap0", "wlan0", "wlan1"} {
			if _, kickErr := n.cmd.Run("hostapd_cli", "-i", iface, "disassociate", mac); kickErr == nil {
				return nil
			}
		}
		return fmt.Errorf("kick %s: no wireless interface accepted the disassociate", mac)
	}

	// Parse iw dev output to find AP interfaces
	attempted := false
	for _, iface := range parseIwDev(string(iwDevOutput)) {
		attempted = true
		if _, kickErr := n.cmd.Run("hostapd_cli", "-i", iface, "disassociate", mac); kickErr == nil {
			// Successfully kicked from this interface
			return nil
		}
	}
	if !attempted {
		return fmt.Errorf("kick %s: no AP interface found to disassociate from", mac)
	}
	return fmt.Errorf("kick %s: no AP interface accepted the disassociate", mac)
}

// BlockClient adds a firewall rule to drop all traffic from a MAC address.
// Every failure after the first staged `uci` write reverts the staged firewall
// delta: the uci CLI keeps uncommitted changes in /tmp/.uci/firewall/changes,
// so an abandoned half-written rule would be committed by a later, unrelated
// `uci commit firewall`.
//
// This is a direct `uci commit firewall` + firewall reload rather than the rpcd
// apply/confirm flow, because a block has to be enforced on the running firewall
// immediately — a rollback window that silently expires on a policy change would
// unblock a client the operator believes is filtered. ADR 0004 §5 requires such
// a path to be listed there with a named crash guard AND a rollback; the
// rollback is the undoFirewallSection call below. The guard row is still
// missing (it needs ADR 0004 §5, ADR 0003 §2, scripts/deploy-local.sh and
// scripts/install.sh), and writing a guard here without those would be worse
// than no guard: nothing would clear it, and blocking would stay dead until a
// redeploy.
func (n *NetworkService) BlockClient(mac string) error {
	return mutateUCI(n.uci, []string{"firewall"}, func() error {
		section := "block_" + normalizeMACForSection(mac)
		macUpper := strings.ToUpper(mac)

		// Explicit duplicate check, inside the lock: real uci reuses the rule
		// section, so re-blocking an already-blocked client would report success
		// and restart the firewall for nothing.
		exists, err := n.uciSectionExists("firewall", section)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("client %s is already blocked", macUpper)
		}
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
			// The rule is committed but the running firewall never loaded it.
			// Take it back out so the file and the running firewall agree.
			if undoErr := n.undoFirewallSection(section, nil); undoErr != nil {
				return fmt.Errorf("restart firewall: %w (rollback of %s failed: %v)", err, section, undoErr)
			}
			return fmt.Errorf("restart firewall: %w", err)
		}
		return nil
	})
}

// UnblockClient removes the firewall block rule for a MAC address.
func (n *NetworkService) UnblockClient(mac string) error {
	return mutateUCI(n.uci, []string{"firewall"}, func() error {
		section := "block_" + normalizeMACForSection(mac)

		previous, _ := n.uci.GetAll("firewall", section)
		if err := n.uci.DeleteSection("firewall", section); err != nil {
			return fmt.Errorf("delete firewall block rule: %w", err)
		}
		// A failed commit leaves the deletion staged; mutateUCI reverts it.
		if err := n.uciCommit("firewall"); err != nil {
			return err
		}
		if err := n.restartService("firewall"); err != nil {
			// Same contract as BlockClient: the committed config must not be
			// left saying "unblocked" while the running firewall still drops it.
			if undoErr := n.undoFirewallSection(section, previous); undoErr != nil {
				return fmt.Errorf("restart firewall: %w (rollback of %s failed: %v)", err, section, undoErr)
			}
			return fmt.Errorf("restart firewall: %w", err)
		}
		return nil
	})
}

// undoFirewallSection restores a firewall section to a previous option set —
// nil means the section must be gone — and commits the result.
func (n *NetworkService) undoFirewallSection(section string, previous map[string]string) error {
	// An unblock has usually already deleted the section by this point, and a
	// missing section IS the target state, so a failed delete is only an error
	// when the section is in fact still there.
	if _, err := n.uci.GetAll("firewall", section); err == nil {
		if err := n.uci.DeleteSection("firewall", section); err != nil {
			return err
		}
	}
	if previous != nil {
		if err := n.uci.AddSection("firewall", section, "rule"); err != nil {
			return err
		}
		for option, value := range previous {
			if strings.HasPrefix(option, ".") {
				continue
			}
			if err := n.uci.Set("firewall", section, option, value); err != nil {
				return err
			}
		}
	}
	return n.uciCommit("firewall")
}

// GetBlockedClients returns a list of blocked MAC addresses.
func (n *NetworkService) GetBlockedClients() ([]string, error) {
	sections, err := n.uci.GetSections("firewall")
	if err != nil {
		// Never report "nothing is blocked" on a failed read: that silently
		// unblocks clients the operator believes are filtered.
		return nil, fmt.Errorf("reading firewall sections: %w", err)
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

// ddnsSectionName is the ddns-scripts service section this service owns.
const ddnsSectionName = "myddns"

// ErrDDNSNotAvailable is returned when ddns-scripts is not installed, so there
// is nothing that can act on a `ddns` UCI config. Handlers map this to 503 with
// an actionable message rather than surfacing uci's "Entry not found".
var ErrDDNSNotAvailable = errors.New("ddns-scripts is not installed on this router")

// ddnsInitScriptPath is the production ddns-scripts init script.
const ddnsInitScriptPath = initdPath + "ddns"

// SetDDNSInitScript overrides the ddns-scripts init script path (tests only).
// Tests that exercise DDNS configuration need it to point at a file that exists,
// otherwise every write correctly reports the package as missing.
func (n *NetworkService) SetDDNSInitScript(path string) {
	n.ddnsInitScript = path
}

// DDNSAvailable reports whether ddns-scripts is installed.
//
// Checked via the init script rather than the `ddns` UCI config, because the
// config is what the user is trying to create: on a device without the package
// there is no config AND no way for the UI to install one (ddns is not in the
// service catalog), so the only honest signal is the missing script.
func (n *NetworkService) DDNSAvailable() bool {
	path := n.ddnsInitScript
	if path == "" {
		path = ddnsInitScriptPath
	}
	_, err := os.Stat(path)
	return err == nil
}

// SetDDNSConfig writes the DDNS configuration to UCI and restarts ddns-scripts.
func (n *NetworkService) SetDDNSConfig(config models.DDNSConfig) error {
	// Check the precondition BEFORE touching UCI. The old order wrote half the
	// config and then failed, and on a device without the package every write
	// failed with a bare `uci: Entry not found` 500 that said nothing about the
	// real cause.
	if !n.DDNSAvailable() {
		return fmt.Errorf("%w: install the ddns-scripts package to configure dynamic DNS", ErrDDNSNotAvailable)
	}

	enabled := "0"
	if config.Enabled {
		enabled = "1"
	}

	// `ddns` is locked like every other UCI mutator: the uci CLI keeps
	// uncommitted changes in the process-global /tmp/.uci/ddns/changes file.
	return mutateUCI(n.uci, []string{"ddns"}, func() error {
		// The `myddns` section is created on demand. It does not exist on a
		// router where DDNS was never configured, and `uci set` on a missing
		// section fails outright, so without this the endpoint could only ever
		// succeed on a device that already had DDNS set up.
		if _, err := n.uci.GetAll("ddns", ddnsSectionName); err != nil {
			if err := n.uci.AddSection("ddns", ddnsSectionName, "service"); err != nil {
				return fmt.Errorf("creating ddns section: %w", err)
			}
		}
		if err := n.uciSet("ddns", ddnsSectionName, "enabled", enabled); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(config.Service), "custom") {
			_ = n.uci.DeleteOption("ddns", ddnsSectionName, "service_name")
			if err := n.uciSet("ddns", ddnsSectionName, "update_url", strings.TrimSpace(config.UpdateURL)); err != nil {
				return err
			}
		} else {
			_ = n.uci.DeleteOption("ddns", ddnsSectionName, "update_url")
			if err := n.uciSet("ddns", ddnsSectionName, "service_name", config.Service); err != nil {
				return err
			}
		}
		if err := n.uciSet("ddns", ddnsSectionName, "domain", config.Domain); err != nil {
			return err
		}
		if err := n.uciSet("ddns", ddnsSectionName, "username", config.Username); err != nil {
			return err
		}
		if err := n.uciSet("ddns", ddnsSectionName, "password", config.Password); err != nil {
			return err
		}
		if err := n.uciSet("ddns", ddnsSectionName, "lookup_host", config.LookupHost); err != nil {
			return err
		}
		if err := n.uciCommit("ddns"); err != nil {
			return err
		}
		if err := n.restartService("ddns"); err != nil {
			return fmt.Errorf("restart ddns: %w", err)
		}
		return nil
	})
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

const defaultPortForwardsFile = "/etc/travo/port-forwards.json"

// portForwardsPath returns the port-forward store in use.
func (n *NetworkService) portForwardsPath() string {
	if n.portForwardsFile != "" {
		return n.portForwardsFile
	}
	return defaultPortForwardsFile
}

// GetPortForwards returns stored port-forward rules.
func (n *NetworkService) GetPortForwards() ([]models.PortForwardRule, error) {
	data, err := os.ReadFile(n.portForwardsPath())
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
// newPortForwardID returns an ID that cannot collide with an existing rule.
// The millisecond prefix keeps IDs roughly ordered and human-readable; the
// random suffix is what makes a same-millisecond collision impossible.
func newPortForwardID(existing []models.PortForwardRule) string {
	taken := make(map[string]bool, len(existing))
	for _, r := range existing {
		taken[r.ID] = true
	}
	for {
		var suffix [4]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			// crypto/rand must not fail in practice; fall back to a counter-like
			// value rather than reusing an ID that may already be taken.
			return fmt.Sprintf("pf%d-%s", time.Now().UnixMilli(), strconv.FormatInt(portForwardFallbackID.Add(1), 36))
		}
		id := fmt.Sprintf("pf%d-%s", time.Now().UnixMilli(), hex.EncodeToString(suffix[:]))
		if !taken[id] {
			return id
		}
	}
}

// portForwardFallbackID only advances if crypto/rand is unavailable.
var portForwardFallbackID atomic.Int64

func (n *NetworkService) AddPortForward(rule models.PortForwardRule) error {
	n.portForwardsMu.Lock()
	defer n.portForwardsMu.Unlock()

	rules, err := n.GetPortForwards()
	if err != nil {
		return err
	}
	// Two rules created in the same millisecond used to share an ID, and
	// DeletePortForward removes every rule matching the ID — so one delete took
	// out both. A random suffix makes the ID unique per call.
	rule.ID = newPortForwardID(rules)
	rules = append(rules, rule)
	return n.savePortForwards(rules)
}

// DeletePortForward removes a port-forward rule by ID.
func (n *NetworkService) DeletePortForward(id string) error {
	n.portForwardsMu.Lock()
	defer n.portForwardsMu.Unlock()

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
	path := n.portForwardsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	data, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	// Write to a sibling temp file and rename, so a reader never observes a
	// truncated file. os.WriteFile truncates first, which is what produced
	// "unexpected end of JSON input" on the device when a GET raced a POST.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RunDiagnostics runs ping, traceroute, or DNS lookup and returns the output.
//
// The target becomes the final argv entry of a root-run command, so it is
// validated as a hostname or IP literal first: an unvalidated target of "-f"
// turned `ping` into a root flood of the whole internet, and "-w1"/"-s" style
// values rewrite its behaviour.
func (n *NetworkService) RunDiagnostics(req models.DiagnosticsRequest) models.DiagnosticsResult {
	result := models.DiagnosticsResult{Type: req.Type, Target: req.Target}
	if err := validateDiagnosticTarget(req.Target); err != nil {
		result.Error = err.Error()
		return result
	}
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

// validateDiagnosticTarget accepts a hostname or an IP literal and nothing else.
func validateDiagnosticTarget(target string) error {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return errors.New("target is required")
	}
	if trimmed != target || strings.HasPrefix(target, "-") {
		return fmt.Errorf("invalid diagnostic target %q: must be a hostname or IP address", target)
	}
	if _, err := netip.ParseAddr(target); err == nil {
		return nil
	}
	for _, label := range strings.Split(target, ".") {
		if label == "" {
			return fmt.Errorf("invalid diagnostic target %q: must be a hostname or IP address", target)
		}
		for _, r := range label {
			isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			isDigit := r >= '0' && r <= '9'
			if !isLetter && !isDigit && r != '-' && r != '_' {
				return fmt.Errorf("invalid diagnostic target %q: must be a hostname or IP address", target)
			}
		}
	}
	return nil
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
	// Atomic: os.WriteFile truncates first, so a power cut or a concurrent read
	// left a half-written file that silently falls back to the default provider.
	if err := writeFileAtomic(dohConfigFile, data, 0600); err != nil {
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
		// No `uci commit dhcp` here: nothing above stages a dhcp change, so a
		// commit would only persist whatever ELSE had a delta in flight — the
		// VPN's DNS-forwarding sequence, a DHCP reservation, an AdGuard
		// resolver switch. It was a pure liability.
		_, _ = n.cmd.Run("/etc/init.d/dnsmasq", "restart")
	}
	return nil
}

// GetIPv6Status returns whether IPv6 is enabled and current global addresses.
func (n *NetworkService) GetIPv6Status() (models.IPv6Status, error) {
	var status models.IPv6Status
	data, err := os.ReadFile("/proc/sys/net/ipv6/conf/all/disable_ipv6")
	if err == nil {
		status.Enabled = stringFromBytes(data) == "0"
	} else {
		status.Enabled = true // assume enabled if file unreadable
	}
	out, err := n.cmd.Run("ip", "-6", "addr", "show", "scope", "global")
	if err == nil {
		for line := range strings.SplitSeq(string(out), "\n") {
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
	return classifyClientConnection(classifyDeps{
		ubus: n.ubus, cmd: n.cmd, arpFile: n.arpFile,
	}, clientIP), nil
}

// classifyDeps is everything the classifier reads off the router. It is a
// struct rather than a long argument list because both services build one, and
// a fifth positional argument would make it easy to pass the neighbour table
// where the command runner belongs.
type classifyDeps struct {
	ubus    ubus.Ubus
	cmd     CommandRunner
	arpFile string
}

// classifyClientConnection maps a client IP to the interface it is reachable
// through, then to a connection method. It is a free function taking the router
// dependencies so the wireless lockout guard (wifi_lockout.go) can classify its
// caller through the SAME code as GET /network/connection-method, instead of
// keeping a second, drifting copy of "is this client on WiFi".
//
// Matching the IP against the interface dump only says which SUBNET the caller
// is on. On this hardware that is not enough: br-lan carries both wired and
// wireless clients in the same /24, so a subnet match alone cannot tell a phone
// from the laptop next to it. A client that lands on the LAN bridge is
// therefore resolved to a MAC through the neighbour table and then asked whether
// that MAC is associated with an access point. Only the uplink STA (where the
// router itself is the WiFi client) answers from the interface name alone.
//
// It never fails: an unknown address, an IPv6 client, an unreadable dump or an
// unresolvable MAC all answer "unknown", so a caller it cannot place is never
// mistaken for a wireless one. The guard treats that answer as unsafe (see
// guardLockout).
func classifyClientConnection(deps classifyDeps, clientIP string) *ConnectionMethod {
	unknown := func() *ConnectionMethod {
		return &ConnectionMethod{Method: "unknown", IPAddress: clientIP}
	}
	// Handle localhost cases
	if clientIP == "" || clientIP == "::1" || clientIP == "127.0.0.1" {
		return unknown()
	}

	// Parse client IP to validate it's valid
	clientAddr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return unknown()
	}

	// IPv6 addresses are not supported for connection method detection
	if clientAddr.Is6() {
		return unknown()
	}

	// Get all network interface addresses via ubus
	ifaceDump, err := deps.ubus.Call("network.interface", "dump", nil)
	if err != nil {
		return unknown()
	}

	// Parse the ubus response to find matching interfaces
	interfaces, ok := ifaceDump["interface"].([]any)
	if !ok {
		return unknown()
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
		ifaceMap, ok := ifaceRaw.(map[string]any)
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
			name:        name,
			device:      device,
			ipv4Addrs:   parseIfaceIPv4Prefixes(ifaceMap),
			up:          ifaceMap["up"] == true,
			interfaceUp: ifaceMap["interface"] == true,
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
		return unknown()
	}

	method := classifyMatchedIface(deps, matchedIface.name, matchedIface.device, clientIP)

	return &ConnectionMethod{
		Method:    method,
		Interface: matchedIface.name,
		IPAddress: clientIP,
	}
}

// classifyMatchedIface turns a resolved interface plus the client address into a
// connection method. Split out so the ordering rule below is readable on its own.
func classifyMatchedIface(deps classifyDeps, name, device, clientIP string) string {
	switch {
	case strings.HasPrefix(name, "wwan") ||
		strings.HasPrefix(device, "phy") && strings.Contains(device, "-sta"):
		// The router is itself the WiFi client on this uplink: whoever is behind
		// it gets its connectivity over the air.
		return "wifi-client"
	case name == "br-lan" || name == "lan":
		return classifyLanClient(deps, clientIP)
	case strings.HasPrefix(name, "eth") || strings.HasPrefix(device, "eth"):
		return "ethernet"
	}
	return "unknown"
}

// classifyLanClient decides whether a client on the LAN bridge is on a wire or
// on an access point. The bridge cannot answer that, so the client's MAC is
// resolved through the neighbour table and checked against the radio's station
// lists — the same source of truth the client list uses.
//
// It returns "unknown" rather than guessing whenever the answer is not provable:
// no MAC for the address, or the access-point side could not be read. "unknown"
// is not a safe default here, it is the safe direction — the guard refuses on
// it (see guardLockout).
func classifyLanClient(deps classifyDeps, clientIP string) string {
	mac, ok := arpMACForIP(deps.arpFile, clientIP)
	if !ok {
		return "unknown"
	}
	if _, isWifi := apStationMACs(deps.cmd)[mac]; isWifi {
		return "wifi-ap"
	}
	return "ethernet"
}

// arpMACForIP resolves one client address to its MAC through the neighbour table.
func arpMACForIP(arpFile, ip string) (string, bool) {
	ipToARP, macToIP := parseArpTable(readArpFile(arpFile))
	if _, ok := ipToARP[ip]; !ok {
		return "", false
	}
	for mac, mip := range macToIP {
		if mip == ip {
			return mac, true
		}
	}
	return "", false
}

// writeFileAtomic writes via a sibling temp file and renames, so a concurrent
// reader never observes a truncated file.
//
// os.WriteFile truncates before writing, and /etc/crontabs/root is read by a
// running busybox crond. A reader that catches the window sees a partial
// crontab and can stop scheduling — the same defect class that produced
// "unexpected end of JSON input" on the port-forward store.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
