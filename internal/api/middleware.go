package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/apitokens"
	"github.com/blackdark-org/travo-greenfield/internal/auth"
	"github.com/danielgtaylor/huma/v2"
)

const (
	cookieName = "tg_session"
	csrfHeader = "X-CSRF-Token"
	publicKey  = "public"
	// sessionOnlyKey marks routes an API token may never call (token management, password).
	sessionOnlyKey = "session_only"
	adminOnlyKey   = "admin_only"
)

type (
	infoKey   struct{}
	authedKey struct{}
)

// reqInfo is what handlers need from the transport.
type reqInfo struct {
	ip  string // unmapped, no port; the rate-limit key
	tls bool
}

// authed is the validated credential of this request: a cookie session, or an API
// token (bearer set, token is then empty).
type authed struct {
	token  string
	sess   auth.Session
	bearer string
}

func infoOf(ctx context.Context) reqInfo { v, _ := ctx.Value(infoKey{}).(reqInfo); return v }

func authedOf(ctx context.Context) authed { v, _ := ctx.Value(authedKey{}).(authed); return v }

func remoteAddr(hostport string) (netip.Addr, bool) {
	ap, err := netip.ParseAddrPort(hostport)
	if err != nil {
		return netip.Addr{}, false
	}
	return ap.Addr().Unmap(), true
}

// hostAllowed is the DNS-rebinding guard: the Host header must name the router itself
// (an IP literal of it or loopback), localhost, or an allowlisted name. A rebinding page
// reaches us with the attacker's name in Host even though the source IP is a LAN client.
func hostAllowed(host string, routerIPs []netip.Addr, names []string) bool {
	h := strings.ToLower(host)
	if hp, _, err := net.SplitHostPort(h); err == nil {
		h = hp
	}
	h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	if h == "" {
		return false
	}
	if h == "localhost" {
		return true
	}
	for _, n := range names {
		if h == strings.ToLower(strings.TrimSpace(n)) {
			return true
		}
	}
	ip, err := netip.ParseAddr(h)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	if ip.IsLoopback() {
		return true
	}
	for _, r := range routerIPs {
		if r.Unmap() == ip {
			return true
		}
	}
	return false
}

func (s *server) hostOK(host string) bool {
	var ips []netip.Addr
	if s.RouterIPs != nil {
		ips = s.RouterIPs()
	}
	return hostAllowed(host, ips, s.HostAllow)
}

// outer is the http-level chain: recover -> trust -> host -> mux. Trust sits here so that
// 404s and the spec are LAN-only too; only /api/health answers everyone.
func (s *server) outer(mux http.Handler) http.Handler {
	return recoverPanics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			ip, ok := remoteAddr(r.RemoteAddr)
			if !ok || s.trusted == nil || !s.trusted(ip) {
				writeJSONError(w, http.StatusForbidden, "not_lan", "admin is reachable from the LAN only")
				return
			}
			if !s.hostOK(r.Host) {
				writeJSONError(w, http.StatusMisdirectedRequest, "bad_host", "unexpected Host header")
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(&jsonErrors{ResponseWriter: w}, r)
	}))
}

func writeJSONError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&apiError{Status: status, Message: msg, Code: code})
}

// recoverPanics keeps the process alive when a handler panics (invariants §1).
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				slog.Error("handler panic", "method", r.Method, "path", r.URL.Path, "panic", v)
				writeJSONError(w, http.StatusInternalServerError, "internal", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func isUnsafe(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func isPublic(op *huma.Operation) bool {
	pub, _ := op.Metadata[publicKey].(bool)
	return pub // absent = not public: default-deny
}

// gate is the Huma middleware: request info, then auth, then CSRF. Routes are protected
// unless registered as public. A bearer token is tried instead of the cookie when the
// request carries an Authorization header.
func (s *server) gate(ctx huma.Context, next func(huma.Context)) {
	ip, _ := remoteAddr(ctx.RemoteAddr())
	ctx = huma.WithValue(ctx, infoKey{}, reqInfo{ip: ip.String(), tls: ctx.TLS() != nil})
	if isPublic(ctx.Operation()) {
		next(ctx)
		return
	}
	if h := ctx.Header("Authorization"); h != "" {
		s.gateBearer(ctx, h, next)
		return
	}
	c, err := huma.ReadCookie(ctx, cookieName)
	if err != nil {
		writeError(ctx, http.StatusUnauthorized, "unauthenticated", "login required")
		return
	}
	sess, ok := s.sessions.Lookup(c.Value, s.clock.Now())
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthenticated", "login required")
		return
	}
	if isUnsafe(ctx.Method()) && subtle.ConstantTimeCompare([]byte(ctx.Header(csrfHeader)), []byte(sess.CSRF)) != 1 {
		writeError(ctx, http.StatusForbidden, "csrf", "missing or wrong "+csrfHeader)
		return
	}
	next(huma.WithValue(ctx, authedKey{}, authed{token: c.Value, sess: sess}))
}

// gateBearer authenticates an API token. No CSRF check: a bearer header is never sent
// by the browser on its own, so it is not an ambient credential. A read token may only
// use safe methods; no token reaches a session-only route.
func (s *server) gateBearer(ctx huma.Context, header string, next func(huma.Context)) {
	raw, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || s.Tokens == nil {
		writeError(ctx, http.StatusUnauthorized, "unauthenticated", "login required")
		return
	}
	tok, ok := s.Tokens.Authenticate(raw)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthenticated", "login required")
		return
	}
	if isSessionOnly(ctx.Operation()) {
		writeError(ctx, http.StatusForbidden, "session_required", "this route needs a browser session, not an API token")
		return
	}
	if tok.Scope != apitokens.ScopeAdmin && (isUnsafe(ctx.Method()) || isAdminOnly(ctx.Operation())) {
		writeError(ctx, http.StatusForbidden, "insufficient_scope", "this API token may only read")
		return
	}
	next(huma.WithValue(ctx, authedKey{}, authed{bearer: raw}))
}

// stillValid re-checks the credential of a long-lived request (the SSE stream).
func (s *server) stillValid(a authed) bool {
	if a.bearer != "" {
		_, ok := s.Tokens.Authenticate(a.bearer)
		return ok
	}
	_, ok := s.sessions.Lookup(a.token, s.clock.Now())
	return ok
}

// isAdminOnly marks safe-method routes whose answer is a secret (the config backup holds
// WiFi keys and the shadow file), so a read token may not fetch them.
func isAdminOnly(op *huma.Operation) bool {
	only, _ := op.Metadata[adminOnlyKey].(bool)
	return only
}

func isSessionOnly(op *huma.Operation) bool {
	only, _ := op.Metadata[sessionOnlyKey].(bool)
	return only
}

// sessionCookie builds tg_session; Secure only when the request came over TLS (the
// daemon serves plain HTTP on the LAN today).
func sessionCookie(token string, info reqInfo, maxAge int) http.Cookie {
	return http.Cookie{
		Name: cookieName, Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: info.tls, SameSite: http.SameSiteStrictMode,
	}
}

func clearedCookie(info reqInfo) http.Cookie { return sessionCookie("", info, -1) }
