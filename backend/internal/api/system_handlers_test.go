package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSystemInfoEndpoint(t *testing.T) {
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
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["hostname"]; !ok {
		t.Error("expected hostname in response")
	}
}

func TestSystemStatsEndpoint(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["cpu"]; !ok {
		t.Error("expected cpu in response")
	}
}

func TestReboot_ReturnsOk(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/reboot", nil)
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
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") {
		t.Errorf("expected ok in response, got: %s", body)
	}
}
