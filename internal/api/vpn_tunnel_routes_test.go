package api

import (
	"context"
	"errors"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type setTunnelFunc func(context.Context, bool) (txn.Status, error)

func (f setTunnelFunc) Set(ctx context.Context, enabled bool) (txn.Status, error) {
	return f(ctx, enabled)
}

func TestVPNTunnelMutation(t *testing.T) {
	var change VPNTunnelSetter
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		d.VPNTunnel = setTunnelFunc(func(ctx context.Context, enabled bool) (txn.Status, error) {
			if change == nil {
				return txn.Status{}, errors.New("unavailable")
			}
			return change.Set(ctx, enabled)
		})
	})
	request := req{method: "POST", path: "/api/v1/vpn/tunnel", body: `{"enabled":false}`}
	if got := e.do(request); got.Code != 401 {
		t.Fatal(got.Code)
	}
	cookie, csrf := e.login()
	request.cookie = cookie
	if got := e.do(request); got.Code != 403 {
		t.Fatal(got.Code)
	}
	request.csrf = csrf
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":false,"extra":true}`, `{"enabled":false}{}`} {
		request.body = body
		if got := e.do(request); got.Code != 400 {
			t.Fatal(body, got.Code, got.Body)
		}
	}
	request.body = `{"enabled":false}`
	for _, state := range []txn.State{txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed} {
		change = setTunnelFunc(func(_ context.Context, enabled bool) (txn.Status, error) {
			if enabled {
				t.Fatal("disable decoded as enable")
			}
			result := txn.Status{Meta: txn.Meta{ID: "1234567890abcdef"}, State: state}
			if state == txn.Pending {
				return result, nil
			}
			return result, txn.ErrProbeFailed
		})
		got := e.do(request)
		if got.Code != 202 || bodyField(t, got, "state") != string(state) {
			t.Fatal(got.Code, got.Body)
		}
	}
}

func TestVPNTunnelUnavailable(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, csrf := e.login()
	got := e.do(req{method: "POST", path: "/api/v1/vpn/tunnel", body: `{"enabled":false}`, cookie: cookie, csrf: csrf})
	if got.Code != 503 || bodyField(t, got, "code") != "vpn_tunnel_unavailable" {
		t.Fatal(got.Code, got.Body)
	}
}
