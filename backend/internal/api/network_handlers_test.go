package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestNetworkStatusEndpoint(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/status", nil)
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
	if _, ok := data["wan"]; !ok {
		t.Error("expected wan in response")
	}
}

func TestSetWanConfig_InvalidType_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"type": "invalid",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
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

func TestSetWanConfig_InvalidIP_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"type":       "static",
		"ip_address": "not-an-ip",
		"gateway":    "192.168.1.1",
		"netmask":    "255.255.255.0",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
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

func TestSetWanConfig_InvalidMTU_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"type": "dhcp",
		"mtu":  50000,
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
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

func TestSetWanConfig_InvalidDNS_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"type":        "dhcp",
		"dns_servers": []string{"not-an-ip"},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
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

func TestSetWanConfig_ValidDHCP_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"type": "dhcp",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
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

func TestGetDHCPReservations_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/dhcp/reservations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAddDHCPReservation_ValidRequest_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"name": "laptop",
		"mac":  "AA:BB:CC:DD:EE:FF",
		"ip":   "192.168.8.50",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
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

func TestAddDHCPReservation_MissingName_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"mac": "AA:BB:CC:DD:EE:FF",
		"ip":  "192.168.8.50",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
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

func TestAddDHCPReservation_InvalidMAC_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"name": "laptop",
		"mac":  "invalid-mac",
		"ip":   "192.168.8.50",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
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

func TestAddDHCPReservation_InvalidIP_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"name": "laptop",
		"mac":  "AA:BB:CC:DD:EE:FF",
		"ip":   "not-an-ip",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
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

func TestDeleteDHCPReservation_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	// First add a reservation
	body, _ := json.Marshal(map[string]interface{}{
		"name": "laptop",
		"mac":  "AA:BB:CC:DD:EE:FF",
		"ip":   "192.168.8.50",
	})
	addReq, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addReq.Header.Set("Authorization", "Bearer "+token)
	addResp, _ := app.Test(addReq, -1)
	addResp.Body.Close()

	// Now delete it
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/network/dhcp/reservations/host_laptop", nil)
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
