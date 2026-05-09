package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetFailoverConfigHandler handles GET /api/v1/network/failover.
func GetFailoverConfigHandler(svc *services.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(cfg)
	}
}

// SetFailoverConfigHandler handles PUT /api/v1/network/failover.
func SetFailoverConfigHandler(svc *services.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.FailoverConfig
		if err := c.Bind().Body(&cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if err := svc.SetConfig(cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return RespondOK(c)
	}
}

// GetFailoverEventsHandler handles GET /api/v1/network/failover/events.
func GetFailoverEventsHandler(svc *services.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetEvents())
	}
}
