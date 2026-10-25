package api

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/wifi"
)

// GetMACHandler handles GET /api/v1/wifi/mac.
func GetMACHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		configs, err := svc.GetMACAddresses()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(configs)
	}
}

// SetMACHandler handles PUT /api/v1/wifi/mac.
func SetMACHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.SetMACRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		// Validate MAC format if provided (empty means reset)
		if req.MAC != "" && !isValidMAC(req.MAC) {
			return RespondWithError(c, fiber.StatusBadRequest, "invalid MAC address format (expected XX:XX:XX:XX:XX:XX)")
		}
		apply, err := svc.SetMACAddress(req.MAC)
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// RandomizeMACHandler handles POST /api/v1/wifi/mac/randomize.
func RandomizeMACHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		mac, apply, err := svc.RandomizeMAC()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		resp := wifiMutationResponse(apply)
		resp["mac"] = mac
		return c.JSON(resp)
	}
}

// GetGuestWifiHandler handles GET /api/v1/wifi/guest.
func GetGuestWifiHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetGuestWifi()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(cfg)
	}
}

// SetGuestWifiHandler handles PUT /api/v1/wifi/guest.
func SetGuestWifiHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.GuestWifiConfig
		if err := BindStrictBodyConfig(c, &cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if cfg.Enabled {
			if strings.TrimSpace(cfg.SSID) == "" {
				return RespondWithError(c, fiber.StatusBadRequest, "ssid is required")
			}
			if cfg.Encryption != "" && !isValidWifiEncryption(cfg.Encryption) {
				return RespondWithError(c, fiber.StatusBadRequest, "encryption must be one of: none, psk2, sae, psk-mixed")
			}
			if cfg.Encryption != "" && cfg.Encryption != "none" && len(cfg.Key) < 8 {
				return RespondWithError(c, fiber.StatusBadRequest, "password must be at least 8 characters")
			}
		}
		apply, err := svc.SetGuestWifi(cfg, lockoutRequest(c, cfg.AcknowledgeLockout))
		if err != nil {
			return respondWifiMutationError(c, err)
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetWiFiScheduleHandler handles GET /api/v1/wifi/schedule.
func GetWiFiScheduleHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		schedule, err := svc.GetWiFiSchedule()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(schedule)
	}
}

// SetWiFiScheduleHandler handles PUT /api/v1/wifi/schedule.
func SetWiFiScheduleHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var schedule models.WiFiSchedule
		if err := BindStrictBodyConfig(c, &schedule); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		// These land in a root crontab line; validate at the boundary.
		if schedule.Enabled && schedule.OnTime != "" && schedule.OffTime != "" {
			if err := platform.ValidateHHMM(schedule.OnTime); err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, "on_time: "+err.Error())
			}
			if err := platform.ValidateHHMM(schedule.OffTime); err != nil {
				return RespondWithError(c, fiber.StatusBadRequest, "off_time: "+err.Error())
			}
		}
		if err := svc.SetWiFiSchedule(schedule); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetMACPoliciesHandler handles GET /api/v1/wifi/mac-policies.
func GetMACPoliciesHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		policies, err := svc.GetMACPolicies()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(policies)
	}
}

// SetMACPoliciesHandler handles PUT /api/v1/wifi/mac-policies.
func SetMACPoliciesHandler(svc *wifi.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		var policies models.MACPolicies
		if err := BindStrictBodyConfig(c, &policies); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetMACPolicies(policies); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}
