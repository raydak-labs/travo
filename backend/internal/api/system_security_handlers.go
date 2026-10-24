package api

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// GetButtonsHandler handles GET /api/v1/system/buttons.
func GetButtonsHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// A parse failure means the UI would show "no buttons configured" while
		// the generated hotplug script still runs the last saved actions, so
		// report the failure instead of a misleading empty list.
		buttons, err := svc.GetHardwareButtons()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(buttons)
	}
}

// SetButtonActionsHandler handles PUT /api/v1/system/button-actions.
func SetButtonActionsHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.ButtonActionsRequest
		// Strict: this persists the whole button map. A body whose field names do
		// not match would otherwise decode to "no buttons" and answer 200 after
		// disabling every hardware button action.
		if err := BindStrictBodyConfig(c, &req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetButtonActions(req.Buttons); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		return RespondOK(c)
	}
}

// GetSSHKeysHandler handles GET /api/v1/system/ssh-keys.
func GetSSHKeysHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		resp, err := svc.GetSSHKeys()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(resp)
	}
}

// AddSSHKeyHandler handles POST /api/v1/system/ssh-keys.
func AddSSHKeyHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.AddSSHKeyRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.AddSSHKey(req.Key); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
	}
}

// DeleteSSHKeyHandler handles DELETE /api/v1/system/ssh-keys/:index.
func DeleteSSHKeyHandler(svc *services.SystemService) fiber.Handler {
	return func(c fiber.Ctx) error {
		indexStr := c.Params("index")
		index, err := strconv.Atoi(indexStr)
		if err != nil || index < 0 {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid index")
		}
		if err := svc.DeleteSSHKey(index); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}
