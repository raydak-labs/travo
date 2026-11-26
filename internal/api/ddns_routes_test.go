package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/ddns"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeDDNS struct {
	err       error
	put       ddns.Request
	installed bool
}

func (f *fakeDDNS) Settings(context.Context) (ddns.View, error) {
	return ddns.View{Installed: true, Service: "duckdns.org", Domain: "trip.duckdns.org", PasswordSet: f.put.Password != "", Services: []string{"duckdns.org"}}, nil
}

func (f *fakeDDNS) Set(_ context.Context, in ddns.Request) error {
	f.put = in
	return f.err
}

func (f *fakeDDNS) Install(context.Context) error {
	f.installed = true
	return f.err
}

func TestDDNSRoutes(t *testing.T) {
	f := &fakeDDNS{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.DDNS = f })
	cookie, csrf := e.login()
	body := `{"enabled":true,"service":"duckdns.org","domain":"trip.duckdns.org","password":"tok-secret"}`
	rec := e.do(req{method: "PUT", path: "/api/v1/network/ddns", body: body, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || f.put.Password != "tok-secret" || strings.Contains(rec.Body.String(), "tok-secret") ||
		!strings.Contains(rec.Body.String(), `"password_set":true`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := e.do(req{method: "POST", path: "/api/v1/network/ddns/install", cookie: cookie, csrf: csrf}); rec.Code != http.StatusOK || !f.installed {
		t.Fatalf("install: %d %s", rec.Code, rec.Body)
	}
	for code, err := range map[string]error{
		"invalid_request":     &ddns.FieldError{Field: "domain", Reason: "bad"},
		"ddns_not_installed":  pkgmgr.ErrNotInstalled,
		"ddns_foreign_config": ddns.ErrForeign,
		"ddns_no_change":      ddns.ErrNoChange,
		"ddns_busy":           ddns.ErrBusy,
		"ddns_no_space":       ddns.ErrNoSpace,
		"uci_busy":            txn.ErrForeignStaged,
		"ddns_unavailable":    ddns.ErrUnavailable,
	} {
		f.err = err
		rec := e.do(req{method: "PUT", path: "/api/v1/network/ddns", body: body, cookie: cookie, csrf: csrf})
		if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) || strings.Contains(rec.Body.String(), "tok-secret") {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
