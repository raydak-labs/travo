package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/failover"
	"github.com/openwrt-travel-gui/backend/internal/models"
)

// GetFailoverConfigHandler handles GET /api/v1/network/failover.
func GetFailoverConfigHandler(svc *failover.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(cfg)
	}
}

// SetFailoverConfigHandler handles PUT /api/v1/network/failover.
func SetFailoverConfigHandler(svc *failover.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.FailoverConfig
		if err := BindStrictBodyConfig(c, &cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetConfig(cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		return RespondOK(c)
	}
}

// GetFailoverEventsHandler handles GET /api/v1/network/failover/events.
func GetFailoverEventsHandler(svc *failover.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetEvents())
	}
}
