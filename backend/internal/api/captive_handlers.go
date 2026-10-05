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
		status.DNSBypassed = svc.IsDNSBypassed()
		status.DNSBypassNeeded = !status.DNSBypassed && status.Detected && svc.CheckDNSBypassNeeded()
		status.STAConnected = svc.IsUpstreamConnected()
		// No side effects here. This handler used to auto-restore DNS,
		// so a GET committed `dhcp` and `network`, rewrote dnsmasq's resolver
		// options and restarted dnsmasq — every poll of the captive status (the
		// dashboard does it on a timer) mutated live state, and a read could
		// interrupt DNS resolution mid-flight.
		//
		// Recovery from a stale bypass has two mutating entrypoints instead:
		// POST /api/v1/captive/dns-restore, and the bounded 5-minute startup
		// auto-restore (captiveDNSRestoreTimeout). Do not move the auto-restore
		// back onto this path; if restore-on-reconnect is wanted again, it
		// belongs on a scheduler that owns its own crash guard, not on a GET.
		return c.JSON(status)
	}
}

// CaptiveAutoAcceptHandler handles POST /api/v1/captive/auto-accept.
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

// CaptiveDNSBypassHandler handles POST /api/v1/captive/dns-bypass.
func CaptiveDNSBypassHandler(svc *services.CaptiveService) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.BypassDNS(); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true, "message": "DNS switched to upstream for captive portal access"})
	}
}

// CaptiveDNSRestoreHandler handles POST /api/v1/captive/dns-restore.
func CaptiveDNSRestoreHandler(svc *services.CaptiveService) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := svc.RestoreDNS(); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true, "message": "DNS restored to original configuration"})
	}
}
