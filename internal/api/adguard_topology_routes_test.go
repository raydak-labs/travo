package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeTopology struct {
	st   txn.Status
	err  error
	mode string
}

func (f *fakeTopology) SetTopology(_ context.Context, mode string) (txn.Status, error) {
	f.mode = mode
	return f.st, f.err
}

func TestAdGuardTopologyRoute(t *testing.T) {
	f := &fakeTopology{st: txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", WindowSeconds: 60}, State: txn.Pending}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.AdGuardTopology = f })
	cookie, csrf := e.login()
	rec := e.do(req{method: "PUT", path: "/api/v1/adguard/topology", body: `{"mode":"expert"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || f.mode != "expert" || !strings.Contains(rec.Body.String(), `"state":"pending"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := e.do(req{method: "PUT", path: "/api/v1/adguard/topology", body: `{"mode":"auto"}`, cookie: cookie, csrf: csrf}); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown mode: %d", rec.Code)
	}
	f.st, f.err = txn.Status{Meta: txn.Meta{ID: "0123456789abcdef"}, State: txn.RolledBack}, errors.Join(txn.ErrProbeFailed, adguard.ErrProbe)
	rec = e.do(req{method: "PUT", path: "/api/v1/adguard/topology", body: `{"mode":"expert"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"state":"rolled_back"`) {
		t.Fatalf("a failed probe reports the rolled back transaction: %d %s", rec.Code, rec.Body)
	}
}

func TestAdGuardTopologyRouteErrors(t *testing.T) {
	for code, err := range map[string]error{
		"adguard_topology_not_ready": adguard.ErrTopologyNotReady,
		"adguard_topology_foreign":   adguard.ErrTopologyForeign,
		"adguard_config_unknown":     adguard.ErrUnknownShape,
		"adguard_busy":               adguard.ErrBusy,
		"adguard_topology_no_change": txn.ErrNoChanges,
		"transaction_pending":        txn.ErrActive,
		"dns_busy":                   txn.ErrForeignStaged,
		"adguard_unavailable":        adguard.ErrTopologyUnavailable,
	} {
		f := &fakeTopology{err: err}
		e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.AdGuardTopology = f })
		cookie, csrf := e.login()
		rec := e.do(req{method: "PUT", path: "/api/v1/adguard/topology", body: `{"mode":"default"}`, cookie: cookie, csrf: csrf})
		if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
	e := newEnvWithDashboard(t, nil, nil, nil, func(*Deps) {})
	cookie, csrf := e.login()
	if rec := e.do(req{method: "PUT", path: "/api/v1/adguard/topology", body: `{"mode":"expert"}`, cookie: cookie, csrf: csrf}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no AdGuard supervisor: %d", rec.Code)
	}
}
