package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/ifrestart"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
)

type fakeRestarter struct {
	err error
	req ifrestart.Request
}

func (f *fakeRestarter) Restart(_ context.Context, req ifrestart.Request) (ifrestart.Outcome, error) {
	f.req = req
	return ifrestart.Outcome{Interface: req.Name, Up: true, ElapsedSeconds: 7}, f.err
}

func restartEnv(t *testing.T, f InterfaceRestarter) (*env, string, string) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.InterfaceRestart = f })
	cookie, csrf := e.login()
	return e, cookie, csrf
}

func TestInterfaceRestartPassesCallerAndAcknowledgement(t *testing.T) {
	f := &fakeRestarter{}
	e, cookie, csrf := restartEnv(t, f)
	rec := e.do(req{method: "POST", path: "/api/v1/network/interfaces/wwan/restart", body: `{"acknowledge_lockout":true}`, cookie: cookie, csrf: csrf})
	var got ifrestart.Outcome
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || !got.Up || got.ElapsedSeconds != 7 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if f.req.Name != "wwan" || !f.req.AcknowledgeLockout || !f.req.Caller.IsValid() {
		t.Fatalf("%+v", f.req)
	}
	rec = e.do(req{method: "POST", path: "/api/v1/network/interfaces/wan/restart", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || f.req.Name != "wan" || f.req.AcknowledgeLockout {
		t.Fatalf("no body: %d %s %+v", rec.Code, rec.Body, f.req)
	}
	rec = e.do(req{method: "POST", path: "/api/v1/network/interfaces/lan/restart", cookie: cookie, csrf: csrf})
	if rec.Code < 400 || rec.Code >= 500 || f.req.Name == "lan" {
		t.Fatalf("lan must be refused before the service: %d %s", rec.Code, rec.Body)
	}
}

func TestInterfaceRestartErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{ifrestart.ErrNotFound, 404, "interface_not_found"},
		{ifrestart.ErrTxnPending, 409, "transaction_pending"},
		{ifrestart.ErrBusy, 409, "interface_restart_busy"},
		{lockout.ErrLockoutRisk, 409, "wifi_lockout_risk"},
		{ifrestart.ErrFailed, 503, "interface_restart_failed"},
		{ifrestart.ErrUnavailable, 503, "interface_restart_unavailable"},
	} {
		e, cookie, csrf := restartEnv(t, &fakeRestarter{err: tc.err})
		rec := e.do(req{method: "POST", path: "/api/v1/network/interfaces/wwan/restart", cookie: cookie, csrf: csrf})
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%v: %d %s", tc.err, rec.Code, rec.Body)
		}
	}
	e, cookie, csrf := restartEnv(t, nil)
	if rec := e.do(req{method: "POST", path: "/api/v1/network/interfaces/wwan/restart", cookie: cookie, csrf: csrf}); rec.Code != 503 {
		t.Fatalf("nil service: %d", rec.Code)
	}
}
