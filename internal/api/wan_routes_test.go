package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wansettings"
)

type fakeWAN struct {
	err error
	st  txn.Status
	put wansettings.Request
}

func (f *fakeWAN) Settings(context.Context) (wansettings.View, error) {
	return wansettings.View{Proto: "pppoe", Username: "u@isp", PasswordSet: true, DNS: []string{}, Device: "wan"}, f.err
}

func (f *fakeWAN) Set(_ context.Context, in wansettings.Request) (txn.Status, error) {
	f.put = in
	return f.st, f.err
}

func TestWANRoutes(t *testing.T) {
	f := &fakeWAN{st: txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", WindowSeconds: 60}, State: txn.Pending}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.WAN = f })
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/network/wan", cookie: cookie})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"password_set":true`) || strings.Contains(rec.Body.String(), `"password"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	body := `{"proto":"pppoe","username":"u@isp","password":"s3cret"}`
	rec = e.do(req{method: "PUT", path: "/api/v1/network/wan", body: body, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || f.put.Password != "s3cret" || strings.Contains(rec.Body.String(), "s3cret") {
		t.Fatalf("%d %s %+v", rec.Code, rec.Body, f.put)
	}
	for _, bad := range []string{`{"proto":"pppoe","extra":1}`, `{"proto":"qmi"}`, `{"username":"x"}`} {
		if rec := e.do(req{method: "PUT", path: "/api/v1/network/wan", body: bad, cookie: cookie, csrf: csrf}); rec.Code != http.StatusBadRequest &&
			rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", bad, rec.Code, rec.Body)
		}
	}
}

func TestWANRouteErrors(t *testing.T) {
	for code, err := range map[string]error{
		"invalid_request":     &wansettings.FieldError{Field: "gateway", Reason: "bad"},
		"wan_overlaps_lan":    wansettings.ErrOverlapsLAN,
		"wan_foreign_config":  wansettings.ErrForeign,
		"transaction_pending": txn.ErrActive,
		"wan_no_change":       txn.ErrNoChanges,
		"uci_busy":            txn.ErrForeignStaged,
		"wan_unavailable":     wansettings.ErrUnavailable,
	} {
		f := &fakeWAN{err: err}
		e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.WAN = f })
		cookie, csrf := e.login()
		rec := e.do(req{method: "PUT", path: "/api/v1/network/wan", body: `{"proto":"dhcp"}`, cookie: cookie, csrf: csrf})
		if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
