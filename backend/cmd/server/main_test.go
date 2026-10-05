package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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
	// t.TempDir() instead of a silent os.MkdirTemp fallback: on failure the old
	// form left cfg.AuthConfigPath pointing at the production path
	// /etc/travo/auth.json, so the test would read or write the real config.
	cfg.AuthConfigPath = t.TempDir() + "/auth.json"
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

// --- Hardened server defaults (Slowloris, body limit, CORS) ---

// Fiber's zero-value config inherits unbounded read/write behaviour, which
// lets idle sockets hold every connection slot and makes graceful shutdown
// hang on keep-alives.
func TestServerTimeoutsAreExplicit(t *testing.T) {
	if readTimeout <= 0 {
		t.Error("ReadTimeout must be set explicitly")
	}
	if writeTimeout <= readTimeout {
		t.Errorf("WriteTimeout (%v) must exceed ReadTimeout (%v) so a long response is never cut by the request deadline", writeTimeout, readTimeout)
	}
	if idleTimeout <= 0 {
		t.Error("IdleTimeout must be set explicitly so Shutdown is not blocked by idle keep-alives")
	}
	if idleTimeout > readTimeout {
		t.Errorf("IdleTimeout (%v) should be shorter than ReadTimeout (%v)", idleTimeout, readTimeout)
	}
	if shutdownTimeout <= 0 {
		t.Error("shutdown must be bounded")
	}
}

// OpenWrt sysupgrade images and backups exceed Fiber's 4 MB default, which
// made POST /system/firmware/upgrade and /system/restore answer 413.
func TestBodyLimitAllowsFirmwareImages(t *testing.T) {
	if bodyLimit < 32*1024*1024 {
		t.Errorf("BodyLimit %d is too small for OpenWrt firmware images", bodyLimit)
	}
}

// An unset CORS_ORIGINS must mean same-origin only, never "*".
func TestSplitCORSOrigins_UnsetIsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", ",", " , "} {
		if got := splitCORSOrigins(in); len(got) != 0 {
			t.Errorf("splitCORSOrigins(%q) = %v, want empty (same-origin only)", in, got)
		}
	}
}

func TestSplitCORSOrigins_ParsesList(t *testing.T) {
	got := splitCORSOrigins("http://a.example, https://b.example ,")
	want := []string{"http://a.example", "https://b.example"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("origin %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Fiber treats an empty AllowOrigins list as "allow all", so same-origin-only
// has to be expressed with an explicit deny-all AllowOriginsFunc.
func TestCORSConfig_DefaultsToSameOriginOnly(t *testing.T) {
	cfg := corsConfig("")
	if len(cfg.AllowOrigins) != 0 {
		t.Errorf("expected no allowed origins, got %v", cfg.AllowOrigins)
	}
	if cfg.AllowOriginsFunc == nil {
		t.Fatal("expected a deny-all AllowOriginsFunc (an empty AllowOrigins means allow-all in Fiber)")
	}
	if cfg.AllowOriginsFunc("http://evil.example") {
		t.Error("a cross-origin request must not be allowed by default")
	}
}

func TestCORSConfig_KeepsConfiguredOrigins(t *testing.T) {
	cfg := corsConfig("https://ui.example")
	if len(cfg.AllowOrigins) != 1 || cfg.AllowOrigins[0] != "https://ui.example" {
		t.Fatalf("expected the configured origin to be kept, got %v", cfg.AllowOrigins)
	}
	if cfg.AllowOriginsFunc != nil {
		t.Error("AllowOriginsFunc must stay nil when origins are configured")
	}
}

// Same-origin requests (no Origin header) must not be broken by the deny-all
// CORS config.
func TestSameOriginRequestHasNoCORSHeaders(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	// t.TempDir() instead of a silent os.MkdirTemp fallback: on failure the old
	// form left cfg.AuthConfigPath pointing at the production path
	// /etc/travo/auth.json, so the test would read or write the real config.
	cfg.AuthConfigPath = t.TempDir() + "/auth.json"
	app, lifecycle := setupAppWithConfig(cfg)
	defer lifecycle.Stop()

	req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin, got %q", got)
	}
}

// A cross-origin request must not receive an allow-origin header by default.
func TestCrossOriginRequestIsNotAllowedByDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	// t.TempDir() instead of a silent os.MkdirTemp fallback: on failure the old
	// form left cfg.AuthConfigPath pointing at the production path
	// /etc/travo/auth.json, so the test would read or write the real config.
	cfg.AuthConfigPath = t.TempDir() + "/auth.json"
	app, lifecycle := setupAppWithConfig(cfg)
	defer lifecycle.Stop()

	req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://evil.example")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("cross-origin request must not be granted access, got %q", got)
	}
}

// --- Lifecycle ---

// Stop() closes the store, so it must be idempotent: the signal handler and a
// deferred cleanup both call it.
func TestAppLifecycle_StopIsIdempotent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	// t.TempDir() instead of a silent os.MkdirTemp fallback: on failure the old
	// form left cfg.AuthConfigPath pointing at the production path
	// /etc/travo/auth.json, so the test would read or write the real config.
	cfg.AuthConfigPath = t.TempDir() + "/auth.json"
	_, lifecycle := setupAppWithConfig(cfg)

	done := make(chan struct{})
	go func() {
		lifecycle.Stop()
		lifecycle.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("lifecycle.Stop did not return")
	}
}

// Background goroutines must be stop-aware: on SIGTERM they must not wake up
// later and commit UCI changes.
func TestAppLifecycle_GoUnwindsOnStop(t *testing.T) {
	l := newAppLifecycle()
	var (
		mu       sync.Mutex
		finished bool
	)
	l.Go(func(stop <-chan struct{}) {
		sleepOrStop(stop, time.Hour)
		mu.Lock()
		finished = true
		mu.Unlock()
	})

	// Give the goroutine a moment to register, then stop it.
	time.Sleep(20 * time.Millisecond)
	l.stopOnce.Do(func() { close(l.stopCh) })

	deadline := time.After(5 * time.Second)
	for {
		l.wg.Wait()
		mu.Lock()
		done := finished
		mu.Unlock()
		if done {
			return
		}
		select {
		case <-deadline:
			t.Fatal("tracked goroutine did not observe the stop signal")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TestProductionAppRequiresAuthOnAPIRoutes drives the app that setupApp builds —
// that is, the real production wiring — and asserts that the protected API is
// actually protected.
//
// This exists because the composition of cmd/server/main.go was untested. Every
// auth test in internal/api and internal/auth builds its own app with its own
// middleware registration, so all of them stayed green while main.go itself
// registered none: commit 93e6e2b moved the middleware onto a route group and,
// when that approach was reverted in the same file's history, the app.Use line
// was never put back. The result was the exact vulnerability 70193d6 exists to
// close — the entire /api/v1 surface reachable without a token — shipping with
// a fully green test suite.
//
// A regression that drops or reorders this middleware fails here and nowhere
// else, which is the point: the gap was that no test observed the real wiring.
func TestProductionAppRequiresAuthOnAPIRoutes(t *testing.T) {
	app := setupApp()

	// One representative route per shape: a plain GET, a parameterised path, and
	// a mutating POST. Each must be rejected without a token.
	protected := []struct {
		method string
		path   string
		why    string
	}{
		{http.MethodGet, "/api/v1/system/info", "plain GET"},
		{http.MethodGet, "/api/v1/system/ssh-keys", "collection GET"},
		{http.MethodPost, "/api/v1/system/ssh-keys", "root SSH key grant (mutating)"},
		{http.MethodPost, "/api/v1/system/reboot", "device control (mutating)"},
		{http.MethodPost, "/api/v1/system/factory-reset", "destructive (mutating)"},
		{http.MethodGet, "/api/v1/wifi/scan", "radio operation"},
		{http.MethodGet, "/api/v1/network/status", "network read"},
	}

	for _, tc := range protected {
		t.Run(tc.why+" "+tc.path, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			status := resp.StatusCode
			resp.Body.Close()
			if status != http.StatusUnauthorized {
				t.Errorf("unauthenticated %s %s returned %d, want 401 — the auth "+
					"middleware is not mounted on the production app",
					tc.method, tc.path, status)
			}
		})
	}
}

// TestProductionAppKeepsPublicEndpointsOpen is the other half: a fix that mounts
// the middleware carelessly must not close the bootstrap paths. Both of these
// broke in this branch's history — time-sync answered 401 while a group-scoped
// middleware swallowed it.
func TestProductionAppKeepsPublicEndpointsOpen(t *testing.T) {
	app := setupApp()

	t.Run("health", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		status := resp.StatusCode
		resp.Body.Close()
		if status != http.StatusOK {
			t.Errorf("GET /api/health = %d, want 200 (public per auth.PublicPaths)", status)
		}
	})

	t.Run("openapi", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/openapi.json", nil)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		status := resp.StatusCode
		resp.Body.Close()
		if status != http.StatusOK {
			t.Errorf("GET /api/openapi.json = %d, want 200 (public: it is the "+
				"machine-readable contract for automation)", status)
		}
	})

	t.Run("time-sync", func(t *testing.T) {
		// Reaches the handler and is rejected for its own reasons (a missing
		// body), NOT by the auth middleware. A 401 here means the pre-login
		// clock recovery path is closed again.
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/time-sync", nil)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		status := resp.StatusCode
		resp.Body.Close()
		if status == http.StatusUnauthorized {
			t.Error("POST /api/v1/system/time-sync = 401: the pre-login clock " +
				"recovery path must stay reachable without a token")
		}
	})

	t.Run("login", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		status := resp.StatusCode
		resp.Body.Close()
		if status == http.StatusUnauthorized {
			t.Error("POST /api/v1/auth/login = 401: login cannot require its own token")
		}
	})
}
