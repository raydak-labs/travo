package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestLoginResponseIncludesExpiresIn(t *testing.T) {
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
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
	respBody, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	expiresIn, ok := data["expires_in"].(float64)
	if !ok {
		t.Fatalf("expected numeric expires_in in login response, got: %s", respBody)
	}
	if expiresIn < 86000 || expiresIn > 86400 {
		t.Errorf("expected expires_in close to 86400 seconds, got %v", expiresIn)
	}
}

func TestSessionEndpointReturnsExpiresIn(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
	respBody, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if valid, _ := data["valid"].(bool); !valid {
		t.Errorf("expected valid=true, got: %s", respBody)
	}
	expiresIn, ok := data["expires_in"].(float64)
	if !ok {
		t.Fatalf("expected numeric expires_in in session response, got: %s", respBody)
	}
	if expiresIn <= 0 {
		t.Errorf("expected positive expires_in, got %v", expiresIn)
	}
}
