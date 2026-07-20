package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	// Arrange: create the app with routes
	app := setupApp()

	// Act: make a request to /api/health
	req, err := http.NewRequest(http.MethodGet, "/api/health", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("failed to perform request: %v", err)
	}
	defer resp.Body.Close()

	// Assert: status code should be 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Assert: content type should be JSON (Fiber v3 may append charset)
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("expected content-type application/json..., got %s", contentType)
	}

	// Assert: body should be {"status":"ok"}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	expected := `{"status":"ok"}`
	if string(body) != expected {
		t.Errorf("expected body %s, got %s", expected, string(body))
	}
}

func TestHealthEndpointMethod(t *testing.T) {
	app := setupApp()

	// POST to health should return 405
	req, _ := http.NewRequest(http.MethodPost, "/api/health", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("failed to perform request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 for POST, got %d", resp.StatusCode)
	}
}

// Unknown /api paths must return a JSON 404, not the SPA index.html — API
// consumers hitting a typo'd endpoint otherwise get 200 text/html.
func TestCatchAllDoesNotServeHTMLForAPIPaths(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(staticDir+"/index.html", []byte("<html>spa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.StaticDir = staticDir
	if tmpDir, err := os.MkdirTemp("", "travo-auth-*"); err == nil {
		cfg.AuthConfigPath = tmpDir + "/auth.json"
	}
	app, lifecycle := setupAppWithConfig(cfg)
	lifecycle.Stop()

	// Login (mock mode password is "admin") to get past auth middleware.
	loginBody := bytes.NewReader([]byte(`{"password":"admin"}`))
	loginReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	defer loginResp.Body.Close()
	var login struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&login); err != nil || login.Token == "" {
		t.Fatalf("could not obtain token: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/definitely-not-a-route", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown API path, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if bytes.Contains(body, []byte("<html>")) {
		t.Errorf("expected JSON error, got HTML: %s", body)
	}

	// SPA routes must still serve index.html.
	spaReq, _ := http.NewRequest(http.MethodGet, "/wifi", nil)
	spaResp, err := app.Test(spaReq, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("spa request failed: %v", err)
	}
	defer spaResp.Body.Close()
	spaBody, _ := io.ReadAll(spaResp.Body)
	if !bytes.Contains(spaBody, []byte("spa")) {
		t.Errorf("expected SPA index.html for non-API route, got: %s", spaBody)
	}
}
