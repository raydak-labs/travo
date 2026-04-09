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
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many login attempts"})
		}

		var req models.LoginRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": ErrInvalidRequestBody})
		}

		token, expiry, err := authSvc.Login(req.Password)
		if err != nil {
			// Record failed attempt
			if rl != nil {
				rl.Record(ip)
			}
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
		}

		// Reset rate limiter on successful login
		if rl != nil {
			rl.Reset(ip)
		}

		return c.JSON(models.LoginResponse{
			Token:     token,
			ExpiresAt: expiry.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// LogoutHandler handles POST /api/v1/auth/logout.
func LogoutHandler(authSvc *auth.AuthService, bl *auth.TokenBlocklist) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader != "" && bl != nil {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenStr := parts[1]
				expiry, err := authSvc.TokenExpiry(tokenStr)
				if err == nil {
					bl.Block(tokenStr, expiry)
				}
			}
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// SessionHandler handles GET /api/v1/auth/session.
func SessionHandler(authSvc *auth.AuthService) fiber.Handler {
	return func(c fiber.Ctx) error {
		_ = authSvc
		return c.JSON(fiber.Map{"valid": true})
	}
}

// ChangePasswordHandler handles PUT /api/v1/auth/password.
func ChangePasswordHandler(authSvc *auth.AuthService, _ *auth.FileAuthStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.ChangePasswordRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": ErrInvalidRequestBody})
		}
		if err := authSvc.ChangePassword(req.CurrentPassword, req.NewPassword); err != nil {
			status := fiber.StatusBadRequest
			if err.Error() == "invalid current password" {
				status = fiber.StatusUnauthorized
			}
			return c.Status(status).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
