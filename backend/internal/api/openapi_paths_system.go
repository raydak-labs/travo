package api

/**
 * Device status, logs, power, firmware and configuration.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsSystem = map[string]any{
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
}
