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
			"post": endpoint("Login", "Authenticate as the hard-coded root user and receive a JWT token (expires_in is the relative session lifetime in seconds)", false,
				body("application/json", obj("password")),
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
			// Changing the password revokes EVERY session, including the
			// caller's, and returns a fresh token for it. A client that keeps
			// using the token it sent is logged out on its very next request.
			"put": endpoint("ChangePassword", "Change the admin password. Revokes all sessions and returns a replacement token for the caller.", true,
				body("application/json", obj("current_password", "new_password")),
				resp200("application/json", obj("status", "token", "expires_at", "expires_in", "revoked_sessions")),
			),
		},
		// System
		"/system/info": map[string]any{
			"get": endpoint("GetSystemInfo", "Hardware model, firmware, kernel, hostname, uptime", true, nil, resp200("application/json", nil)),
		},
		"/system/stats": map[string]any{
			"get": endpoint("GetSystemStats", "CPU, memory, storage usage", true, nil, resp200("application/json", nil)),
		},
		"/system/stats/history": map[string]any{
			"get": endpoint("GetStatsHistory", "Historic CPU/memory/traffic samples; returns a JSON array of {time, cpu, memory, rx_bytes, tx_bytes} points", true, nil,
				resp200("application/json", obj("time", "cpu", "memory", "rx_bytes", "tx_bytes")),
				query("since", "Unix seconds; only return points newer than this timestamp"),
			),
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
		"/system/speed-test": map[string]any{
			"post": endpoint("RunSpeedTest", "Run a WAN download/upload/ping speed test (takes ~30-60s)", true, nil,
				resp200("application/json", obj("download_mbps", "upload_mbps", "ping_ms", "server")),
			),
		},
		"/system/alert-thresholds": map[string]any{
			"get": endpoint("GetAlertThresholds", "Get the storage/CPU/memory percentage thresholds that raise system alerts", true, nil,
				resp200("application/json", obj("storage_percent", "cpu_percent", "memory_percent")),
			),
			"put": endpoint("SetAlertThresholds", "Set the storage/CPU/memory percentage thresholds that raise system alerts", true,
				body("application/json", obj("storage_percent", "cpu_percent", "memory_percent")),
				resp200("application/json", obj("ok")),
			),
		},
		// SSH key management grants root SSH access — keep it documented.
		"/system/ssh-keys": map[string]any{
			"get": endpoint("GetSSHKeys", "List authorized SSH public keys granted root access", true, nil, resp200("application/json", obj("keys"))),
			"post": endpoint("AddSSHKey", "Append a public key to root's authorized_keys (grants root SSH access)", true,
				body("application/json", obj("key")),
				resp200("application/json", obj("ok")),
			),
		},
		"/system/ssh-keys/{index}": map[string]any{
			"delete": endpoint("DeleteSSHKey", "Delete an authorized SSH public key by index (revokes root SSH access)", true, nil,
				resp200("application/json", obj("ok")),
				param("index", "Zero-based index of the key as listed by GET /system/ssh-keys"),
			),
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
				resp200("application/json", obj("status", "reboot_required")),
			),
		},
		"/system/leds": map[string]any{
			"get": endpoint("GetLEDs", "Get LED status and stealth mode state", true, nil, resp200("application/json", nil)),
			// The request field is stealth_mode (models.SetLEDRequest) and the
			// response is the full LED status, not {"ok":…}: documenting
			// "enabled" made the documented request be accepted, ignored and
			// answered 200 after doing the OPPOSITE of what was asked.
			"put": endpoint("SetLEDStealth", "Enable or disable stealth mode (all LEDs off). Returns the resulting LED status.", true,
				body("application/json", obj("stealth_mode")),
				resp200("application/json", obj("stealth_mode", "led_count", "leds")),
			),
		},
		"/system/leds/schedule": map[string]any{
			"get": endpoint("GetLEDSchedule", "Get LED cron schedule", true, nil, resp200("application/json", nil)),
			// Returns the persisted schedule (models.LEDSchedule), not {"ok":…}.
			"put": endpoint("SetLEDSchedule", "Set LED on/off cron schedule. Returns the stored schedule.", true,
				body("application/json", obj("enabled", "on_time", "off_time")),
				resp200("application/json", obj("enabled", "on_time", "off_time")),
			),
		},
		"/system/timezone": map[string]any{
			"get": endpoint("GetTimezone", "Get current timezone", true, nil, resp200("application/json", obj("timezone"))),
			"put": endpoint("SetTimezone", "Set device timezone. Both zonename and timezone are required.", true,
				body("application/json", obj("zonename", "timezone")),
				resp200("application/json", obj("status", "reboot_required")),
			),
		},
		"/system/backup": map[string]any{
			"get": endpoint("Backup", "Download UCI configuration archive", true, nil, resp200("application/octet-stream", nil)),
		},
		// Restore answers {"status","message"} (SystemHandlers.RestoreHandler),
		// not {"ok":…}. The upload is validated before sysupgrade runs: every
		// archive member must live under the config allowlist.
		"/system/restore": map[string]any{
			"post": endpoint("Restore", "Upload and restore a UCI configuration archive (multipart field: backup). Rejected with 400 unless every archive member lives under the config allowlist.", true,
				body("multipart/form-data", nil),
				resp200("application/json", obj("status", "message")),
			),
		},
		// Firmware answers {"status","message","model","supported_devices"}: the
		// image's OpenWrt metadata is parsed and checked against this board
		// before sysupgrade is started, and the parsed model is returned so the
		// UI can show which image was flashed.
		"/system/firmware/upgrade": map[string]any{
			"post": endpoint("FirmwareUpgrade", "Upload and apply a sysupgrade image (multipart field: firmware). Rejected with 400 unless the image metadata declares this board as a supported device.", true,
				body("multipart/form-data", nil),
				resp200("application/json", map[string]any{
					"status":            "ok",
					"message":           "",
					"model":             "",
					"supported_devices": []string{},
				}),
			),
		},
		"/system/ntp": map[string]any{
			"get": endpoint("GetNTP", "Get NTP server configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetNTP", "Set NTP servers", true,
				body("application/json", obj("servers")),
				resp200("application/json", obj("status", "reboot_required")),
			),
		},
		"/system/ntp/sync": map[string]any{
			"post": endpoint("NTPSync", "Trigger manual NTP synchronization", true, nil, resp200("application/json", obj("ok"))),
		},
		"/system/time-sync": map[string]any{
			// client_time_ms is what the handler binds; "timestamp" was documented,
			// which made the pre-login clock-recovery path unusable from a
			// generated client (400 "client_time_ms is required"). The response
			// is {"synced":true,"set_to":…} after a change, or
			// {"synced":false,"reason":…} when the clock was already accurate.
			"post": endpoint("TimeSync", "Sync device clock from browser time (client_time_ms = browser Date.now()). Unauthenticated only while the router clock is implausible (pre-login recovery, rate limited); authenticated callers may always sync.", false,
				body("application/json", obj("client_time_ms")),
				resp200("application/json", obj("synced", "reason", "set_to")),
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
		"/sqm/config": map[string]any{
			"get": endpoint("GetSQMConfig", "Get SQM (traffic shaping) configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetSQMConfig", "Update SQM configuration (does not restart sqm)", true,
				body("application/json", obj("enabled", "interface", "download_kbit", "upload_kbit", "qdisc", "script")),
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
		// Every wireless mutator below answers the apply envelope produced by
		// wifiMutationResponse (wifi_handlers.go): {"status":"ok","apply":{…}}
		// where apply carries the pending token and the rollback timeout. They
		// used to be documented as {"token","confirm_within_seconds"} or
		// {"ok":…}, neither of which any handler returns.
		"/wifi/connect": map[string]any{
			"post": endpoint("WiFiConnect", "Connect to an upstream WiFi network", true,
				body("application/json", obj("ssid", "password", "encryption", "band", "hidden")),
				resp200("application/json", wifiApplyEnvelope(nil)),
			),
		},
		"/wifi/disconnect": map[string]any{
			"post": endpoint("WiFiDisconnect", "Disconnect from the current upstream WiFi", true, nil, resp200("application/json", wifiApplyEnvelope(nil))),
		},
		"/wifi/connection": map[string]any{
			"get": endpoint("GetWiFiConnection", "Current upstream WiFi connection status", true, nil, resp200("application/json", nil)),
		},
		"/wifi/health": map[string]any{
			"get": endpoint("GetWiFiHealth", "WiFi health summary with detected issues, STA association and WWAN state", true, nil,
				resp200("application/json", obj("status", "issues", "repeater_same_radio_ap_sta", "sta", "wwan")),
			),
		},
		"/wifi/mode": map[string]any{
			// acknowledge_lockout is the opt-in for "do it anyway": switching to
			// client mode removes every access point, so a caller on WiFi gets 409
			// with code wifi_lockout_risk instead of the change.
			"put": endpoint("SetWiFiMode", "Switch WiFi operating mode (ap/client/repeater)", true,
				body("application/json", obj("mode", "acknowledge_lockout")),
				resp200("application/json", wifiApplyEnvelope(nil)),
			),
		},
		"/wifi/saved": map[string]any{
			"get": endpoint("GetSavedNetworks", "List saved WiFi profiles", true, nil, resp200("application/json", nil)),
		},
		"/wifi/saved/{section}": map[string]any{
			"delete": endpoint("DeleteSavedNetwork", "Delete a saved WiFi profile", true, nil, resp200("application/json", wifiApplyEnvelope(nil))),
		},
		"/wifi/saved/priority": map[string]any{
			"put": endpoint("SetNetworkPriority", "Set priority ordering for saved networks", true,
				body("application/json", obj("ssids")),
				resp200("application/json", wifiApplyEnvelope(nil)),
			),
		},
		"/wifi/radio": map[string]any{
			"get": endpoint("GetRadioStatus", "Get WiFi radio enabled state", true, nil, resp200("application/json", obj("enabled"))),
			"put": endpoint("SetRadioEnabled", "Enable or disable all WiFi radios", true,
				body("application/json", obj("enabled", "acknowledge_lockout")),
				resp200("application/json", wifiApplyEnvelope(nil)),
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
				body("application/json", obj("ssid", "key", "encryption", "enabled", "acknowledge_lockout")),
				resp200("application/json", wifiApplyEnvelope(nil)),
			),
		},
		"/wifi/repeater-options": map[string]any{
			"get": endpoint("GetRepeaterOptions", "Repeater radio policy (allow AP on STA radio)", true, nil, resp200("application/json", nil)),
			// The documented allow_ap_on_sta_radio escape hatch (ADR 0002 §2) is
			// echoed back next to the apply envelope.
			"put": endpoint("SetRepeaterOptions", "Set repeater radio policy", true,
				body("application/json", obj("allow_ap_on_sta_radio")),
				resp200("application/json", wifiApplyEnvelope(map[string]any{"allow_ap_on_sta_radio": true})),
			),
		},
		"/wifi/repeater/reconcile": map[string]any{
			"post": endpoint("ReconcileRepeaterAPLayout", "Re-apply repeater STA/AP per-radio separation", true, nil, resp200("application/json", wifiApplyEnvelope(nil))),
		},
		"/wifi/radios/{name}/role": map[string]any{
			"put": endpoint("SetRadioRole", "Assign the sta or ap role to a radio (only one active STA)", true,
				body("application/json", obj("role", "acknowledge_lockout")),
				resp200("application/json", wifiApplyEnvelope(nil)),
				param("name", "Radio device name, e.g. radio0"),
			),
		},
		"/wifi/band-switching": map[string]any{
			"get": endpoint("GetBandSwitching", "Automatic 2.4/5 GHz band switching configuration and live monitor state", true, nil,
				resp200("application/json", obj("config", "status")),
			),
			"put": endpoint("SetBandSwitching", "Configure automatic 2.4/5 GHz band switching thresholds and delays", true,
				body("application/json", obj("enabled", "preferred_band", "check_interval_sec", "down_switch_threshold_dbm", "down_switch_delay_sec", "up_switch_threshold_dbm", "up_switch_delay_sec", "min_viable_signal_dbm")),
				resp200("application/json", obj("status")),
			),
		},
		"/wifi/schedule": map[string]any{
			"get": endpoint("GetWiFiSchedule", "Get the WiFi on/off cron schedule", true, nil,
				resp200("application/json", obj("enabled", "on_time", "off_time")),
			),
			"put": endpoint("SetWiFiSchedule", "Set the WiFi on/off cron schedule (HH:MM, 24h)", true,
				body("application/json", obj("enabled", "on_time", "off_time")),
				resp200("application/json", obj("status")),
			),
		},
		"/wifi/mac-policies": map[string]any{
			"get": endpoint("GetMACPolicies", "List per-SSID MAC address policies used when connecting", true, nil, resp200("application/json", obj("policies"))),
			"put": endpoint("SetMACPolicies", "Replace the per-SSID MAC address policies", true,
				body("application/json", obj("policies")),
				resp200("application/json", obj("status")),
			),
		},
		"/wifi/mac": map[string]any{
			"get": endpoint("GetMAC", "Get MAC addresses for all WiFi interfaces", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetMAC", "Set a custom MAC address on the STA interface", true,
				body("application/json", obj("mac")),
				resp200("application/json", wifiApplyEnvelope(nil)),
			),
		},
		"/wifi/mac/randomize": map[string]any{
			"post": endpoint("RandomizeMAC", "Generate and apply a random MAC address", true, nil, resp200("application/json", wifiApplyEnvelope(map[string]any{"mac": ""}))),
		},
		"/wifi/guest": map[string]any{
			"get": endpoint("GetGuestWiFi", "Get guest network configuration", true, nil, resp200("application/json", nil)),
			"put": endpoint("SetGuestWiFi", "Enable/disable guest network and set credentials", true,
				body("application/json", obj("enabled", "ssid", "key", "acknowledge_lockout")),
				resp200("application/json", wifiApplyEnvelope(nil)),
			),
		},
		"/wifi/autoreconnect": map[string]any{
			"get": endpoint("GetAutoReconnect", "Get auto-reconnect configuration", true, nil, resp200("application/json", obj("enabled"))),
			"put": endpoint("SetAutoReconnect", "Enable or disable auto-reconnect to saved networks", true,
				body("application/json", obj("enabled")),
				resp200("application/json", obj("status")),
			),
		},
		"/wifi/apply/confirm": map[string]any{
			"post": endpoint("ConfirmWiFiApply", "Confirm a pending wireless apply (browser-proof rollback)", true,
				body("application/json", obj("token")),
				resp200("application/json", obj("status")),
			),
		},
		// VPN
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
				body("application/json", obj("node_ip", "exit_node")),
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
		"/services": map[string]any{
			"get": endpoint("ListServices", "List installable services with state (installed/running/stopped)", true, nil, resp200("application/json", nil)),
		},
		"/services/{id}/install": map[string]any{
			"post": endpoint("InstallService", "Install a service package", true, nil, resp200("application/json", obj("ok"))),
		},
		// The stream endpoints answer newline-delimited JSON (one {"type","data"}
		// object per line) with Content-Type: application/x-ndjson, not SSE.
		"/services/{id}/install/stream": map[string]any{
			"post": endpoint("InstallServiceStream", "Install a service package, streaming NDJSON log events", true, nil, resp200("application/x-ndjson", nil)),
		},
		"/services/{id}/remove": map[string]any{
			"post": endpoint("RemoveService", "Remove a service package", true, nil, resp200("application/json", obj("ok"))),
		},
		"/services/{id}/remove/stream": map[string]any{
			"post": endpoint("RemoveServiceStream", "Remove a service package, streaming NDJSON log events", true, nil, resp200("application/x-ndjson", nil)),
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
			"get": endpoint("GetAdGuardConfig", "Read the AdGuardHome.yaml configuration file contents. The response contains SECRETS: the web UI bind password hash, query log and statistics credentials and TLS key material. Treat it as sensitive.", true, nil, resp200("application/json", obj("content"))),
			"put": endpoint("SetAdGuardConfig", "Write the AdGuardHome.yaml configuration from the 'content' field and restart the service", true,
				body("application/json", obj("content")),
				resp200("application/json", obj("ok")),
			),
		},
		"/adguard/password": map[string]any{
			"put": endpoint("SetAdGuardPassword", "Set the AdGuard Home web UI password (username defaults to 'admin')", true,
				body("application/json", obj("username", "password")),
				resp200("application/json", obj("ok")),
			),
		},
		"/adguard/dns-mode": map[string]any{
			"get": endpoint("GetAdGuardDNSMode", "Effective LAN DNS mode (default, adguard-forwarding, adguard-direct) and whether DNS is bypassed", true, nil,
				resp200("application/json", obj("mode", "description", "adguard_running", "dns_bypassed")),
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

// endpoint builds an OpenAPI operation object. The optional parameters are
// path (param) and query (query) parameter objects, in that order.
func endpoint(operationID, summary string, requiresAuth bool, requestBody, response map[string]any, parameters ...map[string]any) map[string]any {
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
	if len(parameters) > 0 {
		op["parameters"] = parameters
	}
	return op
}

// param builds a required path parameter object.
func param(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "path",
		"required":    true,
		"description": description,
		"schema":      map[string]any{"type": "string"},
	}
}

// query builds an optional query parameter object.
func query(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"required":    false,
		"description": description,
		"schema":      map[string]any{"type": "string"},
	}
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

// wifiApplyEnvelope is the 200 body every wireless mutator answers, built by
// wifiMutationResponse in wifi_handlers.go: {"status":"ok","apply":{…}} where
// apply carries the pending token the browser must confirm and the rollback
// timeout. probe_budget_seconds is how long ONE confirm call can block on the
// device while it waits for the new interfaces to come up; a client that re-POSTs
// confirm until the rollback deadline has to leave that much room, or a probe it
// starts near the deadline is answered after rpcd has already rolled back. extra
// adds the one endpoint-specific key a handler appends (allow_ap_on_sta_radio, mac).
func wifiApplyEnvelope(extra map[string]any) map[string]any {
	body := map[string]any{
		"status": "ok",
		"apply": map[string]any{
			"pending":                  true,
			"token":                    "",
			"rollback_timeout_seconds": 90,
			"probe_budget_seconds":     4,
		},
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
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
