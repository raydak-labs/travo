package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

func GetFailoverConfigHandler(svc *services.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(cfg)
	}
}

func SetFailoverConfigHandler(svc *services.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.FailoverConfig
		if err := c.Bind().Body(&cfg); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetConfig(cfg); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

func GetFailoverEventsHandler(svc *services.FailoverService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(svc.GetEvents())
	}
}
