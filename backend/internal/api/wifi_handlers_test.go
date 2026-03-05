package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestWifiScanEndpoint(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/scan", nil)
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
	var data []interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON array: %v", err)
	}
	if len(data) < 3 {
		t.Errorf("expected at least 3 scan results, got %d", len(data))
	}
}

func TestWifiConnectEndpoint(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "Test-Net", "password": "test1234", "encryption": "psk2",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestWifiConnect_EmptySSID_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "", "password": "longpassword", "encryption": "psk2",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
	b, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	_ = json.Unmarshal(b, &data)
	if _, ok := data["error"]; !ok {
		t.Error("expected error field in response")
	}
}

func TestWifiConnect_ShortPassword_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "TestNet", "password": "short", "encryption": "psk2",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestWifiConnect_OpenNetworkNoPassword_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "OpenNet", "password": "", "encryption": "none",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestWifiConnectionEndpoint(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/connection", nil)
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
	if _, ok := data["ssid"]; !ok {
		t.Error("expected ssid in response")
	}
}
