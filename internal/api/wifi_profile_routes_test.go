package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
	"github.com/blackdark-org/travo-greenfield/internal/wifiprofiles"
)

type wifiProfileEnv struct {
	*env
	cookie, csrf string
	path         string
	joins        []wifi.JoinRequest
	status       txn.Status
	err          error
}

func newWiFiProfileEnv(t *testing.T) *wifiProfileEnv {
	t.Helper()
	w := &wifiProfileEnv{path: filepath.Join(t.TempDir(), "wifi-profiles.json")}
	join := joinWiFiFunc(func(_ context.Context, r wifi.JoinRequest) (txn.Status, error) {
		w.joins = append(w.joins, r)
		return w.status, w.err
	})
	profiles := wifiprofiles.New(w.path, join, nil)
	w.env = newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.WiFiProfiles = profiles })
	w.cookie, w.csrf = w.login()
	return w
}

func (w *wifiProfileEnv) call(method, path, body string) *profileReply {
	w.t.Helper()
	rec := w.do(req{method: method, path: "/api/v1/wifi/profiles" + path, body: body, cookie: w.cookie, csrf: w.csrf})
	return &profileReply{code: rec.Code, body: rec.Body.String()}
}

type profileReply struct {
	code int
	body string
}

func (r *profileReply) field(t *testing.T, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.body), &m); err != nil {
		t.Fatal(err, r.body)
	}
	s, _ := m[key].(string)
	return s
}

func TestWiFiProfileRoutes(t *testing.T) {
	const key = "sentinel-password"
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	w := newWiFiProfileEnv(t)

	if rec := w.do(req{method: http.MethodGet, path: "/api/v1/wifi/profiles"}); rec.Code != 401 {
		t.Fatal("unauthenticated list", rec.Code)
	}
	if rec := w.do(req{method: http.MethodPost, path: "/api/v1/wifi/profiles", cookie: w.cookie, body: `{"ssid":"HomeUplink","security":"open"}`}); rec.Code != 403 {
		t.Fatal("create without CSRF", rec.Code)
	}
	if r := w.call(http.MethodGet, "", ""); r.code != 200 || !strings.Contains(r.body, `"profiles":[],"connected_id":null`) {
		t.Fatal(r.code, r.body)
	}

	home := w.call(http.MethodPost, "", `{"ssid":"HomeUplink","security":"wpa2","key":"`+key+`"}`)
	neighbor := w.call(http.MethodPost, "", `{"ssid":"Neighbor2","bssid":"02:AB:00:00:00:01","security":"wpa2","key":"`+key+`"}`)
	if home.code != 201 || neighbor.code != 201 || neighbor.field(t, "bssid") != "02:ab:00:00:00:01" {
		t.Fatal(home, neighbor)
	}
	homeID, neighborID := home.field(t, "id"), neighbor.field(t, "id")
	for _, tc := range []struct {
		body, code string
		http       int
	}{
		{`{"ssid":"HomeUplink","security":"open"}`, "wifi_profile_exists", 409},
		{`{"ssid":"Other","security":"wpa2","key":"short"}`, "wifi_key_invalid", 400},
		{`{"ssid":"Other","security":"other"}`, "wifi_security_unsupported", 422},
		{`{"ssid":"Other","security":"wpa2","` + key + `":true}`, "invalid_request", 400},
		{`{"ssid":"Other","security":"wpa2","key":` + `"` + key + `"}{}`, "invalid_request", 400},
	} {
		if r := w.call(http.MethodPost, "", tc.body); r.code != tc.http || r.field(t, "code") != tc.code {
			t.Fatal(tc.code, r)
		}
	}

	if r := w.call(http.MethodPut, "/order", `{"ids":["`+homeID+`"]}`); r.code != 400 || r.field(t, "code") != "wifi_profile_order_invalid" {
		t.Fatal(r)
	}
	r := w.call(http.MethodPut, "/order", `{"ids":["`+neighborID+`","`+homeID+`"]}`)
	if r.code != 200 || strings.Index(r.body, neighborID) > strings.Index(r.body, homeID) || !strings.Contains(r.body, `"connected_id":null`) {
		t.Fatal(r)
	}

	unknown := strings.Repeat("f", 32)
	if r := w.call(http.MethodDelete, "/"+unknown, ""); r.code != 404 || r.field(t, "code") != "wifi_profile_not_found" {
		t.Fatal(r)
	}
	if r := w.call(http.MethodDelete, "/NOT-AN-ID", ""); r.code != 400 {
		t.Fatal(r)
	}
	if r := w.call(http.MethodPost, "/"+unknown+"/select", ""); r.code != 404 {
		t.Fatal(r)
	}

	// Select delegates the stored credentials to the existing join contract.
	for _, tc := range []struct {
		err   error
		state txn.State
		http  int
		code  string
	}{
		{nil, txn.Pending, 202, ""},
		{txn.ErrProbeFailed, txn.RolledBack, 202, ""},
		{wifi.ErrNetworkNotFound, "", 409, "wifi_network_not_found"},
		{wifi.ErrTransactionPending, "", 409, "transaction_pending"},
		{wifi.ErrUnavailable, "", 503, "wifi_unavailable"},
	} {
		w.status, w.err = txn.Status{State: tc.state}, tc.err
		if tc.state != "" {
			w.status.Meta.ID = "1234567890abcdef"
		}
		r := w.call(http.MethodPost, "/"+homeID+"/select", "")
		if r.code != tc.http || (tc.code != "" && r.field(t, "code") != tc.code) || (tc.state != "" && r.field(t, "state") != string(tc.state)) {
			t.Fatal(tc, r)
		}
	}
	if len(w.joins) != 5 || w.joins[0] != (wifi.JoinRequest{SSID: "HomeUplink", Key: key, Security: wifi.SecurityWPA2}) {
		t.Fatal("select did not deliver the saved credentials", len(w.joins))
	}

	if r := w.call(http.MethodDelete, "/"+homeID, ""); r.code != 200 || strings.Contains(r.body, homeID) {
		t.Fatal(r)
	}
	list := w.call(http.MethodGet, "", "")
	if list.code != 200 || !strings.Contains(list.body, neighborID) {
		t.Fatal(list)
	}
	for _, body := range []string{home.body, neighbor.body, list.body, r.body} {
		if strings.Contains(body, key) || strings.Contains(body, `"key"`) {
			t.Fatal("response leaks credentials", body)
		}
	}
	if strings.Contains(logs.String(), key) {
		t.Fatal("WiFi key leaked into logs")
	}
	stored, err := os.ReadFile(w.path)
	if err != nil || !strings.Contains(string(stored), key) {
		t.Fatal("credentials not persisted", err)
	}
}

func TestWiFiProfileStoreFailure(t *testing.T) {
	w := newWiFiProfileEnv(t)
	if err := os.WriteFile(w.path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, r := range []*profileReply{
		w.call(http.MethodGet, "", ""),
		w.call(http.MethodPost, "", `{"ssid":"HomeUplink","security":"open"}`),
		w.call(http.MethodPost, "/"+strings.Repeat("a", 32)+"/select", ""),
	} {
		if r.code != 503 || r.field(t, "code") != "wifi_profiles_unavailable" {
			t.Fatal(r)
		}
	}
	if len(w.joins) != 0 {
		t.Fatal("join ran without a valid store")
	}
}

func TestWiFiProfilesUnavailable(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, _ := e.login()
	rec := e.do(req{method: http.MethodGet, path: "/api/v1/wifi/profiles", cookie: cookie})
	if rec.Code != 503 || bodyField(t, rec, "code") != "device_unavailable" {
		t.Fatal(rec.Code, rec.Body)
	}
}
