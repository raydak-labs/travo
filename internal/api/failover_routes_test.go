package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/failover"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type failoverFake struct {
	st         failover.Status
	err        error
	tx         txn.Status
	enableErr  error
	disableErr error
	uplinks    []string
	disabled   int
}

func (f *failoverFake) Status(context.Context) (failover.Status, error) { return f.st, f.err }
func (f *failoverFake) Enable(_ context.Context, u []string) (txn.Status, error) {
	f.uplinks = u
	return f.tx, f.enableErr
}
func (f *failoverFake) Disable(context.Context) error { f.disabled++; return f.disableErr }

func TestFailoverRoutes(t *testing.T) {
	fake := &failoverFake{st: failover.Status{Settings: failover.Settings{Uplinks: []string{"wan", "wwan"}}, States: []failover.UplinkState{}, Warnings: []failover.Warning{}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Failover = fake })
	cookie, csrf := e.login()
	got := e.do(req{path: "/api/v1/failover", cookie: cookie})
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"uplinks":["wan","wwan"]`) {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	post := func(body string) *httptest.ResponseRecorder {
		return e.do(req{method: "POST", path: "/api/v1/failover", body: body, cookie: cookie, csrf: csrf})
	}
	for _, bad := range []string{`{"uplinks":["wan"]}`, `{"uplinks":["wan","lan"]}`, `{"uplinks":["wan","wwan"],"x":1}`} {
		if got := post(bad); got.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", bad, got.Code, got.Body)
		}
	}
	fake.tx = txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", Started: time.Unix(0, 0).UTC(), Summary: "Enable failover", WindowSeconds: 60}, State: txn.Pending}
	if got := post(`{"uplinks":["wwan","wan"]}`); got.Code != http.StatusAccepted || !slices.Equal(fake.uplinks, []string{"wwan", "wan"}) {
		t.Fatalf("%d %s %v", got.Code, got.Body, fake.uplinks)
	}
	fake.tx = txn.Status{}
	for err, code := range map[error]string{
		failover.ErrOffline:   "failover_offline",
		failover.ErrConflict:  "failover_conflict",
		failover.ErrTailscale: "failover_tailscale_conflict",
		failover.ErrInvalid:   "validation",
		txn.ErrActive:         "transaction_pending",
		txn.ErrNoChanges:      "failover_no_change",
		errors.New("apk"):     "failover_failed",
	} {
		fake.enableErr = err
		if got := post(`{"uplinks":["wan","wwan"]}`); errCode(t, got) != code {
			t.Fatalf("%v: %d %s", err, got.Code, got.Body)
		}
	}
	del := func() *httptest.ResponseRecorder {
		return e.do(req{method: "DELETE", path: "/api/v1/failover", cookie: cookie, csrf: csrf})
	}
	if got := del(); got.Code != http.StatusOK || fake.disabled != 1 {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	fake.disableErr = failover.ErrBusy
	if got := del(); errCode(t, got) != "failover_busy" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	fake.err = errors.New("uci")
	if got := e.do(req{path: "/api/v1/failover", cookie: cookie}); errCode(t, got) != "failover_unavailable" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}
