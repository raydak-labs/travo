package app

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/config"
)

func TestWSTicketRouteWorksThroughProductionMiddleware(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.AuthConfigPath = filepath.Join(t.TempDir(), "auth.json")
	app, lifecycle := mustNew(t, cfg)
	lifecycle.Stop()
	token := mockLogin(t, app)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/ws/ticket", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("ticket request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ticket status = %d, body: %s", resp.StatusCode, body)
	}
	var ticket struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &ticket); err != nil {
		t.Fatalf("decode ticket: %v", err)
	}
	if len(ticket.Ticket) != 64 || ticket.ExpiresIn != 30 {
		t.Errorf("unexpected ticket response: %+v", ticket)
	}
}
