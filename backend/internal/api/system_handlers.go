package api

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// SystemInfoHandler handles GET /api/v1/system/info.
func SystemInfoHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		info, err := svc.GetSystemInfo()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(info)
	}
}

// SystemStatsHandler handles GET /api/v1/system/stats.
func SystemStatsHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		stats, err := svc.GetSystemStats()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(stats)
	}
}

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

// SystemLogsHandler handles GET /api/v1/system/logs.
func SystemLogsHandler(svc *services.SystemService) fiber.Handler {
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
func SystemKernelLogsHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		logs, err := svc.GetKernelLogs()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(logs)
	}
}

// SetHostnameHandler handles PUT /api/v1/system/hostname.
func SetHostnameHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.SetHostnameRequest
		if err := BindStrictBodyConfig(c, &req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if req.Hostname == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "hostname is required")
		}
		if err := svc.SetHostname(req.Hostname); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok", "reboot_required": true})
	}
}

// GetTimezoneHandler handles GET /api/v1/system/timezone.
func GetTimezoneHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetTimezone()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(config)
	}
}

// SetTimezoneHandler handles PUT /api/v1/system/timezone.
func SetTimezoneHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.TimezoneConfig
		if err := BindStrictBodyConfig(c, &config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if config.Zonename == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "zonename is required")
		}
		if config.Timezone == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "timezone is required")
		}
		if err := svc.SetTimezone(config); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok", "reboot_required": true})
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

// maxRestoreUploadBytes bounds the uploaded restore archive. /tmp is a tmpfs
// on the router, so an unbounded upload is a way to fill RAM.
const maxRestoreUploadBytes = 32 << 20 // 32 MiB

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

// GetLEDStatusHandler handles GET /api/v1/system/leds.
func GetLEDStatusHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetLEDStatus())
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

// SetLEDStealthHandler handles PUT /api/v1/system/leds.
func SetLEDStealthHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.SetLEDRequest
		// Strict: the request is a single boolean that drives every LED. On the
		// permissive binder a body naming "enabled" (which is what the spec
		// used to document) decoded to stealth_mode=false and answered 200 with
		// the LEDs unchanged — the opposite of what was asked, behind a success
		// code. A 400 naming the field is the only honest answer.
		if err := BindStrictBodyConfig(c, &req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetLEDStealthMode(req.StealthMode); err != nil {
			return RespondWithServerError(c, err)
		}
		// The full LED status, not {"ok":…}: the caller needs the state that
		// was actually applied.
		return c.JSON(svc.GetLEDStatus())
	}
}

// GetLEDScheduleHandler handles GET /api/v1/system/leds/schedule.
func GetLEDScheduleHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetLEDSchedule())
	}
}

// SetLEDScheduleHandler handles PUT /api/v1/system/leds/schedule.
func SetLEDScheduleHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.LEDSchedule
		// Strict: this persists the whole schedule. On the permissive binder a
		// body naming fields this handler does not bind decoded to an empty
		// schedule, skipped validation, and answered 200 with the LEDs unchanged —
		// silent data loss behind a success code.
		if err := BindStrictBodyConfig(c, &req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		// Times are interpolated into a root crontab line; validate at the
		// boundary so a bad value is a 400, not a 500 from the service layer.
		if req.Enabled && req.OnTime != "" && req.OffTime != "" {
			if err := services.ValidateHHMM(req.OnTime); err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, "on_time: "+err.Error())
			}
			if err := services.ValidateHHMM(req.OffTime); err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, "off_time: "+err.Error())
			}
		}
		if err := svc.SetLEDSchedule(req); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(svc.GetLEDSchedule())
	}
}

// GetNTPConfigHandler handles GET /api/v1/system/ntp.
func GetNTPConfigHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetNTPConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(config)
	}
}

// SetNTPConfigHandler handles PUT /api/v1/system/ntp.
func SetNTPConfigHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.NTPConfig
		if err := BindStrictBodyConfig(c, &config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetNTPConfig(config); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok", "reboot_required": true})
	}
}

// NTPSyncHandler handles POST /api/v1/system/ntp/sync.
func NTPSyncHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.SyncNTP(); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetSetupCompleteHandler handles GET /api/v1/system/setup-complete.
func GetSetupCompleteHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetSetupComplete())
	}
}

// SetSetupCompleteHandler handles POST /api/v1/system/setup-complete.
func SetSetupCompleteHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.SetSetupComplete(); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// defaultSetSystemTime sets the system clock and persists it to the hardware
// clock (best-effort).
func defaultSetSystemTime(epochSec int64) error {
	if err := execx.Run(execx.Quick, "date", "-s", fmt.Sprintf("@%d", epochSec)); err != nil {
		return err
	}
	_ = execx.Run(execx.Quick, "hwclock", "-w")
	return nil
}

// SyncTimeHandler handles POST /api/v1/system/time-sync.
// Sets the system clock to the client's browser time, fixing clock-skew issues
// on devices that boot without NTP access. Only applied when skew > 60 seconds.
// Unauthenticated callers are rate limited and only allowed while the router
// clock is implausible (before build time) — an attacker must not be able to
// move a healthy clock. Authenticated callers may always sync.
func SyncTimeHandler(deps *Dependencies) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			ClientTimeMs int64 `json:"client_time_ms"`
		}
		if err := c.Bind().Body(&req); err != nil || req.ClientTimeMs <= 0 {
			return RespondWithError(c, fiber.StatusBadRequest, "client_time_ms is required")
		}

		authorized := false
		if parts := strings.SplitN(c.Get("Authorization"), " ", 2); len(parts) == 2 && parts[0] == "Bearer" {
			authorized = deps.Auth.ValidateToken(parts[1]) == nil
		}
		if !authorized {
			if deps.TimeSyncLimiter != nil {
				if !deps.TimeSyncLimiter.Allow(c.IP()) {
					return RespondWithError(c, fiber.StatusTooManyRequests, "too many time-sync attempts")
				}
				deps.TimeSyncLimiter.Record(c.IP())
			}
			if deps.TimeSyncGate != nil {
				if deps.TimeSyncGate.UnauthBlocked(time.Now(), deps.TimeSyncMinPlausible) {
					return RespondWithError(c, fiber.StatusForbidden, "system clock is plausible; authentication required to change time")
				}
			} else if !time.Now().Before(deps.TimeSyncMinPlausible) {
				return RespondWithError(c, fiber.StatusForbidden, "system clock is plausible; authentication required to change time")
			}
		}

		clientTime := time.UnixMilli(req.ClientTimeMs)
		// Clamp the target: an unauthenticated caller must not be able to park
		// the clock far outside the plausible window (see time_sync_gate.go).
		if !authorized {
			if err := validateClientTimeWindow(clientTime, deps.TimeSyncMinPlausible); err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
			}
		}
		skew := time.Until(clientTime)
		if skew < 0 {
			skew = -skew
		}
		if skew < 60*time.Second {
			return c.JSON(fiber.Map{"synced": false, "reason": "clock already accurate"})
		}

		setTime := deps.TimeSyncSetTime
		if setTime == nil {
			setTime = defaultSetSystemTime
		}
		if err := setTime(clientTime.Unix()); err != nil {
			return RespondWithError(c, fiber.StatusInternalServerError, "failed to set system time")
		}
		if deps.TimeSyncGate != nil {
			deps.TimeSyncGate.NoteClockSet()
		}

		return c.JSON(fiber.Map{"synced": true, "set_to": clientTime.UTC().Format(time.RFC3339)})
	}
}

// SystemAlertsHandler handles GET /api/v1/system/alerts.
func SystemAlertsHandler(svc *services.AlertService) fiber.Handler {
	return func(c fiber.Ctx) error {
		alerts := svc.GetAlerts()
		return c.JSON(models.AlertsResponse{Alerts: alerts})
	}
}

// GetButtonsHandler handles GET /api/v1/system/buttons.
func GetButtonsHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// A parse failure means the UI would show "no buttons configured" while
		// the generated hotplug script still runs the last saved actions, so
		// report the failure instead of a misleading empty list.
		buttons, err := svc.GetHardwareButtons()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(buttons)
	}
}

// SetButtonActionsHandler handles PUT /api/v1/system/button-actions.
func SetButtonActionsHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.ButtonActionsRequest
		// Strict: this persists the whole button map. A body whose field names do
		// not match would otherwise decode to "no buttons" and answer 200 after
		// disabling every hardware button action.
		if err := BindStrictBodyConfig(c, &req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetButtonActions(req.Buttons); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		return RespondOK(c)
	}
}

// GetSSHKeysHandler handles GET /api/v1/system/ssh-keys.
func GetSSHKeysHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		resp, err := svc.GetSSHKeys()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(resp)
	}
}

// AddSSHKeyHandler handles POST /api/v1/system/ssh-keys.
func AddSSHKeyHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.AddSSHKeyRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.AddSSHKey(req.Key); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
	}
}

// DeleteSSHKeyHandler handles DELETE /api/v1/system/ssh-keys/:index.
func DeleteSSHKeyHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		indexStr := c.Params("index")
		index, err := strconv.Atoi(indexStr)
		if err != nil || index < 0 {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid index")
		}
		if err := svc.DeleteSSHKey(index); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// RunSpeedTestHandler handles POST /api/v1/system/speed-test.
func RunSpeedTestHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		result, err := svc.RunSpeedTest()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(result)
	}
}

// GetAlertThresholdsHandler handles GET /api/v1/system/alert-thresholds.
func GetAlertThresholdsHandler(svc *services.AlertService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetAlertThresholds())
	}
}

// SetAlertThresholdsHandler handles PUT /api/v1/system/alert-thresholds.
func SetAlertThresholdsHandler(svc *services.AlertService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var t models.AlertThresholds
		if err := BindStrictBodyConfig(c, &t); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		// Without this, storage_percent=500 was accepted and persisted: a
		// threshold no usage level can reach, so the alert silently never fires
		// while the UI shows a saved configuration.
		if err := services.ValidateAlertThresholds(t); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetAlertThresholds(t); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}
