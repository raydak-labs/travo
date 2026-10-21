package api

/**
 * AdGuard Home DNS configuration.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsAdguard = map[string]any{
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
}
