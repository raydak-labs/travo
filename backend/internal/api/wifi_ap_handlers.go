package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetRadioStatusHandler handles GET /api/v1/wifi/radio.
func GetRadioStatusHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		enabled, err := svc.GetRadioStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"enabled": enabled})
	}
}

// SetRadioEnabledHandler handles PUT /api/v1/wifi/radio.
func SetRadioEnabledHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Enabled            bool `json:"enabled"`
			AcknowledgeLockout bool `json:"acknowledge_lockout"`
		}
		// Strict: see WifiSetModeHandler.
		if err := BindStrictBodyConfig(c, &body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		apply, err := svc.SetRadioEnabled(body.Enabled, lockoutRequest(c, body.AcknowledgeLockout))
		if err != nil {
			return respondWifiMutationError(c, err)
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetAPConfigHandler handles GET /api/v1/wifi/ap.
func GetAPConfigHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		configs, err := svc.GetAPConfigs()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(configs)
	}
}

// SetAPConfigHandler handles PUT /api/v1/wifi/ap/:section.
func SetAPConfigHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		section := c.Params("section")
		if section == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "section parameter is required")
		}
		var update models.APConfigUpdate
		if err := c.Bind().Body(&update); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if update.SSID == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "ssid is required")
		}
		if update.Encryption != "" && !isValidWifiEncryption(update.Encryption) {
			return RespondWithError(c, fiber.StatusBadRequest, "encryption must be one of: none, psk2, sae, psk-mixed")
		}
		if update.Encryption != "" && update.Encryption != "none" && len(update.Key) < 8 {
			return RespondWithError(c, fiber.StatusBadRequest, "password must be at least 8 characters")
		}
		apply, err := svc.SetAPConfig(section, update, lockoutRequest(c, update.AcknowledgeLockout))
		if err != nil {
			return respondWifiMutationError(c, err)
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetRadiosHandler handles GET /api/v1/wifi/radios.
func GetRadiosHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		radios, err := svc.GetRadios()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(radios)
	}
}

// SetRadioRoleHandler handles PUT /api/v1/wifi/radios/:name/role.
func SetRadioRoleHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		radioName := c.Params("name")
		var req models.RadioRoleRequest
		// Strict: see WifiSetModeHandler.
		if err := BindStrictBodyConfig(c, &req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		result, err := svc.SetRadioRole(radioName, req.Role, lockoutRequest(c, req.AcknowledgeLockout))
		if err != nil {
			return respondWifiMutationError(c, err)
		}
		// Not c.JSON(result): every other wireless mutator returns the shared
		// envelope, and a client that reads response.apply finds nothing in the raw
		// WirelessApplyResult — it then never calls confirmWifiApply and rpcd's
		// 30 s rollback silently reverts the change.
		return c.JSON(wifiMutationResponse(result))
	}
}
