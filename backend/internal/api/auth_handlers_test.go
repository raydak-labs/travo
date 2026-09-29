package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func setupTestApp() (*fiber.App, *Dependencies) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	authSvc := auth.NewAuthService("admin", "test-secret")
	blocklist := auth.NewTokenBlocklist()
	authSvc.SetBlocklist(blocklist)
	rateLimiter := auth.NewRateLimiter(5, time.Minute)

	tmpDir, _ := os.MkdirTemp("", "vpn-test-*")
	profilesPath := tmpDir + "/wireguard_profiles.json"
	priorityPath := tmpDir + "/wifi-priorities.json"
	autoReconnectPath := tmpDir + "/autoreconnect.json"
	reconnectScriptPath := tmpDir + "/wifi-reconnect.sh"
	bandSwitchConfigPath := tmpDir + "/band-switching.json"
	authConfigPath := tmpDir + "/auth.json"
	authStore := auth.NewFileAuthStore(authConfigPath)

	systemSvc := services.NewSystemService(ub, u, &services.MockStorageProvider{})
	wifiSvc := services.NewWifiServiceForTesting(u, ub, &services.NoopWifiReloader{}, &services.MockCommandRunner{}, priorityPath, autoReconnectPath, reconnectScriptPath)

	deps := &Dependencies{
		Auth:        authSvc,
		AuthStore:   authStore,
		Blocklist:   blocklist,
		RateLimiter: rateLimiter,
		System:      systemSvc,
		Network:     services.NewNetworkServiceWithRunner(u, ub, &services.MockCommandRunner{}),
		Wifi:        wifiSvc,
		Vpn: services.NewVpnServiceWithProfilesPath(u, &services.MockCommandRunner{
			Output: []byte("PRIV\tPUB_KEY\t51820\toff\nPEER1\t(none)\t1.2.3.4:51820\t0.0.0.0/0\t1710000000\t100\t200\toff\n"),
		}, profilesPath),
		ServiceManager: services.NewServiceManager(),
		Speedtest:      services.NewSpeedtestServiceWith(services.NewMockPackageManager()),
		Captive:        services.NewCaptiveService(&services.MockHTTPProber{StatusCode: 200, Body: "success\n"}),
		Alerts:         services.NewAlertService(systemSvc),
		BandSwitching:  services.NewBandSwitchingService(wifiSvc, bandSwitchConfigPath),
	}

	app := fiber.New()
	app.Use(authSvc.Middleware())

	// Health endpoint (excluded from auth)
	app.Get("/api/health", func(c fiber.Ctx) error {
		return RespondOK(c)
	})

	SetupRoutes(app, deps)
	return app, deps
}

func TestLoginSuccess(t *testing.T) {
	app, _ := setupTestApp()
	body, _ := json.Marshal(map[string]string{"password": "admin"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	app, _ := setupTestApp()
	body, _ := json.Marshal(map[string]string{"password": "wrong"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtectedRouteWithoutToken(t *testing.T) {
	app, _ := setupTestApp()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtectedRouteWithToken(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestLogoutBlocksToken(t *testing.T) {
	app, deps := setupTestApp()

	// Login to get a token
	token, _, _ := deps.Auth.Login("admin")

	// Verify the token works first
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 before logout, got %d", resp.StatusCode)
	}

	// Logout
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for logout, got %d", resp.StatusCode)
	}

	// Verify the token is now blocked
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

func TestLoginRateLimited(t *testing.T) {
	app, _ := setupTestApp()

	// Make 5 failed login attempts
	for i := range 5 {
		body, _ := json.Marshal(map[string]string{"password": "wrong"})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i, resp.StatusCode)
		}
	}

	// 6th attempt should be rate limited (429)
	body, _ := json.Marshal(map[string]string{"password": "wrong"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("rate limited request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 429, got %d, body: %s", resp.StatusCode, b)
	}
}

// A password change must invalidate every other session and hand the caller a
// fresh token, so a stolen token cannot outlive the change.
func TestChangePasswordRevokesSessionsAndReturnsToken(t *testing.T) {
	app, deps := setupTestApp()
	deps.Auth.SetSessionRegistry(auth.NewSessionRegistry(time.Hour))

	stolen, _, err := deps.Auth.Login("admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	caller, _, err := deps.Auth.Login("admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"current_password": "admin",
		"new_password":     "newpassword123",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+caller)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, respBody)
	}

	var data map[string]any
	if err := json.Unmarshal(respBody, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	fresh, _ := data["token"].(string)
	if fresh == "" {
		t.Fatalf("expected a fresh token in the response, got %s", respBody)
	}
	if status, _ := data["status"].(string); status != "ok" {
		t.Errorf("expected status ok for backwards compatibility, got %s", respBody)
	}
	if err := deps.Auth.ValidateToken(fresh); err != nil {
		t.Errorf("expected the fresh token to be valid, got %v", err)
	}

	// The old caller token and the other session are both dead.
	for name, token := range map[string]string{"caller": caller, "stolen": stolen} {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("%s session check: %v", name, err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s token: expected 401 after the password change, got %d", name, r.StatusCode)
		}
	}

	// The fresh token keeps the caller logged in.
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+fresh)
	r, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("fresh token session check: %v", err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Errorf("fresh token: expected 200, got %d", r.StatusCode)
	}
}

func TestChangePasswordRejectsShortPassword(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"current_password": "admin",
		"new_password":     "short12",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}
