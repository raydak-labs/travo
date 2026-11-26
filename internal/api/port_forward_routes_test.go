package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/portfwd"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakePortForwards struct {
	err     error
	st      txn.Status
	added   portfwd.ForwardRequest
	removed string
}

func (f *fakePortForwards) Forwards(context.Context) ([]portfwd.Forward, error) { return nil, f.err }

func (f *fakePortForwards) Add(_ context.Context, in portfwd.ForwardRequest) (txn.Status, error) {
	f.added = in
	return f.st, f.err
}

func (f *fakePortForwards) Remove(_ context.Context, id string) (txn.Status, error) {
	f.removed = id
	return f.st, f.err
}

func TestPortForwardRoutes(t *testing.T) {
	f := &fakePortForwards{st: txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", WindowSeconds: 60}, State: txn.Pending}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.PortForwards = f })
	cookie, csrf := e.login()
	if rec := e.do(req{method: "GET", path: "/api/v1/network/port-forwards", cookie: cookie}); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"forwards":[]`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	body := `{"name":"NAS","proto":"both","external_port":"8080","target_ip":"192.168.1.50","target_port":"80"}`
	rec := e.do(req{method: "POST", path: "/api/v1/network/port-forwards", body: body, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || f.added.Proto != "both" || f.added.TargetPort != "80" {
		t.Fatalf("%d %s %+v", rec.Code, rec.Body, f.added)
	}
	rec = e.do(req{method: "DELETE", path: "/api/v1/network/port-forwards/travo_fwd_1", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || f.removed != "travo_fwd_1" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, bad := range []string{`{"proto":"icmp","external_port":"1","target_ip":"192.168.1.5"}`,
		`{"proto":"tcp","external_port":"1","target_ip":"192.168.1.5","extra":1}`, `{"proto":"tcp"}`} {
		if rec := e.do(req{method: "POST", path: "/api/v1/network/port-forwards", body: bad, cookie: cookie, csrf: csrf}); rec.Code != http.StatusBadRequest &&
			rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", bad, rec.Code, rec.Body)
		}
	}
}

func TestPortForwardRouteErrors(t *testing.T) {
	for code, err := range map[string]error{
		"invalid_request":           &portfwd.FieldError{Field: "target_ip", Reason: "bad"},
		"port_forward_in_use":       portfwd.ErrPortInUse,
		"port_forwards_full":        portfwd.ErrFull,
		"port_forward_no_zone":      portfwd.ErrNoZone,
		"port_forward_foreign":      portfwd.ErrForeign,
		"not_found":                 portfwd.ErrNotFound,
		"transaction_pending":       portfwd.ErrTransactionPending,
		"uci_busy":                  txn.ErrForeignStaged,
		"port_forwards_unavailable": portfwd.ErrUnavailable,
	} {
		f := &fakePortForwards{err: err}
		e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.PortForwards = f })
		cookie, csrf := e.login()
		rec := e.do(req{method: "DELETE", path: "/api/v1/network/port-forwards/travo_fwd_1", cookie: cookie, csrf: csrf})
		if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
