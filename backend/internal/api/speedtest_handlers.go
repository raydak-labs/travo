package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetSpeedtestServiceStatusHandler handles GET /api/v1/system/speedtest-service.
func GetSpeedtestServiceStatusHandler(svc *services.SpeedtestService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetSpeedtestServiceStatus()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(status)
	}
}

// InstallSpeedtestCLIHandler handles POST /api/v1/system/speedtest-service/install.
func InstallSpeedtestCLIHandler(svc *services.SpeedtestService) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := svc.InstallSpeedtestCLI()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// UninstallSpeedtestCLIHandler handles POST /api/v1/system/speedtest-service/uninstall.
func UninstallSpeedtestCLIHandler(svc *services.SpeedtestService) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := svc.UninstallSpeedtestCLI()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// RunSpeedtestCLIHandler handles POST /api/v1/system/speedtest-service/run.
func RunSpeedtestCLIHandler(svc *services.SpeedtestService) fiber.Handler {
	return func(c fiber.Ctx) error {
		result, err := svc.RunSpeedtestCLI()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	}
}

// SpeedTestResultAPI is the frontend API response format.
type SpeedTestResultAPI struct {
	DownloadMbps float64 `json:"download_mbps"`
	UploadMbps   float64 `json:"upload_mbps"`
	PingMs       float64 `json:"ping_ms"`
	Server       string  `json:"server"`
}

// SpeedtestServiceStatusAPI is the frontend API response format for service status.
type SpeedtestServiceStatusAPI struct {
	Installed      bool   `json:"installed"`
	Supported      bool   `json:"supported"`
	Architecture   string `json:"architecture"`
	Version        string `json:"version"`
	PackageName    string `json:"package_name"`
	StorageSizeMB  int    `json:"storage_size_mb"`
}

// ConvertToAPI converts models.SpeedtestService to API response.
func ConvertToAPI(s models.SpeedtestService) SpeedtestServiceStatusAPI {
	return SpeedtestServiceStatusAPI{
		Installed:      s.Installed,
		Supported:      s.Supported,
		Architecture:   s.Architecture,
		Version:        s.Version,
		PackageName:    s.PackageName,
		StorageSizeMB:  s.StorageSizeMB,
	}
}