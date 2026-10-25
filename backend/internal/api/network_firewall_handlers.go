package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/network"
)

// GetFirewallZonesHandler handles GET /api/v1/network/firewall/zones.
func GetFirewallZonesHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		zones, err := svc.GetFirewallZones()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"zones": zones})
	}
}

// GetPortForwardsHandler handles GET /api/v1/network/firewall/port-forwards.
func GetPortForwardsHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		rules, err := svc.GetPortForwards()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"rules": rules})
	}
}

// AddPortForwardHandler handles POST /api/v1/network/firewall/port-forwards.
func AddPortForwardHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var rule models.PortForwardRule
		if err := c.Bind().Body(&rule); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.AddPortForward(rule); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
	}
}

// DeletePortForwardHandler handles DELETE /api/v1/network/firewall/port-forwards/:id.
func DeletePortForwardHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if err := svc.DeletePortForward(id); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}
