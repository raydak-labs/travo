package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/models"
)

// ChangePasswordResponse carries the replacement session: the change revokes
// every live session, including the caller's.
type ChangePasswordResponse struct {
	Status          string `json:"status" example:"ok"`
	Token           string `json:"token"`
	ExpiresIn       int64  `json:"expires_in"`
	RevokedSessions int    `json:"revoked_sessions"`
}

func registerAuth(api *API, d *Dependencies) {
	post(api, "/api/v1/auth/login", "Login", "Authenticate with the admin password",
		func(ctx context.Context, in *Body[models.LoginRequest]) (*Body[models.LoginResponse], error) {
			ip := fiberCtx(ctx).IP()
			if d.RateLimiter != nil && !d.RateLimiter.Allow(ip) {
				return nil, errorf(http.StatusTooManyRequests, "too many login attempts")
			}
			token, _, err := d.Auth.Login(in.Body.Password)
			if err != nil {
				if d.RateLimiter != nil {
					d.RateLimiter.Record(ip)
				}
				return nil, errorf(http.StatusUnauthorized, "invalid password")
			}
			if d.RateLimiter != nil {
				d.RateLimiter.Reset(ip)
			}
			return reply(models.LoginResponse{Token: token, ExpiresIn: int64(d.Auth.TokenTTL().Seconds())})
		}, public)

	post(api, "/api/v1/auth/logout", "Logout", "Revoke the current session",
		func(ctx context.Context, _ *none) (*Body[Status], error) {
			if token := bearerToken(ctx); token != "" {
				d.Auth.RevokeSession(token)
				blockSupersededToken(d.Auth, d.Blocklist, token)
			}
			return statusOK, nil
		})

	get(api, "/api/v1/auth/session", "GetSession", "Remaining lifetime of the current session",
		func(ctx context.Context, _ *none) (*Body[models.SessionResponse], error) {
			resp := models.SessionResponse{Valid: true}
			if remaining, err := d.Auth.TokenRemaining(bearerToken(ctx)); err == nil && remaining > 0 {
				resp.ExpiresIn = int64(remaining.Seconds())
			}
			return reply(resp)
		})

	put(api, "/api/v1/auth/password", "ChangePassword", "Change the admin password and rotate sessions",
		func(ctx context.Context, in *Body[models.ChangePasswordRequest]) (*Body[ChangePasswordResponse], error) {
			res, err := d.Auth.ChangePassword(in.Body.CurrentPassword, in.Body.NewPassword)
			if err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, auth.ErrInvalidCurrentPassword) {
					status = http.StatusUnauthorized
				}
				return nil, errorf(status, err.Error())
			}
			blockSupersededToken(d.Auth, d.Auth.Blocklist(), bearerToken(ctx))
			return reply(ChangePasswordResponse{
				Status:          "ok",
				Token:           res.Token,
				ExpiresIn:       int64(d.Auth.TokenTTL().Seconds()),
				RevokedSessions: res.RevokedSessions,
			})
		})
}

// bearerToken extracts the Bearer token from the Authorization header.
func bearerToken(ctx context.Context) string {
	parts := strings.SplitN(fiberCtx(ctx).Get("Authorization"), " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return ""
}

// blockSupersededToken blocklists a token string that must never be accepted
// again. The token's jti is already revoked by the session registry; the hash
// entry additionally covers the raw token after a restart.
func blockSupersededToken(authSvc *auth.AuthService, bl *auth.TokenBlocklist, tokenStr string) {
	if tokenStr == "" {
		return
	}
	if bl == nil {
		bl = authSvc.Blocklist()
	}
	if bl == nil {
		return
	}
	if expiry, err := authSvc.TokenExpiry(tokenStr); err == nil {
		bl.Block(tokenStr, expiry)
	}
}
