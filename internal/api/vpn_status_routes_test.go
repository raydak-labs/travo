package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

type vpnStatusResult struct {
	status vpnstatus.TunnelStatus
	err    error
}

func (f *vpnStatusResult) Status(context.Context) (vpnstatus.TunnelStatus, error) {
	return f.status, f.err
}

func TestVPNStatusRoutes(t *testing.T) {
	const path = "/api/v1/vpn/status"
	observed := time.Date(2026, time.November, 13, 22, 47, 16, 0, time.UTC)
	age := int64(14)
	reader := &vpnStatusResult{status: vpnstatus.TunnelStatus{State: vpnstatus.Connected, ObservedAt: observed, HandshakeAgeSeconds: &age}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.VPNStatus = reader })
	if got := e.do(req{path: path}); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d %s", got.Code, got.Body)
	}
	cookie, _ := e.login()
	for _, tc := range []struct {
		name   string
		status vpnstatus.TunnelStatus
		body   string
	}{
		{
			name:   "connected",
			status: vpnstatus.TunnelStatus{State: vpnstatus.Connected, ObservedAt: observed, HandshakeAgeSeconds: &age},
			body:   `{"state":"connected","stale":false,"observed_at":"2026-11-13T22:47:16Z","handshake_age_seconds":14}`,
		},
		{
			name:   "null age",
			status: vpnstatus.TunnelStatus{State: vpnstatus.NoHandshake, ObservedAt: observed},
			body:   `{"state":"no_handshake","stale":false,"observed_at":"2026-11-13T22:47:16Z","handshake_age_seconds":null}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader.status = tc.status
			got := e.do(req{path: path, cookie: cookie})
			if got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != tc.body {
				t.Fatalf("%d %s", got.Code, got.Body)
			}
			if strings.Contains(got.Body.String(), "KgTUh3") || strings.Contains(got.Body.String(), "endpoint") {
				t.Fatalf("response leaks peer data: %s", got.Body)
			}
		})
	}
	for _, err := range []error{vpnstatus.ErrUnavailable, errors.New("read failed")} {
		reader.err = err
		got := e.do(req{path: path, cookie: cookie})
		if got.Code != http.StatusServiceUnavailable || errCode(t, got) != "vpn_status_unavailable" {
			t.Fatalf("%d %s", got.Code, got.Body)
		}
	}
}

func TestVPNStatusSchemaName(t *testing.T) {
	rec := newEnv(t).do(req{path: "/api/openapi.json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var spec struct {
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if _, ok := spec.Components.Schemas["TunnelStatus"]; !ok {
		t.Fatal("TunnelStatus schema missing")
	}
	if !strings.Contains(rec.Body.String(), `"$ref":"#/components/schemas/TunnelStatus"`) {
		t.Fatal("VPN status response does not reference TunnelStatus")
	}
}

func TestVPNStatusNilDependency(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, _ := e.login()
	got := e.do(req{path: "/api/v1/vpn/status", cookie: cookie})
	if got.Code != http.StatusServiceUnavailable || errCode(t, got) != "vpn_status_unavailable" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}
