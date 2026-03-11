package api

import (
	"os"

	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/models"
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

// SetHostnameHandler handles PUT /api/v1/system/hostname.
func SetHostnameHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.SetHostnameRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.Hostname == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "hostname is required"})
		}
		if err := svc.SetHostname(req.Hostname); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetTimezoneHandler handles GET /api/v1/system/timezone.
func GetTimezoneHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		config, err := svc.GetTimezone()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(config)
	}
}

// SetTimezoneHandler handles PUT /api/v1/system/timezone.
func SetTimezoneHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var config models.TimezoneConfig
		if err := c.BodyParser(&config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if config.Zonename == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "zonename is required"})
		}
		if config.Timezone == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "timezone is required"})
		}
		if err := svc.SetTimezone(config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// BackupHandler handles GET /api/v1/system/backup — downloads config backup.
func BackupHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path, err := svc.CreateBackup()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer os.Remove(path)
		c.Set("Content-Disposition", "attachment; filename=openwrt-backup.tar.gz")
		return c.SendFile(path)
	}
}

// RestoreHandler handles POST /api/v1/system/restore — uploads and restores config backup.
func RestoreHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		file, err := c.FormFile("backup")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "backup file is required"})
		}
		// Validate file type
		ct := file.Header.Get("Content-Type")
		if ct != "" && ct != "application/gzip" && ct != "application/x-gzip" && ct != "application/octet-stream" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid file type, expected tar.gz"})
		}
		// Save to temp path
		tmpPath := "/tmp/restore-upload.tar.gz"
		if err := c.SaveFile(file, tmpPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save uploaded file"})
		}
		defer os.Remove(tmpPath)
		if err := svc.RestoreBackup(tmpPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok", "message": "Configuration restored. Reboot to apply changes."})
	}
}

// GetLEDStatusHandler handles GET /api/v1/system/leds.
func GetLEDStatusHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(svc.GetLEDStatus())
	}
}

// FactoryResetHandler handles POST /api/v1/system/factory-reset.
func FactoryResetHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := svc.FactoryReset(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// SetLEDStealthHandler handles PUT /api/v1/system/leds.
func SetLEDStealthHandler(svc *services.SystemService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.SetLEDRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetLEDStealthMode(req.StealthMode); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(svc.GetLEDStatus())
	}
}
