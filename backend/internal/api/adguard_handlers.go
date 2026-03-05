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
