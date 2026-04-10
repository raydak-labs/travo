package api

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// CaptiveStatusHandler handles GET /api/v1/captive/status.
func CaptiveStatusHandler(svc *services.CaptiveService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.CheckCaptivePortal()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

func CaptiveAutoAcceptHandler(svc *services.CaptiveService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			PortalURL string `json:"portal_url"`
		}
		_ = c.Bind().Body(&body)

		portalURL := strings.TrimSpace(body.PortalURL)
		if portalURL == "" {
			st, err := svc.CheckCaptivePortal()
			if err != nil {
				return RespondWithServerError(c, err)
			}
			if st.PortalURL != nil {
				portalURL = strings.TrimSpace(*st.PortalURL)
			}
		}

		if portalURL == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "no portal URL available")
		}

		res, err := svc.AutoAcceptCaptivePortal(portalURL)
		if err != nil {
			return RespondWithServerError(c, err)
		}

		return c.JSON(res)
	}
}
