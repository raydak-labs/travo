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

// GetDNSConfigHandler handles GET /api/v1/network/dns.
func GetDNSConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		config, err := svc.GetDNSConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(config)
	}
}

// SetDNSConfigHandler handles PUT /api/v1/network/dns.
func SetDNSConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var config models.DNSConfig
		if err := c.BodyParser(&config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if config.UseCustomDNS {
			if len(config.Servers) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "at least one DNS server is required when using custom DNS"})
			}
			for _, s := range config.Servers {
				if !isValidIPv4(s) {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("invalid DNS server IP: %s", s)})
				}
			}
		}
		if err := svc.SetDNSConfig(config); err != nil {
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

// GetDHCPConfigHandler handles GET /api/v1/network/dhcp.
func GetDHCPConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		config, err := svc.GetDHCPConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(config)
	}
}

// SetDHCPConfigHandler handles PUT /api/v1/network/dhcp.
func SetDHCPConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var config models.DHCPConfig
		if err := c.BodyParser(&config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		// Validate DHCP config
		if config.Start < 2 || config.Start > 254 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "start must be between 2 and 254"})
		}
		if config.Limit < 1 || config.Limit > 253 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "limit must be between 1 and 253"})
		}
		if config.Start+config.Limit > 255 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "start + limit must not exceed 255"})
		}
		if config.LeaseTime == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "lease_time is required"})
		}

		if err := svc.SetDHCPConfig(config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetDHCPLeasesHandler handles GET /api/v1/network/dhcp/leases.
func GetDHCPLeasesHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		leases := svc.GetDHCPLeases()
		return c.JSON(leases)
	}
}

// SetClientAliasHandler handles PUT /api/v1/network/clients/alias.
func SetClientAliasHandler(svc *services.NetworkService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.SetAliasRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.MAC == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "MAC address is required"})
		}
		if !isValidMAC(req.MAC) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid MAC address format"})
		}
		if err := svc.SetAlias(req.MAC, req.Alias); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
