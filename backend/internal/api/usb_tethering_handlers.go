package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetUSBTetherStatusHandler returns current USB tethering detection state.
func GetUSBTetherStatusHandler(svc *services.USBTetheringService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(svc.GetStatus())
	}
}

// ConfigureUSBTetherHandler configures the detected USB interface as a WAN source.
func ConfigureUSBTetherHandler(svc *services.USBTetheringService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Interface string `json:"interface"`
		}
		if err := c.BodyParser(&req); err != nil || req.Interface == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "interface is required",
			})
		}
		if err := svc.Configure(req.Interface); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// UnconfigureUSBTetherHandler removes the USB tethering WAN configuration.
func UnconfigureUSBTetherHandler(svc *services.USBTetheringService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := svc.Unconfigure(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
