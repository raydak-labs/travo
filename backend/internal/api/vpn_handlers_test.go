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
