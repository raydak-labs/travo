package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// ListServicesHandler handles GET /api/v1/services.
func ListServicesHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		list, err := sm.ListServices()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(list)
	}
}

// InstallServiceHandler handles POST /api/v1/services/:id/install.
func InstallServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Install(id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// RemoveServiceHandler handles POST /api/v1/services/:id/remove.
func RemoveServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Remove(id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// StartServiceHandler handles POST /api/v1/services/:id/start.
func StartServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Start(id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// StopServiceHandler handles POST /api/v1/services/:id/stop.
func StopServiceHandler(sm *services.ServiceManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := sm.Stop(id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
