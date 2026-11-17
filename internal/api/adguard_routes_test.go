package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

type fakeAdGuard struct {
	status     adguard.Status
	statusErr  error
	installErr error
	installs   []bool
	toggleErr  error
	toggles    []string
}

func (f *fakeAdGuard) SetDefaultDNS(_ context.Context, on bool) error {
	f.toggles = append(f.toggles, fmt.Sprint("dns ", on))
	return f.toggleErr
}

func (f *fakeAdGuard) SetEnabled(_ context.Context, on bool) error {
	f.toggles = append(f.toggles, fmt.Sprint("service ", on))
	return f.toggleErr
}

func (f *fakeAdGuard) Status(context.Context) (adguard.Status, error) { return f.status, f.statusErr }
func (f *fakeAdGuard) Install(_ context.Context, explicit bool) error {
	f.installs = append(f.installs, explicit)
	return f.installErr
}

func TestAdGuardStatusRoute(t *testing.T) {
	fake := &fakeAdGuard{status: adguard.Status{Installed: true, Version: "0.107.79-r1", Running: true, Loopback: true}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.AdGuard = fake })
	if got := e.do(req{path: "/api/v1/adguard"}); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", got.Code)
	}
	cookie, _ := e.login()
	got := e.do(req{path: "/api/v1/adguard", cookie: cookie})
	want := `{"installed":true,"version":"0.107.79-r1","running":true,"loopback":true,"busy":false,"default_dns":false,"fail_open_since":null}`
	if got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != want {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	fake.statusErr = errors.New("apk: database locked")
	got = e.do(req{path: "/api/v1/adguard", cookie: cookie})
	if got.Code != http.StatusServiceUnavailable || errCode(t, got) != "adguard_unavailable" {
		t.Fatalf("a failed presence check is 503, never absent: %d %s", got.Code, got.Body)
	}
	if !strings.Contains(got.Body.String(), "adguardhome") {
		t.Fatalf("503 must name the package: %s", got.Body)
	}
}

func TestAdGuardInstallRoute(t *testing.T) {
	fake := &fakeAdGuard{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.AdGuard = fake })
	cookie, csrf := e.login()
	if got := e.do(req{method: "POST", path: "/api/v1/adguard/install", cookie: cookie}); got.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF: %d", got.Code)
	}
	for _, tc := range []struct {
		err  error
		code int
		want string
	}{
		{nil, 200, ""},
		{adguard.ErrBusy, 409, "adguard_busy"},
		{adguard.ErrOffline, 409, "adguard_offline"},
		{fmt.Errorf("%w: 10 MiB free", adguard.ErrNoSpace), 409, "adguard_no_space"},
		{adguard.ErrUnknownShape, 409, "adguard_config_unknown"},
		{errors.New("apk: fetch failed"), 503, "adguard_install_failed"},
	} {
		fake.installErr = tc.err
		got := e.do(req{method: "POST", path: "/api/v1/adguard/install", cookie: cookie, csrf: csrf})
		if got.Code != tc.code || (tc.want != "" && errCode(t, got) != tc.want) {
			t.Fatalf("%v: %d %s", tc.err, got.Code, got.Body)
		}
	}
	for _, explicit := range fake.installs {
		if !explicit {
			t.Fatal("the API install is always the explicit retry")
		}
	}
}

func TestAdGuardRoutesWithoutService(t *testing.T) {
	e := newEnv(t)
	cookie, _ := e.login()
	if got := e.do(req{path: "/api/v1/adguard", cookie: cookie}); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d", got.Code)
	}
}

func TestAdGuardToggleRoutes(t *testing.T) {
	fake := &fakeAdGuard{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.AdGuard = fake })
	cookie, csrf := e.login()
	post := func(path, body string) *httptest.ResponseRecorder {
		return e.do(req{method: "POST", path: path, body: body, cookie: cookie, csrf: csrf})
	}
	for _, path := range []string{"/api/v1/adguard/dns", "/api/v1/adguard/service"} {
		if got := post(path, `{}`); got.Code != 400 && got.Code != 422 {
			t.Fatalf("%s missing enabled: %d %s", path, got.Code, got.Body)
		}
		if got := post(path, `{"enabled":true}`); got.Code != 200 {
			t.Fatalf("%s: %d %s", path, got.Code, got.Body)
		}
	}
	if strings.Join(fake.toggles, ",") != "dns true,service true" {
		t.Fatalf("toggles = %v", fake.toggles)
	}
	for _, tc := range []struct {
		err  error
		code int
		want string
	}{
		{pkgmgr.ErrNotInstalled, 409, "adguard_not_installed"},
		{fmt.Errorf("%w: timeout", adguard.ErrNotAnswering), 409, "adguard_not_answering"},
		{adguard.ErrBusy, 409, "adguard_busy"},
		{fmt.Errorf("dns layers: %w", guard.ErrActive), 409, "dns_busy"},
		{dnslayer.ErrForeignStaged, 409, "dns_busy"},
		{errors.New("procd: timeout"), 503, "adguard_unavailable"},
	} {
		fake.toggleErr = tc.err
		got := post("/api/v1/adguard/dns", `{"enabled":false}`)
		if got.Code != tc.code || errCode(t, got) != tc.want {
			t.Fatalf("%v: %d %s", tc.err, got.Code, got.Body)
		}
	}
}
