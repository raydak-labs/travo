package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetBandSwitchingHandler handles GET /api/v1/wifi/band-switching.
func GetBandSwitchingHandler(svc *services.BandSwitchingService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"config": svc.GetConfig(),
			"status": svc.GetStatus(),
		})
	}
}

// SetBandSwitchingHandler handles PUT /api/v1/wifi/band-switching.
func SetBandSwitchingHandler(svc *services.BandSwitchingService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var cfg services.BandSwitchConfig
		if err := c.BodyParser(&cfg); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetConfig(cfg); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
