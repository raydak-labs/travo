package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

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

	deps := &Dependencies{
		Auth:           authSvc,
		Blocklist:      blocklist,
		RateLimiter:    rateLimiter,
		System:         services.NewSystemService(ub, u, &services.MockStorageProvider{}),
		Network:        services.NewNetworkService(u, ub),
		Wifi:           services.NewWifiServiceWithReloader(u, ub, &services.NoopWifiReloader{}),
		Vpn:            services.NewVpnService(u),
		ServiceManager: services.NewServiceManager(),
		Captive:        services.NewCaptiveService(&services.MockHTTPProber{StatusCode: 200, Body: "success\n"}),
	}

	app := fiber.New()
	app.Use(authSvc.Middleware())

	// Health endpoint (excluded from auth)
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	SetupRoutes(app, deps)
	return app, deps
}

func TestLoginSuccess(t *testing.T) {
	app, _ := setupTestApp()
	body, _ := json.Marshal(map[string]string{"password": "admin"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
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
	resp, err := app.Test(req, -1)
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
	resp, err := app.Test(req, -1)
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
	resp, err := app.Test(req, -1)
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
	resp, err := app.Test(req, -1)
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
	resp, err = app.Test(req, -1)
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
	resp, err = app.Test(req, -1)
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
	for i := 0; i < 5; i++ {
		body, _ := json.Marshal(map[string]string{"password": "wrong"})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("rate limited request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 429, got %d, body: %s", resp.StatusCode, b)
	}
}
