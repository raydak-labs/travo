package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestNetworkStatusEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["wan"]; !ok {
		t.Error("expected wan in response")
	}
}

func TestSetWanConfig_InvalidType_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"type": "invalid",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"type":       "static",
		"ip_address": "not-an-ip",
		"gateway":    "192.168.1.1",
		"netmask":    "255.255.255.0",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"type": "dhcp",
		"mtu":  50000,
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestDetectWanTypeEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/wan/detect", nil)
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
	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["detected_type"]; !ok {
		t.Error("expected detected_type in response")
	}
	if _, ok := data["current_type"]; !ok {
		t.Error("expected current_type in response")
	}
}

func TestSetWanConfig_InvalidDNS_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"type":        "dhcp",
		"dns_servers": []string{"not-an-ip"},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"type": "dhcp",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/wan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestGetDHCPReservations_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/dhcp/reservations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAddDHCPReservation_ValidRequest_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"name": "laptop",
		"mac":  "AA:BB:CC:DD:EE:FF",
		"ip":   "192.168.8.50",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestAddDHCPReservation_MissingName_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"mac": "AA:BB:CC:DD:EE:FF",
		"ip":  "192.168.8.50",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"name": "laptop",
		"mac":  "invalid-mac",
		"ip":   "192.168.8.50",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"name": "laptop",
		"mac":  "AA:BB:CC:DD:EE:FF",
		"ip":   "not-an-ip",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	// First add a reservation
	body, _ := json.Marshal(map[string]any{
		"name": "laptop",
		"mac":  "AA:BB:CC:DD:EE:FF",
		"ip":   "192.168.8.50",
	})
	addReq, _ := http.NewRequest(http.MethodPost, "/api/v1/network/dhcp/reservations", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addReq.Header.Set("Authorization", "Bearer "+token)
	addResp, _ := app.Test(addReq, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	addResp.Body.Close()

	// Now delete it
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/network/dhcp/reservations/host_laptop", nil)
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

// The shared test app has no AP interface in its command runner, so a kick
// cannot succeed: the handler must surface that failure instead of returning a
// silent 200. The success path is covered in services.TestKickClient_SucceedsWhenAPInterfaceAccepts.
func TestKickClient_NoAPInterfaceReturns500(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"mac": "AA:BB:CC:DD:EE:FF",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/clients/kick", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 500 when no client was disassociated, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestKickClient_MissingMAC_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/clients/kick", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestKickClient_InvalidMAC_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"mac": "invalid-mac",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/clients/kick", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestBlockClient_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"mac": "AA:BB:CC:DD:EE:FF",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/clients/block", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestBlockClient_InvalidMAC_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"mac": "not-a-mac",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/clients/block", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestUnblockClient_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	// Block first
	blockBody, _ := json.Marshal(map[string]any{
		"mac": "AA:BB:CC:DD:EE:FF",
	})
	blockReq, _ := http.NewRequest(http.MethodPost, "/api/v1/network/clients/block", bytes.NewReader(blockBody))
	blockReq.Header.Set("Content-Type", "application/json")
	blockReq.Header.Set("Authorization", "Bearer "+token)
	blockResp, _ := app.Test(blockReq, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	blockResp.Body.Close()

	// Unblock
	body, _ := json.Marshal(map[string]any{
		"mac": "AA:BB:CC:DD:EE:FF",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/clients/unblock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestGetBlockedClients_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/clients/blocked", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var blocked []string
	if err := json.Unmarshal(body, &blocked); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked clients initially, got %d", len(blocked))
	}
}

func TestSetInterfaceState_Up_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{"up": true})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/interfaces/wan/state", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestSetInterfaceState_Down_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{"up": false})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/interfaces/lan/state", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestSetInterfaceState_InvalidInterface_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{"up": true})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/interfaces/invalid/state", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestSetInterfaceState_InvalidBody_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/interfaces/wan/state", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestGetDDNSConfig_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/ddns", nil)
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

func TestSetDDNSConfig_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled":     true,
		"service":     "duckdns.org",
		"domain":      "test.duckdns.org",
		"username":    "mytoken",
		"password":    "",
		"lookup_host": "test.duckdns.org",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/ddns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestSetDDNSConfig_MissingService_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled": true,
		"service": "",
		"domain":  "test.duckdns.org",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/ddns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestSetDDNSConfig_MissingDomain_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled": true,
		"service": "duckdns.org",
		"domain":  "",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/ddns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestSetDDNSConfig_Custom_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled":     true,
		"service":     "custom",
		"domain":      "router.example.com",
		"username":    "u",
		"password":    "p",
		"lookup_host": "router.example.com",
		"update_url":  "https://ddns.example.com/nic/update?hostname=[DOMAIN]&myip=[IP]",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/ddns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestSetDDNSConfig_Custom_MissingUpdateURL_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled":     true,
		"service":     "custom",
		"domain":      "router.example.com",
		"username":    "",
		"password":    "",
		"lookup_host": "router.example.com",
		"update_url":  "",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/ddns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestSetDDNSConfig_Custom_InvalidURL_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled":     true,
		"service":     "custom",
		"domain":      "router.example.com",
		"username":    "",
		"password":    "",
		"lookup_host": "router.example.com",
		"update_url":  "not-a-valid-url",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/ddns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestGetDDNSStatus_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/ddns/status", nil)
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

// Without ddns-scripts there is nothing that can service a `ddns` UCI config,
// and ddns is not in the service catalog so the UI cannot install it either. The
// endpoint used to answer 500 with `uci: Entry not found`, which named neither
// the cause nor a way out. It must answer 503 and name the package.
func TestSetDDNSConfig_MissingPackage_Returns503(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	deps.Network.SetDDNSInitScript(t.TempDir() + "/definitely-not-installed")

	body, _ := json.Marshal(map[string]any{
		"enabled": true,
		"service": "duckdns.org",
		"domain":  "test.duckdns.org",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/network/ddns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d, body: %s", resp.StatusCode, b)
	}
	if !strings.Contains(string(b), "ddns-scripts") {
		t.Errorf("response does not name the missing package: %s", b)
	}
}

// The UI needs to know up front that DDNS cannot be configured, otherwise it
// offers a form whose only possible outcome is the 503 above.
func TestGetDDNSConfig_ReportsAvailability(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	get := func() map[string]any {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/network/ddns", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	if avail, ok := get()["available"].(bool); !ok || avail != true {
		t.Errorf("with ddns-scripts present, available should be true, got %v", get()["available"])
	}
	deps.Network.SetDDNSInitScript(t.TempDir() + "/definitely-not-installed")
	if avail, ok := get()["available"].(bool); !ok || avail != false {
		t.Errorf("without ddns-scripts, available should be false, got %v", get()["available"])
	}
}

// The diagnostics endpoint passes the target to ping / traceroute / nslookup.
// A target that a tool would read as an option, or that is not a host at all,
// must be a 400 at the boundary — the service answered 200 with the tool's
// complaint in an "error" field.
func TestRunDiagnostics_ValidatesTypeAndTarget(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	post := func(body string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/network/diagnostics", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"unknown type", `{"type":"curl","target":"8.8.8.8"}`},
		{"empty type", `{"type":"","target":"8.8.8.8"}`},
		{"target starting with a dash", `{"type":"ping","target":"-f"}`},
		{"target with a shell metacharacter", `{"type":"ping","target":"8.8.8.8; reboot"}`},
		{"empty target", `{"type":"ping","target":""}`},
		{"target is a path", `{"type":"dns","target":"/etc/passwd"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code, body := post(tc.body); code != http.StatusBadRequest {
				t.Errorf("%s returned %d, want 400: %s", tc.body, code, body)
			}
		})
	}

	// A well-formed request still reaches the service.
	if code, body := post(`{"type":"ping","target":"8.8.8.8"}`); code != http.StatusOK {
		t.Errorf("a valid ping returned %d, want 200: %s", code, body)
	}
}

// PUT /network/clients/alias persists the alias per MAC. A body that does not
// match the documented shape used to be accepted and stored under an empty key.
func TestSetClientAlias_RejectsUnknownField(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	code, body := putJSON(t, app, token, "/api/v1/network/clients/alias",
		`{"mac":"aa:bb:cc:dd:ee:ff","nickname":"nope"}`)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 for an unknown field, got %d: %s", code, body)
	}
}
