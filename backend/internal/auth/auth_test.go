package auth

import (
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

func TestLoginSuccess(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	token, expiry, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if expiry.Before(time.Now()) {
		t.Error("expected future expiry")
	}
}

func TestLoginFailure(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	_, _, err := svc.Login("wrongpassword")
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestValidateTokenValid(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	token, _, _ := svc.Login("admin")
	if err := svc.ValidateToken(token); err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		Subject:   "admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte("test-secret"))
	if err := svc.ValidateToken(signed); err == nil {
		t.Error("expected error for expired token")
	}
}

func TestValidateTokenInvalid(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	if err := svc.ValidateToken("not-a-valid-token"); err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestMiddlewareBlocksWithoutToken(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	app := fiber.New()
	app.Use(svc.Middleware())
	app.Get("/api/v1/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/test", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMiddlewareAllowsWithValidToken(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	token, _, _ := svc.Login("admin")
	app := fiber.New()
	app.Use(svc.Middleware())
	app.Get("/api/v1/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, body)
	}
}

// TestMiddlewareExemptsOnlyPublicPaths pins the auth middleware's allowlist.
//
// This replaced an earlier test asserting the middleware had NO path logic at
// all. That turned out to be unworkable: Fiber v3 scopes route-group middleware
// by PATH PREFIX rather than by which router a route was registered on, so
// moving the middleware onto the /api/v1 group also intercepted routes
// registered directly on the app -- the WebSocket upgrade and the public
// time-sync endpoint -- breaking both. The allowlist is back, and what makes it
// safe is that routing is case-sensitive (fiber.Config.CaseSensitive), so a
// case-varied request matches no route and cannot reach a handler.
//
// internal/api's TestAuthCoversEveryRoute is the companion gate: it walks the
// live route table and asserts this allowlist covers the public surface exactly.
func TestMiddlewareExemptsOnlyPublicPaths(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	app := fiber.New(fiber.Config{CaseSensitive: true})
	app.Use(svc.Middleware())
	handler := func(name string) fiber.Handler {
		return func(c fiber.Ctx) error { return c.SendString("handled:" + name) }
	}
	app.Get("/api/health", handler("health"))
	app.Get("/api/openapi.json", handler("openapi"))
	app.Post("/api/v1/auth/login", handler("login"))
	app.Post("/api/v1/system/time-sync", handler("time-sync"))
	app.Get("/api/v1/ws", handler("ws"))
	app.Get("/api/v1/system/info", handler("system-info"))
	app.Get("/index.html", handler("index"))

	status := func(method, p string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, p, nil)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("request %s %s failed: %v", method, p, err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	// Public API paths and static files must not require a token.
	for _, tc := range []struct{ method, path, want string }{
		{http.MethodGet, "/api/health", "handled:health"},
		{http.MethodGet, "/api/openapi.json", "handled:openapi"},
		{http.MethodPost, "/api/v1/auth/login", "handled:login"},
		{http.MethodPost, "/api/v1/system/time-sync", "handled:time-sync"},
		{http.MethodGet, "/api/v1/ws", "handled:ws"},
		{http.MethodGet, "/index.html", "handled:index"},
	} {
		got, body := status(tc.method, tc.path)
		if got == http.StatusUnauthorized {
			t.Errorf("%s %s: expected no 401, got one", tc.method, tc.path)
		}
		if body != tc.want {
			t.Errorf("%s %s: handler did not run (body %q)", tc.method, tc.path, body)
		}
	}

	// Every other API path is gated, and the handler must not run.
	for _, p := range []string{"/api/v1/system/info", "/api/v1/wifi/scan", "/api/v1/auth/logout"} {
		got, body := status(http.MethodGet, p)
		if got != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", p, got)
		}
		if body == "handled:system-info" {
			t.Errorf("%s: the handler ran without a token", p)
		}
	}

	// A case-varied spelling matches no route at all, so the allowlist cannot
	// be reached through it. This is the property the original bypass rested on
	// and the reason CaseSensitive must stay true.
	for _, p := range []string{
		"/API/V1/SYSTEM/INFO",
		"/Api/V1/System/Info",
		"/API/v1/auth/login",
		"/API/v1/system/time-sync",
		"/API/v1/ws",
	} {
		got, body := status(http.MethodGet, p)
		if got != http.StatusNotFound {
			t.Errorf("%s: expected 404 (no route matches a case variant), got %d (body %q)", p, got, body)
		}
	}
}

func TestChangePasswordSuccess(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	if _, err := svc.ChangePassword("admin", "newpassword123"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Old password should no longer work
	if _, _, err := svc.Login("admin"); err == nil {
		t.Error("expected old password to fail")
	}
	// New password should work
	if _, _, err := svc.Login("newpassword123"); err != nil {
		t.Errorf("expected new password to work, got: %v", err)
	}
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	_, err := svc.ChangePassword("wrongpassword", "newpassword123")
	if err == nil {
		t.Error("expected error for wrong current password")
	}
	if err.Error() != "invalid current password" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestChangePasswordTooShort(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	_, err := svc.ChangePassword("admin", "short")
	if err == nil {
		t.Error("expected error for short password")
	}
	if err.Error() != "new password must be at least 8 characters" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuthServiceWithUbus_StoresPasswordOnLogin(t *testing.T) {
	mub := ubus.NewMockUbus()
	mub.RegisterResponse("session.login", map[string]any{
		"ubus_rpc_session": "test-session",
	})

	pw := NewRootPassword()
	svc := NewAuthServiceWithUbus(mub, "test-secret", pw, "")

	token, _, err := svc.Login("my-router-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	if pw.Get() != "my-router-password" {
		t.Errorf("expected password holder to contain 'my-router-password', got %q", pw.Get())
	}
}

func TestAuthServiceWithUbus_DoesNotStorePasswordOnFailedLogin(t *testing.T) {
	mub := ubus.NewMockUbus()
	// Don't register session.login response — will fail
	pw := NewRootPassword()
	pw.Set("old-password")
	svc := NewAuthServiceWithUbus(mub, "test-secret", pw, "")

	_, _, err := svc.Login("wrong-password")
	if err == nil {
		t.Fatal("expected error for failed login")
	}

	// Password should remain unchanged
	if pw.Get() != "old-password" {
		t.Errorf("expected password holder to remain 'old-password', got %q", pw.Get())
	}
}

func TestAuthServiceWithUbus_NilPasswordHolder(t *testing.T) {
	mub := ubus.NewMockUbus()
	mub.RegisterResponse("session.login", map[string]any{
		"ubus_rpc_session": "test-session",
	})

	svc := NewAuthServiceWithUbus(mub, "test-secret", nil, "")

	token, _, err := svc.Login("any-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestAuthServiceWithUbus_PersistsSealOnLogin(t *testing.T) {
	mub := ubus.NewMockUbus()
	mub.RegisterResponse("session.login", map[string]any{
		"ubus_rpc_session": "test-session",
	})
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	pw := NewRootPassword()
	svc := NewAuthServiceWithUbus(mub, "jwt-seal-test", pw, authPath)

	_, _, err := svc.Login("stored-login-pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got := LoadSealedRPCDPassword(authPath, "jwt-seal-test"); got != "stored-login-pw" {
		t.Fatalf("LoadSealedRPCDPassword: got %q", got)
	}
}

func TestAuthServiceWithUbus_ChangePasswordUpdatesRootPasswordAndSeal(t *testing.T) {
	mub := ubus.NewMockUbus()
	mub.RegisterResponse("session.login", map[string]any{
		"ubus_rpc_session": "test-session",
	})
	mub.RegisterResponse("luci.setPassword", map[string]any{})

	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	pw := NewRootPassword()
	svc := NewAuthServiceWithUbus(mub, "jwt-change", pw, authPath)

	if _, err := svc.ChangePassword("current-ok", "newpassword99"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if pw.Get() != "newpassword99" {
		t.Fatalf("holder got %q want newpassword99", pw.Get())
	}
	if got := LoadSealedRPCDPassword(authPath, "jwt-change"); got != "newpassword99" {
		t.Fatalf("sealed got %q want newpassword99", got)
	}
}

func TestAuthServiceWithUbus_ChangePasswordWrongCurrentNoUbusSession(t *testing.T) {
	mub := ubus.NewMockUbus()
	// session.login not registered → tryUbusLogin fails

	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	pw := NewRootPassword()
	pw.Set("unchanged-holder")
	svc := NewAuthServiceWithUbus(mub, "jwt-x", pw, authPath)

	_, err := svc.ChangePassword("wrong-current", "newpassword99")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "invalid current password" {
		t.Fatalf("got %v", err)
	}
	if pw.Get() != "unchanged-holder" {
		t.Fatalf("holder should not change, got %q", pw.Get())
	}
}
