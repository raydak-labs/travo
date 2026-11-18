package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/clients"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeClients struct {
	err     error
	caller  netip.Addr
	blocked *bool
	alias   string
}

func (f *fakeClients) List(context.Context) (clients.Listing, error) {
	return clients.Listing{Clients: []clients.Client{{MAC: "22:4e:76:6c:2d:62", Connection: clients.WiFi}}, Partial: true}, f.err
}

func (f *fakeClients) SetAlias(mac, alias string) error {
	f.alias = mac + "=" + alias
	return f.err
}

func (f *fakeClients) ListReservations(context.Context) ([]clients.Reservation, error) {
	return nil, f.err
}

func (f *fakeClients) Reserve(_ context.Context, r clients.Reservation) (clients.Reservation, error) {
	return r, f.err
}

func (f *fakeClients) Unreserve(context.Context, string) error { return f.err }

func (f *fakeClients) SetBlocked(_ context.Context, _ string, blocked bool, caller netip.Addr) (txn.Status, error) {
	f.blocked, f.caller = &blocked, caller
	if f.err != nil {
		return txn.Status{}, f.err
	}
	return txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", Configs: []string{"firewall"}, WindowSeconds: 60}, State: txn.Pending}, nil
}

func (f *fakeClients) Kick(_ context.Context, _ string, caller netip.Addr) error {
	f.caller = caller
	return f.err
}

func clientsEnv(t *testing.T, f *fakeClients) (*env, string, string) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		if f != nil {
			d.Clients = f
		}
	})
	cookie, csrf := e.login()
	return e, cookie, csrf
}

func TestClientsListAndAlias(t *testing.T) {
	f := &fakeClients{}
	e, cookie, csrf := clientsEnv(t, f)
	rec := e.do(req{method: "GET", path: "/api/v1/clients", cookie: cookie, csrf: csrf})
	var got struct {
		Clients []map[string]any `json:"clients"`
		Partial bool             `json:"partial"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || !got.Partial || got.Clients[0]["connection"] != "wifi" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/clients/22:4e:76:6c:2d:62/alias", body: `{"alias":"Phone"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || f.alias != "22:4e:76:6c:2d:62=Phone" {
		t.Fatalf("%d %s %s", rec.Code, rec.Body, f.alias)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/clients/not-a-mac/alias", body: `{"alias":"x"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/clients/22:4e:76:6c:2d:62/alias", body: `{"alias":"x","extra":1}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("strict body: %d %s", rec.Code, rec.Body)
	}
}

func TestClientsBlockPassesTheCallerAndReturnsTheTransaction(t *testing.T) {
	f := &fakeClients{}
	e, cookie, csrf := clientsEnv(t, f)
	rec := e.do(req{method: "POST", path: "/api/v1/clients/22:4e:76:6c:2d:62/block", body: `{"blocked":true}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"state":"pending"`) || f.blocked == nil || !*f.blocked || !f.caller.IsValid() {
		t.Fatalf("%d %s %v", rec.Code, rec.Body, f.caller)
	}
}

func TestClientsErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		err          error
		method, path string
		body         string
		status       int
		code         string
	}{
		{clients.ErrIsCaller, "POST", "/api/v1/clients/22:4e:76:6c:2d:62/block", `{"blocked":true}`, 409, "client_is_caller"},
		{txn.ErrNoChanges, "POST", "/api/v1/clients/22:4e:76:6c:2d:62/block", `{"blocked":false}`, 409, "client_no_change"},
		{clients.ErrTransactionPending, "POST", "/api/v1/clients/22:4e:76:6c:2d:62/block", `{"blocked":true}`, 409, "transaction_pending"},
		{clients.ErrNotAssociated, "POST", "/api/v1/clients/22:4e:76:6c:2d:62/kick", "", 404, "client_not_associated"},
		{clients.ErrIPTaken, "POST", "/api/v1/clients/reservations", `{"mac":"22:4e:76:6c:2d:62","ip":"192.168.1.5"}`, 409, "reservation_ip_taken"},
		{&clients.FieldError{Field: "ip", Reason: "bad"}, "POST", "/api/v1/clients/reservations", `{"mac":"22:4e:76:6c:2d:62","ip":"x"}`, 400, "invalid_request"},
		{clients.ErrReservationNotFound, "DELETE", "/api/v1/clients/reservations/22:4e:76:6c:2d:62", "", 404, "reservation_not_found"},
		{clients.ErrStoreCorrupt, "GET", "/api/v1/clients", "", 503, "clients_unavailable"},
	} {
		e, cookie, csrf := clientsEnv(t, &fakeClients{err: tc.err})
		rec := e.do(req{method: tc.method, path: tc.path, body: tc.body, cookie: cookie, csrf: csrf})
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%v: %d %s", tc.err, rec.Code, rec.Body)
		}
	}
	e, cookie, csrf := clientsEnv(t, nil)
	if rec := e.do(req{method: "GET", path: "/api/v1/clients/reservations", cookie: cookie, csrf: csrf}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d", rec.Code)
	}
}
