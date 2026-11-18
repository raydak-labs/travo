package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/wifischedule"
)

type fakeSchedule struct {
	rules wifischedule.Rules
	err   error
}

func (f *fakeSchedule) Get(context.Context) (wifischedule.Rules, error) { return f.rules, nil }
func (f *fakeSchedule) Put(_ context.Context, r wifischedule.Rules) (wifischedule.Rules, error) {
	if f.err != nil {
		return wifischedule.Rules{}, f.err
	}
	f.rules = r
	return r, nil
}
func (f *fakeSchedule) Status(context.Context) wifischedule.Status {
	return wifischedule.Status{Runs: []wifischedule.Run{}}
}

func TestWiFiScheduleContract(t *testing.T) {
	const path = "/api/v1/wifi/schedule"
	sched := &fakeSchedule{rules: wifischedule.Rules{Rules: []wifischedule.Rule{}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.WiFiSchedule = sched })
	cookie, csrf := e.login()
	if rec := e.do(req{method: "GET", path: path, cookie: cookie}); rec.Code != http.StatusOK || rec.Body.String() != "{\"rules\":[]}\n" {
		t.Fatalf("get: %d %q", rec.Code, rec.Body)
	}
	body := `{"rules":[{"id":"","section":"default_radio1","days":[1,2],"off":"22:00","on":"07:00","enabled":true,"acknowledge_lockout":false}]}`
	if rec := e.do(req{method: "PUT", path: path, body: body, cookie: cookie, csrf: csrf}); rec.Code != http.StatusOK || len(sched.rules.Rules) != 1 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	strict := `{"rules":[],"runs":[]}`
	if rec := e.do(req{method: "PUT", path: path, body: strict, cookie: cookie, csrf: csrf}); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("unknown field accepted: %d", rec.Code)
	}
	for err, want := range map[error]string{
		lockout.ErrLockoutRisk:      "wifi_lockout_risk",
		wifischedule.ErrAPNotFound:  "wifi_ap_not_found",
		wifischedule.ErrInvalid:     "wifi_schedule_invalid",
		wifischedule.ErrUnavailable: "wifi_schedule_unavailable",
	} {
		sched.err = err
		if rec := e.do(req{method: "PUT", path: path, body: body, cookie: cookie, csrf: csrf}); errCode(t, rec) != want {
			t.Errorf("%v: %d %s", err, rec.Code, rec.Body)
		}
	}
	rec := e.do(req{method: "GET", path: path + "/status", cookie: cookie})
	var st map[string]json.RawMessage
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &st) != nil || string(st["runs"]) != "[]" || string(st["cron_running"]) != "null" {
		t.Fatalf("status: %d %s", rec.Code, rec.Body)
	}
}
