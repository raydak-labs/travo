package api

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/network"
)

// GetDNSConfigHandler handles GET /api/v1/network/dns.
func GetDNSConfigHandler(svc *network.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetDNSConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(config)
	}
}

// SetDNSConfigHandler handles PUT /api/v1/network/dns.
func SetDNSConfigHandler(svc *network.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.DNSConfig
		if err := BindStrictBodyConfig(c, &config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if config.UseCustomDNS {
			if len(config.Servers) == 0 {
				return RespondWithError(c, fiber.StatusBadRequest, "at least one DNS server is required when using custom DNS")
			}
			for _, s := range config.Servers {
				if !isValidIPv4(s) {
					return RespondWithError(c, fiber.StatusBadRequest, fmt.Sprintf("invalid DNS server IP: %s", s))
				}
			}
		}
		if err := svc.SetDNSConfig(config); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetDNSEntriesHandler handles GET /api/v1/network/dns/entries.
func GetDNSEntriesHandler(svc *network.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		entries, err := svc.GetDNSEntries()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(entries)
	}
}

// AddDNSEntryHandler handles POST /api/v1/network/dns/entries.
func AddDNSEntryHandler(svc *network.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var entry models.DNSEntry
		if err := c.Bind().Body(&entry); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if entry.Name == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "name is required")
		}
		if !isValidHostname(entry.Name) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid hostname")
		}
		if entry.IP == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "ip is required")
		}
		if !isValidIPv4(entry.IP) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid IP address")
		}
		if err := svc.AddDNSEntry(entry); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// DeleteDNSEntryHandler handles DELETE /api/v1/network/dns/entries/:section.
func DeleteDNSEntryHandler(svc *network.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		section := c.Params("section")
		if section == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "section is required")
		}
		if err := svc.DeleteDNSEntry(section); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
