package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetClientsHandler handles GET /api/v1/network/clients.
func GetClientsHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		clients, err := svc.GetClients()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(clients)
	}
}

// SetClientAliasHandler handles PUT /api/v1/network/clients/alias.
func SetClientAliasHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.SetAliasRequest
		// Strict: the alias is persisted per MAC, so a body that does not match
		// the documented shape must not be accepted and silently stored under an
		// empty key.
		if err := BindStrictBodyConfig(c, &req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if req.MAC == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "MAC address is required")
		}
		if !isValidMAC(req.MAC) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid MAC address format")
		}
		if err := svc.SetAlias(req.MAC, req.Alias); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// KickClientHandler handles POST /api/v1/network/clients/kick.
func KickClientHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.ClientActionRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if req.MAC == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "MAC address is required")
		}
		if !isValidMAC(req.MAC) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid MAC address format")
		}
		if err := svc.KickClient(req.MAC); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// BlockClientHandler handles POST /api/v1/network/clients/block.
func BlockClientHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.ClientActionRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if req.MAC == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "MAC address is required")
		}
		if !isValidMAC(req.MAC) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid MAC address format")
		}
		if err := svc.BlockClient(req.MAC); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// UnblockClientHandler handles POST /api/v1/network/clients/unblock.
func UnblockClientHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.ClientActionRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if req.MAC == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "MAC address is required")
		}
		if !isValidMAC(req.MAC) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid MAC address format")
		}
		if err := svc.UnblockClient(req.MAC); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetBlockedClientsHandler handles GET /api/v1/network/clients/blocked.
func GetBlockedClientsHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		blocked, err := svc.GetBlockedClients()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(blocked)
	}
}
