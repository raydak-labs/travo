package api

/**
 * Wireless: radios, access points, repeater, guest and schedules.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsWifi = map[string]any{
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
}
