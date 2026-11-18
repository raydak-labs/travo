package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/systemlog"
)

func logEnv(t *testing.T) (*env, string, string) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		d.SystemLog = &systemlog.Service{Log: d.Ubus.(systemlog.Reader)}
	})
	cookie, csrf := e.login()
	return e, cookie, csrf
}

func TestSystemLogQuery(t *testing.T) {
	e, cookie, csrf := logEnv(t)
	rec := e.do(req{method: "GET", path: "/api/v1/system/logs?lines=2&level=notice&filter=dropbear", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got systemlog.LogPage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 2 || !got.Truncated || got.Entries[0].Program != "dropbear" || got.Entries[0].Level != "notice" {
		t.Fatalf("%+v", got)
	}
}

func TestSystemLogRejectsBadQueries(t *testing.T) {
	e, cookie, csrf := logEnv(t)
	for _, q := range []string{"lines=0", "lines=1001", "level=loud", "lines=x"} {
		rec := e.do(req{method: "GET", path: "/api/v1/system/logs?" + q, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_request" {
			t.Errorf("%s: %d %s", q, rec.Code, rec.Body)
		}
	}
}

func TestSystemLogUnavailable(t *testing.T) {
	e, cookie, csrf := logEnv(t)
	e.dev.LogErr = errors.New("timeout")
	rec := e.do(req{method: "GET", path: "/api/v1/system/logs", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "logs_unavailable" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
