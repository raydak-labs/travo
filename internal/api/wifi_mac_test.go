package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

type fakeMAC struct {
	last wifi.MACRequest
	err  error
}

func (f *fakeMAC) UplinkMAC(context.Context) (wifi.MACPolicy, error) {
	return wifi.MACPolicy{Mode: wifi.MACRandom}, f.err
}

func (f *fakeMAC) SetUplinkMAC(_ context.Context, r wifi.MACRequest) (txn.Status, error) {
	f.last = r
	return txn.Status{State: txn.Pending}, f.err
}

func TestWiFiMACRoutes(t *testing.T) {
	mac := &fakeMAC{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.WiFiMAC = mac })
	cookie, csrf := e.login()
	if rec := e.do(req{method: http.MethodGet, path: "/api/v1/wifi/uplink/mac", cookie: cookie}); rec.Code != 200 || bodyField(t, rec, "mode") != "random" {
		t.Fatal(rec.Code, rec.Body)
	}
	request := req{method: http.MethodPost, path: "/api/v1/wifi/uplink/mac", cookie: cookie, csrf: csrf, body: `{"mode":"clone"}`}
	for _, tc := range []struct {
		err  error
		http int
		code string
	}{
		{nil, 202, ""},
		{wifi.ValidationError{Code: "wifi_mac_invalid"}, 400, "wifi_mac_invalid"},
		{wifi.ErrNoUplink, 409, "wifi_no_uplink"},
		{wifi.ErrMACCloneUnavailable, 409, "wifi_mac_clone_unavailable"},
		{txn.ErrNoChanges, 409, "wifi_no_change"},
		{wifi.ErrUnavailable, 503, "wifi_unavailable"},
	} {
		mac.err = tc.err
		rec := e.do(request)
		if rec.Code != tc.http || (tc.code != "" && bodyField(t, rec, "code") != tc.code) {
			t.Fatal(rec.Code, rec.Body)
		}
	}
	if mac.last.Mode != wifi.MACClone || !mac.last.Caller.IsValid() {
		t.Fatal("caller not delivered")
	}
	if rec := e.do(req{method: http.MethodPost, path: "/api/v1/wifi/uplink/mac", cookie: cookie, csrf: csrf, body: `{"mode":"bogus"}`}); rec.Code/100 != 4 {
		t.Fatal(rec.Code)
	}
}
