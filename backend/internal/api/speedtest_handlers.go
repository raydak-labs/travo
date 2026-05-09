package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetSpeedtestServiceStatusHandler handles GET /api/v1/system/speedtest-service.
func GetSpeedtestServiceStatusHandler(svc *services.SpeedtestService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetSpeedtestServiceStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// InstallSpeedtestCLIHandler handles POST /api/v1/system/speedtest-service/install.
func InstallSpeedtestCLIHandler(svc *services.SpeedtestService) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := svc.InstallSpeedtestCLI()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// UninstallSpeedtestCLIHandler handles POST /api/v1/system/speedtest-service/uninstall.
func UninstallSpeedtestCLIHandler(svc *services.SpeedtestService) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := svc.UninstallSpeedtestCLI()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// RunSpeedtestCLIHandler handles POST /api/v1/system/speedtest-service/run.
func RunSpeedtestCLIHandler(svc *services.SpeedtestService) fiber.Handler {
	return func(c fiber.Ctx) error {
		result, err := svc.RunSpeedtestCLI()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(result)
	}
}
