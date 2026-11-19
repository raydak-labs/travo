package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
)

type fakeDoH struct {
	err error
	put adguard.DoHRequest
}

func (f *fakeDoH) Get(context.Context) (adguard.DoHView, error) {
	return adguard.DoHView{Upstreams: []string{"https://dns10.quad9.net/dns-query"}, Encrypted: true}, f.err
}

func (f *fakeDoH) Set(_ context.Context, in adguard.DoHRequest) (adguard.DoHView, error) {
	f.put = in
	return adguard.DoHView{Upstreams: []string{in.URL}, Encrypted: true}, f.err
}

func TestDoHRoutes(t *testing.T) {
	f := &fakeDoH{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.AdGuardDoH = f })
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/adguard/doh", cookie: cookie})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"encrypted":true`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	body := `{"url":"https://cloudflare-dns.com/dns-query"}`
	rec = e.do(req{method: "PUT", path: "/api/v1/adguard/doh", body: body, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || f.put.URL != "https://cloudflare-dns.com/dns-query" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for code, err := range map[string]error{
		"invalid_request":          adguard.ErrDoHInvalid,
		"doh_upstream_unreachable": adguard.ErrDoHUnreachable,
		"doh_no_change":            adguard.ErrDoHNoChange,
		"adguard_busy":             adguard.ErrBusy,
		"adguard_unavailable":      adguard.ErrAdminAPI,
	} {
		f.err = err
		rec := e.do(req{method: "PUT", path: "/api/v1/adguard/doh", body: body, cookie: cookie, csrf: csrf})
		if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
