package api

import (
	"errors"
	"os"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// AdGuardStatusHandler handles GET /api/v1/services/adguardhome/status.
func AdGuardStatusHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c fiber.Ctx) error {
		installed := adguard.IsInstalled()
		running := adguard.IsRunning()
		version := adguard.Version()

		response := fiber.Map{
			"installed": installed,
			"running":   running,
			"version":   version,
		}

		if running {
			stats, err := adguard.GetStatus()
			if err == nil {
				response["stats"] = stats
			}
		}

		return c.JSON(response)
	}
}

// AdGuardDNSStatusHandler handles GET /api/v1/adguard/dns.
func AdGuardDNSStatusHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := adguard.GetDNSStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// SetAdGuardDNSHandler handles PUT /api/v1/adguard/dns.
func SetAdGuardDNSHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if err := adguard.SetDNS(body.Enabled); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetAdGuardConfigHandler handles GET /api/v1/adguard/config.
func GetAdGuardConfigHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c fiber.Ctx) error {
		content, err := adguard.GetConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return RespondWithError(c, fiber.StatusNotFound, "AdGuard config file not found")
			}
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"content": content})
	}
}

// SetAdGuardPasswordHandler handles PUT /api/v1/adguard/password.
func SetAdGuardPasswordHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if body.Password == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "password is required")
		}
		if body.Username == "" {
			body.Username = "admin"
		}
		if err := adguard.SetPassword(body.Username, body.Password); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// SetAdGuardConfigHandler handles PUT /api/v1/adguard/config.
func SetAdGuardConfigHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Content string `json:"content"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if body.Content == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "content is required")
		}
		if err := adguard.SetConfig(body.Content); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
