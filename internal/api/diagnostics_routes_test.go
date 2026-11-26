package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/diagnostics"
)

func diagnosticsEnv(t *testing.T, d Diagnostics) (*env, string, string) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.Diagnostics = d })
	cookie, csrf := e.login()
	return e, cookie, csrf
}

func TestDiagnosticsRunsThroughTheService(t *testing.T) {
	e, cookie, csrf := diagnosticsEnv(t, &diagnostics.Service{Device: mock.New()})
	rec := e.do(req{method: "POST", path: "/api/v1/network/diagnostics", body: `{"tool":"lookup","target":"localhost"}`, cookie: cookie, csrf: csrf})
	var got diagnostics.Diagnosis
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || !got.OK || got.Tool != "lookup" || !strings.Contains(got.Output, "Name:\tlocalhost") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, body := range []string{
		`{"tool":"ping","target":"-c100"}`,
		`{"tool":"ping","target":"a;reboot"}`,
	} {
		rec = e.do(req{method: "POST", path: "/api/v1/network/diagnostics", body: body, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_target") {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body)
		}
	}
	for _, body := range []string{`{"tool":"nmap","target":"x"}`, `{"tool":"ping","target":"x","extra":1}`, `{"tool":"ping","target":""}`} {
		rec = e.do(req{method: "POST", path: "/api/v1/network/diagnostics", body: body, cookie: cookie, csrf: csrf})
		if rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("schema: %s: %d %s", body, rec.Code, rec.Body)
		}
	}
}

type busyDiagnostics struct{}

func (busyDiagnostics) Run(context.Context, string, string) (diagnostics.Diagnosis, error) {
	return diagnostics.Diagnosis{}, diagnostics.ErrBusy
}

func TestDiagnosticsBusyRateLimitedAndUnavailable(t *testing.T) {
	e, cookie, csrf := diagnosticsEnv(t, busyDiagnostics{})
	body := `{"tool":"ping","target":"10.0.1.1"}`
	rec := e.do(req{method: "POST", path: "/api/v1/network/diagnostics", body: body, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "diagnostics_busy") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	limited := false
	for range 10 {
		rec = e.do(req{method: "POST", path: "/api/v1/network/diagnostics", body: body, cookie: cookie, csrf: csrf})
		limited = limited || rec.Code == http.StatusTooManyRequests
	}
	if !limited {
		t.Fatal("never rate-limited")
	}
	e, cookie, csrf = diagnosticsEnv(t, nil)
	if rec := e.do(req{method: "POST", path: "/api/v1/network/diagnostics", body: body, cookie: cookie, csrf: csrf}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil service: %d", rec.Code)
	}
}
