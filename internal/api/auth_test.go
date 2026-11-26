package api

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/auth"
)

func TestDefaultDenyWalk(t *testing.T) {
	e := newEnv(t)
	public := map[string]bool{"getHealth": true, "getOpenAPI": true, "postLogin": true, "getSetupState": true, "postSetupPassword": true}
	for key, op := range operations(e.api) {
		method, path, _ := strings.Cut(key, " ")
		path = strings.NewReplacer("{id}", pendingTxn).Replace(path)
		if isPublic(op) != public[op.OperationID] {
			t.Errorf("%s: public=%v, expected %v (adding a public route is a security change)", op.OperationID, isPublic(op), public[op.OperationID])
		}
		if isPublic(op) {
			continue
		}
		rec := e.do(req{method: method, path: path, body: "{}"})
		if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthenticated" {
			t.Errorf("%s %s without cookie: %d %s", method, path, rec.Code, rec.Body)
		}
		rec = e.do(req{method: method, path: path, body: "{}", cookie: "forged-token"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with forged cookie: %d", method, path, rec.Code)
		}
	}
}

func TestPublicSetIsExactlyTheContractList(t *testing.T) {
	e := newEnv(t)
	got := map[string]bool{}
	for _, op := range operations(e.api) {
		if isPublic(op) {
			got[op.OperationID] = true
		}
	}
	// getOpenAPI is served by Huma itself, outside the walk; it is public by the trust-only path
	delete(got, "getOpenAPI")
	if len(got) != 4 {
		t.Fatalf("public ops: %v", got)
	}
}

func TestLoginCookieFlagsAndBody(t *testing.T) {
	e := newEnv(t)
	for _, tlsOn := range []bool{false, true} {
		rec := e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"` + rootPW + `"}`, tls: tlsOn})
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		var c *http.Cookie
		for _, x := range rec.Result().Cookies() {
			if x.Name == cookieName {
				c = x
			}
		}
		if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Secure != tlsOn {
			t.Fatalf("tls=%v cookie %+v", tlsOn, c)
		}
		if bodyField(t, rec, "csrf_token") == "" || !strings.Contains(rec.Body.String(), `"expires_in":86400`) {
			t.Fatalf("body %s", rec.Body)
		}
		if strings.Contains(rec.Body.String(), rootPW) {
			t.Fatal("password echoed")
		}
	}
}

func TestLoginWrongPasswordAndUnsetAndDeviceError(t *testing.T) {
	e := newEnv(t)
	rec := e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"nope-nope"}`})
	if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "invalid_credentials" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("cookie set on failed login")
	}
	e.dev.SetPassword("")
	rec = e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"anything1"}`})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "setup_required" {
		t.Fatalf("unset: %d %s", rec.Code, rec.Body)
	}
	e.dev.AccountsErr = errTest
	rec = e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"anything1"}`})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("device error: %d %s (never 500, never a login)", rec.Code, rec.Body)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		rec := e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"wrong-wrong"}`})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, rec.Code)
		}
	}
	rec := e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"` + rootPW + `"}`})
	if rec.Code != http.StatusTooManyRequests || errCode(t, rec) != "rate_limited" {
		t.Fatalf("6th attempt: %d %s", rec.Code, rec.Body)
	}
	other := e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"` + rootPW + `"}`, remote: "192.168.1.77:1"})
	if other.Code != http.StatusOK {
		t.Fatalf("another IP must not be limited: %d", other.Code)
	}
	e.clock.Advance(time.Minute)
	if rec := e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"` + rootPW + `"}`}); rec.Code != http.StatusOK {
		t.Fatalf("after a minute: %d", rec.Code)
	}
}

func TestCSRFRequiredOnUnsafeMethodsOnly(t *testing.T) {
	e := newEnv(t)
	cookie, csrf := e.login()
	if rec := e.do(req{method: "GET", path: "/api/v1/auth/session", cookie: cookie}); rec.Code != http.StatusOK {
		t.Fatalf("GET needs no csrf: %d", rec.Code)
	}
	path := "/api/v1/transactions/" + pendingTxn + "/confirm"
	for name, token := range map[string]string{"missing": "", "wrong": "x" + csrf[1:], "other session": "abc"} {
		rec := e.do(req{method: "POST", path: path, cookie: cookie, csrf: token})
		if rec.Code != http.StatusForbidden || errCode(t, rec) != "csrf" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if rec := e.do(req{method: "POST", path: path, cookie: cookie, csrf: csrf}); rec.Code != http.StatusOK {
		t.Fatalf("valid csrf: %d %s", rec.Code, rec.Body)
	}
}

func TestSessionEndpointRecoversCSRFAndExpires(t *testing.T) {
	e := newEnv(t)
	cookie, csrf := e.login()
	e.clock.Advance(time.Hour)
	rec := e.do(req{method: "GET", path: "/api/v1/auth/session", cookie: cookie})
	if bodyField(t, rec, "csrf_token") != csrf || !strings.Contains(rec.Body.String(), `"expires_in":82800`) {
		t.Fatalf("%s", rec.Body)
	}
	e.clock.Advance(auth.Lifetime)
	if rec := e.do(req{method: "GET", path: "/api/v1/auth/session", cookie: cookie}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired session accepted: %d", rec.Code)
	}
}

func TestLogoutRevokesAndClearsCookie(t *testing.T) {
	e := newEnv(t)
	cookie, csrf := e.login()
	rec := e.do(req{method: "POST", path: "/api/v1/auth/logout", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || bodyField(t, rec, "status") != "ok" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	cleared := false
	for _, c := range rec.Result().Cookies() {
		cleared = cleared || (c.Name == cookieName && c.MaxAge < 0 && c.HttpOnly)
	}
	if !cleared {
		t.Fatal("cookie not cleared")
	}
	if rec := e.do(req{method: "GET", path: "/api/v1/auth/session", cookie: cookie}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked cookie still works: %d", rec.Code)
	}
}

func TestPasswordChangeRevokesOldAndIssuesWorkingSession(t *testing.T) {
	e := newEnv(t)
	oldCookie, oldCSRF := e.login()
	otherCookie, _ := e.login()
	body := `{"current":"` + rootPW + `","new":"brand-new-pass"}`
	rec := e.do(req{method: "POST", path: "/api/v1/auth/password", body: body, cookie: oldCookie, csrf: oldCSRF})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(e.epoch) != 1 || e.epoch[0] != 1 {
		t.Fatalf("epoch persisted: %v", e.epoch)
	}
	for name, c := range map[string]string{"changing session": oldCookie, "other session": otherCookie} {
		if r := e.do(req{method: "GET", path: "/api/v1/auth/session", cookie: c}); r.Code != http.StatusUnauthorized {
			t.Errorf("%s survived the password change: %d", name, r.Code)
		}
	}
	newCookie, newCSRF := cookieOf(t, rec), bodyField(t, rec, "csrf_token")
	if r := e.do(req{method: "GET", path: "/api/v1/auth/session", cookie: newCookie}); r.Code != http.StatusOK || bodyField(t, r, "csrf_token") != newCSRF {
		t.Fatalf("replacement session unusable: %d %s", r.Code, r.Body)
	}
	login := e.do(req{method: "POST", path: "/api/v1/auth/login", body: `{"password":"brand-new-pass"}`})
	if login.Code != http.StatusOK {
		t.Fatalf("new password does not log in: %d", login.Code)
	}
	if strings.Contains(rec.Body.String(), "brand-new-pass") {
		t.Fatal("password echoed")
	}
}

func TestPasswordChangeRefusals(t *testing.T) {
	e := newEnv(t)
	cookie, csrf := e.login()
	call := func(body string) int {
		return e.do(req{method: "POST", path: "/api/v1/auth/password", body: body, cookie: cookie, csrf: csrf}).Code
	}
	if c := call(`{"current":"wrong-wrong","new":"brand-new-pass"}`); c != http.StatusForbidden {
		t.Errorf("wrong current: %d", c)
	}
	if c := call(`{"current":"` + rootPW + `","new":"short"}`); c != http.StatusBadRequest {
		t.Errorf("short new: %d", c)
	}
	if c := call(`{"current":"` + rootPW + `","new":"line\nbreak-pass"}`); c != http.StatusBadRequest {
		t.Errorf("newline: %d", c)
	}
	if e.acc.sets != 0 || len(e.epoch) != 0 {
		t.Fatal("a refused change wrote something")
	}
	if r := e.do(req{method: "GET", path: "/api/v1/auth/session", cookie: cookie}); r.Code != http.StatusOK {
		t.Fatal("a refused change must keep the session")
	}
}

func TestPasswordChangeEpochPersistFailureDoesNotLeaveSessionsAlive(t *testing.T) {
	e := newEnv(t)
	cookie, csrf := e.login()
	e.saveErr = errTest
	rec := e.do(req{method: "POST", path: "/api/v1/auth/password", cookie: cookie, csrf: csrf,
		body: `{"current":"` + rootPW + `","new":"brand-new-pass"}`})
	if rec.Code != http.StatusServiceUnavailable || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if r := e.do(req{method: "GET", path: "/api/v1/auth/session", cookie: cookie}); r.Code != http.StatusUnauthorized {
		t.Fatal("old session alive although the password changed")
	}
}

func TestSetupOnlyWhenUnsetAndOnlyOnce(t *testing.T) {
	e := newEnv(t)
	if rec := e.do(req{method: "GET", path: "/api/v1/setup"}); bodyBool(t, rec, "password_set") != true {
		t.Fatalf("%s", rec.Body)
	}
	rec := e.do(req{method: "POST", path: "/api/v1/setup/password", body: `{"password":"takeover-pass"}`})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "already_set" || e.acc.sets != 0 {
		t.Fatalf("setup on a configured router: %d %s", rec.Code, rec.Body)
	}

	e.dev.SetPassword("")
	if rec := e.do(req{method: "GET", path: "/api/v1/setup"}); bodyBool(t, rec, "password_set") != false {
		t.Fatalf("%s", rec.Body)
	}
	if rec := e.do(req{method: "POST", path: "/api/v1/setup/password", body: `{"password":"short"}`}); rec.Code != http.StatusBadRequest {
		t.Fatalf("short: %d", rec.Code)
	}
	rec = e.do(req{method: "POST", path: "/api/v1/setup/password", body: `{"password":"first-password"}`})
	if rec.Code != http.StatusOK || cookieOf(t, rec) == "" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := e.do(req{method: "POST", path: "/api/v1/setup/password", body: `{"password":"second-password"}`}); rec.Code != http.StatusConflict {
		t.Fatalf("second setup: %d", rec.Code)
	}
}

func TestSetupConcurrentExactlyOneWins(t *testing.T) {
	e := newEnv(t)
	e.dev.SetPassword("")
	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// distinct source IPs: the rate limiter must not be what serialises them
			r := req{method: "POST", path: "/api/v1/setup/password", body: `{"password":"racer-password"}`, remote: "192.168.1." + string(rune('1'+i)) + "0:9"}
			codes <- e.do(r).Code
		}(i)
	}
	wg.Wait()
	close(codes)
	ok, conflict := 0, 0
	for c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		}
	}
	if ok != 1 || conflict != 3 || e.acc.sets != 1 {
		t.Fatalf("ok=%d conflict=%d sets=%d", ok, conflict, e.acc.sets)
	}
}

func TestSetupFailsClosedWhenShadowUnreadable(t *testing.T) {
	e := newEnv(t)
	e.dev.SetPassword("")
	e.dev.AccountsErr = errTest
	if rec := e.do(req{method: "POST", path: "/api/v1/setup/password", body: `{"password":"first-password"}`}); rec.Code != http.StatusServiceUnavailable || e.acc.sets != 0 {
		t.Fatalf("%d", rec.Code)
	}
	if rec := e.do(req{method: "GET", path: "/api/v1/setup"}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("setup state on read error: %d", rec.Code)
	}
}

func TestSetupRateLimited(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		e.do(req{method: "POST", path: "/api/v1/setup/password", body: `{"password":"x"}`})
	}
	rec := e.do(req{method: "POST", path: "/api/v1/setup/password", body: `{"password":"valid-password"}`})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("%d", rec.Code)
	}
}
