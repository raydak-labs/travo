package api

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
)

// openAPISpec is the OpenAPI 3.0 specification for the openwrt-travel-gui backend.
// Served at GET /api/openapi.json for agent/test automation use.
var openAPISpec = map[string]interface{}{
	"openapi": "3.0.3",
	"info": map[string]interface{}{
		"title":       "OpenWRT Travel Router GUI API",
		"description": "REST API for managing an OpenWRT travel router",
		"version":     "1.0.0",
	},
	"servers": []map[string]interface{}{
		{"url": "/api/v1", "description": "Local device API"},
	},
	"components": map[string]interface{}{
		"securitySchemes": map[string]interface{}{
			"bearerAuth": map[string]interface{}{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
			},
		},
	},
	"security": []map[string]interface{}{
		{"bearerAuth": []string{}},
	},
	"paths": map[string]interface{}{
		// Auth
		"/auth/login": map[string]interface{}{
			"post": endpoint("Login", "Authenticate and receive a JWT token", false,
				body("application/json", obj("username", "password")),
				resp200("application/json", obj("token")),
			),
		},
		"/auth/logout": map[string]interface{}{
			"post": endpoint("Logout", "Invalidate the current JWT token", true, nil, resp200("application/json", obj("ok"))),
		},
		"/auth/session": map[string]interface{}{
			"get": endpoint("GetSession", "Get current session info", true, nil, resp200("application/json", obj("username", "expires_at"))),
		},
		"/auth/password": map[string]interface{}{
			"put": endpoint("ChangePassword", "Change the admin password", true,
				body("application/json", obj("current_password", "new_password")),
				resp200("application/json", obj("ok")),
			),
		},
		// System
		"/system/info": map[string]interface{}{
			"get": endpoint("GetSystemInfo", "Hardware model, firmware, kernel, hostname, uptime", true, nil, resp200("application/json", nil)),
		},
		"/system/stats": map[string]interface{}{
			"get": endpoint("GetSystemStats", "CPU, memory, storage usage", true, nil, resp200("application/json", nil)),
		},
		"/system/logs": map[string]interface{}{
			"get": endpoint("GetSystemLogs", "System log (logread/syslog)", true, nil, resp200("application/json", nil)),
		},
		"/system/logs/kernel": map[string]interface{}{
			"get": endpoint("GetKernelLogs", "Kernel log (dmesg)", true, nil, resp200("application/json", nil)),
		},
		"/system/reboot": map[string]interface{}{
			"post": endpoint("Reboot", "Reboot the device", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/shutdown": map[string]interface{}{
			"post": endpoint("Shutdown", "Shut down the device", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/factory-reset": map[string]interface{}{
			"post": endpoint("FactoryReset", "Factory reset the device", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/hostname": map[string]interface{}{
			"put": endpoint("SetHostname", "Change the device hostname", true,
				body("application/json", obj("hostname")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/leds": map[string]interface{}{
			"get": endpoint("GetLEDs", "Get LED status and stealth mode state", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetLEDStealth", "Enable or disable stealth mode (all LEDs off)", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/leds/schedule": map[string]interface{}{
			"get": endpoint("GetLEDSchedule", "Get LED cron schedule", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetLEDSchedule", "Set LED on/off cron schedule", true,
				body("application/json", obj("on_cron", "off_cron")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/timezone": map[string]interface{}{
			"get": endpoint("GetTimezone", "Get current timezone", true, nil, resp200("application/json", obj("timezone"))),
			"put": endpoint("SetTimezone", "Set device timezone", true,
				body("application/json", obj("timezone")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/backup": map[string]interface{}{
			"get": endpoint("Backup", "Download UCI configuration archive", true, nil, resp200("application/octet-stream", nil)),
		},
		"/system/restore": map[string]interface{}{
			"post": endpoint("Restore", "Upload and restore a UCI configuration archive", true,
				body("multipart/form-data", nil),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/firmware/upgrade": map[string]interface{}{
			"post": endpoint("FirmwareUpgrade", "Upload and apply a sysupgrade image", true,
				body("multipart/form-data", nil),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/ntp": map[string]interface{}{
			"get": endpoint("GetNTP", "Get NTP server configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetNTP", "Set NTP servers", true,
				body("application/json", obj("servers")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/ntp/sync": map[string]interface{}{
			"post": endpoint("NTPSync", "Trigger manual NTP synchronization", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/time-sync": map[string]interface{}{
			"post": endpoint("TimeSync", "Sync device clock from browser time (pre-login)", false,
				body("application/json", obj("timestamp")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/setup-complete": map[string]interface{}{
			"get":  endpoint("GetSetupComplete", "Get setup wizard completion state", true, nil, resp200("application/json", obj("complete"))),
			"post": endpoint("SetSetupComplete", "Mark setup wizard as complete", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/alerts": map[string]interface{}{
			"get": endpoint("GetAlerts", "Get recent system alerts (last 50)", true, nil, resp200("application/json", nil)),
		},
		"/system/buttons": map[string]interface{}{
			"get": endpoint("GetButtons", "Get hardware button configuration", true, nil, resp200("application/json", nil)),
		},
		"/system/button-actions": map[string]interface{}{
			"put": endpoint("SetButtonActions", "Configure hardware button actions", true,
				body("application/json", nil),
				resp200("application/json", obj("ok")),
			),
		},
		// Network
		"/network/status": map[string]interface{}{
			"get": endpoint("GetNetworkStatus", "WAN/LAN/WWAN interface status, internet reachability", true, nil, resp200("application/json", nil)),
		},
		"/network/wan": map[string]interface{}{
			"get": endpoint("GetWANConfig", "Get WAN configuration (type, IP, DNS, MTU)", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetWANConfig", "Update WAN configuration", true,
				body("application/json", obj("proto", "ipaddr", "netmask", "gateway", "dns", "mtu")),
				resp200("application/json", obj("ok")),
			),
		},
		"/network/wan/detect": map[string]interface{}{
			"get": endpoint("DetectWANType", "Auto-detect WAN connection type (DHCP/PPPoE/static)", true, nil, resp200("application/json", obj("proto"))),
		},
		"/network/clients": map[string]interface{}{
			"get": endpoint("GetClients", "List DHCP clients with IP, MAC, hostname, traffic stats", true, nil, resp200("application/json", nil)),
		},
		"/network/clients/alias": map[string]interface{}{
			"put": endpoint("SetClientAlias", "Set a friendly alias for a client MAC address", true,
				body("application/json", obj("mac", "alias")),
				resp200("application/json", obj("ok")),
			),
		},
		"/network/clients/kick": map[string]interface{}{
			"post": endpoint("KickClient", "Disconnect a client from the network", true,
				body("application/json", obj("mac")),
				resp200("application/json", obj("ok")),
			),
		},
		"/network/clients/block": map[string]interface{}{
			"post": endpoint("BlockClient", "Block a client by MAC address", true,
				body("application/json", obj("mac")),
				resp200("application/json", obj("ok")),
			),
		},
		"/network/clients/unblock": map[string]interface{}{
			"post": endpoint("UnblockClient", "Remove a MAC block rule", true,
				body("application/json", obj("mac")),
				resp200("application/json", obj("ok")),
			),
		},
		"/network/clients/blocked": map[string]interface{}{
			"get": endpoint("GetBlockedClients", "List blocked client MAC addresses", true, nil, resp200("application/json", nil)),
		},
		"/network/dhcp": map[string]interface{}{
			"get": endpoint("GetDHCPConfig", "Get DHCP pool configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetDHCPConfig", "Update DHCP pool (range, lease time)", true,
				body("application/json", obj("start", "limit", "leasetime")),
				resp200("application/json", obj("ok")),
			),
		},
		"/network/dhcp/leases": map[string]interface{}{
			"get": endpoint("GetDHCPLeases", "List active DHCP leases with expiry", true, nil, resp200("application/json", nil)),
		},
		"/network/dhcp/reservations": map[string]interface{}{
			"get":  endpoint("GetDHCPReservations", "List static DHCP reservations", true, nil, resp200("application/json", nil)),
			"post": endpoint("AddDHCPReservation", "Add a static DHCP reservation", true, body("application/json", obj("mac", "ip", "hostname")), resp200("application/json", obj("ok"))),
		},
		"/network/dhcp/reservations/{section}": map[string]interface{}{
			"delete": endpoint("DeleteDHCPReservation", "Remove a static DHCP reservation", true, nil, resp200("application/json", obj("ok"))),
		},
		"/network/dns": map[string]interface{}{
			"get": endpoint("GetDNSConfig", "Get custom DNS servers for LAN", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetDNSConfig", "Set custom DNS servers for LAN", true, body("application/json", obj("servers")), resp200("application/json", obj("ok"))),
		},
		"/network/dns/entries": map[string]interface{}{
			"get":  endpoint("GetDNSEntries", "List local DNS hostname→IP entries", true, nil, resp200("application/json", nil)),
			"post": endpoint("AddDNSEntry", "Add a local DNS entry", true, body("application/json", obj("hostname", "ip")), resp200("application/json", obj("ok"))),
		},
		"/network/dns/entries/{section}": map[string]interface{}{
			"delete": endpoint("DeleteDNSEntry", "Remove a local DNS entry", true, nil, resp200("application/json", obj("ok"))),
		},
		"/network/interfaces/{name}/state": map[string]interface{}{
			"post": endpoint("SetInterfaceState", "Bring an interface up or down", true,
				body("application/json", obj("up")),
				resp200("application/json", obj("ok")),
			),
		},
		"/network/ddns": map[string]interface{}{
			"get": endpoint("GetDDNSConfig", "Get Dynamic DNS provider configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetDDNSConfig", "Update DDNS configuration", true, body("application/json", nil), resp200("application/json", obj("ok"))),
		},
		"/network/ddns/status": map[string]interface{}{
			"get": endpoint("GetDDNSStatus", "Get DDNS current public IP and last update", true, nil, resp200("application/json", nil)),
		},
		"/network/uptime-log": map[string]interface{}{
			"get": endpoint("GetUptimeLog", "Connection uptime event log (internet up/down timeline)", true, nil, resp200("application/json", nil)),
		},
		// WiFi
		"/wifi/scan": map[string]interface{}{
			"get": endpoint("WiFiScan", "Scan for available networks (SSID, signal, encryption, band)", true, nil, resp200("application/json", nil)),
		},
		"/wifi/connect": map[string]interface{}{
			"post": endpoint("WiFiConnect", "Connect to an upstream WiFi network", true,
				body("application/json", obj("ssid", "password", "encryption", "band", "hidden")),
				resp200("application/json", obj("token", "confirm_within_seconds")),
			),
		},
		"/wifi/disconnect": map[string]interface{}{
			"post": endpoint("WiFiDisconnect", "Disconnect from the current upstream WiFi", true, nil, resp200("application/json", obj("ok"))),
		},
		"/wifi/connection": map[string]interface{}{
			"get": endpoint("GetWiFiConnection", "Current upstream WiFi connection status", true, nil, resp200("application/json", nil)),
		},
		"/wifi/mode": map[string]interface{}{
			"put": endpoint("SetWiFiMode", "Switch WiFi operating mode (ap/client/repeater)", true,
				body("application/json", obj("mode")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/saved": map[string]interface{}{
			"get": endpoint("GetSavedNetworks", "List saved WiFi profiles", true, nil, resp200("application/json", nil)),
		},
		"/wifi/saved/{section}": map[string]interface{}{
			"delete": endpoint("DeleteSavedNetwork", "Delete a saved WiFi profile", true, nil, resp200("application/json", obj("ok"))),
		},
		"/wifi/saved/priority": map[string]interface{}{
			"put": endpoint("SetNetworkPriority", "Set priority ordering for saved networks", true,
				body("application/json", obj("ssids")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/radio": map[string]interface{}{
			"get": endpoint("GetRadioStatus", "Get WiFi radio enabled state", true, nil, resp200("application/json", obj("enabled"))),
			"put": endpoint("SetRadioEnabled", "Enable or disable all WiFi radios", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/radios": map[string]interface{}{
			"get": endpoint("GetRadios", "List radio hardware (band, channel, type)", true, nil, resp200("application/json", nil)),
		},
		"/wifi/ap": map[string]interface{}{
			"get": endpoint("GetAPConfig", "Get AP configuration for all radios", true, nil, resp200("application/json", nil)),
		},
		"/wifi/ap/{section}": map[string]interface{}{
			"put": endpoint("SetAPConfig", "Update AP configuration for a section", true,
				body("application/json", obj("ssid", "key", "encryption", "enabled")),
				resp200("application/json", obj("token", "confirm_within_seconds")),
			),
		},
		"/wifi/mac": map[string]interface{}{
			"get": endpoint("GetMAC", "Get MAC addresses for all WiFi interfaces", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetMAC", "Set a custom MAC address on the STA interface", true,
				body("application/json", obj("mac")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/mac/randomize": map[string]interface{}{
			"post": endpoint("RandomizeMAC", "Generate and apply a random MAC address", true, nil, resp200("application/json", obj("mac"))),
		},
		"/wifi/guest": map[string]interface{}{
			"get": endpoint("GetGuestWiFi", "Get guest network configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetGuestWiFi", "Enable/disable guest network and set credentials", true,
				body("application/json", obj("enabled", "ssid", "key")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/autoreconnect": map[string]interface{}{
			"get": endpoint("GetAutoReconnect", "Get auto-reconnect configuration", true, nil, resp200("application/json", obj("enabled"))),
			"put": endpoint("SetAutoReconnect", "Enable or disable auto-reconnect to saved networks", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/wifi/apply/confirm": map[string]interface{}{
			"post": endpoint("ConfirmWiFiApply", "Confirm a pending wireless apply (browser-proof rollback)", true,
				body("application/json", obj("token")),
				resp200("application/json", obj("ok")),
			),
		},
		// VPN
		"/vpn/status": map[string]interface{}{
			"get": endpoint("GetVPNStatus", "WireGuard VPN connection status and transfer stats", true, nil, resp200("application/json", nil)),
		},
		"/vpn/wireguard": map[string]interface{}{
			"get": endpoint("GetWireGuard", "Get WireGuard UCI configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetWireGuard", "Update WireGuard configuration", true, body("application/json", nil), resp200("application/json", obj("ok"))),
		},
		"/vpn/wireguard/toggle": map[string]interface{}{
			"post": endpoint("ToggleWireGuard", "Enable or disable the WireGuard tunnel", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/vpn/wireguard/import": map[string]interface{}{
			"post": endpoint("ImportWireGuard", "Import a WireGuard .conf profile", true,
				body("application/json", obj("config")),
				resp200("application/json", obj("ok")),
			),
		},
		"/vpn/wireguard/status": map[string]interface{}{
			"get": endpoint("GetWireGuardStatus", "Live wg show interface and peer stats", true, nil, resp200("application/json", nil)),
		},
		"/vpn/wireguard/profiles": map[string]interface{}{
			"get":  endpoint("GetWireGuardProfiles", "List saved WireGuard profiles", true, nil, resp200("application/json", nil)),
			"post": endpoint("AddWireGuardProfile", "Save a new WireGuard profile", true, body("application/json", obj("name", "config")), resp200("application/json", obj("id"))),
		},
		"/vpn/wireguard/profiles/{id}": map[string]interface{}{
			"delete": endpoint("DeleteWireGuardProfile", "Delete a WireGuard profile", true, nil, resp200("application/json", obj("ok"))),
		},
		"/vpn/wireguard/profiles/{id}/activate": map[string]interface{}{
			"post": endpoint("ActivateWireGuardProfile", "Activate a saved WireGuard profile", true, nil, resp200("application/json", obj("ok"))),
		},
		"/vpn/killswitch": map[string]interface{}{
			"get": endpoint("GetKillSwitch", "Get VPN kill switch state", true, nil, resp200("application/json", obj("enabled"))),
			"put": endpoint("SetKillSwitch", "Enable or disable the VPN kill switch", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/vpn/tailscale": map[string]interface{}{
			"get": endpoint("GetTailscale", "Get Tailscale status", true, nil, resp200("application/json", nil)),
		},
		"/vpn/tailscale/toggle": map[string]interface{}{
			"post": endpoint("ToggleTailscale", "Enable or disable Tailscale", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/vpn/dns-leak-test": map[string]interface{}{
			"get": endpoint("DNSLeakTest", "Router-side check: WireGuard DNS vs effective upstream (resolv.conf; dnsmasq server= when resolv is loopback-only)", true, nil, resp200("application/json", nil)),
		},
		"/vpn/speed-test": map[string]interface{}{
			"post": endpoint("RunWireGuardSpeedTest", "Download + ping speed test bound to WireGuard (wg0); requires tunnel enabled and up", true, nil, resp200("application/json", nil)),
		},
		"/vpn/wireguard/verify": map[string]interface{}{
			"get": endpoint("VerifyWireGuard", "Verify WireGuard tunnel health: interface, handshake, route, firewall", true, nil, resp200("application/json", nil)),
		},
		// Services
		"/services": map[string]interface{}{
			"get": endpoint("ListServices", "List installable services with state (installed/running/stopped)", true, nil, resp200("application/json", nil)),
		},
		"/services/{id}/install": map[string]interface{}{
			"post": endpoint("InstallService", "Install a service package", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/install/stream": map[string]interface{}{
			"post": endpoint("InstallServiceStream", "Install a service package with streaming log output", true, nil, resp200("text/event-stream", nil)),
		},
		"/services/{id}/remove": map[string]interface{}{
			"post": endpoint("RemoveService", "Remove a service package", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/remove/stream": map[string]interface{}{
			"post": endpoint("RemoveServiceStream", "Remove a service package with streaming log output", true, nil, resp200("text/event-stream", nil)),
		},
		"/services/{id}/start": map[string]interface{}{
			"post": endpoint("StartService", "Start a service via init.d", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/stop": map[string]interface{}{
			"post": endpoint("StopService", "Stop a service via init.d", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/autostart": map[string]interface{}{
			"post": endpoint("SetAutoStart", "Enable or disable service auto-start on boot", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/services/adguardhome/status": map[string]interface{}{
			"get": endpoint("AdGuardStatus", "AdGuard Home status, version, query statistics", true, nil, resp200("application/json", nil)),
		},
		// AdGuard
		"/adguard/dns": map[string]interface{}{
			"get": endpoint("GetAdGuardDNS", "AdGuard DNS forwarding status and health", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetAdGuardDNS", "Enable or disable AdGuard as LAN DNS", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("ok")),
			),
		},
		"/adguard/config": map[string]interface{}{
			"get": endpoint("GetAdGuardConfig", "Read AdGuardHome.yaml configuration", true, nil, resp200("application/json", obj("config"))),
			"put": endpoint("SetAdGuardConfig", "Write AdGuardHome.yaml and restart service", true,
				body("application/json", obj("config")),
				resp200("application/json", obj("ok")),
			),
		},
		// Captive portal
		"/captive/status": map[string]interface{}{
			"get": endpoint("CaptiveStatus", "Detect captive portal and return redirect URL if present", true, nil, resp200("application/json", nil)),
		},
		"/captive/auto-accept": map[string]interface{}{
			"post": endpoint("CaptiveAutoAccept", "Attempt common captive portal acceptance patterns", true,
				body("application/json", obj("portal_url")),
				resp200("application/json", nil),
			),
		},
	},
}

// endpoint builds an OpenAPI operation object.
func endpoint(operationID, summary string, requiresAuth bool, requestBody, response map[string]interface{}) map[string]interface{} {
	op := map[string]interface{}{
		"operationId": operationID,
		"summary":     summary,
		"responses":   map[string]interface{}{"200": response},
	}
	if requiresAuth {
		op["security"] = []map[string]interface{}{{"bearerAuth": []string{}}}
	} else {
		op["security"] = []map[string]interface{}{}
	}
	if requestBody != nil {
		op["requestBody"] = requestBody
	}
	return op
}

// body builds a requestBody object.
func body(contentType string, example map[string]interface{}) map[string]interface{} {
	content := map[string]interface{}{}
	if example != nil {
		content[contentType] = map[string]interface{}{
			"schema": map[string]interface{}{"type": "object", "example": example},
		}
	} else {
		content[contentType] = map[string]interface{}{}
	}
	return map[string]interface{}{"required": true, "content": content}
}

// resp200 builds a 200 response object.
func resp200(contentType string, example map[string]interface{}) map[string]interface{} {
	content := map[string]interface{}{}
	if example != nil {
		content[contentType] = map[string]interface{}{
			"schema": map[string]interface{}{"type": "object", "example": example},
		}
	} else {
		content[contentType] = map[string]interface{}{}
	}
	return map[string]interface{}{"description": "OK", "content": content}
}

// obj builds a simple string-keyed example object where all values are empty strings.
func obj(keys ...string) map[string]interface{} {
	m := make(map[string]interface{}, len(keys))
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
