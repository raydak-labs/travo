package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/wifireconnect"
)

type wifiReconnectResult struct {
	status wifireconnect.Status
	err    error
}

func (f *wifiReconnectResult) Status(context.Context) (wifireconnect.Status, error) {
	return f.status, f.err
}

func TestWiFiReconnectRoute(t *testing.T) {
	const path = "/api/v1/wifi/reconnect"
	reader := &wifiReconnectResult{status: wifireconnect.Status{State: wifireconnect.StateExhausted, Attempts: 5, MaxAttempts: 5}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.WiFiReconnect = reader })
	if got := e.do(req{path: path}); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d %s", got.Code, got.Body)
	}
	cookie, _ := e.login()
	got := e.do(req{path: path, cookie: cookie})
	if got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != `{"state":"exhausted","attempts":5,"max_attempts":5}` {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	reader.err = wifireconnect.ErrUnavailable
	got = e.do(req{path: path, cookie: cookie})
	if got.Code != http.StatusServiceUnavailable || errCode(t, got) != "wifi_reconnect_unavailable" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}

func TestWiFiReconnectNilDependency(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, _ := e.login()
	got := e.do(req{path: "/api/v1/wifi/reconnect", cookie: cookie})
	if got.Code != http.StatusServiceUnavailable || errCode(t, got) != "device_unavailable" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}
