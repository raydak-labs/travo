package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewAdGuardProxy serves AGH's UI and API from target on its own port, root path, no
// rewriting (ADR 0006, SPIKES S2). AGH has no accounts (`users: []`), so this gate is its
// only lock: LAN source, router Host, a valid Travo session, and for unsafe methods an
// Origin naming this proxy (AGH's own JS cannot send Travo's CSRF header).
func NewAdGuardProxy(d Deps, target *url.URL) http.Handler {
	s := &server{Deps: d, sessions: d.Sessions, clock: d.Clock, trusted: d.Trusted}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = pr.In.Host // as a plain proxy would; AGH serves any Host
			pr.SetXForwarded()
			stripCookie(pr.Out.Header, cookieName)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Warn("adguard proxy", "path", r.URL.Path, "err", err)
			writeJSONError(w, http.StatusBadGateway, "adguard_unavailable", "adguardhome is not answering")
		},
	}
	return recoverPanics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, ok := remoteAddr(r.RemoteAddr)
		if !ok || s.trusted == nil || !s.trusted(ip) {
			writeJSONError(w, http.StatusForbidden, "not_lan", "admin is reachable from the LAN only")
			return
		}
		if !s.hostOK(r.Host) {
			writeJSONError(w, http.StatusMisdirectedRequest, "bad_host", "unexpected Host header")
			return
		}
		c, err := r.Cookie(cookieName)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "log in to Travo first")
			return
		}
		if _, ok := s.sessions.Lookup(c.Value, s.clock.Now()); !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "log in to Travo first")
			return
		}
		if isUnsafe(r.Method) && !sameOrigin(r) {
			writeJSONError(w, http.StatusForbidden, "csrf", "cross-origin request refused")
			return
		}
		rp.ServeHTTP(w, r)
	}))
}

// sameOrigin requires the browser's Origin to be this proxy's own origin.
func sameOrigin(r *http.Request) bool {
	scheme := "http://"
	if r.TLS != nil {
		scheme = "https://"
	}
	want := scheme + r.Host
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("Origin")), []byte(want)) == 1
}

// stripCookie removes one cookie so the Travo session never reaches AGH.
func stripCookie(h http.Header, name string) {
	var keep []string
	for _, line := range h.Values("Cookie") {
		for _, part := range strings.Split(line, ";") {
			if k, _, _ := strings.Cut(strings.TrimSpace(part), "="); k != name && strings.TrimSpace(part) != "" {
				keep = append(keep, strings.TrimSpace(part))
			}
		}
	}
	h.Del("Cookie")
	if len(keep) > 0 {
		h.Set("Cookie", strings.Join(keep, "; "))
	}
}
