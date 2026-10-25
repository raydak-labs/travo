package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/wifi"
)

// GetRepeaterOptionsHandler handles GET /api/v1/wifi/repeater-options.
func GetRepeaterOptionsHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		opts, err := svc.GetRepeaterOptions()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(opts)
	}
}

// SetRepeaterOptionsHandler handles PUT /api/v1/wifi/repeater-options.
func SetRepeaterOptionsHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body models.RepeaterOptions
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		apply, err := svc.SetRepeaterOptions(body)
		if err != nil {
			return RespondWithServerError(c, err)
		}
		resp := wifiMutationResponse(apply)
		resp["allow_ap_on_sta_radio"] = body.AllowAPOnSTARadio
		return c.JSON(resp)
	}
}

// ReconcileRepeaterAPLayoutHandler handles POST /api/v1/wifi/repeater/reconcile.
func ReconcileRepeaterAPLayoutHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		apply, err := svc.ReconcileRepeaterAPLayout()
		if err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetAutoReconnectHandler handles GET /api/v1/wifi/autoreconnect.
func GetAutoReconnectHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		enabled, err := svc.GetAutoReconnect()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"enabled": enabled})
	}
}

// SetAutoReconnectHandler handles PUT /api/v1/wifi/autoreconnect.
func SetAutoReconnectHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetAutoReconnect(body.Enabled); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
