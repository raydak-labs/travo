package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestSetWireguard_InvalidPrivateKey_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"private_key": "not-a-valid-key",
		"address":     "10.0.0.2/32",
		"peers":       []interface{}{},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/vpn/wireguard", bytes.NewReader(body))
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

func TestSetWireguard_InvalidEndpoint_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"private_key": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
		"address":     "10.0.0.2/32",
		"peers": []map[string]interface{}{
			{
				"public_key":  "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
				"endpoint":    "no-port",
				"allowed_ips": []string{"0.0.0.0/0"},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/vpn/wireguard", bytes.NewReader(body))
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

func TestSetWireguard_InvalidAllowedIPs_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"private_key": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
		"address":     "10.0.0.2/32",
		"peers": []map[string]interface{}{
			{
				"public_key":  "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
				"endpoint":    "vpn.example.com:51820",
				"allowed_ips": []string{"not-cidr"},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/vpn/wireguard", bytes.NewReader(body))
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

func TestSetWireguard_ValidConfig_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]interface{}{
		"private_key": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
		"address":     "10.0.0.2/32",
		"dns":         []string{"1.1.1.1"},
		"peers": []map[string]interface{}{
			{
				"public_key":  "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
				"endpoint":    "vpn.example.com:51820",
				"allowed_ips": []string{"0.0.0.0/0"},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/vpn/wireguard", bytes.NewReader(body))
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

func TestGetWireguardStatus_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/vpn/wireguard/status", nil)
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

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["interface"] != "wg0" {
		t.Errorf("expected interface wg0, got %v", result["interface"])
	}
	if result["public_key"] != "PUB_KEY" {
		t.Errorf("expected public key PUB_KEY, got %v", result["public_key"])
	}
	peers, ok := result["peers"].([]interface{})
	if !ok || len(peers) != 1 {
		t.Errorf("expected 1 peer, got %v", result["peers"])
	}
}

func TestGetWireguardProfiles_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/vpn/wireguard/profiles", nil)
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

func TestToggleWireguard_EnabledField_TogglesOn(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]bool{"enabled": true})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vpn/wireguard/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}

	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/vpn/status", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200, got %d, body: %s", resp2.StatusCode, b)
	}
	var statuses []map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&statuses); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	found := false
	for _, s := range statuses {
		if s["type"] == "wireguard" {
			found = true
			if s["enabled"] != true {
				t.Errorf("expected wireguard enabled=true, got %v", s["enabled"])
			}
		}
	}
	if !found {
		t.Fatalf("expected wireguard status entry, got %v", statuses)
	}
}

func TestToggleWireguard_EnableField_BackwardCompat_TogglesOn(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]bool{"enable": true})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vpn/wireguard/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}

	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/vpn/status", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200, got %d, body: %s", resp2.StatusCode, b)
	}
	var statuses []map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&statuses); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	found := false
	for _, s := range statuses {
		if s["type"] == "wireguard" {
			found = true
			if s["enabled"] != true {
				t.Errorf("expected wireguard enabled=true, got %v", s["enabled"])
			}
		}
	}
	if !found {
		t.Fatalf("expected wireguard status entry, got %v", statuses)
	}
}

func TestAddWireguardProfile_Returns201(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"name":   "Test VPN",
		"config": "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vpn/wireguard/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 201, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestAddWireguardProfile_MissingName_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"config": "[Interface]\nPrivateKey = test\n",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vpn/wireguard/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAddWireguardProfile_MissingConfig_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"name": "Test",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vpn/wireguard/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDeleteWireguardProfile_NotFound_Returns404(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/vpn/wireguard/profiles/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 404, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestActivateWireguardProfile_NotFound_Returns400(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/vpn/wireguard/profiles/nonexistent/activate", nil)
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

func TestGetKillSwitch_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/vpn/killswitch", nil)
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
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["enabled"] != false {
		t.Errorf("expected enabled=false by default, got %v", result["enabled"])
	}
}

func TestSetKillSwitch_Enable_Returns200(t *testing.T) {
	app, deps := setupTestApp()
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]bool{"enabled": true})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/vpn/killswitch", bytes.NewReader(body))
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

	// Verify it was enabled
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/vpn/killswitch", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, _ := app.Test(req2, -1)
	defer resp2.Body.Close()
	var result map[string]interface{}
	_ = json.NewDecoder(resp2.Body).Decode(&result)
	if result["enabled"] != true {
		t.Errorf("expected enabled=true after setting, got %v", result["enabled"])
	}
}
