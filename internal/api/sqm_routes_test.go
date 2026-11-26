package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/raydak-labs/travo/internal/pkgmgr"
	"github.com/raydak-labs/travo/internal/sqm"
	"github.com/raydak-labs/travo/internal/txn"
)

type fakeSQM struct {
	cur sqm.Settings
	err error
}

func (f *fakeSQM) Get(context.Context) (sqm.Settings, error) {
	if f.err != nil {
		return sqm.Settings{}, f.err
	}
	return f.cur, nil
}

func (f *fakeSQM) Set(_ context.Context, in sqm.Settings) (sqm.Settings, error) {
	if f.err != nil {
		return sqm.Settings{}, f.err
	}
	f.cur = in
	return f.cur, nil
}

func TestSQMRoutes(t *testing.T) {
	f := &fakeSQM{cur: sqm.Settings{
		Enabled: false, Interface: "eth1", Download: 85000, Upload: 10000,
		Qdisc: "cake", Script: "piece_of_cake.qos", Linklayer: "none", Overhead: 0,
	}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.SQM = f })
	cookie, csrf := e.login()

	rec := e.do(req{method: "GET", path: "/api/v1/sqm", cookie: cookie})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"interface":"eth1"`) {
		t.Fatalf("GET: %d %s", rec.Code, rec.Body)
	}

	body := `{"enabled":true,"interface":"phy0-sta0","download":50000,"upload":15000,"qdisc":"cake","script":"piece_of_cake.qos","linklayer":"ethernet","overhead":44}`
	rec = e.do(req{method: "PUT", path: "/api/v1/sqm", body: body, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || !f.cur.Enabled || f.cur.Interface != "phy0-sta0" || f.cur.Overhead != 44 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}

	// Strict PUT rejects unknown fields, null, bad enums, out-of-range values.
	for _, bad := range []string{
		`{"enabled":true,"interface":"eth1","download":1000,"upload":1000,"qdisc":"cake","script":"piece_of_cake.qos","linklayer":"none","overhead":0,"extra":1}`,
		`{"enabled":null,"interface":"eth1","download":1000,"upload":1000,"qdisc":"cake","script":"piece_of_cake.qos","linklayer":"none","overhead":0}`,
		`{"enabled":true,"interface":"eth1","download":1000,"upload":1000,"qdisc":"bad","script":"piece_of_cake.qos","linklayer":"none","overhead":0}`,
		`{"enabled":true,"interface":"eth1","download":1000,"upload":1000,"qdisc":"cake","script":"piece_of_cake.qos","linklayer":"none","overhead":300}`,
	} {
		rec := e.do(req{method: "PUT", path: "/api/v1/sqm", body: bad, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("bad body %s: status %d %s", bad, rec.Code, rec.Body)
		}
	}

	// Error mapping including missing sqm-scripts -> 503 naming sqm-scripts.
	for code, tc := range map[string]struct {
		err    error
		status int
		substr string
	}{
		"invalid_request":     {&sqm.FieldError{Field: "interface", Reason: "bad"}, http.StatusBadRequest, "interface"},
		"sqm_not_installed":   {pkgmgr.ErrNotInstalled, http.StatusServiceUnavailable, "sqm-scripts"},
		"transaction_pending": {txn.ErrActive, http.StatusConflict, ""},
		"uci_busy":            {txn.ErrForeignStaged, http.StatusConflict, ""},
		"sqm_unavailable":     {sqm.ErrUnavailable, http.StatusServiceUnavailable, ""},
	} {
		f.err = tc.err
		rec := e.do(req{method: "PUT", path: "/api/v1/sqm", body: body, cookie: cookie, csrf: csrf})
		if rec.Code != tc.status || errCode(t, rec) != code || (tc.substr != "" && !strings.Contains(rec.Body.String(), tc.substr)) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
