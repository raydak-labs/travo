package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/alerts"
)

type fakeAlerts struct{ err error }

func (f fakeAlerts) List(context.Context) (alerts.AlertList, error) {
	if f.err != nil {
		return alerts.AlertList{}, f.err
	}
	since := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	return alerts.AlertList{Alerts: []alerts.Alert{{ID: "adguard_fail_open", Severity: alerts.Danger, Code: alerts.CodeAdGuardFailOpen, Since: &since}}}, nil
}

func TestSystemAlerts(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Alerts = fakeAlerts{} })
	cookie, _ := e.login()
	rec := e.do(req{method: http.MethodGet, path: "/api/v1/system/alerts", cookie: cookie})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var body map[string][]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	a := body["alerts"]
	if len(a) != 1 || a[0]["code"] != "adguard_fail_open" || a[0]["severity"] != "danger" || a[0]["since"] != "2026-11-18T10:00:00Z" {
		t.Fatalf("%s", rec.Body)
	}
}

func TestSystemAlertsUnavailable(t *testing.T) {
	for _, alerts := range []Alerts{nil, fakeAlerts{err: errors.New("boom")}} {
		e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Alerts = alerts })
		cookie, _ := e.login()
		rec := e.do(req{method: http.MethodGet, path: "/api/v1/system/alerts", cookie: cookie})
		if rec.Code != http.StatusServiceUnavailable || !json.Valid(rec.Body.Bytes()) {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
	}
}
