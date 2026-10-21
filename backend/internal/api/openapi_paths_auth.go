package api

/**
 * Authentication, sessions and the password change.
 *
 * The path entries of the OpenAPI spec for this domain. The spec is
 * assembled from these per-domain maps in openapi_handler.go.
 */
var openAPIPathsAuth = map[string]any{
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
}
