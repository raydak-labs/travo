package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

type joinWiFiFunc func(context.Context, wifi.JoinRequest) (txn.Status, error)

func (f joinWiFiFunc) Join(ctx context.Context, req wifi.JoinRequest) (txn.Status, error) {
	return f(ctx, req)
}

func TestWiFiJoinRoute(t *testing.T) {
	const key = "sentinel-password"
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	var calls int
	var status txn.Status
	var replyErr error
	join := joinWiFiFunc(func(_ context.Context, r wifi.JoinRequest) (txn.Status, error) {
		calls++
		if r.Key != key {
			t.Fatal("request did not deliver intended key")
		}
		return status, replyErr
	})
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.WiFiJoin = join })
	request := req{method: http.MethodPost, path: "/api/v1/wifi/join", body: `{"ssid":"Network","key":"sentinel-password","security":"wpa2"}`}
	if rec := e.do(request); rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	cookie, csrf := e.login()
	request.cookie = cookie
	if rec := e.do(request); rec.Code != 403 {
		t.Fatal(rec.Code)
	}
	request.csrf = csrf
	original := request.body
	request.body = `{"ssid":"Network","sentinel-password":true,"security":"wpa2"}`
	if rec := e.do(request); rec.Code != 400 || strings.Contains(rec.Body.String(), key) {
		t.Fatal(rec.Code, rec.Body)
	}
	if calls != 0 {
		t.Fatal(calls)
	}
	request.body = original + original
	if rec := e.do(request); rec.Code != 400 || strings.Contains(rec.Body.String(), key) {
		t.Fatal(rec.Code, rec.Body)
	}
	request.body = original
	for _, tc := range []struct {
		err   error
		state txn.State
		http  int
		code  string
	}{
		{nil, txn.Pending, 202, ""},
		{txn.ErrProbeFailed, txn.RolledBack, 202, ""},
		{txn.ErrProbeFailed, txn.Restoring, 202, ""},
		{txn.ErrProbeFailed, txn.RollbackFailed, 202, ""},
		{wifi.ErrNoFreeRadio, "", 409, "wifi_no_free_radio"},
		{wifi.ErrTransactionPending, "", 409, "transaction_pending"},
		{wifi.ErrNetworkNotFound, "", 409, "wifi_network_not_found"},
		{wifi.ErrSecurityUnsupported, "", 422, "wifi_security_unsupported"},
		{txn.ErrNoChanges, "", 409, "wifi_no_change"},
		{lockout.ErrLockoutRisk, "", 409, "wifi_lockout_risk"},
		{wifi.ErrFirewallUnavailable, "", 503, "wifi_unavailable"},
		{wifi.ValidationError{Code: "wifi_key_invalid"}, "", 400, "wifi_key_invalid"},
		{errors.New(key), "", 503, "wifi_unavailable"},
	} {
		status = txn.Status{State: tc.state}
		if tc.state != "" {
			status.Meta.ID = "1234567890abcdef"
		}
		replyErr = tc.err
		rec := e.do(request)
		if rec.Code != tc.http || strings.Contains(rec.Body.String(), key) {
			t.Fatal(rec.Code, rec.Body)
		}
		if tc.code != "" && bodyField(t, rec, "code") != tc.code {
			t.Fatal(rec.Body)
		}
		if tc.state != "" && bodyField(t, rec, "state") != string(tc.state) {
			t.Fatal(rec.Body)
		}
	}

	if strings.Contains(logs.String(), key) {
		t.Fatal("WiFi key leaked into logs")
	}
}

func TestWiFiJoinUnavailable(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, csrf := e.login()
	rec := e.do(req{method: http.MethodPost, path: "/api/v1/wifi/join", cookie: cookie, csrf: csrf, body: `{"ssid":"Network","security":"open"}`})
	if rec.Code != 503 || bodyField(t, rec, "code") != "device_unavailable" {
		t.Fatal(rec.Code, rec.Body)
	}
}
