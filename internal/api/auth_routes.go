package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/auth"
)

type sessionBody struct {
	ExpiresIn int    `json:"expires_in" doc:"Seconds until the session ends; relative on purpose"`
	CSRFToken string `json:"csrf_token" doc:"Send as X-CSRF-Token on POST/PUT/PATCH/DELETE"`
}

type sessionOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      sessionBody
}

type okBody struct {
	Status string `json:"status" enum:"ok"`
}

type loginInput struct {
	Body struct {
		Password string `json:"password" minLength:"1" maxLength:"256" doc:"Root password"`
	}
}

type passwordInput struct {
	Body struct {
		Current string `json:"current" minLength:"1" maxLength:"256"`
		New     string `json:"new" maxLength:"256" doc:"8 to 128 bytes, no NUL or line breaks"`
	}
}

func (s *server) registerAuth(a huma.API) {
	register(s, true, op("postLogin", http.MethodPost, "/api/v1/auth/login", "Log in with the root password", "auth", 401, 409, 429, 503), a, s.login)
	registerSessionOnly(s, op("postLogout", http.MethodPost, "/api/v1/auth/logout", "Log out and revoke the session", "auth"), a, s.logout)
	registerSessionOnly(s, op("getSession", http.MethodGet, "/api/v1/auth/session", "Current session (recovers the CSRF token after a reload)", "auth"), a, s.session)
	registerSessionOnly(s, op("postPassword", http.MethodPost, "/api/v1/auth/password", "Change the root password and revoke all sessions", "auth", 400, 403, 429, 503), a, s.changePassword)
}

// issue starts a session for this request and builds the response with its cookie.
func (s *server) issue(ctx context.Context) (*sessionOutput, error) {
	now := s.clock.Now()
	token, sess, err := s.sessions.Create(now)
	if err != nil {
		slog.Error("session", "err", err) // entropy failure: never fall back to a weaker token
		return nil, fail(http.StatusInternalServerError, "internal", "cannot create a session")
	}
	out := &sessionOutput{SetCookie: sessionCookie(token, infoOf(ctx), int(auth.Lifetime.Seconds()))}
	out.Body = sessionBody{ExpiresIn: auth.ExpiresIn(sess, now), CSRFToken: sess.CSRF}
	return out, nil
}

func (s *server) limited(key string) error {
	if s.Limiter.Allow(key, s.clock.Now()) {
		return nil
	}
	return fail(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in a minute")
}

func unavailable(what string, err error) error {
	slog.Error(what, "err", err)
	return fail(http.StatusServiceUnavailable, "device_unavailable", "cannot read the router account state")
}

func (s *server) login(ctx context.Context, in *loginInput) (*sessionOutput, error) {
	if err := s.limited(infoOf(ctx).ip); err != nil {
		return nil, err
	}
	set, err := s.Accounts.RootPasswordSet(ctx)
	if err != nil {
		return nil, unavailable("shadow", err)
	}
	if !set {
		return nil, fail(http.StatusConflict, "setup_required", "no root password yet; set one first")
	}
	ok, err := s.Accounts.VerifyRootPassword(ctx, in.Body.Password)
	if err != nil {
		return nil, unavailable("verify", err)
	}
	if !ok {
		return nil, fail(http.StatusUnauthorized, "invalid_credentials", "wrong password")
	}
	return s.issue(ctx)
}

func (s *server) logout(ctx context.Context, _ *struct{}) (*struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      okBody
}, error) {
	s.sessions.Revoke(authedOf(ctx).token)
	out := &struct {
		SetCookie http.Cookie `header:"Set-Cookie"`
		Body      okBody
	}{SetCookie: clearedCookie(infoOf(ctx))}
	out.Body.Status = "ok"
	return out, nil
}

func (s *server) session(ctx context.Context, _ *struct{}) (*struct{ Body sessionBody }, error) {
	sess := authedOf(ctx).sess
	return &struct{ Body sessionBody }{sessionBody{ExpiresIn: auth.ExpiresIn(sess, s.clock.Now()), CSRFToken: sess.CSRF}}, nil
}

func (s *server) changePassword(ctx context.Context, in *passwordInput) (*sessionOutput, error) {
	if err := s.limited("pw:" + infoOf(ctx).ip); err != nil {
		return nil, err
	}
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	ok, err := s.Accounts.VerifyRootPassword(ctx, in.Body.Current)
	if err != nil {
		return nil, unavailable("verify", err)
	}
	if !ok { // 403 not 401: the session is fine, the SPA must not treat this as a logout
		return nil, fail(http.StatusForbidden, "invalid_credentials", "current password is wrong")
	}
	if err := auth.ValidatePassword(in.Body.New); err != nil {
		return nil, fail(http.StatusBadRequest, "invalid_password", err.Error())
	}
	if err := s.Accounts.SetRootPassword(ctx, in.Body.New); err != nil {
		slog.Error("set password", "err", err)
		return nil, fail(http.StatusServiceUnavailable, "device_unavailable", "could not set the password")
	}
	epoch := s.sessions.BumpEpoch()
	if err := s.SaveEpoch(epoch); err != nil {
		slog.Error("persist epoch", "err", err)
		s.sessions.RevokeAll()
		return nil, fail(http.StatusServiceUnavailable, "persist_failed", "password changed but state not saved; log in again")
	}
	s.sessions.RevokeAll()
	return s.issue(ctx)
}
