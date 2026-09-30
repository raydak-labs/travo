package api

import (
	"encoding/json"
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

// bandSwitchingRequest is a shape probe: it is decoded WITHOUT
// DisallowUnknownFields, purely to see whether the body uses the wrapper.
type bandSwitchingRequest struct {
	Config *services.BandSwitchConfig `json:"config"`
}

// SetBandSwitchingHandler handles PUT /api/v1/wifi/band-switching.
func SetBandSwitchingHandler(svc *services.BandSwitchingService) fiber.Handler {
	return func(c fiber.Ctx) error {
		body := c.Body()

		// Which shape is this? Probe permissively, then decode strictly into the
		// matching target so an unknown field is still a 400.
		var probe bandSwitchingRequest
		if err := json.Unmarshal(body, &probe); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}

		var cfg services.BandSwitchConfig
		if probe.Config != nil {
			if err := decodeStrictJSON(body, &cfg, "config"); err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
			}
		} else if err := decodeStrictJSON(body, &cfg, ""); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}

		if err := services.ValidateBandSwitchConfig(cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetConfig(cfg); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
