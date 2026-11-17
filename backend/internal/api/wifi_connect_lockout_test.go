package api

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/network/networktest"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
	"github.com/openwrt-travel-gui/backend/internal/wifi"
)

func connectLockoutService(t *testing.T) (*wifi.Service, *uci.MockUCI) {
	t.Helper()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", networktest.DeviceInterfaceDump(t))
	ub.RegisterResponse("network.wireless.status", map[string]any{
		"radio1": map[string]any{
			"interfaces": []any{map[string]any{
				"ifname":  "phy1-ap0",
				"section": "default_radio1",
				"config":  map[string]any{"mode": "ap"},
			}},
		},
	})
	cmd := &platform.MockCommandRunner{RunFunc: networktest.FakeIw(map[string]string{
		"phy1-ap0": networktest.Phy1APStationDump,
	})}
	return wifi.New(wifi.Deps{
		UCI: u, Ubus: ub, Cmd: cmd, Applier: &stubApplier{token: "connect"},
		ARPFile: networktest.WriteTempFile(t, "arp", networktest.DeviceNeighborTable(t)),
	}), u
}

func connectApp(svc *wifi.Service) *fiber.App {
	app := fiber.New(fiber.Config{
		ProxyHeader:      fiber.HeaderXForwardedFor,
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0/0"}},
	})
	registerWifi(NewAPI(app), &Dependencies{Wifi: svc})
	return app
}

func postWithClientIP(t *testing.T, app *fiber.App, clientIP string, body any) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wifi/connect", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(fiber.HeaderXForwardedFor, clientIP)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("connect request: %v", err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	return resp, responseBody
}

func connectBody() map[string]any {
	return map[string]any{
		"ssid": "Cafe-WiFi", "band": "5g", "encryption": "psk2", "password": "cafepass1",
	}
}

func TestWifiConnect_RefusesWhenCallerUsesAPBeingDisabled(t *testing.T) {
	svc, u := connectLockoutService(t)
	before, err := u.GetSections("wireless")
	if err != nil {
		t.Fatal(err)
	}

	resp, body := postWithClientIP(t, connectApp(svc), networktest.DeviceWiFiClientIP, connectBody())

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("connect status = %d, want 409: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != wifi.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, wifi.LockoutErrorCode)
	}
	after, err := u.GetSections("wireless")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("refused connect wrote wireless config:\nbefore: %#v\nafter: %#v", before, after)
	}
}

func TestWifiConnect_AcknowledgementAllowsDisablingCallerAP(t *testing.T) {
	svc, u := connectLockoutService(t)
	body := connectBody()
	body["acknowledge_lockout"] = true

	resp, responseBody := postWithClientIP(t, connectApp(svc), networktest.DeviceWiFiClientIP, body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("acknowledged connect status = %d: %s", resp.StatusCode, responseBody)
	}
	if disabled, _ := u.Get("wireless", "default_radio1", "disabled"); disabled != "1" {
		t.Errorf("caller AP disabled = %q, want 1", disabled)
	}
	if disabled, _ := u.Get("wireless", "default_radio0", "disabled"); disabled == "1" {
		t.Error("the other band's AP must remain enabled")
	}
}

func TestWifiConnect_OtherBandDoesNotRequireAcknowledgement(t *testing.T) {
	svc, u := connectLockoutService(t)
	body := connectBody()
	body["band"] = "2.4ghz"

	resp, responseBody := postWithClientIP(t, connectApp(svc), networktest.DeviceWiFiClientIP, body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("other-band connect status = %d: %s", resp.StatusCode, responseBody)
	}
	if disabled, _ := u.Get("wireless", "default_radio0", "disabled"); disabled != "1" {
		t.Errorf("selected other-band AP disabled = %q, want 1", disabled)
	}
	if disabled, _ := u.Get("wireless", "default_radio1", "disabled"); disabled == "1" {
		t.Error("caller AP on the unselected band must remain enabled")
	}
}
