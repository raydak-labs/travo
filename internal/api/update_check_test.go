package api

import (
	"net/http"
	"testing"
)

func TestUpdateCheckAuthentication(t *testing.T) {
	e := newEnv(t)
	r := req{method: "POST", path: "/api/v1/system/update/check", body: `{"mode":"manual"}`}
	if got := e.do(r); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d %s", got.Code, got.Body)
	}
	cookie, csrf := e.login()
	r.cookie = cookie
	if got := e.do(r); got.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF: %d %s", got.Code, got.Body)
	}
	r.csrf = csrf
	if got := e.do(r); got.Code != http.StatusOK || bodyField(t, got, "status") != "disabled" {
		t.Fatalf("authenticated: %d %s", got.Code, got.Body)
	}
	r.body = `{"mode":"invalid"}`
	if got := e.do(r); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode: %d %s", got.Code, got.Body)
	}
}
