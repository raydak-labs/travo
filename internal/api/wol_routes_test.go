package api

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/wol"
)

func wolEnv(t *testing.T, w WakeOnLAN) (*env, string, string) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.WakeOnLAN = w })
	cookie, csrf := e.login()
	return e, cookie, csrf
}

func TestWakeOnLANSendsThroughTheService(t *testing.T) {
	d := mock.New()
	mock.SeedClients(d)
	e, cookie, csrf := wolEnv(t, &wol.Service{Ubus: d, Sender: d})
	rec := e.do(req{method: "POST", path: "/api/v1/network/wol", body: `{"mac":"AA:BB:CC:DD:EE:FF"}`, cookie: cookie, csrf: csrf})
	var got wol.Wake
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.MAC != "aa:bb:cc:dd:ee:ff" || !slices.Equal(got.Interfaces, []string{"lan"}) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for body, code := range map[string]string{
		`{"mac":"ff:ff:ff:ff:ff:ff"}`:                    "invalid_mac",
		`{"mac":"aa:bb:cc:dd:ee:f;"}`:                    "invalid_mac",
		`{"mac":"aa:bb:cc:dd:ee:ff","interface":"wwan"}`: "invalid_interface",
	} {
		rec = e.do(req{method: "POST", path: "/api/v1/network/wol", body: body, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), code) {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body)
		}
	}
	for _, body := range []string{`{"mac":""}`, `{"mac":"aa:bb:cc:dd:ee:ff","extra":1}`, `{"mac":"aa:bb:cc:dd:ee:ff","interface":"abcdefghijklmnop"}`} {
		rec = e.do(req{method: "POST", path: "/api/v1/network/wol", body: body, cookie: cookie, csrf: csrf})
		if rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("schema: %s: %d %s", body, rec.Code, rec.Body)
		}
	}
	if len(d.Wakes) != 1 {
		t.Fatalf("refused requests sent: %+v", d.Wakes)
	}
}

type busyWake struct{}

func (busyWake) Wake(context.Context, string, string) (wol.Wake, error) {
	return wol.Wake{}, wol.ErrBusy
}

func TestWakeOnLANBusyRateLimitedAndUnavailable(t *testing.T) {
	e, cookie, csrf := wolEnv(t, busyWake{})
	body := `{"mac":"aa:bb:cc:dd:ee:ff"}`
	rec := e.do(req{method: "POST", path: "/api/v1/network/wol", body: body, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "wol_busy") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	limited := false
	for range 10 {
		rec = e.do(req{method: "POST", path: "/api/v1/network/wol", body: body, cookie: cookie, csrf: csrf})
		limited = limited || rec.Code == http.StatusTooManyRequests
	}
	if !limited {
		t.Fatal("never rate-limited")
	}
	e, cookie, csrf = wolEnv(t, nil)
	if rec := e.do(req{method: "POST", path: "/api/v1/network/wol", body: body, cookie: cookie, csrf: csrf}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil service: %d", rec.Code)
	}
}
