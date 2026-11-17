package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
)

type killSwitchFake struct {
	state vpnkill.KillSwitchState
	st    txn.Status
	err   error
	set   []bool
}

func (f *killSwitchFake) Status(context.Context) (vpnkill.KillSwitchState, error) {
	return f.state, f.err
}
func (f *killSwitchFake) Set(_ context.Context, enabled bool) (txn.Status, error) {
	f.set = append(f.set, enabled)
	return f.st, f.err
}

func TestVPNKillSwitchRoutes(t *testing.T) {
	fake := &killSwitchFake{state: vpnkill.KillSwitchState{Enabled: true, Zone: true}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.VPNKillSwitch = fake })
	cookie, csrf := e.login()
	got := e.do(req{path: "/api/v1/vpn/killswitch", cookie: cookie})
	if got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != `{"enabled":true,"zone":true}` {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	post := func(body string) *httptest.ResponseRecorder {
		return e.do(req{method: "POST", path: "/api/v1/vpn/killswitch", body: body, cookie: cookie, csrf: csrf})
	}
	if got := post(`{}`); got.Code != http.StatusBadRequest && got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing enabled: %d %s", got.Code, got.Body)
	}
	fake.st = txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", Started: time.Unix(0, 0).UTC(), Summary: "Enable VPN kill switch", WindowSeconds: 60}, State: txn.Pending}
	if got := post(`{"enabled":true}`); got.Code != http.StatusAccepted || !strings.Contains(got.Body.String(), `"state":"pending"`) {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	fake.st = txn.Status{}
	for err, code := range map[error]string{
		vpnkill.ErrNotConnected:    "vpn_not_connected",
		vpnkill.ErrUnsupported:     "vpn_killswitch_unsupported",
		txn.ErrActive:              "transaction_pending",
		txn.ErrNoChanges:           "vpn_no_change",
		errors.New("uci exploded"): "vpn_killswitch_unavailable",
		vpnkill.ErrUnavailable:     "vpn_killswitch_unavailable",
	} {
		fake.err = err
		if got := post(`{"enabled":false}`); errCode(t, got) != code {
			t.Fatalf("%v: %d %s", err, got.Code, got.Body)
		}
	}
}

func TestVPNKillSwitchNilDependency(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, _ := e.login()
	if got := e.do(req{path: "/api/v1/vpn/killswitch", cookie: cookie}); got.Code != http.StatusServiceUnavailable || errCode(t, got) != "vpn_killswitch_unavailable" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}
