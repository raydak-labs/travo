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
	if tmpDir, err := os.MkdirTemp("", "travo-auth-*"); err == nil {
		cfg.AuthConfigPath = tmpDir + "/auth.json"
	}
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
	if tmpDir, err := os.MkdirTemp("", "travo-auth-*"); err == nil {
		cfg.AuthConfigPath = tmpDir + "/auth.json"
	}
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
	if tmpDir, err := os.MkdirTemp("", "travo-auth-*"); err == nil {
		cfg.AuthConfigPath = tmpDir + "/auth.json"
	}
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
