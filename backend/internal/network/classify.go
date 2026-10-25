package network

import (
	"net/netip"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

// ConnectionMethod describes how the client is connected.
type ConnectionMethod struct {
	Method    string `json:"method"`     // "wifi-client", "wifi-ap", "ethernet", "unknown"
	Interface string `json:"interface"`  // interface name (e.g., "br-lan", "wwan0")
	IPAddress string `json:"ip_address"` // client's IP address
}

// ClassifyDeps is everything the classifier reads off the router. It is a
// struct rather than a long argument list because both services build one, and
// a fifth positional argument would make it easy to pass the neighbour table
// where the command runner belongs.
type ClassifyDeps struct {
	Ubus    ubus.Ubus
	Cmd     platform.CommandRunner
	ARPFile string
}

// ClassifyClientConnection maps a client IP to the interface it is reachable
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
func ClassifyClientConnection(deps ClassifyDeps, clientIP string) *ConnectionMethod {
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
	ifaceDump, err := deps.Ubus.Call("network.interface", "dump", nil)
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
func classifyMatchedIface(deps ClassifyDeps, name, device, clientIP string) string {
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
func classifyLanClient(deps ClassifyDeps, clientIP string) string {
	mac, ok := arpMACForIP(deps.ARPFile, clientIP)
	if !ok {
		return "unknown"
	}
	if _, isWifi := apStationMACs(deps.Cmd)[mac]; isWifi {
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

// GetConnectionMethod determines how the client is connected by matching the
// client IP against network interface addresses. Uses ubus network interface
// dump for accurate address detection. On error, logs details and returns
// "unknown" to avoid breaking UI.
func (n *Service) GetConnectionMethod(clientIP string) (*ConnectionMethod, error) {
	return ClassifyClientConnection(ClassifyDeps{
		Ubus: n.ubus, Cmd: n.cmd, ARPFile: n.arpFile,
	}, clientIP), nil
}
