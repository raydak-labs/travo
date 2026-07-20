package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/auth"
)

func postTimeSync(t *testing.T, app *fiber.App, token string, clientTimeMs int64) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]int64{"client_time_ms": clientTimeMs})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/time-sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

// With a plausible clock (now >= MinPlausible) an unauthenticated caller must
// not be able to change the system time.
func TestTimeSync_UnauthenticatedPlausibleClockRejected(t *testing.T) {
	app, deps := setupTestApp()
	deps.TimeSyncSetTime = func(epochSec int64) error { t.Fatal("SetTime must not be called"); return nil }

	resp := postTimeSync(t, app, "", time.Now().Add(2*time.Hour).UnixMilli())
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 403, got %d, body: %s", resp.StatusCode, b)
	}
}

// When the router clock is implausible (before build time), an unauthenticated
// pre-login sync is allowed so the user can log in at all.
func TestTimeSync_UnauthenticatedImplausibleClockAllowed(t *testing.T) {
	app, deps := setupTestApp()
	var setTo int64
	deps.TimeSyncMinPlausible = time.Now().Add(time.Hour) // pretend build time is ahead of clock
	deps.TimeSyncSetTime = func(epochSec int64) error { setTo = epochSec; return nil }

	client := time.Now().Add(2 * time.Hour)
	resp := postTimeSync(t, app, "", client.UnixMilli())
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
	if setTo != client.Unix() {
		t.Errorf("expected SetTime called with %d, got %d", client.Unix(), setTo)
	}
}

// Authenticated callers may sync regardless of clock plausibility.
func TestTimeSync_AuthenticatedAllowed(t *testing.T) {
	app, deps := setupTestApp()
	var called bool
	deps.TimeSyncSetTime = func(epochSec int64) error { called = true; return nil }
	token, _, _ := deps.Auth.Login("admin")

	resp := postTimeSync(t, app, token, time.Now().Add(2*time.Hour).UnixMilli())
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
	if !called {
		t.Error("expected SetTime to be called for authenticated sync")
	}
}

// Small skew is a no-op regardless of auth.
func TestTimeSync_SmallSkewNoOp(t *testing.T) {
	app, deps := setupTestApp()
	deps.TimeSyncSetTime = func(epochSec int64) error { t.Fatal("SetTime must not be called"); return nil }
	token, _, _ := deps.Auth.Login("admin")

	resp := postTimeSync(t, app, token, time.Now().UnixMilli())
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	_ = json.Unmarshal(body, &data)
	if synced, _ := data["synced"].(bool); synced {
		t.Error("expected synced=false for small skew")
	}
}

// Unauthenticated syncs are rate limited per IP.
func TestTimeSync_UnauthenticatedRateLimited(t *testing.T) {
	app, deps := setupTestApp()
	deps.TimeSyncMinPlausible = time.Now().Add(time.Hour)
	deps.TimeSyncSetTime = func(epochSec int64) error { return nil }
	deps.TimeSyncLimiter = auth.NewRateLimiter(1, time.Minute)

	resp1 := postTimeSync(t, app, "", time.Now().Add(2*time.Hour).UnixMilli())
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected first sync to pass, got %d", resp1.StatusCode)
	}

	resp2 := postTimeSync(t, app, "", time.Now().Add(2*time.Hour).UnixMilli())
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 for second unauthenticated sync, got %d", resp2.StatusCode)
	}
}
