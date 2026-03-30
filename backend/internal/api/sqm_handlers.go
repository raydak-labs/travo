package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetSQMConfigHandler handles GET /api/v1/sqm/config.
func GetSQMConfigHandler(svc *services.SQMService) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(cfg)
	}
}

// SetSQMConfigHandler handles PUT /api/v1/sqm/config.
func SetSQMConfigHandler(svc *services.SQMService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.SQMConfig
		if err := c.Bind().Body(&cfg); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetConfig(cfg); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// ApplySQMHandler handles POST /api/v1/sqm/apply.
func ApplySQMHandler(svc *services.SQMService) fiber.Handler {
	return func(c fiber.Ctx) error {
		out, err := svc.Apply()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.SQMApplyResult{
				OK:     false,
				Output: out,
				Error:  err.Error(),
			})
		}
		return c.JSON(models.SQMApplyResult{OK: true, Output: out})
	}
}
