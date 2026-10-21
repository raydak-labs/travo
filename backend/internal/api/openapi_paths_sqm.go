package api

/**
 * Smart queue management: configuration and apply.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsSqm = map[string]any{
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
}
