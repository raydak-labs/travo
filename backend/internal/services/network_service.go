package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
}

// NewNetworkService creates a new NetworkService.
func NewNetworkService(u uci.UCI, ub ubus.Ubus) *NetworkService {
	return &NetworkService{uci: u, ubus: ub, aliasFile: "/etc/openwrt-travel-gui/aliases.json", cmd: &RealCommandRunner{}}
}

// NewNetworkServiceWithAliasFile creates a NetworkService with a custom alias file path.
func NewNetworkServiceWithAliasFile(u uci.UCI, ub ubus.Ubus, aliasFile string) *NetworkService {
	return &NetworkService{uci: u, ubus: ub, aliasFile: aliasFile, cmd: &RealCommandRunner{}}
}

// NewNetworkServiceWithRunner creates a NetworkService with a custom command runner (for testing).
func NewNetworkServiceWithRunner(u uci.UCI, ub ubus.Ubus, cmd CommandRunner) *NetworkService {
	return &NetworkService{uci: u, ubus: ub, aliasFile: "/etc/openwrt-travel-gui/aliases.json", cmd: cmd}
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

// parseDHCPLeasesFile reads /tmp/dhcp.leases and returns a map of MAC → lease expiry time.
func parseDHCPLeasesFile() map[string]int64 {
	result := map[string]int64{}
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
		result[mac] = expiry
	}
	return result
}

// fetchDHCPClients queries ubus for DHCP lease information with ARP fallback.
func (n *NetworkService) fetchDHCPClients() []models.Client {
	var clients []models.Client
	leaseTimeSec := n.dhcpLeaseTimeSeconds()

	// Try ubus dhcp ipv4leases first
	data, err := n.ubus.Call("dhcp", "ipv4leases", nil)
	if err == nil {
		if device, ok := data["device"].(map[string]interface{}); ok {
			for ifaceName, ifaceData := range device {
				ifaceMap, ok := ifaceData.(map[string]interface{})
				if !ok {
					continue
				}
				leases, ok := ifaceMap["leases"].([]interface{})
				if !ok {
					continue
				}
				for _, lease := range leases {
					lm, ok := lease.(map[string]interface{})
					if !ok {
						continue
					}
					ip, _ := lm["ip"].(string)
					mac, _ := lm["mac"].(string)
					hostname, _ := lm["hostname"].(string)
					expires, _ := lm["expires"].(float64)

					// Calculate connected_since from lease expiry.
					// The "expires" field counts down from the lease time,
					// so connected_since ≈ now - (leaseTime - expires).
					var connectedSince string
					if expires > 0 {
						elapsed := leaseTimeSec - expires
						if elapsed < 0 {
							elapsed = 0
						}
						connSince := time.Now().Add(-time.Duration(elapsed) * time.Second)
						connectedSince = connSince.UTC().Format(time.RFC3339)
					}

					clients = append(clients, models.Client{
						IPAddress: ip, MACAddress: mac,
						Hostname: hostname, InterfaceName: ifaceName,
						ConnectedSince: connectedSince,
					})
				}
			}
			if len(clients) > 0 {
				return clients
			}
		}
	}

	// Fallback: read /tmp/dhcp.leases then ARP table
	dhcpLeases := parseDHCPLeasesFile()

	arpData, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return clients
	}
	lines := strings.Split(string(arpData), "\n")
	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		ip := fields[0]
		mac := fields[3]
		iface := fields[5]
		// Skip incomplete entries and the router's own interface IPs
		if mac == "00:00:00:00:00:00" || ip == "0.0.0.0" {
			continue
		}
		// Only include LAN clients (br-lan), not upstream (phy0-sta0, wan)
		if iface != "br-lan" {
			continue
		}
		// Try to compute connected_since from dhcp.leases expiry
		var connectedSince string
		if expiry, ok := dhcpLeases[strings.ToUpper(mac)]; ok && expiry > 0 {
			connSince := time.Unix(expiry, 0).Add(-time.Duration(leaseTimeSec) * time.Second)
			connectedSince = connSince.UTC().Format(time.RFC3339)
		}
		clients = append(clients, models.Client{
			IPAddress: ip, MACAddress: mac,
			InterfaceName: iface,
			ConnectedSince: connectedSince,
		})
	}

	return clients
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
		if err := n.uci.Set("network", "wan", "proto", config.Type); err != nil {
			return fmt.Errorf("setting WAN proto: %w", err)
		}
	}
	if config.IPAddress != "" {
		if err := n.uci.Set("network", "wan", "ip4addr", config.IPAddress); err != nil {
			return fmt.Errorf("setting WAN ip4addr: %w", err)
		}
	}
	if config.Netmask != "" {
		if err := n.uci.Set("network", "wan", "netmask", config.Netmask); err != nil {
			return fmt.Errorf("setting WAN netmask: %w", err)
		}
	}
	if config.Gateway != "" {
		if err := n.uci.Set("network", "wan", "gateway", config.Gateway); err != nil {
			return fmt.Errorf("setting WAN gateway: %w", err)
		}
	}
	return n.uci.Commit("network")
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
		if err := n.uci.Set("network", "wan", "peerdns", "0"); err != nil {
			return fmt.Errorf("setting peerdns: %w", err)
		}
		dns := strings.Join(config.Servers, " ")
		if err := n.uci.Set("network", "wan", "dns", dns); err != nil {
			return fmt.Errorf("setting dns: %w", err)
		}
	} else {
		if err := n.uci.Set("network", "wan", "peerdns", "1"); err != nil {
			return fmt.Errorf("setting peerdns: %w", err)
		}
		if err := n.uci.Set("network", "wan", "dns", ""); err != nil {
			return fmt.Errorf("clearing dns: %w", err)
		}
	}
	return n.uci.Commit("network")
}

// SetDHCPConfig updates the DHCP configuration for the LAN.
func (n *NetworkService) SetDHCPConfig(config models.DHCPConfig) error {
	if err := n.uci.Set("dhcp", "lan", "start", strconv.Itoa(config.Start)); err != nil {
		return fmt.Errorf("setting DHCP start: %w", err)
	}
	if err := n.uci.Set("dhcp", "lan", "limit", strconv.Itoa(config.Limit)); err != nil {
		return fmt.Errorf("setting DHCP limit: %w", err)
	}
	if err := n.uci.Set("dhcp", "lan", "leasetime", config.LeaseTime); err != nil {
		return fmt.Errorf("setting DHCP leasetime: %w", err)
	}
	return n.uci.Commit("dhcp")
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
	// Try common AP interfaces
	for _, iface := range []string{"phy0-ap0", "phy1-ap0", "wlan0", "wlan1"} {
		_, _ = n.cmd.Run("hostapd_cli", "-i", iface, "disassociate", mac)
	}
	return nil
}

// BlockClient adds a firewall rule to drop all traffic from a MAC address.
func (n *NetworkService) BlockClient(mac string) error {
	section := "block_" + strings.ReplaceAll(strings.ToUpper(mac), ":", "")
	if err := n.uci.AddSection("firewall", section, "rule"); err != nil {
		return fmt.Errorf("adding firewall block rule: %w", err)
	}
	if err := n.uci.Set("firewall", section, "name", "Block-"+strings.ToUpper(mac)); err != nil {
		return fmt.Errorf("setting block rule name: %w", err)
	}
	if err := n.uci.Set("firewall", section, "src", "lan"); err != nil {
		return fmt.Errorf("setting block rule src: %w", err)
	}
	if err := n.uci.Set("firewall", section, "src_mac", strings.ToUpper(mac)); err != nil {
		return fmt.Errorf("setting block rule src_mac: %w", err)
	}
	if err := n.uci.Set("firewall", section, "target", "DROP"); err != nil {
		return fmt.Errorf("setting block rule target: %w", err)
	}
	if err := n.uci.Commit("firewall"); err != nil {
		return fmt.Errorf("committing firewall: %w", err)
	}
	_, _ = n.cmd.Run("/etc/init.d/firewall", "restart")
	return nil
}

// UnblockClient removes the firewall block rule for a MAC address.
func (n *NetworkService) UnblockClient(mac string) error {
	section := "block_" + strings.ReplaceAll(strings.ToUpper(mac), ":", "")
	if err := n.uci.DeleteSection("firewall", section); err != nil {
		return fmt.Errorf("deleting firewall block rule: %w", err)
	}
	if err := n.uci.Commit("firewall"); err != nil {
		return fmt.Errorf("committing firewall: %w", err)
	}
	_, _ = n.cmd.Run("/etc/init.d/firewall", "restart")
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
