package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type proxyReq struct {
	method, path, remote, host, origin, cookie string
}

func TestAdGuardProxyGate(t *testing.T) {
	var seen *http.Request
	agh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		_, _ = io.WriteString(w, "agh:"+r.Method+" "+r.URL.Path)
	}))
	defer agh.Close()
	target, _ := url.Parse(agh.URL)
	e := newEnv(t)
	cookie, _ := e.login()
	p := NewAdGuardProxy(e.deps, target)
	do := func(r proxyReq) *httptest.ResponseRecorder {
		if r.method == "" {
			r.method = http.MethodGet
		}
		hr := httptest.NewRequest(r.method, "http://x"+r.path, strings.NewReader("{}"))
		hr.RemoteAddr, hr.Host = lanAddr, "192.168.1.1:3001"
		if r.remote != "" {
			hr.RemoteAddr = r.remote
		}
		if r.host != "" {
			hr.Host = r.host
		}
		if r.origin != "" {
			hr.Header.Set("Origin", r.origin)
		}
		if r.cookie != "" {
			hr.Header.Set("Cookie", r.cookie)
		}
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, hr)
		return rec
	}
	auth := "theme=dark; " + cookieName + "=" + cookie
	for _, tc := range []struct {
		name string
		r    proxyReq
		code int
	}{
		{"no cookie", proxyReq{path: "/"}, 401},
		{"bad session", proxyReq{path: "/", cookie: cookieName + "=nope"}, 401},
		{"foreign source", proxyReq{path: "/", cookie: auth, remote: "203.0.113.9:5000"}, 403},
		{"rebinding host", proxyReq{path: "/", cookie: auth, host: "evil.example:3001"}, 421},
		{"cross-origin post", proxyReq{method: "POST", path: "/control/filtering/refresh", cookie: auth, origin: "http://evil.example"}, 403},
		{"post without origin", proxyReq{method: "POST", path: "/control/filtering/refresh", cookie: auth}, 403},
	} {
		seen = nil
		if got := do(tc.r); got.Code != tc.code || seen != nil {
			t.Errorf("%s: %d %s (forwarded=%v)", tc.name, got.Code, got.Body, seen != nil)
		}
	}
	got := do(proxyReq{method: "POST", path: "/control/filtering/refresh", cookie: auth, origin: "http://192.168.1.1:3001"})
	if got.Code != 200 || got.Body.String() != "agh:POST /control/filtering/refresh" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if c := seen.Header.Get("Cookie"); c != "theme=dark" {
		t.Fatalf("Travo session leaked to AGH: %q", c)
	}
	if seen.Host != "192.168.1.1:3001" || seen.Header.Get("X-Forwarded-For") != "192.168.1.50" {
		t.Fatalf("host=%q xff=%q", seen.Host, seen.Header.Get("X-Forwarded-For"))
	}
	agh.Close()
	if got := do(proxyReq{path: "/", cookie: auth}); got.Code != http.StatusBadGateway || !strings.Contains(got.Body.String(), "adguardhome") {
		t.Fatalf("AGH down: %d %s", got.Code, got.Body)
	}
}
