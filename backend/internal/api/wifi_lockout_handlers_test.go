package api

import (
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

// The interface every test request appears to arrive on under app.Test.
//
// app.Test's in-memory connection always reports 0.0.0.0 as the peer address
// (fiber's testConn), so a test cannot choose its own source IP the way a real
// phone does. What it CAN do is choose which interface that address is reachable
// through, by registering the `network.interface dump` answer the router really
// returns. That is what these two helpers do: same request, same 0.0.0.0 caller,
// a dump that classifies it as a WiFi client or as a wired one. The rule built
// on top of that classification is pinned on real addresses in
// internal/services/wifi_lockout_test.go.
// interfaceDump answers `ubus call network.interface dump` with one up L3 device
// carrying prefix 0.0.0.0/0, which is exactly the shape
// classifyClientConnection reads. The device name is what decides the method:
// br-lan -> wifi-ap, eth0 -> ethernet.
func interfaceDump(l3Device string) map[string]any {
	return map[string]any{
		"interface": []any{map[string]any{
			"interface":   true,
			"up":          true,
			"device":      l3Device,
			"l3_device":   l3Device,
			"ipv4-prefix": []any{map[string]any{"address": "0.0.0.0/0"}},
		}},
	}
}

func lockoutService(t *testing.T, reachableBy string) *services.WifiService {
	t.Helper()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", interfaceDump(reachableBy))
	return services.NewWifiServiceWithApplier(u, ub, &stubApplier{token: "lockout"})
}

func putFromIP(t *testing.T, app *fiber.App, path string, body any) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, path, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

// codeOf reads the machine-readable code out of an error body.
func codeOf(t *testing.T, body []byte) string {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON %s: %v", body, err)
	}
	code, _ := result["code"].(string)
	return code
}

func TestSetMode_LockoutRefusalIs409WithACode(t *testing.T) {
	app := fiber.New()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", interfaceDump("br-lan"))
	svc := services.NewWifiServiceWithApplier(u, ub, &stubApplier{token: "lockout"})
	app.Put("/api/v1/wifi/mode", WifiSetModeHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/mode", map[string]any{"mode": "client"})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	msg, _ := result["error"].(string)
	// The message has to name the remedy, or the operator does not know what to do.
	if !strings.Contains(msg, "Ethernet") {
		t.Errorf("error message does not name the remedy: %q", msg)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v == "1" {
		t.Error("refused mode switch disabled an access point")
	}
}

func TestSetMode_LockoutAcknowledgementIsAccepted(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/mode", WifiSetModeHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/mode",
		map[string]any{"mode": "client", "acknowledge_lockout": true})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for an acknowledged switch, got %d: %s", resp.StatusCode, body)
	}
}

func TestSetMode_WiredCallerIsNotRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "eth0")
	app.Put("/api/v1/wifi/mode", WifiSetModeHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/mode", map[string]any{"mode": "client"})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for a wired operator, got %d: %s", resp.StatusCode, body)
	}
}

func TestSetAPConfig_DisablingTheLastAccessPointIsRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))

	// The two default access points are the only ones, so taking the first down
	// still leaves the second: that one must go through.
	resp, body := putFromIP(t, app, "/api/v1/wifi/ap/default_radio0",
		map[string]any{"ssid": "OpenWrt-Travel", "enabled": false})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 while another access point stays up, got %d: %s",
			resp.StatusCode, body)
	}

	resp, body = putFromIP(t, app, "/api/v1/wifi/ap/default_radio1",
		map[string]any{"ssid": "OpenWrt-Travel-5G", "enabled": false})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the last access point, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

func TestSetAPConfig_SSIDChangeOverWiFiIsNotRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/ap/default_radio0",
		map[string]any{"ssid": "Renamed", "encryption": "psk2", "key": "travelrouter"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for a rename over WiFi, got %d: %s", resp.StatusCode, body)
	}
}

func TestSetRadioEnabled_TurningTheRadiosOffIsRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/radio", SetRadioEnabledHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/radio", map[string]any{"enabled": false})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 when the radios go off, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

func TestSetRadioRole_TakingTheLastRadioOffIsRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/radios/:name/role", SetRadioRoleHandler(svc))

	// radio0 hosts the only other access point, so switching radio1 off is safe.
	resp, body := putFromIP(t, app, "/api/v1/wifi/radios/radio1/role",
		map[string]any{"role": "none"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 while radio0's access point stays up, got %d: %s",
			resp.StatusCode, body)
	}

	resp, body = putFromIP(t, app, "/api/v1/wifi/radios/radio0/role",
		map[string]any{"role": "none"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the last radio, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

func TestSetGuestWifi_DisablingTheOnlyAccessPointIsRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))
	app.Put("/api/v1/wifi/guest", SetGuestWifiHandler(svc))

	// The two default access points go first (acknowledged): once they are down
	// guest WiFi is the only access point left, and taking it down is the lockout.
	for _, section := range []string{"default_radio0", "default_radio1"} {
		resp, body := putFromIP(t, app, "/api/v1/wifi/ap/"+section, map[string]any{
			"ssid": section, "enabled": false, "acknowledge_lockout": true})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("disabling %s: %d %s", section, resp.StatusCode, body)
		}
	}
	resp, body := putFromIP(t, app, "/api/v1/wifi/guest", map[string]any{
		"enabled": true, "ssid": "Guest", "encryption": "psk2", "key": "guestpass"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enabling guest WiFi: %d %s", resp.StatusCode, body)
	}

	resp, body = putFromIP(t, app, "/api/v1/wifi/guest", map[string]any{"enabled": false})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the last access point, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

// A 409 that is NOT the lockout refusal must not carry the lockout code, or the
// frontend would raise the acknowledge dialog for an unrelated conflict.
func TestSetRadioRole_SameRadioConflictIsNotReportedAsLockout(t *testing.T) {
	app := fiber.New()
	u := uci.NewMockUCI()
	// An enabled uplink STA on radio0 and an access point still up on radio1, so
	// the request is refused for the radio layout, not for the lockout: an
	// access point would remain.
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", interfaceDump("br-lan"))
	svc := services.NewWifiServiceWithApplier(u, ub, &stubApplier{token: "lockout"})
	app.Put("/api/v1/wifi/radios/:name/role", SetRadioRoleHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/radios/radio0/role",
		map[string]any{"role": "both"})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the same-radio conflict, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got == services.LockoutErrorCode {
		t.Error("the same-radio conflict must not be reported as a lockout")
	}
}
