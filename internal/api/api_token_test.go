package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/apitokens"
)

func newTokenEnv(t *testing.T) *env {
	t.Helper()
	store := apitokens.New(filepath.Join(t.TempDir(), "api-tokens.json"))
	return newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Tokens = store })
}

// mint creates a token through the API with a cookie session and returns the raw secret.
func (e *env) mint(scope string) (id, secret string) {
	e.t.Helper()
	cookie, csrf := e.login()
	rec := e.do(req{method: "POST", path: "/api/v1/auth/tokens", body: `{"name":"ci","scope":"` + scope + `"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	return bodyField(e.t, rec, "id"), bodyField(e.t, rec, "token")
}

func TestAPITokenLifecycleThroughTheAPI(t *testing.T) {
	e := newTokenEnv(t)
	id, secret := e.mint("read")
	if !strings.HasPrefix(secret, apitokens.Prefix) {
		t.Fatalf("secret %q", secret)
	}
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/auth/tokens", cookie: cookie})
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), secret) || strings.Contains(rec.Body.String(), "hash") {
		t.Fatalf("list must not leak the secret or hash: %d %s", rec.Code, rec.Body)
	}
	var list apiTokenList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Tokens) != 1 || list.Tokens[0].ID != id {
		t.Fatalf("list: %s", rec.Body)
	}
	if rec := e.do(req{method: "GET", path: "/api/v1/system/info", auth: "Bearer " + secret}); rec.Code == http.StatusUnauthorized {
		t.Fatalf("bearer read refused: %s", rec.Body)
	}
	rec = e.do(req{method: "DELETE", path: "/api/v1/auth/tokens/" + id, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "GET", path: "/api/v1/system/info", auth: "Bearer " + secret})
	if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthenticated" {
		t.Fatalf("revoked token: %d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "DELETE", path: "/api/v1/auth/tokens/" + id, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second delete: %d", rec.Code)
	}
}

func TestReadTokenCannotWriteOrFetchSecrets(t *testing.T) {
	e := newTokenEnv(t)
	_, secret := e.mint("read")
	for _, r := range []req{
		{method: "POST", path: "/api/v1/transactions/" + pendingTxn + "/confirm", body: "{}"},
		{method: "GET", path: "/api/v1/system/backup"},
	} {
		r.auth = "Bearer " + secret
		rec := e.do(r)
		if rec.Code != http.StatusForbidden || errCode(t, rec) != "insufficient_scope" {
			t.Errorf("%s %s: %d %s", r.method, r.path, rec.Code, rec.Body)
		}
	}
}

func TestAdminTokenWritesWithoutCSRF(t *testing.T) {
	e := newTokenEnv(t)
	_, secret := e.mint("admin")
	rec := e.do(req{method: "POST", path: "/api/v1/transactions/" + pendingTxn + "/confirm", body: "{}", auth: "Bearer " + secret})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin confirm: %d %s", rec.Code, rec.Body)
	}
}

func TestNoTokenReachesSessionOnlyRoutes(t *testing.T) {
	e := newTokenEnv(t)
	_, secret := e.mint("admin")
	sessionOnly := 0
	for key, op := range operations(e.api) {
		if !isSessionOnly(op) {
			continue
		}
		sessionOnly++
		method, path, _ := strings.Cut(key, " ")
		path = strings.ReplaceAll(path, "{id}", strings.Repeat("a", 32))
		rec := e.do(req{method: method, path: path, body: `{"name":"x","scope":"admin","current":"a","new":"b"}`, auth: "Bearer " + secret})
		if rec.Code != http.StatusForbidden || errCode(t, rec) != "session_required" {
			t.Errorf("%s: %d %s", key, rec.Code, rec.Body)
		}
	}
	want := map[string]bool{"getAPITokens": true, "createAPIToken": true, "deleteAPIToken": true, "postPassword": true, "postLogout": true, "getSession": true,
		"addSSHKey": true, "deleteSSHKey": true, "postFactoryReset": true,
		"getFirmware": true, "postFirmware": true, "postFirmwareApply": true}
	if sessionOnly != len(want) {
		t.Fatalf("session-only routes: %d, want %d", sessionOnly, len(want))
	}
	for _, op := range operations(e.api) {
		if want[op.OperationID] != isSessionOnly(op) {
			t.Errorf("%s session-only=%v", op.OperationID, isSessionOnly(op))
		}
	}
}

func TestBadBearerHeadersAre401(t *testing.T) {
	e := newTokenEnv(t)
	_, secret := e.mint("admin")
	cookie, _ := e.login()
	for _, h := range []string{"Bearer", "Bearer ", "bearer " + secret, "Basic " + secret, "Bearer tgt_forged", "Bearer " + cookie} {
		// A cookie next to a bad header must not rescue it: the header decides.
		rec := e.do(req{method: "GET", path: "/api/v1/system/info", auth: h, cookie: cookie})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%q: %d", h, rec.Code)
		}
	}
}

func TestBearerStillNeedsLANAndHost(t *testing.T) {
	e := newTokenEnv(t)
	_, secret := e.mint("admin")
	if rec := e.do(req{method: "GET", path: "/api/v1/system/info", auth: "Bearer " + secret, remote: "203.0.113.9:1234"}); rec.Code != http.StatusForbidden {
		t.Fatalf("off-LAN: %d", rec.Code)
	}
	if rec := e.do(req{method: "GET", path: "/api/v1/system/info", auth: "Bearer " + secret, host: "evil.test"}); rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("rebinding host: %d", rec.Code)
	}
}

func TestTokensSurvivePasswordChange(t *testing.T) {
	e := newTokenEnv(t)
	_, secret := e.mint("read")
	cookie, csrf := e.login()
	rec := e.do(req{method: "POST", path: "/api/v1/auth/password", body: `{"current":"` + rootPW + `","new":"another-horse"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK {
		t.Fatalf("password: %d %s", rec.Code, rec.Body)
	}
	if rec := e.do(req{method: "GET", path: "/api/v1/system/info", auth: "Bearer " + secret}); rec.Code == http.StatusUnauthorized {
		t.Fatal("token revoked by password change")
	}
}

func TestTokenRoutesWithoutStoreAre503AndBearerIs401(t *testing.T) {
	e := newEnv(t)
	cookie, csrf := e.login()
	rec := e.do(req{method: "POST", path: "/api/v1/auth/tokens", body: `{"name":"x","scope":"read"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "tokens_unavailable" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if rec := e.do(req{method: "GET", path: "/api/v1/system/info", auth: "Bearer tgt_x"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bearer: %d", rec.Code)
	}
}

func TestCreateTokenValidation(t *testing.T) {
	e := newTokenEnv(t)
	cookie, csrf := e.login()
	for _, body := range []string{`{"name":"","scope":"read"}`, `{"name":"x","scope":"root"}`, `{"name":"a\nb","scope":"read"}`, `{"name":"x"}`} {
		rec := e.do(req{method: "POST", path: "/api/v1/auth/tokens", body: body, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_request" {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body)
		}
	}
}
