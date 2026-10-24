package api

/**
 * Connectivity: WAN, clients, DHCP, DNS, firewall and traffic.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsNetwork = map[string]any{
	"/network/status": map[string]any{
		"get": endpoint("GetNetworkStatus", "WAN/LAN/WWAN interface status, internet reachability", true, nil, resp200("application/json", nil)),
	},
	"/network/traffic-history": map[string]any{
		"get": endpoint("GetTrafficHistory", "Recent per-interface RX/TX counters (~10 minutes, empty right after boot) plus the seconds actually retained", true, nil,
			resp200("application/json", obj("points", "retained_seconds")),
		),
	},
	"/network/connection-method": map[string]any{
		"get": endpoint("GetConnectionMethod", "How the calling client is connected (wifi-client/wifi-ap/ethernet) and its interface/IP", true, nil,
			resp200("application/json", obj("method", "interface", "ip_address")),
		),
	},
	"/network/wan": map[string]any{
		"get": endpoint("GetWANConfig", "Get WAN configuration (type, IP, DNS, MTU)", true, nil, resp200("application/json", nil)),
		"put": endpoint("SetWANConfig", "Update WAN configuration", true,
			body("application/json", obj("type", "interface_name", "ip_address", "netmask", "gateway", "dns_servers", "mtu")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/wan/detect": map[string]any{
		"get": endpoint("DetectWANType", "Auto-detect WAN connection type (DHCP/PPPoE/static)", true, nil, resp200("application/json", obj("proto"))),
	},
	"/network/clients": map[string]any{
		"get": endpoint("GetClients", "List DHCP clients with IP, MAC, hostname, traffic stats", true, nil, resp200("application/json", nil)),
	},
	"/network/clients/alias": map[string]any{
		"put": endpoint("SetClientAlias", "Set a friendly alias for a client MAC address", true,
			body("application/json", obj("mac", "alias")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/clients/kick": map[string]any{
		"post": endpoint("KickClient", "Disconnect a client from the network", true,
			body("application/json", obj("mac")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/clients/block": map[string]any{
		"post": endpoint("BlockClient", "Block a client by MAC address", true,
			body("application/json", obj("mac")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/clients/unblock": map[string]any{
		"post": endpoint("UnblockClient", "Remove a MAC block rule", true,
			body("application/json", obj("mac")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/clients/blocked": map[string]any{
		"get": endpoint("GetBlockedClients", "List blocked client MAC addresses", true, nil, resp200("application/json", nil)),
	},
	"/network/dhcp": map[string]any{
		"get": endpoint("GetDHCPConfig", "Get DHCP pool configuration", true, nil, resp200("application/json", nil)),
		"put": endpoint("SetDHCPConfig", "Update DHCP pool (range, lease time)", true,
			body("application/json", obj("start", "limit", "lease_time")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/dhcp/leases": map[string]any{
		"get": endpoint("GetDHCPLeases", "List active DHCP leases with expiry", true, nil, resp200("application/json", nil)),
	},
	"/network/dhcp/reservations": map[string]any{
		"get":  endpoint("GetDHCPReservations", "List static DHCP reservations", true, nil, resp200("application/json", nil)),
		"post": endpoint("AddDHCPReservation", "Add a static DHCP reservation", true, body("application/json", obj("name", "mac", "ip")), resp200("application/json", obj("ok"))),
	},
	"/network/dhcp/reservations/{section}": map[string]any{
		"delete": endpoint("DeleteDHCPReservation", "Remove a static DHCP reservation", true, nil, resp200("application/json", obj("ok"))),
	},
	"/network/dns": map[string]any{
		"get": endpoint("GetDNSConfig", "Get custom DNS servers for LAN", true, nil, resp200("application/json", nil)),
		"put": endpoint("SetDNSConfig", "Set custom DNS servers for LAN", true, body("application/json", obj("servers")), resp200("application/json", obj("ok"))),
	},
	"/network/dns/entries": map[string]any{
		"get":  endpoint("GetDNSEntries", "List local DNS hostname→IP entries", true, nil, resp200("application/json", nil)),
		"post": endpoint("AddDNSEntry", "Add a local DNS entry", true, body("application/json", obj("name", "ip")), resp200("application/json", obj("ok"))),
	},
	"/network/dns/entries/{section}": map[string]any{
		"delete": endpoint("DeleteDNSEntry", "Remove a local DNS entry", true, nil, resp200("application/json", obj("ok"))),
	},
	"/network/interfaces/{name}/state": map[string]any{
		"post": endpoint("SetInterfaceState", "Bring an interface up or down", true,
			body("application/json", obj("up")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/ddns": map[string]any{
		"get": endpoint("GetDDNSConfig", "Get Dynamic DNS provider configuration. `available` is false when ddns-scripts is not installed, in which case every write answers 503.", true, nil, resp200("application/json", obj("config", "available"))),
		"put": endpoint("SetDDNSConfig", "Update DDNS configuration. Answers 503 when ddns-scripts is not installed.", true, body("application/json", obj("enabled", "service", "domain", "username", "password", "lookup_host", "update_url")), resp200("application/json", obj("ok"))),
	},
	"/network/ddns/status": map[string]any{
		"get": endpoint("GetDDNSStatus", "Get DDNS current public IP and last update", true, nil, resp200("application/json", nil)),
	},
	"/network/uptime-log": map[string]any{
		"get": endpoint("GetUptimeLog", "Connection uptime event log (internet up/down timeline)", true, nil, resp200("application/json", nil)),
	},
	"/network/failover": map[string]any{
		"get": endpoint("GetFailoverConfig", "Get connection failover configuration and runtime status", true, nil, resp200("application/json", nil)),
		"put": endpoint("SetFailoverConfig", "Update connection failover ordering, enabled uplinks, and health tracking", true,
			body("application/json", obj("enabled", "active_interface", "candidates", "health")),
			resp200("application/json", obj("status")),
		),
	},
	"/network/failover/events": map[string]any{
		"get": endpoint("GetFailoverEvents", "Get recent failover switch events", true, nil, resp200("application/json", nil)),
	},
	// Firewall
	"/network/firewall/zones": map[string]any{
		"get": endpoint("GetFirewallZones", "List firewall zones with input/output/forward policies and networks", true, nil, resp200("application/json", obj("zones"))),
	},
	"/network/firewall/port-forwards": map[string]any{
		"get": endpoint("GetPortForwards", "List configured port forward rules", true, nil, resp200("application/json", obj("rules"))),
		"post": endpoint("AddPortForward", "Create a port forward rule", true,
			body("application/json", obj("id", "name", "protocol", "src_dport", "dest_ip", "dest_port", "enabled")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/firewall/port-forwards/{id}": map[string]any{
		"delete": endpoint("DeletePortForward", "Delete a port forward rule", true, nil,
			resp200("application/json", obj("ok")),
			param("id", "Rule identifier as returned by GET /network/firewall/port-forwards"),
		),
	},
	// Network diagnostics / services
	"/network/diagnostics": map[string]any{
		"post": endpoint("RunDiagnostics", "Run a diagnostic tool (ping, traceroute, dns) and return its output", true,
			body("application/json", obj("type", "target")),
			resp200("application/json", obj("type", "target", "output", "error")),
		),
	},
	"/network/doh": map[string]any{
		"get": endpoint("GetDoHConfig", "Get DNS-over-HTTPS configuration (cloudflare/google/quad9/custom)", true, nil,
			resp200("application/json", obj("enabled", "provider", "url")),
		),
		"put": endpoint("SetDoHConfig", "Set DNS-over-HTTPS configuration (url is used when provider is custom)", true,
			body("application/json", obj("enabled", "provider", "url")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/ipv6": map[string]any{
		"get": endpoint("GetIPv6Status", "Get IPv6 enablement state and global addresses", true, nil,
			resp200("application/json", obj("enabled", "addresses")),
		),
		"put": endpoint("SetIPv6Enabled", "Enable or disable IPv6", true,
			body("application/json", obj("enabled")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/wol": map[string]any{
		"post": endpoint("SendWoL", "Send a Wake-on-LAN magic packet to a MAC address", true,
			body("application/json", obj("mac", "interface")),
			resp200("application/json", obj("ok")),
		),
	},
	// Data usage (vnstat)
	"/network/data-usage": map[string]any{
		"get": endpoint("GetDataUsage", "Current data usage per interface from vnstat (requires vnstat)", true, nil,
			resp200("application/json", obj("available", "interfaces")),
		),
	},
	"/network/data-usage/reset": map[string]any{
		"post": endpoint("ResetDataUsage", "Reset the vnstat counters for a single interface", true,
			body("application/json", obj("interface")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/data-usage/budget": map[string]any{
		"get": endpoint("GetDataBudget", "Get per-interface monthly data budgets", true, nil, resp200("application/json", obj("budgets"))),
		"put": endpoint("SetDataBudget", "Set per-interface monthly data budgets", true,
			body("application/json", obj("budgets")),
			resp200("application/json", obj("ok")),
		),
	},
	// USB tethering
	"/network/usb-tethering": map[string]any{
		"get": endpoint("GetUSBTetheringStatus", "USB tethering detection state (device type, interface, IP, configured)", true, nil,
			resp200("application/json", obj("detected", "device_type", "interface", "is_up", "ip_address", "configured")),
		),
	},
	"/network/usb-tethering/configure": map[string]any{
		"post": endpoint("ConfigureUSBTethering", "Configure the given USB interface as a WAN source", true,
			body("application/json", obj("interface")),
			resp200("application/json", obj("ok")),
		),
	},
	"/network/usb-tethering/unconfigure": map[string]any{
		"post": endpoint("UnconfigureUSBTethering", "Remove the USB tethering WAN configuration", true, nil, resp200("application/json", obj("ok"))),
	},
	// SQM
}
