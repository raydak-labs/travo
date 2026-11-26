package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestTransactionEndpoints(t *testing.T) {
	e := newEnv(t)
	cookie, csrf := e.login()
	do := func(method, id, suffix string) (int, string) {
		rec := e.do(req{method: method, path: "/api/v1/transactions/" + id + suffix, cookie: cookie, csrf: csrf})
		return rec.Code, rec.Body.String()
	}
	code, body := do("GET", pendingTxn, "")
	if code != 200 || !strings.Contains(body, `"state":"pending"`) || !strings.Contains(body, `"configs":[]`) {
		t.Fatalf("get: %d %s", code, body)
	}
	if code, body := do("GET", "0000000000000000", ""); code != 404 || !strings.Contains(body, `"not_found"`) {
		t.Fatalf("missing: %d %s", code, body)
	}
	if code, _ := do("GET", "not-an-id", ""); code != http.StatusUnprocessableEntity && code != 400 {
		t.Fatalf("malformed id: %d", code)
	}
	if code, body := do("POST", "fedcba9876543210", "/confirm"); code != 409 || !strings.Contains(body, `"closed"`) {
		t.Fatalf("confirm closed: %d %s", code, body)
	}
	if code, body := do("POST", pendingTxn, "/confirm"); code != 200 || !strings.Contains(body, `"confirmed"`) {
		t.Fatalf("confirm: %d %s", code, body)
	}
	if code, body := do("POST", pendingTxn, "/rollback"); code != 200 || !strings.Contains(body, `"rolled_back"`) {
		t.Fatalf("rollback: %d %s", code, body)
	}
	if code, _ := do("POST", "0000000000000000", "/rollback"); code != 404 {
		t.Fatalf("rollback missing: %d", code)
	}
}
