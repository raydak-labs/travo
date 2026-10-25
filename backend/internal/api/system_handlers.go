package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/system"
)

// SystemInfoHandler handles GET /api/v1/system/info.
func SystemInfoHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		info, err := svc.GetSystemInfo()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(info)
	}
}

// SystemStatsHandler handles GET /api/v1/system/stats.
func SystemStatsHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		stats, err := svc.GetSystemStats()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(stats)
	}
}

// SystemLogsHandler handles GET /api/v1/system/logs.
func SystemLogsHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		service := c.Query("service")
		level := c.Query("level")
		logs, err := svc.GetLogs(service, level)
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(logs)
	}
}

// SystemKernelLogsHandler handles GET /api/v1/system/logs/kernel.
func SystemKernelLogsHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		logs, err := svc.GetKernelLogs()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(logs)
	}
}

// maxRestoreUploadBytes bounds the uploaded restore archive. /tmp is a tmpfs
// on the router, so an unbounded upload is a way to fill RAM.
const maxRestoreUploadBytes = 32 << 20 // 32 MiB

// SystemAlertsHandler handles GET /api/v1/system/alerts.
func SystemAlertsHandler(svc *system.AlertService) fiber.Handler {
	return func(c fiber.Ctx) error {
		alerts := svc.GetAlerts()
		return c.JSON(models.AlertsResponse{Alerts: alerts})
	}
}

// RunSpeedTestHandler handles POST /api/v1/system/speed-test.
func RunSpeedTestHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		result, err := svc.RunSpeedTest()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(result)
	}
}

// GetAlertThresholdsHandler handles GET /api/v1/system/alert-thresholds.
func GetAlertThresholdsHandler(svc *system.AlertService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetAlertThresholds())
	}
}

// SetAlertThresholdsHandler handles PUT /api/v1/system/alert-thresholds.
func SetAlertThresholdsHandler(svc *system.AlertService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var t models.AlertThresholds
		if err := BindStrictBodyConfig(c, &t); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		// Without this, storage_percent=500 was accepted and persisted: a
		// threshold no usage level can reach, so the alert silently never fires
		// while the UI shows a saved configuration.
		if err := system.ValidateAlertThresholds(t); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetAlertThresholds(t); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}
