package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetBandSwitchingHandler(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/band-switching", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["config"]; !ok {
		t.Error("expected 'config' field in response")
	}
	if _, ok := data["status"]; !ok {
		t.Error("expected 'status' field in response")
	}
}

func TestSetBandSwitchingHandler(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	payload := map[string]any{
		"enabled":                   true,
		"preferred_band":            "5g",
		"check_interval_sec":        10,
		"down_switch_threshold_dbm": -70,
		"down_switch_delay_sec":     30,
		"up_switch_threshold_dbm":   -60,
		"up_switch_delay_sec":       60,
		"min_viable_signal_dbm":     -80,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/band-switching", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	b, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", result["status"])
	}
}

func TestSetBandSwitchingHandler_InvalidBody(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/band-switching", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetRadioRoleHandler(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	payload := map[string]string{"role": "ap"}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/radios/radio0/role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	// The mock UCI will return an error (no radio0 section), so we expect 500 — but
	// crucially we must get a valid JSON error response, not a panic or 404.
	if resp.StatusCode == http.StatusNotFound {
		t.Error("expected 500 (service error) or 200, not 404 — route not registered?")
	}
	b, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("expected JSON response, got: %s", b)
	}
}

func TestSetRadioRoleHandler_InvalidRole(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	payload := map[string]string{"role": "invalid-role"}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/radios/radio0/role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 500 for invalid role, got %d: %s", resp.StatusCode, b)
	}
	b, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("expected JSON: %s", b)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected 'error' field in response")
	}
}

// stubApplier stands in for the rpcd apply/confirm applier, which only exists in
// production wiring (the shared test app configures no applier).
type stubApplier struct {
	token string
}

func (s *stubApplier) StartApply(configs []string) (string, error) { return s.token, nil }

func (s *stubApplier) Confirm(sessionID string) error { return nil }

func (s *stubApplier) ApplyAndConfirm(configs []string) error { return nil }

// The radio-role mutator must answer with the same envelope as every other
// wireless mutator: the client reads response.apply and, when it is pending,
// calls confirmWifiApply. A raw WirelessApplyResult ({"Token": ...}) leaves the
// client with no apply state, so it never confirms and rpcd's 30 s rollback
// reverts every role change. The invented WPA passphrase also has to travel with
// it, or the operator never learns the key of the access point Travo just
// created for them.
func TestSetRadioRoleHandler_ReturnsApplyEnvelopeAndGeneratedKey(t *testing.T) {
	u := uci.NewMockUCI()
	// No AP section on radio0, so the service has to create one and invent a key.
	if err := u.DeleteSection("wireless", "default_radio0"); err != nil {
		t.Fatal(err)
	}
	svc := services.NewWifiServiceWithApplier(u, ubus.NewMockUbus(),
		&stubApplier{token: "role-session-1"})

	app := fiber.New()
	app.Put("/api/v1/wifi/radios/:name/role", SetRadioRoleHandler(svc))

	body, _ := json.Marshal(map[string]string{"role": "ap"})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/radios/radio0/role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	b, _ := io.ReadAll(resp.Body)
	var result struct {
		Status    string `json:"status"`
		Generated string `json:"generated_key"`
		Apply     *struct {
			Pending                bool   `json:"pending"`
			Token                  string `json:"token"`
			RollbackTimeoutSeconds int    `json:"rollback_timeout_seconds"`
		} `json:"apply"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("invalid JSON %s: %v", b, err)
	}
	if result.Status != "ok" {
		t.Errorf("expected status=ok, got %q", result.Status)
	}
	if result.Apply == nil {
		t.Fatalf("expected the shared apply envelope in the response, got %s", b)
	}
	if !result.Apply.Pending || result.Apply.Token != "role-session-1" {
		t.Errorf("expected a pending apply for role-session-1, got %+v", result.Apply)
	}
	if result.Apply.RollbackTimeoutSeconds != 30 {
		t.Errorf("expected rollback_timeout_seconds=30, got %d", result.Apply.RollbackTimeoutSeconds)
	}
	key, _ := u.Get("wireless", "ap_radio0", "key")
	if result.Generated != key {
		t.Errorf("expected generated_key %q (the created AP's passphrase), got %q", key, result.Generated)
	}
	if len(result.Generated) < 8 {
		t.Errorf("generated_key must be a usable WPA passphrase, got %q", result.Generated)
	}
}
