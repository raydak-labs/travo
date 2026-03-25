package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// CaptiveStatusHandler handles GET /api/v1/captive/status.
func CaptiveStatusHandler(svc *services.CaptiveService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status, err := svc.CheckCaptivePortal()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(status)
	}
}

func CaptiveAutoAcceptHandler(svc *services.CaptiveService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			PortalURL string `json:"portal_url"`
		}
		_ = c.BodyParser(&body)

		portalURL := strings.TrimSpace(body.PortalURL)
		if portalURL == "" {
			st, err := svc.CheckCaptivePortal()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			if st.PortalURL != nil {
				portalURL = strings.TrimSpace(*st.PortalURL)
			}
		}

		if portalURL == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no portal URL available"})
		}

		res, err := svc.AutoAcceptCaptivePortal(portalURL)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(res)
	}
}
