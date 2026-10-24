package api

/**
 * Service management, SQM, AdGuard, captive portal and data usage.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsServices = map[string]any{
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
}
