package api

import (
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// The speedtest-service endpoints are called by the frontend; they must be
// registered (they were once defined but never routed).
func TestSpeedtestServiceStatusRouteRegistered(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/speedtest-service", nil)
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
