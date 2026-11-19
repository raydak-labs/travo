package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/leds"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeLEDs struct {
	st     leds.Status
	setErr error
}

func (f *fakeLEDs) Get(context.Context) (leds.Status, error) { return f.st, nil }

func (f *fakeLEDs) SetOff(_ context.Context, off bool) (leds.Status, error) {
	f.st.Off = off
	return f.st, f.setErr
}

func (f *fakeLEDs) SetSchedule(_ context.Context, s leds.Schedule) (leds.Status, error) {
	if _, err := s.Lines(); err != nil {
		return leds.Status{}, err
	}
	f.st.Schedule = s
	return f.st, nil
}

func TestLEDRoutes(t *testing.T) {
	f := &fakeLEDs{st: leds.Status{LEDs: []leds.Entry{{Name: "blue:run", Trigger: "none", On: true}},
		Schedule: leds.Schedule{OffAt: leds.DefaultOffAt, OnAt: leds.DefaultOnAt}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.LEDs = f })
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/system/leds", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"blue:run"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/system/leds", body: `{"off":true}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || !f.st.Off {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/system/leds/schedule", body: `{"enabled":true,"off_at":"07:00","on_at":"07:00"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "led_schedule_invalid") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	f.setErr = txn.ErrForeignStaged
	rec = e.do(req{method: "PUT", path: "/api/v1/system/leds", body: `{"off":false}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "uci_busy") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestLEDRoutesUnavailable(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(*Deps) {})
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/system/leds", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "leds_unavailable") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
