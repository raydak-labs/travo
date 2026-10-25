package services

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
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

	// USB tethering (typically a phone) creates its own logical interface in
	// the wan zone. It has to appear in Interfaces, or a phone-tethered router
	// reports the uplink as down while it is up.
	usbData, usbErr := n.ubus.Call("network.interface."+usbTetherUCIName, "status", nil)

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

	if usbErr == nil {
		usbIface := parseInterface(usbTetherUCIName, usbTetherUCIName, usbData, n.ubus)
		status.Interfaces = append(status.Interfaces, usbIface)
		if usbIface.IsUp && (status.WAN == nil || !status.WAN.IsUp) {
			status.WAN = &usbIface
		}
	}

	// Fetch DHCP clients from ubus
	status.Clients = n.fetchDHCPClients()

	status.InternetReachable = status.WAN != nil && status.WAN.IsUp
	return status, nil
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
				return fmt.Errorf("restart firewall: %w (rollback of %s failed: %w)", err, section, undoErr)
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
				return fmt.Errorf("restart firewall: %w (rollback of %s failed: %w)", err, section, undoErr)
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
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
