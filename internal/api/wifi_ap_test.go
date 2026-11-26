package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

type fakeAPs struct {
	list    []wifi.APConfig
	last    wifi.APRequest
	calls   int
	status  txn.Status
	err     error
	listErr error
}

func (f *fakeAPs) AccessPoints(context.Context) ([]wifi.APConfig, error) { return f.list, f.listErr }
func (f *fakeAPs) UpdateAP(_ context.Context, r wifi.APRequest) (txn.Status, error) {
	f.calls++
	f.last = r
	return f.status, f.err
}

func TestWiFiAPRoutes(t *testing.T) {
	const key = "sentinel-password"
	aps := &fakeAPs{list: []wifi.APConfig{}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.WiFiAPs = aps })
	cookie, csrf := e.login()
	if rec := e.do(req{method: http.MethodGet, path: "/api/v1/wifi/aps", cookie: cookie}); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"aps":[]`) {
		t.Fatal(rec.Code, rec.Body)
	}
	request := req{method: http.MethodPost, path: "/api/v1/wifi/aps/default_radio1", cookie: cookie, csrf: csrf,
		body: `{"ssid":"TravelAP","key":"sentinel-password","enabled":false,"acknowledge_lockout":true}`}
	for _, tc := range []struct {
		err   error
		state txn.State
		http  int
		code  string
	}{
		{nil, txn.Pending, 202, ""},
		{txn.ErrProbeFailed, txn.RolledBack, 202, ""},
		{wifi.ValidationError{Code: "wifi_ssid_invalid"}, "", 400, "wifi_ssid_invalid"},
		{wifi.ErrAPNotFound, "", 404, "wifi_ap_not_found"},
		{wifi.ErrRadioInUse, "", 409, "wifi_radio_in_use"},
		{lockout.ErrLockoutRisk, "", 409, "wifi_lockout_risk"},
		{wifi.ErrTransactionPending, "", 409, "transaction_pending"},
		{txn.ErrNoChanges, "", 409, "wifi_no_change"},
		{wifi.ErrUnavailable, "", 503, "wifi_unavailable"},
	} {
		aps.status, aps.err = txn.Status{State: tc.state}, tc.err
		if tc.state != "" {
			aps.status.Meta.ID = "1234567890abcdef"
		}
		rec := e.do(request)
		if rec.Code != tc.http || strings.Contains(rec.Body.String(), key) {
			t.Fatal(rec.Code, rec.Body)
		}
		if tc.code != "" && bodyField(t, rec, "code") != tc.code {
			t.Fatal(rec.Body)
		}
	}
	got := aps.last
	if got.Section != "default_radio1" || got.Key != key || got.Enabled || !got.Acknowledge || !got.Caller.IsValid() {
		t.Fatalf("request not delivered: %+v", got.Section)
	}
	calls := aps.calls
	request.body = `{"ssid":"TravelAP","enabled":true,"unknown":1}`
	if rec := e.do(request); rec.Code != 400 || aps.calls != calls {
		t.Fatal(rec.Code, rec.Body)
	}
}

func TestWiFiAPRoutesUnavailable(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, csrf := e.login()
	rec := e.do(req{method: http.MethodPost, path: "/api/v1/wifi/aps/default_radio1", cookie: cookie, csrf: csrf, body: `{"ssid":"x","enabled":true}`})
	if rec.Code != 503 || bodyField(t, rec, "code") != "device_unavailable" {
		t.Fatal(rec.Code, rec.Body)
	}
}
