package api

/**
 * Captive portal detection and DNS bypass.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsCaptive = map[string]any{
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
}
