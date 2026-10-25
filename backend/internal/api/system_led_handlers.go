package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/system"
)

// GetLEDStatusHandler handles GET /api/v1/system/leds.
func GetLEDStatusHandler(svc *system.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetLEDStatus())
	}
}

// SetLEDStealthHandler handles PUT /api/v1/system/leds.
func SetLEDStealthHandler(svc *system.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.SetLEDRequest
		// Strict: a body naming the wrong field must be a 400, not a 200 that
		// leaves the LEDs unchanged.
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
func GetLEDScheduleHandler(svc *system.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetLEDSchedule())
	}
}

// SetLEDScheduleHandler handles PUT /api/v1/system/leds/schedule.
func SetLEDScheduleHandler(svc *system.SystemService) fiber.Handler {
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
			if err := platform.ValidateHHMM(req.OnTime); err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, "on_time: "+err.Error())
			}
			if err := platform.ValidateHHMM(req.OffTime); err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, "off_time: "+err.Error())
			}
		}
		if err := svc.SetLEDSchedule(req); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(svc.GetLEDSchedule())
	}
}
