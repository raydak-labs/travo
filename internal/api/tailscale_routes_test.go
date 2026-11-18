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
