package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

type fakeGuest struct {
	last   wifi.GuestRequest
	status txn.Status
	err    error
}

func (f *fakeGuest) Guest(context.Context) (wifi.GuestConfig, error) {
	return wifi.GuestConfig{Subnet: "192.168.2.0/24"}, nil
}

func (f *fakeGuest) SetGuest(_ context.Context, r wifi.GuestRequest) (txn.Status, error) {
	f.last = r
	return f.status, f.err
}

func TestWiFiGuestRoutes(t *testing.T) {
	const key = "sentinel-password"
	guest := &fakeGuest{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.WiFiGuest = guest })
	cookie, csrf := e.login()
	if rec := e.do(req{method: http.MethodGet, path: "/api/v1/wifi/guest", cookie: cookie}); rec.Code != 200 || bodyField(t, rec, "subnet") != "192.168.2.0/24" {
		t.Fatal(rec.Code, rec.Body)
	}
	request := req{method: http.MethodPost, path: "/api/v1/wifi/guest", cookie: cookie, csrf: csrf,
		body: `{"ssid":"Guests","key":"sentinel-password","enabled":true,"radio":"radio1"}`}
	for _, tc := range []struct {
		err  error
		http int
		code string
	}{
		{nil, 202, ""},
		{wifi.ValidationError{Code: "wifi_key_required"}, 400, "wifi_key_required"},
		{wifi.ErrGuestOverlap, 409, "wifi_guest_subnet_overlap"},
		{wifi.ErrGuestConflict, 409, "wifi_guest_conflict"},
		{vpnkill.ErrUnsupported, 409, "vpn_killswitch_unsupported"},
		{wifi.ErrNoGuestRadio, 409, "wifi_no_free_radio"},
		{wifi.ErrRadioInUse, 409, "wifi_radio_in_use"},
		{lockout.ErrLockoutRisk, 409, "wifi_lockout_risk"},
		{txn.ErrNoChanges, 409, "wifi_no_change"},
		{wifi.ErrUnavailable, 503, "wifi_unavailable"},
	} {
		guest.status, guest.err = txn.Status{State: txn.Pending}, tc.err
		rec := e.do(request)
		if rec.Code != tc.http || strings.Contains(rec.Body.String(), key) {
			t.Fatal(rec.Code, rec.Body)
		}
		if tc.code != "" && bodyField(t, rec, "code") != tc.code {
			t.Fatal(rec.Body)
		}
	}
	if got := guest.last; got.Key != key || got.Radio != "radio1" || !got.Enabled || !got.Caller.IsValid() {
		t.Fatal("request not delivered")
	}
}
