package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetUSBTetherStatusHandler returns current USB tethering detection state.
func GetUSBTetherStatusHandler(svc *services.USBTetheringService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetStatus())
	}
}

// ConfigureUSBTetherHandler configures the detected USB interface as a WAN source.
func ConfigureUSBTetherHandler(svc *services.USBTetheringService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Interface string `json:"interface"`
		}
		if err := c.Bind().Body(&req); err != nil || req.Interface == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "interface is required")
		}
		if err := svc.Configure(req.Interface); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// UnconfigureUSBTetherHandler removes the USB tethering WAN configuration.
func UnconfigureUSBTetherHandler(svc *services.USBTetheringService) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.Unconfigure(); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
