package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/ipv6settings"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeIPv6 struct {
	err error
	st  txn.Status
	put ipv6settings.Request
}

func (f *fakeIPv6) Settings(context.Context) (ipv6settings.View, error) {
	return ipv6settings.View{Enabled: true}, f.err
}

func (f *fakeIPv6) Set(_ context.Context, in ipv6settings.Request) (txn.Status, error) {
	f.put = in
	return f.st, f.err
}

func TestIPv6Routes(t *testing.T) {
	f := &fakeIPv6{put: ipv6settings.Request{Enabled: true}, st: txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", WindowSeconds: 60}, State: txn.Pending}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.IPv6 = f })
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/network/ipv6", cookie: cookie})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/network/ipv6", body: `{"enabled":false}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || f.put.Enabled {
		t.Fatalf("%d %s %+v", rec.Code, rec.Body, f.put)
	}
	if rec := e.do(req{method: "PUT", path: "/api/v1/network/ipv6", body: `{"enabled":true,"x":1}`, cookie: cookie, csrf: csrf}); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Errorf("unknown field: %d", rec.Code)
	}
}

func TestIPv6RouteErrors(t *testing.T) {
	for code, err := range map[string]error{
		"ipv6_foreign_config": ipv6settings.ErrForeign,
		"ipv6_vpn_leak":       ipv6settings.ErrVPNLeak,
		"transaction_pending": txn.ErrActive,
		"ipv6_no_change":      txn.ErrNoChanges,
		"uci_busy":            txn.ErrForeignStaged,
		"ipv6_unavailable":    ipv6settings.ErrUnavailable,
	} {
		f := &fakeIPv6{err: err}
		e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.IPv6 = f })
		cookie, csrf := e.login()
		rec := e.do(req{method: "PUT", path: "/api/v1/network/ipv6", body: `{"enabled":true}`, cookie: cookie, csrf: csrf})
		if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
