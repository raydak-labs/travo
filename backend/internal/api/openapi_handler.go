package api

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
)

// openAPISpec is the OpenAPI 3.0 specification for the openwrt-travel-gui backend.
// Served at GET /api/openapi.json for agent/test automation use.
var openAPISpec = map[string]any{
	"openapi": "3.0.3",
	"info": map[string]any{
		"title":       "OpenWRT Travel Router GUI API",
		"description": "REST API for managing an OpenWRT travel router",
		"version":     "1.0.0",
	},
	"servers": []map[string]any{
		{"url": "/api/v1", "description": "Local device API"},
	},
	"components": map[string]any{
		"securitySchemes": map[string]any{
			"bearerAuth": map[string]any{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
			},
		},
	},
	"security": []map[string]any{
		{"bearerAuth": []string{}},
	},
	"paths": map[string]any{
		// Auth
		"/auth/login": map[string]any{
			"post": endpoint("Login", "Authenticate and receive a JWT token (expires_in is the relative session lifetime in seconds)", false,
				body("application/json", obj("username", "password")),
				resp200("application/json", obj("token", "expires_at", "expires_in")),
			),
		},
		"/auth/logout": map[string]any{
			"post": endpoint("Logout", "Invalidate the current JWT token", true, nil, resp200("application/json", obj("status"))),
		},
		"/auth/session": map[string]any{
			"get": endpoint("GetSession", "Get current session info (expires_in = remaining seconds relative to the server clock)", true, nil, resp200("application/json", obj("valid", "expires_in"))),
		},
		"/auth/password": map[string]any{
			"put": endpoint("ChangePassword", "Change the admin password", true,
				body("application/json", obj("current_password", "new_password")),
				resp200("application/json", obj("status")),
			),
		},
		// System
		"/system/info": map[string]any{
			"get": endpoint("GetSystemInfo", "Hardware model, firmware, kernel, hostname, uptime", true, nil, resp200("application/json", nil)),
		},
		"/system/stats": map[string]any{
			"get": endpoint("GetSystemStats", "CPU, memory, storage usage", true, nil, resp200("application/json", nil)),
		},
		"/system/logs": map[string]any{
			"get": endpoint("GetSystemLogs", "System log (logread/syslog)", true, nil, resp200("application/json", nil)),
		},
		"/system/logs/kernel": map[string]any{
			"get": endpoint("GetKernelLogs", "Kernel log (dmesg)", true, nil, resp200("application/json", nil)),
		},
		"/system/reboot": map[string]any{
			"post": endpoint("Reboot", "Reboot the device", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/shutdown": map[string]any{
			"post": endpoint("Shutdown", "Shut down the device", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/speedtest-service": map[string]any{
			"get": endpoint("GetSpeedtestService", "Ookla speedtest CLI availability and install state", true, nil, resp200("application/json", obj("installed", "supported", "architecture", "version"))),
		},
		"/system/speedtest-service/install": map[string]any{
			"post": endpoint("InstallSpeedtestCLI", "Install the Ookla speedtest CLI package (opkg or apk)", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/speedtest-service/uninstall": map[string]any{
			"post": endpoint("UninstallSpeedtestCLI", "Remove the Ookla speedtest CLI package", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/speedtest-service/run": map[string]any{
			"post": endpoint("RunSpeedtestCLI", "Run an Ookla speedtest (takes ~30-60s)", true, nil, resp200("application/json", nil)),
		},
		"/system/factory-reset": map[string]any{
			"post": endpoint("FactoryReset", "Factory reset the device", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/hostname": map[string]any{
			"put": endpoint("SetHostname", "Change the device hostname", true,
				body("application/json", obj("hostname")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/leds": map[string]any{
			"get": endpoint("GetLEDs", "Get LED status and stealth mode state", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetLEDStealth", "Enable or disable stealth mode (all LEDs off)", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/leds/schedule": map[string]any{
			"get": endpoint("GetLEDSchedule", "Get LED cron schedule", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetLEDSchedule", "Set LED on/off cron schedule", true,
				body("application/json", obj("on_cron", "off_cron")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/timezone": map[string]any{
			"get": endpoint("GetTimezone", "Get current timezone", true, nil, resp200("application/json", obj("timezone"))),
			"put": endpoint("SetTimezone", "Set device timezone", true,
				body("application/json", obj("timezone")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/backup": map[string]any{
			"get": endpoint("Backup", "Download UCI configuration archive", true, nil, resp200("application/octet-stream", nil)),
		},
		"/system/restore": map[string]any{
			"post": endpoint("Restore", "Upload and restore a UCI configuration archive", true,
				body("multipart/form-data", nil),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/firmware/upgrade": map[string]any{
			"post": endpoint("FirmwareUpgrade", "Upload and apply a sysupgrade image", true,
				body("multipart/form-data", nil),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/ntp": map[string]any{
			"get": endpoint("GetNTP", "Get NTP server configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetNTP", "Set NTP servers", true,
				body("application/json", obj("servers")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/ntp/sync": map[string]any{
			"post": endpoint("NTPSync", "Trigger manual NTP synchronization", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/time-sync": map[string]any{
			"post": endpoint("TimeSync", "Sync device clock from browser time. Unauthenticated only while the router clock is implausible (pre-login recovery, rate limited); authenticated callers may always sync.", false,
				body("application/json", obj("timestamp")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/setup-complete": map[string]any{
			"get":  endpoint("GetSetupComplete", "Get setup wizard completion state", true, nil, resp200("application/json", obj("complete"))),
			"post": endpoint("SetSetupComplete", "Mark setup wizard as complete", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/alerts": map[string]any{
			"get": endpoint("GetAlerts", "Get recent system alerts (last 50)", true, nil, resp200("application/json", nil)),
		},
		"/system/buttons": map[string]any{
			"get": endpoint("GetButtons", "Get hardware button configuration", true, nil, resp200("application/json", nil)),
		},
		"/system/button-actions": map[string]any{
			"put": endpoint("SetButtonActions", "Configure hardware button actions", true,
				body("application/json", nil),
				resp200("application/json", obj("ok")),
			),
		},
		// Network
		"/network/status": map[string]any{
			"get": endpoint("GetNetworkStatus", "WAN/LAN/WWAN interface status, internet reachability", true, nil, resp200("application/json", nil)),
		},
		"/network/wan": map[string]any{
			"get": endpoint("GetWANConfig", "Get WAN configuration (type, IP, DNS, MTU)", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetWANConfig", "Update WAN configuration", true,
				body("application/json", obj("proto", "ipaddr", "netmask", "gateway", "dns", "mtu")),
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
				body("application/json", obj("start", "limit", "leasetime")),
				resp200("application/json", obj("ok")),
			),
		},
		"/network/dhcp/leases": map[string]any{
			"get": endpoint("GetDHCPLeases", "List active DHCP leases with expiry", true, nil, resp200("application/json", nil)),
		},
		"/network/dhcp/reservations": map[string]any{
			"get":  endpoint("GetDHCPReservations", "List static DHCP reservations", true, nil, resp200("application/json", nil)),
			"post": endpoint("AddDHCPReservation", "Add a static DHCP reservation", true, body("application/json", obj("mac", "ip", "hostname")), resp200("application/json", obj("ok"))),
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
			"post": endpoint("AddDNSEntry", "Add a local DNS entry", true, body("application/json", obj("hostname", "ip")), resp200("application/json", obj("ok"))),
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
			"get": endpoint("GetDDNSConfig", "Get Dynamic DNS provider configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetDDNSConfig", "Update DDNS configuration", true, body("application/json", nil), resp200("application/json", obj("ok"))),
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
				body("application/json", nil),
				resp200("application/json", obj("status")),
			),
		},
		"/network/failover/events": map[string]any{
			"get": endpoint("GetFailoverEvents", "Get recent failover switch events", true, nil, resp200("application/json", nil)),
		},
		// SQM
		"/sqm/config": map[string]any{
			"get": endpoint("GetSQMConfig", "Get SQM (traffic shaping) configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetSQMConfig", "Update SQM configuration (does not restart sqm)", true,
				body("application/json", nil),
				resp200("application/json", obj("status")),
			),
		},
		"/sqm/apply": map[string]any{
			"post": endpoint("ApplySQM", "Restart SQM service to apply configuration", true, nil, resp200("application/json", nil)),
		},
		// WiFi
		"/wifi/scan": map[string]any{
			"get": endpoint("WiFiScan", "Scan for available networks (SSID, signal, encryption, band)", true, nil, resp200("application/json", nil)),
		},
		"/wifi/connect": map[string]any{
			"post": endpoint("WiFiConnect", "Connect to an upstream WiFi network", true,
				body("application/json", obj("ssid", "password", "encryption", "band", "hidden")),
				resp200("application/json", obj("token", "confirm_within_seconds")),
			),
		},
		"/wifi/disconnect": map[string]any{
			"post": endpoint("WiFiDisconnect", "Disconnect from the current upstream WiFi", true, nil, resp200("application/json", obj("ok"))),
		},
		"/wifi/connection": map[string]any{
			"get": endpoint("GetWiFiConnection", "Current upstream WiFi connection status", true, nil, resp200("application/json", nil)),
		},
		"/wifi/mode": map[string]any{
			"put": endpoint("SetWiFiMode", "Switch WiFi operating mode (ap/client/repeater)", true,
				body("application/json", obj("mode")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/saved": map[string]any{
			"get": endpoint("GetSavedNetworks", "List saved WiFi profiles", true, nil, resp200("application/json", nil)),
		},
		"/wifi/saved/{section}": map[string]any{
			"delete": endpoint("DeleteSavedNetwork", "Delete a saved WiFi profile", true, nil, resp200("application/json", obj("ok"))),
		},
		"/wifi/saved/priority": map[string]any{
			"put": endpoint("SetNetworkPriority", "Set priority ordering for saved networks", true,
				body("application/json", obj("ssids")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/radio": map[string]any{
			"get": endpoint("GetRadioStatus", "Get WiFi radio enabled state", true, nil, resp200("application/json", obj("enabled"))),
			"put": endpoint("SetRadioEnabled", "Enable or disable all WiFi radios", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/radios": map[string]any{
			"get": endpoint("GetRadios", "List radio hardware (band, channel, type)", true, nil, resp200("application/json", nil)),
		},
		"/wifi/ap": map[string]any{
			"get": endpoint("GetAPConfig", "Get AP configuration for all radios", true, nil, resp200("application/json", nil)),
		},
		"/wifi/ap/{section}": map[string]any{
			"put": endpoint("SetAPConfig", "Update AP configuration for a section", true,
				body("application/json", obj("ssid", "key", "encryption")),
				resp200("application/json", obj("token", "confirm_within_seconds")),
			),
		},
		"/wifi/repeater-options": map[string]any{
			"get": endpoint("GetRepeaterOptions", "Repeater radio policy (allow AP on STA radio)", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetRepeaterOptions", "Set repeater radio policy", true,
				body("application/json", obj("allow_ap_on_sta_radio")),
				resp200("application/json", nil),
			),
		},
		"/wifi/repeater/reconcile": map[string]any{
			"post": endpoint("ReconcileRepeaterAPLayout", "Re-apply repeater STA/AP per-radio separation", true, nil, resp200("application/json", obj("ok"))),
		},
		"/wifi/mac": map[string]any{
			"get": endpoint("GetMAC", "Get MAC addresses for all WiFi interfaces", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetMAC", "Set a custom MAC address on the STA interface", true,
				body("application/json", obj("mac")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/mac/randomize": map[string]any{
			"post": endpoint("RandomizeMAC", "Generate and apply a random MAC address", true, nil, resp200("application/json", obj("mac"))),
		},
		"/wifi/guest": map[string]any{
			"get": endpoint("GetGuestWiFi", "Get guest network configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetGuestWiFi", "Enable/disable guest network and set credentials", true,
				body("application/json", obj("enabled", "ssid", "key")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/autoreconnect": map[string]any{
			"get": endpoint("GetAutoReconnect", "Get auto-reconnect configuration", true, nil, resp200("application/json", obj("enabled"))),
			"put": endpoint("SetAutoReconnect", "Enable or disable auto-reconnect to saved networks", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/apply/confirm": map[string]any{
			"post": endpoint("ConfirmWiFiApply", "Confirm a pending wireless apply (browser-proof rollback)", true,
				body("application/json", obj("token")),
				resp200("application/json", obj("ok")),
			),
		},
		// VPN
		"/vpn/status": map[string]any{
			"get": endpoint("GetVPNStatus", "WireGuard VPN connection status and transfer stats", true, nil, resp200("application/json", nil)),
		},
		"/vpn/wireguard": map[string]any{
			"get": endpoint("GetWireGuard", "Get WireGuard UCI configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetWireGuard", "Update WireGuard configuration", true, body("application/json", nil), resp200("application/json", obj("ok"))),
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
		"/services": map[string]any{
			"get": endpoint("ListServices", "List installable services with state (installed/running/stopped)", true, nil, resp200("application/json", nil)),
		},
		"/services/{id}/install": map[string]any{
			"post": endpoint("InstallService", "Install a service package", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/install/stream": map[string]any{
			"post": endpoint("InstallServiceStream", "Install a service package with streaming log output", true, nil, resp200("text/event-stream", nil)),
		},
		"/services/{id}/remove": map[string]any{
			"post": endpoint("RemoveService", "Remove a service package", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/remove/stream": map[string]any{
			"post": endpoint("RemoveServiceStream", "Remove a service package with streaming log output", true, nil, resp200("text/event-stream", nil)),
		},
		"/services/{id}/start": map[string]any{
			"post": endpoint("StartService", "Start a service via init.d", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/stop": map[string]any{
			"post": endpoint("StopService", "Stop a service via init.d", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/autostart": map[string]any{
			"post": endpoint("SetAutoStart", "Enable or disable service auto-start on boot", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/services/adguardhome/status": map[string]any{
			"get": endpoint("AdGuardStatus", "AdGuard Home status, version, query statistics", true, nil, resp200("application/json", nil)),
		},
		// AdGuard
		"/adguard/dns": map[string]any{
			"get": endpoint("GetAdGuardDNS", "AdGuard DNS forwarding status and health", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetAdGuardDNS", "Enable or disable AdGuard as LAN DNS", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/adguard/config": map[string]any{
			"get": endpoint("GetAdGuardConfig", "Read AdGuardHome.yaml configuration", true, nil, resp200("application/json", obj("config"))),
			"put": endpoint("SetAdGuardConfig", "Write AdGuardHome.yaml and restart service", true,
				body("application/json", obj("config")),
				resp200("application/json", obj("ok")),
			),
		},
		// Captive portal
		"/captive/status": map[string]any{
			"get": endpoint("CaptiveStatus", "Detect captive portal and return redirect URL if present", true, nil, resp200("application/json", nil)),
		},
		"/captive/auto-accept": map[string]any{
			"post": endpoint("CaptiveAutoAccept", "Attempt common captive portal acceptance patterns", true,
				body("application/json", obj("portal_url")),
				resp200("application/json", nil),
			),
		},
		"/captive/dns-bypass": map[string]any{
			"post": endpoint("CaptiveDNSBypass", "Temporarily switch WAN DNS to upstream for captive portal access", true, nil, resp200("application/json", nil)),
		},
		"/captive/dns-restore": map[string]any{
			"post": endpoint("CaptiveDNSRestore", "Restore original DNS configuration after captive portal login", true, nil, resp200("application/json", nil)),
		},
	},
}

// endpoint builds an OpenAPI operation object.
func endpoint(operationID, summary string, requiresAuth bool, requestBody, response map[string]any) map[string]any {
	op := map[string]any{
		"operationId": operationID,
		"summary":     summary,
		"responses":   map[string]any{"200": response},
	}
	if requiresAuth {
		op["security"] = []map[string]any{{"bearerAuth": []string{}}}
	} else {
		op["security"] = []map[string]any{}
	}
	if requestBody != nil {
		op["requestBody"] = requestBody
	}
	return op
}

// body builds a requestBody object.
func body(contentType string, example map[string]any) map[string]any {
	content := map[string]any{}
	if example != nil {
		content[contentType] = map[string]any{
			"schema": map[string]any{"type": "object", "example": example},
		}
	} else {
		content[contentType] = map[string]any{}
	}
	return map[string]any{"required": true, "content": content}
}

// resp200 builds a 200 response object.
func resp200(contentType string, example map[string]any) map[string]any {
	content := map[string]any{}
	if example != nil {
		content[contentType] = map[string]any{
			"schema": map[string]any{"type": "object", "example": example},
		}
	} else {
		content[contentType] = map[string]any{}
	}
	return map[string]any{"description": "OK", "content": content}
}

// obj builds a simple string-keyed example object where all values are empty strings.
func obj(keys ...string) map[string]any {
	m := make(map[string]any, len(keys))
	for _, k := range keys {
		m[k] = ""
	}
	return m
}

// openAPIJSON is the cached JSON encoding of openAPISpec.
var openAPIJSON []byte

func init() {
	b, err := json.Marshal(openAPISpec)
	if err != nil {
		panic("openapi: failed to marshal spec: " + err.Error())
	}
	openAPIJSON = b
}

// OpenAPIHandler serves the OpenAPI 3.0 specification as JSON.
func OpenAPIHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		return c.Send(openAPIJSON)
	}
}
