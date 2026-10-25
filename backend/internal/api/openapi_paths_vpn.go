package api

/**
 * VPN tunnels, kill switch, split tunnel and Tailscale.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsVpn = map[string]any{
	"/vpn/status": map[string]any{
		"get": endpoint("GetVPNStatus", "WireGuard VPN connection status and transfer stats", true, nil, resp200("application/json", nil)),
	},
	"/vpn/wireguard": map[string]any{
		"get": endpoint("GetWireGuard", "Get WireGuard UCI configuration", true, nil, resp200("application/json", nil)),
		"put": endpoint("SetWireGuard", "Update WireGuard configuration", true, body("application/json", obj("private_key", "address", "dns", "peers")), resp200("application/json", obj("ok"))),
	},
	"/vpn/wireguard/toggle": map[string]any{
		"post": endpoint("ToggleWireGuard", "Enable or disable the WireGuard tunnel", true,
			body("application/json", obj("enabled")),
			resp200("application/json", obj("ok")),
		),
	},
	"/vpn/wireguard/import": map[string]any{
		"post": endpoint("ImportWireGuard", "Import a WireGuard .conf profile", true,
			body("application/json", obj("config")),
			resp200("application/json", obj("ok")),
		),
	},
	"/vpn/wireguard/status": map[string]any{
		"get": endpoint("GetWireGuardStatus", "Live wg show interface and peer stats", true, nil, resp200("application/json", nil)),
	},
	"/vpn/wireguard/profiles": map[string]any{
		"get":  endpoint("GetWireGuardProfiles", "List saved WireGuard profiles", true, nil, resp200("application/json", nil)),
		"post": endpoint("AddWireGuardProfile", "Save a new WireGuard profile", true, body("application/json", obj("name", "config")), resp200("application/json", obj("id"))),
	},
	"/vpn/wireguard/profiles/{id}": map[string]any{
		"delete": endpoint("DeleteWireGuardProfile", "Delete a WireGuard profile", true, nil, resp200("application/json", obj("ok"))),
	},
	"/vpn/wireguard/profiles/{id}/activate": map[string]any{
		"post": endpoint("ActivateWireGuardProfile", "Activate a saved WireGuard profile", true, nil, resp200("application/json", obj("ok"))),
	},
	"/vpn/killswitch": map[string]any{
		"get": endpoint("GetKillSwitch", "Get VPN kill switch state", true, nil, resp200("application/json", obj("enabled"))),
		"put": endpoint("SetKillSwitch", "Enable or disable the VPN kill switch", true,
			body("application/json", obj("enabled")),
			resp200("application/json", obj("ok")),
		),
	},
	"/vpn/tailscale": map[string]any{
		"get": endpoint("GetTailscale", "Get Tailscale status", true, nil, resp200("application/json", nil)),
	},
	"/vpn/tailscale/toggle": map[string]any{
		"post": endpoint("ToggleTailscale", "Enable or disable Tailscale", true,
			body("application/json", obj("enabled")),
			resp200("application/json", obj("ok")),
		),
	},
	"/vpn/tailscale/auth": map[string]any{
		"post": endpoint("StartTailscaleAuth", "Run tailscale up and return the browser auth URL when login is required", true,
			body("application/json", obj("auth_key")),
			resp200("application/json", obj("auth_url")),
		),
	},
	"/vpn/tailscale/exit-node": map[string]any{
		"post": endpoint("SetTailscaleExitNode", "Select (or clear) the Tailscale exit node by node IP", true,
			body("application/json", obj("node_ip")),
			resp200("application/json", obj("ok")),
		),
	},
	"/vpn/tailscale/ssh": map[string]any{
		"get": endpoint("GetTailscaleSSH", "Get whether Tailscale SSH is enabled", true, nil, resp200("application/json", obj("enabled"))),
		"put": endpoint("SetTailscaleSSH", "Enable or disable Tailscale SSH", true,
			body("application/json", obj("enabled")),
			resp200("application/json", obj("ok")),
		),
	},
	"/vpn/split-tunnel": map[string]any{
		"get": endpoint("GetSplitTunnel", "Get WireGuard split tunneling mode and routes", true, nil,
			resp200("application/json", obj("mode", "routes")),
		),
		"put": endpoint("SetSplitTunnel", "Set WireGuard split tunneling (mode is 'all' or 'custom')", true,
			body("application/json", obj("mode", "routes")),
			resp200("application/json", obj("ok")),
		),
	},
	"/vpn/dns-leak-test": map[string]any{
		"get": endpoint("DNSLeakTest", "Router-side check: WireGuard DNS vs effective upstream (resolv.conf; dnsmasq server= when resolv is loopback-only)", true, nil, resp200("application/json", nil)),
	},
	"/vpn/speed-test": map[string]any{
		"post": endpoint("RunWireGuardSpeedTest", "Download + ping speed test bound to WireGuard (wg0); requires tunnel enabled and up", true, nil, resp200("application/json", nil)),
	},
	"/vpn/wireguard/verify": map[string]any{
		"get": endpoint("VerifyWireGuard", "Verify WireGuard tunnel health: interface, handshake, route, firewall", true, nil, resp200("application/json", nil)),
	},
	// Services
}
