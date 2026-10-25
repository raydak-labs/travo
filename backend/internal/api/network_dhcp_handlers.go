package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/network"
)

// GetDHCPConfigHandler handles GET /api/v1/network/dhcp.
func GetDHCPConfigHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetDHCPConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(config)
	}
}

// SetDHCPConfigHandler handles PUT /api/v1/network/dhcp.
func SetDHCPConfigHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.DHCPConfig
		if err := BindStrictBodyConfig(c, &config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}

		// Validate DHCP config
		if config.Start < 2 || config.Start > 254 {
			return RespondWithError(c, fiber.StatusBadRequest, "start must be between 2 and 254")
		}
		if config.Limit < 1 || config.Limit > 253 {
			return RespondWithError(c, fiber.StatusBadRequest, "limit must be between 1 and 253")
		}
		if config.Start+config.Limit > 255 {
			return RespondWithError(c, fiber.StatusBadRequest, "start + limit must not exceed 255")
		}
		if config.LeaseTime == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "lease_time is required")
		}

		if err := svc.SetDHCPConfig(config); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetDHCPLeasesHandler handles GET /api/v1/network/dhcp/leases.
func GetDHCPLeasesHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		leases := svc.GetDHCPLeases()
		return c.JSON(leases)
	}
}

// GetDHCPReservationsHandler handles GET /api/v1/network/dhcp/reservations.
func GetDHCPReservationsHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		reservations, err := svc.GetDHCPReservations()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(reservations)
	}
}

// AddDHCPReservationHandler handles POST /api/v1/network/dhcp/reservations.
func AddDHCPReservationHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var reservation models.DHCPReservation
		if err := c.Bind().Body(&reservation); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if reservation.Name == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "name is required")
		}
		if !isValidHostname(reservation.Name) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid hostname")
		}
		if reservation.MAC == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "mac is required")
		}
		if !isValidMAC(reservation.MAC) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid MAC address")
		}
		if reservation.IP == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "ip is required")
		}
		if !isValidIPv4(reservation.IP) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid IP address")
		}
		if err := svc.AddDHCPReservation(reservation); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// DeleteDHCPReservationHandler handles DELETE /api/v1/network/dhcp/reservations/:section.
func DeleteDHCPReservationHandler(svc *network.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		section := c.Params("section")
		if section == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "section is required")
		}
		if err := svc.DeleteDHCPReservation(section); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
