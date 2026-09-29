package api

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/models"
)

// LoginHandler handles POST /api/v1/auth/login.
func LoginHandler(authSvc *auth.AuthService, rl *auth.RateLimiter) fiber.Handler {
	return func(c fiber.Ctx) error {
		ip := c.IP()

		// Check rate limit before processing
		if rl != nil && !rl.Allow(ip) {
			return RespondWithError(c, fiber.StatusTooManyRequests, "too many login attempts")
		}

		var req models.LoginRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}

		token, expiry, err := authSvc.Login(req.Password)
		if err != nil {
			// Record failed attempt
			if rl != nil {
				rl.Record(ip)
			}
			return RespondWithError(c, fiber.StatusUnauthorized, "invalid password")
		}

		// Reset rate limiter on successful login
		if rl != nil {
			rl.Reset(ip)
		}

		return c.JSON(models.LoginResponse{
			Token:     token,
			ExpiresAt: expiry.Format("2006-01-02T15:04:05Z"),
			ExpiresIn: int64(authSvc.TokenTTL().Seconds()),
		})
	}
}

// LogoutHandler handles POST /api/v1/auth/logout.
func LogoutHandler(authSvc *auth.AuthService, bl *auth.TokenBlocklist) fiber.Handler {
	return func(c fiber.Ctx) error {
		tokenStr := bearerToken(c)
		if tokenStr != "" {
			authSvc.RevokeSession(tokenStr)
			blockSupersededToken(authSvc, bl, tokenStr)
		}
		return RespondOK(c)
	}
}

// SessionHandler handles GET /api/v1/auth/session.
// Returns the remaining session lifetime in seconds so clients can count down
// locally without comparing absolute timestamps across clocks.
func SessionHandler(authSvc *auth.AuthService) fiber.Handler {
	return func(c fiber.Ctx) error {
		resp := models.SessionResponse{Valid: true}
		authHeader := c.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			if remaining, err := authSvc.TokenRemaining(parts[1]); err == nil && remaining > 0 {
				resp.ExpiresIn = int64(remaining.Seconds())
			}
		}
		return c.JSON(resp)
	}
}

// ChangePasswordHandler handles PUT /api/v1/auth/password.
// The change revokes every live session (see auth.ChangePassword), so the
// response carries a fresh token for the caller and the superseded token is
// added to the blocklist — a token stolen before the change stays dead, also
// across a backend restart.
func ChangePasswordHandler(authSvc *auth.AuthService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.ChangePasswordRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		res, err := authSvc.ChangePassword(req.CurrentPassword, req.NewPassword)
		if err != nil {
			status := fiber.StatusBadRequest
			if err.Error() == "invalid current password" {
				status = fiber.StatusUnauthorized
			}
			return RespondWithError(c, status, err.Error())
		}
		blockSupersededToken(authSvc, authSvc.Blocklist(), bearerToken(c))
		return c.JSON(fiber.Map{
			"status":           "ok",
			"token":            res.Token,
			"expires_at":       res.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
			"expires_in":       int64(authSvc.TokenTTL().Seconds()),
			"revoked_sessions": res.RevokedSessions,
		})
	}
}

// bearerToken extracts the Bearer token from the Authorization header.
func bearerToken(c fiber.Ctx) string {
	parts := strings.SplitN(c.Get("Authorization"), " ", 2)
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
