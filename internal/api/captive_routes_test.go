package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/captive"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

type fakeCaptive struct {
	err      error
	calls    []string
	status   captive.BypassStatus
	statuses int
}

func (f *fakeCaptive) Enable(context.Context) error {
	f.calls = append(f.calls, "enable")
	return f.err
}
func (f *fakeCaptive) Restore(context.Context) error {
	f.calls = append(f.calls, "restore")
	return f.err
}
func (f *fakeCaptive) Status() captive.BypassStatus { f.statuses++; return f.status }

func TestCaptiveDNSRoutes(t *testing.T) {
	fake := &fakeCaptive{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Captive = fake })
	for _, path := range []string{"/api/v1/captive/dns-bypass", "/api/v1/captive/dns-restore"} {
		if got := e.do(req{method: "POST", path: path}); got.Code != 401 {
			t.Fatal(path, got.Code)
		}
	}
	cookie, csrf := e.login()
	if got := e.do(req{method: "POST", path: "/api/v1/captive/dns-bypass", cookie: cookie}); got.Code != 403 || len(fake.calls) != 0 {
		t.Fatal("missing CSRF", got.Code)
	}
	for path, call := range map[string]string{"/api/v1/captive/dns-bypass": "enable", "/api/v1/captive/dns-restore": "restore"} {
		fake.calls, fake.err = nil, nil
		got := e.do(req{method: "POST", path: path, cookie: cookie, csrf: csrf})
		if got.Code != 200 || bodyField(t, got, "status") != "ok" || len(fake.calls) != 1 || fake.calls[0] != call {
			t.Fatal(path, got.Code, got.Body, fake.calls)
		}
	}
	for _, tc := range []struct {
		err  error
		code int
		want string
	}{
		{captive.ErrNoUplink, 409, "captive_no_uplink"},
		{fmt.Errorf("wrapped: %w", captive.ErrNoUplinkDNS), 409, "captive_no_uplink_dns"},
		{fmt.Errorf("dns layers: %w", guard.ErrActive), 409, "dns_change_pending"},
		{fmt.Errorf("dns layers: %w", dnslayer.ErrForeignStaged), 409, "dns_foreign_staged"},
		{errors.New("apply failed (guard kept)"), 503, "captive_dns_failed"},
	} {
		fake.err = tc.err
		got := e.do(req{method: "POST", path: "/api/v1/captive/dns-restore", cookie: cookie, csrf: csrf})
		if got.Code != tc.code || bodyField(t, got, "code") != tc.want {
			t.Fatal(tc.err, got.Code, got.Body)
		}
	}
}

func TestCaptiveUnavailableAndDashboardState(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, csrf := e.login()
	if got := e.do(req{method: "POST", path: "/api/v1/captive/dns-bypass", cookie: cookie, csrf: csrf}); got.Code != 503 || bodyField(t, got, "code") != "captive_unavailable" {
		t.Fatal(got.Code, got.Body)
	}
	left := 42
	fake := &fakeCaptive{status: captive.BypassStatus{DNSBypass: true, RestoreInSeconds: &left}}
	snapshot := &connectivitySnapshot{Internet: connectivity.Captive}
	e = newEnvWithDashboard(t, nil, nil, snapshot, func(d *Deps) { d.Captive = fake })
	cookie, _ = e.login()
	got := e.do(req{method: "GET", path: "/api/v1/dashboard/connectivity", cookie: cookie})
	var body map[string]json.RawMessage
	if got.Code != 200 || json.Unmarshal(got.Body.Bytes(), &body) != nil {
		t.Fatal(got.Code, got.Body)
	}
	if string(body["internet"]) != `"captive"` || string(body["captive"]) != `{"dns_bypass":true,"restore_in_seconds":42,"pending":false}` {
		t.Fatalf("body %s", got.Body)
	}
	if len(fake.calls) != 0 {
		t.Fatal("GET mutated", fake.calls)
	}
}
