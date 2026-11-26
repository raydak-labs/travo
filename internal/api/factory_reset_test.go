package api

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/apitokens"
	"github.com/blackdark-org/travo-greenfield/internal/factoryreset"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

func resetEnv(t *testing.T) (*env, guard.Dir) {
	t.Helper()
	g := guard.Dir{Path: t.TempDir(), Now: time.Now}
	store := apitokens.New(filepath.Join(t.TempDir(), "api-tokens.json"))
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		d.Tokens = store
		d.FactoryReset = &factoryreset.Service{Device: d.Ubus.(factoryreset.Resetter), Guards: g}
	})
	return e, g
}

func TestFactoryResetNeedsThePhraseThenSchedulesOnce(t *testing.T) {
	e, _ := resetEnv(t)
	cookie, csrf := e.login()
	const path = "/api/v1/system/factory-reset"
	for _, body := range []string{`{"confirm":"yes"}`, `{"confirm":"Factory Reset"}`, `{"confirm":true}`, `{}`} {
		rec := e.do(req{method: "POST", path: path, body: body, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body)
		}
	}
	if e.dev.FactoryResets != 0 {
		t.Fatal("scheduled without the phrase")
	}
	rec := e.do(req{method: "POST", path: path, body: `{"confirm":"factory reset"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || e.dev.FactoryResets != 1 {
		t.Fatalf("%d %s resets %d", rec.Code, rec.Body, e.dev.FactoryResets)
	}
	rec = e.do(req{method: "POST", path: path, body: `{"confirm":"factory reset"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "factory_reset_pending" || e.dev.FactoryResets != 1 {
		t.Fatalf("again: %d %s", rec.Code, rec.Body)
	}
}

func TestFactoryResetRefusedWhileAnotherGuardIsHeld(t *testing.T) {
	e, g := resetEnv(t)
	if err := g.Acquire("reboot-in-progress"); err != nil {
		t.Fatal(err)
	}
	cookie, csrf := e.login()
	rec := e.do(req{method: "POST", path: "/api/v1/system/factory-reset", body: `{"confirm":"factory reset"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "operation_in_progress" || e.dev.FactoryResets != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestNoTokenMayFactoryReset(t *testing.T) {
	e, _ := resetEnv(t)
	_, admin := e.mint("admin")
	rec := e.do(req{method: "POST", path: "/api/v1/system/factory-reset", body: `{"confirm":"factory reset"}`, auth: "Bearer " + admin})
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "session_required" || e.dev.FactoryResets != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
