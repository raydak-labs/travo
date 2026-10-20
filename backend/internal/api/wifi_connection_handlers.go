package api

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// WifiScanHandler handles GET /api/v1/wifi/scan.
func WifiScanHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		results, err := svc.Scan()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(results)
	}
}

// WifiConnectHandler handles POST /api/v1/wifi/connect.
func WifiConnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.WifiConfig
		if err := c.Bind().Body(&config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}

		// Validate SSID
		if strings.TrimSpace(config.SSID) == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "SSID must not be empty")
		}

		// Validate password for encrypted networks
		if config.Encryption != "none" && config.Password != "" && len(config.Password) < 8 {
			return RespondWithError(c, fiber.StatusBadRequest, "password must be at least 8 characters for WPA networks")
		}

		apply, err := svc.Connect(config)
		if err != nil {
			if errors.Is(err, services.ErrPasswordRequiredForNewSTA) ||
				errors.Is(err, services.ErrEncryptionRequiredForNewSTA) {
				return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
			}
			return RespondWithServerError(c, err)
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// WifiDisconnectHandler handles POST /api/v1/wifi/disconnect.
func WifiDisconnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		apply, err := svc.Disconnect()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// WifiConnectionHandler handles GET /api/v1/wifi/connection.
func WifiConnectionHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		conn, err := svc.GetConnection()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(conn)
	}
}

// WifiHealthHandler handles GET /api/v1/wifi/health.
func WifiHealthHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		health, err := svc.GetHealth()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(health)
	}
}

// WifiSetModeHandler handles PUT /api/v1/wifi/mode.
func WifiSetModeHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Mode               string `json:"mode"`
			AcknowledgeLockout bool   `json:"acknowledge_lockout"`
		}
		// Strict, like every other whole-config write: a client that misspells
		// "acknowledge_lockout" must be told, not silently answered 200 with the
		// change refused by the guard.
		if err := BindStrictBodyConfig(c, &body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if strings.TrimSpace(body.Mode) == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "mode is required")
		}
		apply, err := svc.SetMode(body.Mode, lockoutRequest(c, body.AcknowledgeLockout))
		if err != nil {
			return respondWifiMutationError(c, err)
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// WifiSavedHandler handles GET /api/v1/wifi/saved.
func WifiSavedHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		networks, err := svc.GetSavedNetworks()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		if networks == nil {
			networks = []models.SavedNetwork{}
		}
		return c.JSON(networks)
	}
}

// WifiDeleteHandler handles DELETE /api/v1/wifi/saved/:section.
func WifiDeleteHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		section := c.Params("section")
		if section == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "section parameter is required")
		}
		apply, err := svc.DeleteNetwork(section)
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// WifiSetPriorityHandler handles PUT /api/v1/wifi/saved/priority.
func WifiSetPriorityHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			SSIDs []string `json:"ssids"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if len(body.SSIDs) == 0 {
			return RespondWithError(c, fiber.StatusBadRequest, "ssids list must not be empty")
		}
		if err := svc.ReorderNetworks(body.SSIDs); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
