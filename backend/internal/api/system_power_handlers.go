package api

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// SystemRebootHandler handles POST /api/v1/system/reboot.
func SystemRebootHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.Reboot(); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// SystemShutdownHandler handles POST /api/v1/system/shutdown.
func SystemShutdownHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.Shutdown(); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// BackupHandler handles GET /api/v1/system/backup — downloads config backup.
func BackupHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		path, err := svc.CreateBackup()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		defer os.Remove(path)
		c.Set("Content-Disposition", "attachment; filename=openwrt-backup.tar.gz")
		return c.SendFile(path)
	}
}

// RestoreHandler handles POST /api/v1/system/restore — uploads and restores config backup.
//
// The upload is saved to a UNIQUE temp path: the old fixed
// /tmp/restore-upload.tar.gz let two concurrent restores overwrite each other's
// payload in world-writable /tmp, so the archive sysupgrade restored was not
// necessarily the one that was validated.
//
// svc.RestoreBackup validates the archive (member allowlist, no links, no
// traversal, size caps) before sysupgrade runs and reports a rejection as
// ErrInvalidBackupArchive, which answers 400 rather than 500.
func RestoreHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		file, err := c.FormFile("backup")
		if err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, "backup file is required")
		}
		// Validate file type
		ct := file.Header.Get("Content-Type")
		if ct != "" && ct != "application/gzip" && ct != "application/x-gzip" && ct != "application/octet-stream" {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid file type, expected tar.gz")
		}
		if file.Size > maxRestoreUploadBytes {
			return RespondWithError(c, fiber.StatusBadRequest,
				fmt.Sprintf("backup file is too large (%d bytes, limit %d)", file.Size, maxRestoreUploadBytes))
		}
		// Unique path per upload: concurrent restores must not share a file.
		f, err := os.CreateTemp("", "restore-upload-*.tar.gz")
		if err != nil {
			return RespondWithServerError(c, fmt.Errorf("creating temp file: %w", err))
		}
		tmpPath := f.Name()
		if err := f.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return RespondWithServerError(c, fmt.Errorf("creating temp file: %w", err))
		}
		defer os.Remove(tmpPath)
		if err := c.SaveFile(file, tmpPath); err != nil {
			return RespondWithServerError(c, fmt.Errorf("saving uploaded file: %w", err))
		}
		if err := svc.RestoreBackup(tmpPath); err != nil {
			if errors.Is(err, services.ErrInvalidBackupArchive) {
				return RespondWithError(c, fiber.StatusBadRequest, err.Error())
			}
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok", "message": "Configuration restored. Reboot to apply changes."})
	}
}

// FactoryResetHandler handles POST /api/v1/system/factory-reset.
func FactoryResetHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.FactoryReset(); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// FirmwareUpgradeHandler handles POST /api/v1/system/firmware/upgrade.
//
// A ".bin" extension is not validation. The image's OpenWrt metadata is parsed
// and checked against this board BEFORE the flash is started, so a junk or
// foreign image is a 400 that names the model, instead of a 200 followed by a
// brick and no serial console. The parsed model is returned so the UI can show
// what is about to be flashed.
func FirmwareUpgradeHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		file, err := c.FormFile("firmware")
		if err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, "firmware file is required")
		}
		// Validate file extension
		if !strings.HasSuffix(file.Filename, ".bin") {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid file type, expected .bin firmware image")
		}
		keepSettings := c.FormValue("keep_settings", "true") == "true"
		f, err := file.Open()
		if err != nil {
			return RespondWithError(c, fiber.StatusInternalServerError, "failed to read uploaded file")
		}
		defer func() { _ = f.Close() }()
		meta, err := svc.UpgradeFirmware(f, keepSettings)
		if err != nil {
			if errors.Is(err, services.ErrInvalidFirmware) {
				return RespondWithError(c, fiber.StatusBadRequest, err.Error())
			}
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{
			"status":            "ok",
			"message":           "Firmware upgrade initiated. Device will reboot.",
			"model":             meta.Model,
			"supported_devices": meta.SupportedDevices,
		})
	}
}
