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
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenStr := parts[1]
				authSvc.RevokeSession(tokenStr)
				if bl != nil {
					expiry, err := authSvc.TokenExpiry(tokenStr)
					if err == nil {
						bl.Block(tokenStr, expiry)
					}
				}
			}
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
func ChangePasswordHandler(authSvc *auth.AuthService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.ChangePasswordRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if err := authSvc.ChangePassword(req.CurrentPassword, req.NewPassword); err != nil {
			status := fiber.StatusBadRequest
			if err.Error() == "invalid current password" {
				status = fiber.StatusUnauthorized
			}
			return RespondWithError(c, status, err.Error())
		}
		return RespondOK(c)
	}
}
