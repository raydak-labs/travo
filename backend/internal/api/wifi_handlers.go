package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// WifiScanHandler handles GET /api/v1/wifi/scan.
func WifiScanHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		results, err := svc.Scan()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(results)
	}
}

// WifiConnectHandler handles POST /api/v1/wifi/connect.
func WifiConnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var config models.WifiConfig
		if err := c.BodyParser(&config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		// Validate SSID
		if strings.TrimSpace(config.SSID) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "SSID must not be empty"})
		}

		// Validate password for encrypted networks
		if config.Encryption != "none" && config.Password != "" && len(config.Password) < 8 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters for WPA networks"})
		}

		if err := svc.Connect(config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// WifiDisconnectHandler handles POST /api/v1/wifi/disconnect.
func WifiDisconnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := svc.Disconnect(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// WifiConnectionHandler handles GET /api/v1/wifi/connection.
func WifiConnectionHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		conn, err := svc.GetConnection()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(conn)
	}
}

// WifiSetModeHandler handles PUT /api/v1/wifi/mode.
func WifiSetModeHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Mode string `json:"mode"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetMode(body.Mode); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// WifiSavedHandler handles GET /api/v1/wifi/saved.
func WifiSavedHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		networks, err := svc.GetSavedNetworks()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(networks)
	}
}
