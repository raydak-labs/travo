package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// AdGuardStatusHandler handles GET /api/v1/services/adguardhome/status.
func AdGuardStatusHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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
	return func(c *fiber.Ctx) error {
		status, err := adguard.GetDNSStatus()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(status)
	}
}

// SetAdGuardDNSHandler handles PUT /api/v1/adguard/dns.
func SetAdGuardDNSHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := adguard.SetDNS(body.Enabled); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetAdGuardConfigHandler handles GET /api/v1/adguard/config.
func GetAdGuardConfigHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		content, err := adguard.GetConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"content": content})
	}
}

// SetAdGuardConfigHandler handles PUT /api/v1/adguard/config.
func SetAdGuardConfigHandler(adguard *services.AdGuardService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Content string `json:"content"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if body.Content == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "content is required"})
		}
		if err := adguard.SetConfig(body.Content); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
