package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// NetworkStatusHandler handles GET /api/v1/network/status.
func NetworkStatusHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status, err := svc.GetNetworkStatus()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(status)
	}
}

// GetWanConfigHandler handles GET /api/v1/network/wan.
func GetWanConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		config, err := svc.GetWanConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(config)
	}
}

// SetWanConfigHandler handles PUT /api/v1/network/wan.
func SetWanConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var config models.WanConfig
		if err := c.BodyParser(&config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		// Validate WAN type
		if !isValidWanType(config.Type) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "type must be one of: dhcp, static, pppoe"})
		}

		// For static config, validate IP fields
		if config.Type == "static" {
			if config.IPAddress != "" && !isValidIPv4(config.IPAddress) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid IP address"})
			}
			if config.Gateway != "" && !isValidIPv4(config.Gateway) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid gateway address"})
			}
			if config.Netmask != "" && !isValidNetmask(config.Netmask) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid netmask"})
			}
		}

		// Validate MTU if provided
		if config.MTU != 0 && !isValidMTU(config.MTU) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("MTU must be between 68 and 9000, got %d", config.MTU)})
		}

		// Validate DNS servers if provided
		for _, dns := range config.DNSServers {
			if !isValidIPv4(dns) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("invalid DNS server IP: %s", dns)})
			}
		}

		if err := svc.SetWanConfig(config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetClientsHandler handles GET /api/v1/network/clients.
func GetClientsHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		clients, err := svc.GetClients()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(clients)
	}
}
