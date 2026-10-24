package services

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

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
