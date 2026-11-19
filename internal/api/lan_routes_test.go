package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/lanaddress"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeLAN struct {
	err error
	st  txn.Status
	put lanaddress.Request
}

func (f *fakeLAN) Settings(context.Context) (lanaddress.View, error) {
	return lanaddress.View{Address: "192.168.1.1/24", Subnet: "192.168.1.0/24",
		Pending: &lanaddress.Pending{ID: "0123456789abcdef", Address: "192.168.8.1/24", RemainingSeconds: 120}}, f.err
}

func (f *fakeLAN) Set(_ context.Context, in lanaddress.Request) (txn.Status, error) {
	f.put = in
	return f.st, f.err
}

func TestLANRoutes(t *testing.T) {
	f := &fakeLAN{st: txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", WindowSeconds: 180}, State: txn.Pending}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.LAN = f })
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/network/lan", cookie: cookie})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"remaining_seconds":120`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/network/lan", body: `{"address":"192.168.8.1/24"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || f.put.Address != "192.168.8.1/24" || !strings.Contains(rec.Body.String(), `"window_seconds":180`) {
		t.Fatalf("%d %s %+v", rec.Code, rec.Body, f.put)
	}
	for _, bad := range []string{`{"address":"192.168.8.1/24","extra":1}`, `{}`, `{"address":"` + strings.Repeat("1", 19) + `"}`} {
		if rec := e.do(req{method: "PUT", path: "/api/v1/network/lan", body: bad, cookie: cookie, csrf: csrf}); rec.Code != http.StatusBadRequest &&
			rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", bad, rec.Code, rec.Body)
		}
	}
}

func TestLANRouteErrors(t *testing.T) {
	for code, err := range map[string]error{
		"invalid_request":        &lanaddress.FieldError{Field: "address", Reason: "bad"},
		"lan_subnet_overlap":     &lanaddress.OverlapError{Interface: "guest", Prefix: "192.168.2.0/24"},
		"lan_dhcp_pool_outside":  lanaddress.ErrPoolOutside,
		"lan_dependents_outside": &lanaddress.DependentsError{Sections: []string{"dhcp.x (192.168.1.5)"}},
		"lan_foreign_config":     lanaddress.ErrForeign,
		"transaction_pending":    txn.ErrActive,
		"lan_no_change":          txn.ErrNoChanges,
		"uci_busy":               txn.ErrForeignStaged,
		"lan_unavailable":        lanaddress.ErrUnavailable,
	} {
		f := &fakeLAN{err: err}
		e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.LAN = f })
		cookie, csrf := e.login()
		rec := e.do(req{method: "PUT", path: "/api/v1/network/lan", body: `{"address":"192.168.8.1/24"}`, cookie: cookie, csrf: csrf})
		if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
