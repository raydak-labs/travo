package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestWifiScanEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/scan", nil)
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
	var data []any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON array: %v", err)
	}
	if len(data) < 3 {
		t.Errorf("expected at least 3 scan results, got %d", len(data))
	}
}

func TestWifiConnectEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "Test-Net", "password": "test1234", "encryption": "psk2",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
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

func TestWifiConnect_EmptySSID_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "", "password": "longpassword", "encryption": "psk2",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
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
	b, _ := io.ReadAll(resp.Body)
	var data map[string]any
	_ = json.Unmarshal(b, &data)
	if _, ok := data["error"]; !ok {
		t.Error("expected error field in response")
	}
}

func TestWifiConnect_ShortPassword_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "TestNet", "password": "short", "encryption": "psk2",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
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

func TestWifiConnect_OpenNetworkNoPassword_Returns200(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "OpenNet", "password": "", "encryption": "none",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
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

func TestWifiConnect_NewSecuredWithoutPassword_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "Totally-New-SSID-For-400-Test", "password": "", "encryption": "psk2",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
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

func TestWifiConnect_NewWithoutEncryption_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{
		"ssid": "No-Enc-SSID-Api-Test", "password": "abcdefgh", "encryption": "",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", bytes.NewReader(body))
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

func TestWifiConnectionEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/connection", nil)
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
	if _, ok := data["ssid"]; !ok {
		t.Error("expected ssid in response")
	}
}

func TestWifiHealthEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/health", nil)
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
	if _, ok := data["status"]; !ok {
		t.Error("expected status in response")
	}
}

func TestWifiRepeaterReconcileEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/repeater/reconcile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}
}

func TestWifiDisconnectEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/disconnect", nil)
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
	if data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", data["status"])
	}
}

func TestWifiApplyConfirmEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]string{"token": "apply-123"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/apply/confirm", bytes.NewReader(body))
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

func TestWifiDeleteEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/wifi/saved/sta0", nil)
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

func TestWifiDeleteEndpoint_NonexistentSection(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/wifi/saved/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for nonexistent section, got %d", resp.StatusCode)
	}
}

func TestGuestWifiGetEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/guest", nil)
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
	if _, ok := data["enabled"]; !ok {
		t.Error("expected enabled field in response")
	}
}

func TestGuestWifiSetEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled": true, "ssid": "Guest-Net", "encryption": "psk2", "key": "guestpass123",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/guest", bytes.NewReader(body))
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

func TestGuestWifiSet_EmptySSID_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled": true, "ssid": "", "encryption": "psk2", "key": "guestpass123",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/guest", bytes.NewReader(body))
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

func TestGuestWifiSet_ShortPassword_Returns400(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"enabled": true, "ssid": "Guest", "encryption": "psk2", "key": "short",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/guest", bytes.NewReader(body))
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

func TestRadioStatusGetEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/radio", nil)
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
	if _, ok := data["enabled"]; !ok {
		t.Error("expected enabled field in response")
	}
}

func TestRadioStatusSetEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{"enabled": false})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/radio", bytes.NewReader(body))
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

func TestWifiSetPriorityEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"ssids": []string{"Hotel-WiFi", "Office-Net"},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/saved/priority", bytes.NewReader(body))
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
	b, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", data["status"])
	}
}

func TestWifiSetPriorityEndpoint_EmptySSIDs(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{
		"ssids": []string{},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/saved/priority", bytes.NewReader(body))
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

func TestGetAutoReconnectEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wifi/autoreconnect", nil)
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
	if _, ok := data["enabled"]; !ok {
		t.Error("expected enabled field in response")
	}
}

func TestSetAutoReconnectEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, _ := json.Marshal(map[string]any{"enabled": true})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/wifi/autoreconnect", bytes.NewReader(body))
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
	b, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", data["status"])
	}
}

func TestRandomizeMACEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/mac/randomize", nil)
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
	if data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", data["status"])
	}
	mac, ok := data["mac"].(string)
	if !ok || mac == "" {
		t.Error("expected mac field in response")
	}
}

// ---------------------------------------------------------------------------
// The same-radio refusal (services.ErrAPAndSTASameRadio) is a conflict with the
// device's radio layout, not a server failure: a 500 tells the operator Travo
// broke, and a client that retries on 5xx keeps asking for the impossible.
// ---------------------------------------------------------------------------

// uplinkSTAService returns a wifi service on multi-radio hardware in client
// mode: the STA section sta0 is enabled, bound to network=wwan and sitting on
// radio0, and no access point is enabled. Anything that enables an AP on radio0
// is the AP+STA-on-one-PHY state ADR 0002 §2 refuses.
func uplinkSTAService(t *testing.T) (*services.WifiService, *uci.MockUCI) {
	t.Helper()
	u := uci.NewMockUCI()
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "disabled", "1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	// A wired caller, so the lockout guard classifies these requests as ethernet
	// and leaves the radio-layout rule as the thing under test.
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", interfaceDump("eth0"))
	return services.NewWifiServiceWithApplier(u, ub, &stubApplier{token: "s"}), u
}

func putJSONRequest(
	t *testing.T, app *fiber.App, method, path string, body any,
) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestSetRadioRole_SameRadioRefusalIs409(t *testing.T) {
	app := fiber.New()
	svc, _ := uplinkSTAService(t)
	app.Put("/api/v1/wifi/radios/:name/role", SetRadioRoleHandler(svc))

	resp, body := putJSONRequest(t, app, http.MethodPut, "/api/v1/wifi/radios/radio0/role",
		map[string]string{"role": "both"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for an impossible radio layout, got %d: %s", resp.StatusCode, body)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON %s: %v", body, err)
	}
	msg, _ := result["error"].(string)
	if !strings.Contains(msg, "same radio") {
		t.Errorf("expected the message to name the problem, got %q", msg)
	}
}

// TestGuestWifiSet_FallsBackToTheOtherRadio pins the corrected behaviour.
//
// This used to assert 409: enabling a guest AP on the radio carrying the uplink
// STA was refused outright. But preferredGuestRadio picked the 2.4 GHz radio
// unconditionally, and on the archetypal travel router -- a client or repeater
// whose uplink STA sits on the 2.4 GHz band -- that made guest WiFi impossible
// to enable at all, with no way out except allow_ap_on_sta_radio, i.e.
// deliberately re-enabling the AP+STA-on-one-PHY state the guard exists to
// prevent.
//
// It now falls back to the other radio, exactly as the downlink AP does, so the
// refusal only applies when there is genuinely nowhere else to put the AP.
func TestGuestWifiSet_FallsBackToTheOtherRadio(t *testing.T) {
	app := fiber.New()
	svc, u := uplinkSTAService(t)
	app.Put("/api/v1/wifi/guest", SetGuestWifiHandler(svc))

	resp, body := putJSONRequest(t, app, http.MethodPut, "/api/v1/wifi/guest", map[string]any{
		"enabled": true, "ssid": "Guest-Travel", "encryption": "psk2", "key": "guestpass123",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected the guest AP to fall back to the non-uplink radio, got %d: %s",
			resp.StatusCode, body)
	}

	opts, err := u.GetAll("wireless", "guest")
	if err != nil {
		t.Fatalf("guest section was not created: %v", err)
	}
	// radio1 is the only radio that does not carry the enabled uplink STA.
	if got := opts["device"]; got != "radio1" {
		t.Errorf("guest AP device = %q, want radio1 (the radio without the uplink STA); "+
			"falling back to the 2.4 GHz uplink radio is what made guest WiFi unusable", got)
	}
	if got := opts["disabled"]; got != "0" {
		t.Errorf("guest AP disabled = %q, want 0", got)
	}
}

func TestSetAPConfig_SameRadioRefusalIs409(t *testing.T) {
	svc, u := uplinkSTAService(t)
	// default_radio0 is the AP on the uplink radio; the operator is enabling it.
	if err := u.Set("wireless", "default_radio0", "disabled", "1"); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))

	resp, body := putJSONRequest(t, app, http.MethodPut, "/api/v1/wifi/ap/default_radio0",
		map[string]any{
			"ssid": "OpenWrt-Travel", "encryption": "psk2", "key": "travelrouter", "enabled": true,
		})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 when enabling the AP would share the uplink radio, got %d: %s",
			resp.StatusCode, body)
	}
}

// The apply envelope tells the client how long one confirm call can block, so a
// client that re-POSTs confirm until the rollback deadline can leave that much
// room and never issue a probe that lands after rpcd rolled back.
func TestWifiMutationEnvelope_ReportsProbeBudget(t *testing.T) {
	app := fiber.New()
	svc, _ := uplinkSTAService(t)
	app.Put("/api/v1/wifi/mode", WifiSetModeHandler(svc))

	resp, body := putJSONRequest(t, app, http.MethodPut, "/api/v1/wifi/mode",
		map[string]string{"mode": "ap"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var result struct {
		Apply struct {
			Token              string `json:"token"`
			ProbeBudgetSeconds int    `json:"probe_budget_seconds"`
		} `json:"apply"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON %s: %v", body, err)
	}
	if result.Apply.Token == "" {
		t.Fatalf("expected a pending apply, got %s", body)
	}
	if result.Apply.ProbeBudgetSeconds <= 0 {
		t.Fatalf("expected a positive probe_budget_seconds, got %d", result.Apply.ProbeBudgetSeconds)
	}
}
