package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetBandSwitchingHandler handles GET /api/v1/wifi/band-switching.
func GetBandSwitchingHandler(svc *services.BandSwitchingService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"config": svc.GetConfig(),
			"status": svc.GetStatus(),
		})
	}
}

// SetBandSwitchingHandler handles PUT /api/v1/wifi/band-switching.
func SetBandSwitchingHandler(svc *services.BandSwitchingService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg services.BandSwitchConfig
		if err := c.Bind().Body(&cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if err := svc.SetConfig(cfg); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
