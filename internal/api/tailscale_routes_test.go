package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/tailscale"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeTailscale struct {
	status    tailscale.Status
	statusErr error
	err       error
	calls     []string
}

func (f *fakeTailscale) Status(context.Context) (tailscale.Status, error) {
	return f.status, f.statusErr
}

func (f *fakeTailscale) Install(context.Context) error {
	f.calls = append(f.calls, "install")
	return f.err
}

func (f *fakeTailscale) Login(context.Context) (tailscale.Status, error) {
	f.calls = append(f.calls, "login")
	return f.status, f.err
}

func (f *fakeTailscale) Logout(context.Context) (tailscale.Status, error) {
	f.calls = append(f.calls, "logout")
	return f.status, f.err
}

func (f *fakeTailscale) SetSettings(_ context.Context, in tailscale.Settings) (tailscale.Status, error) {
	f.calls = append(f.calls, fmt.Sprintf("settings %+v", in))
	return f.status, f.err
}

func (f *fakeTailscale) ApplyFirewall(context.Context) (txn.Status, error) {
	f.calls = append(f.calls, "firewall")
	if f.err != nil {
		return txn.Status{}, f.err
	}
	return txn.Status{ID: "0123456789abcdef", State: txn.Pending, Configs: []string{"firewall"}}, nil
}

func TestTailscaleSettingsRoute(t *testing.T) {
	fake := &fakeTailscale{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Tailscale = fake })
	cookie, csrf := e.login()
	put := func(body string) int {
		return e.do(req{method: "PUT", path: "/api/v1/tailscale/settings", body: body, cookie: cookie, csrf: csrf}).Code
	}
	for _, bad := range []string{`{}`, `{"exit_node":"","advertise_lan":true}`, `{"exit_node":null,"advertise_lan":true,"ssh":false}`,
		`{"exit_node":"","advertise_lan":true,"ssh":false,"extra":1}`, `{"exit_node":"","advertise_lan":true,"ssh":false} {}`,
		`{"Exit_Node":"","advertise_lan":true,"ssh":false}`} {
		if code := put(bad); code != 400 {
			t.Fatalf("%s: %d", bad, code)
		}
	}
	if len(fake.calls) != 0 {
		t.Fatalf("invalid bodies reached the service: %v", fake.calls)
	}
	if code := put(`{"exit_node":"nA","advertise_lan":true,"ssh":false}`); code != 200 {
		t.Fatalf("%d", code)
	}
	if fake.calls[0] != "settings {ExitNode:nA AdvertiseLAN:true SSH:false}" {
		t.Fatalf("calls %v", fake.calls)
	}
	for _, tc := range []struct {
		err  error
		want string
	}{
		{tailscale.ErrExitNodeUnknown, "tailscale_exit_node_unknown"},
		{tailscale.ErrExitNodeConflict, "tailscale_exit_node_conflict"},
		{tailscale.ErrExitNodeOffline, "tailscale_exit_node_offline"},
		{tailscale.ErrNoLAN, "tailscale_no_lan"},
	} {
		fake.err = tc.err
		got := e.do(req{method: "PUT", path: "/api/v1/tailscale/settings", body: `{"exit_node":"","advertise_lan":false,"ssh":false}`, cookie: cookie, csrf: csrf})
		if got.Code != 409 || errCode(t, got) != tc.want {
			t.Fatalf("%v: %d %s", tc.err, got.Code, got.Body)
		}
	}
}

func TestTailscaleFirewallRoute(t *testing.T) {
	fake := &fakeTailscale{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Tailscale = fake })
	cookie, csrf := e.login()
	got := e.do(req{method: "POST", path: "/api/v1/tailscale/firewall", cookie: cookie, csrf: csrf})
	if got.Code != http.StatusAccepted {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	for _, tc := range []struct {
		err  error
		want string
	}{
		{txn.ErrActive, "transaction_pending"},
		{txn.ErrNoChanges, "tailscale_no_change"},
		{tailscale.ErrFirewallConflict, "tailscale_firewall_conflict"},
	} {
		fake.err = tc.err
		got := e.do(req{method: "POST", path: "/api/v1/tailscale/firewall", cookie: cookie, csrf: csrf})
		if got.Code != 409 || errCode(t, got) != tc.want {
			t.Fatalf("%v: %d %s", tc.err, got.Code, got.Body)
		}
	}
}

func TestTailscaleStatusRoute(t *testing.T) {
	fake := &fakeTailscale{status: tailscale.Status{Installed: true, Version: "1.104.1-r1", Running: true,
		Backend: tailscale.Backend{State: tailscale.StateNeedsLogin, AuthURL: "https://login.tailscale.com/a/x",
			Self: tailscale.Node{IPs: []string{}}, ExitNodes: []tailscale.ExitNode{}, Health: []string{},
			Prefs: tailscale.Prefs{AdvertiseRoutes: []string{}}}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Tailscale = fake })
	if got := e.do(req{path: "/api/v1/tailscale"}); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", got.Code)
	}
	cookie, _ := e.login()
	got := e.do(req{path: "/api/v1/tailscale", cookie: cookie})
	var body map[string]any
	if err := json.Unmarshal(got.Body.Bytes(), &body); err != nil || got.Code != 200 {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if body["backend_state"] != "NeedsLogin" || body["auth_url"] != "https://login.tailscale.com/a/x" || body["installed"] != true {
		t.Fatalf("body %v", body)
	}
	if _, ok := body["exit_nodes"].([]any); !ok {
		t.Fatalf("exit_nodes must encode [] : %s", got.Body)
	}
	fake.statusErr = errors.New("apk: database locked")
	got = e.do(req{path: "/api/v1/tailscale", cookie: cookie})
	if got.Code != 503 || errCode(t, got) != "tailscale_unavailable" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}

func TestTailscaleActionRoutes(t *testing.T) {
	fake := &fakeTailscale{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Tailscale = fake })
	cookie, csrf := e.login()
	paths := []string{"/api/v1/tailscale/install", "/api/v1/tailscale/login", "/api/v1/tailscale/logout"}
	for _, p := range paths {
		if got := e.do(req{method: "POST", path: p, cookie: cookie}); got.Code != http.StatusForbidden {
			t.Fatalf("%s missing CSRF: %d", p, got.Code)
		}
	}
	for _, tc := range []struct {
		err  error
		code int
		want string
	}{
		{nil, 200, ""},
		{pkgmgr.ErrNotInstalled, 503, "tailscale_not_installed"},
		{tailscale.ErrNotRunning, 503, "tailscale_not_running"},
		{tailscale.ErrBusy, 409, "tailscale_busy"},
		{fmt.Errorf("x: %w", guard.ErrActive), 409, "tailscale_busy"},
		{tailscale.ErrOffline, 409, "tailscale_offline"},
		{fmt.Errorf("%w: 10 MiB free", tailscale.ErrNoSpace), 409, "tailscale_no_space"},
		{errors.New("backend error"), 503, "tailscale_failed"},
	} {
		fake.err = tc.err
		for _, p := range paths {
			got := e.do(req{method: "POST", path: p, cookie: cookie, csrf: csrf})
			if got.Code != tc.code || (tc.want != "" && errCode(t, got) != tc.want) {
				t.Fatalf("%s %v: %d %s", p, tc.err, got.Code, got.Body)
			}
		}
	}
	if len(fake.calls) != 3*8 {
		t.Fatalf("calls %d", len(fake.calls))
	}
}

func TestTailscaleRoutesWithoutService(t *testing.T) {
	e := newEnv(t)
	cookie, csrf := e.login()
	if got := e.do(req{path: "/api/v1/tailscale", cookie: cookie}); got.Code != 503 {
		t.Fatalf("%d", got.Code)
	}
	if got := e.do(req{method: "POST", path: "/api/v1/tailscale/login", cookie: cookie, csrf: csrf}); got.Code != 503 {
		t.Fatalf("%d", got.Code)
	}
}
