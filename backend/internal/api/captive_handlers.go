package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// CaptiveStatusHandler handles GET /api/v1/captive/status.
func CaptiveStatusHandler(svc *services.CaptiveService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status, err := svc.CheckCaptivePortal()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(status)
	}
}
