package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// SystemInfoHandler handles GET /api/v1/system/info.
func SystemInfoHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := svc.GetSystemInfo()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(info)
	}
}

// SystemStatsHandler handles GET /api/v1/system/stats.
func SystemStatsHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := svc.GetSystemStats()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(stats)
	}
}

// SystemRebootHandler handles POST /api/v1/system/reboot.
func SystemRebootHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := svc.Reboot(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// SystemLogsHandler handles GET /api/v1/system/logs.
func SystemLogsHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logs, err := svc.GetLogs()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(logs)
	}
}

// SystemKernelLogsHandler handles GET /api/v1/system/logs/kernel.
func SystemKernelLogsHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logs, err := svc.GetKernelLogs()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(logs)
	}
}
