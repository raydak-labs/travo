package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/system"
)

// SetHostnameHandler handles PUT /api/v1/system/hostname.
func SetHostnameHandler(svc *system.Service) fiber.Handler {
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
func GetTimezoneHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetTimezone()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(config)
	}
}

// SetTimezoneHandler handles PUT /api/v1/system/timezone.
func SetTimezoneHandler(svc *system.Service) fiber.Handler {
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

// GetNTPConfigHandler handles GET /api/v1/system/ntp.
func GetNTPConfigHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetNTPConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(config)
	}
}

// SetNTPConfigHandler handles PUT /api/v1/system/ntp.
func SetNTPConfigHandler(svc *system.Service) fiber.Handler {
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
func NTPSyncHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.SyncNTP(); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetSetupCompleteHandler handles GET /api/v1/system/setup-complete.
func GetSetupCompleteHandler(svc *system.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetSetupComplete())
	}
}

// SetSetupCompleteHandler handles POST /api/v1/system/setup-complete.
func SetSetupCompleteHandler(svc *system.Service) fiber.Handler {
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
