package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// WifiScanHandler handles GET /api/v1/wifi/scan.
func WifiScanHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		results, err := svc.Scan()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(results)
	}
}

// WifiConnectHandler handles POST /api/v1/wifi/connect.
func WifiConnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var config models.WifiConfig
		if err := c.BodyParser(&config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		// Validate SSID
		if strings.TrimSpace(config.SSID) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "SSID must not be empty"})
		}

		// Validate password for encrypted networks
		if config.Encryption != "none" && config.Password != "" && len(config.Password) < 8 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters for WPA networks"})
		}

		if err := svc.Connect(config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// WifiDisconnectHandler handles POST /api/v1/wifi/disconnect.
func WifiDisconnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := svc.Disconnect(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// WifiConnectionHandler handles GET /api/v1/wifi/connection.
func WifiConnectionHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		conn, err := svc.GetConnection()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(conn)
	}
}

// WifiSetModeHandler handles PUT /api/v1/wifi/mode.
func WifiSetModeHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Mode string `json:"mode"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetMode(body.Mode); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// WifiSavedHandler handles GET /api/v1/wifi/saved.
func WifiSavedHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		networks, err := svc.GetSavedNetworks()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(networks)
	}
}

// WifiDeleteHandler handles DELETE /api/v1/wifi/saved/:section.
func WifiDeleteHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		section := c.Params("section")
		if section == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "section parameter is required"})
		}
		if err := svc.DeleteNetwork(section); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetRadioStatusHandler handles GET /api/v1/wifi/radio.
func GetRadioStatusHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		enabled, err := svc.GetRadioStatus()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"enabled": enabled})
	}
}

// SetRadioEnabledHandler handles PUT /api/v1/wifi/radio.
func SetRadioEnabledHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetRadioEnabled(body.Enabled); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetAPConfigHandler handles GET /api/v1/wifi/ap.
func GetAPConfigHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		configs, err := svc.GetAPConfigs()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(configs)
	}
}

// SetAPConfigHandler handles PUT /api/v1/wifi/ap/:section.
func SetAPConfigHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		section := c.Params("section")
		if section == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "section parameter is required"})
		}
		var config models.APConfig
		if err := c.BodyParser(&config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if config.SSID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ssid is required"})
		}
		validEnc := map[string]bool{"none": true, "psk2": true, "sae": true, "psk-mixed": true}
		if config.Encryption != "" && !validEnc[config.Encryption] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "encryption must be one of: none, psk2, sae, psk-mixed"})
		}
		if config.Encryption != "" && config.Encryption != "none" && len(config.Key) < 8 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters"})
		}
		if err := svc.SetAPConfig(section, config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetMACHandler handles GET /api/v1/wifi/mac.
func GetMACHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		configs, err := svc.GetMACAddresses()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(configs)
	}
}

// SetMACHandler handles PUT /api/v1/wifi/mac.
func SetMACHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.SetMACRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		// Validate MAC format if provided (empty means reset)
		if req.MAC != "" && !isValidMAC(req.MAC) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid MAC address format (expected XX:XX:XX:XX:XX:XX)"})
		}
		if err := svc.SetMACAddress(req.MAC); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetGuestWifiHandler handles GET /api/v1/wifi/guest.
func GetGuestWifiHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cfg, err := svc.GetGuestWifi()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(cfg)
	}
}

// SetGuestWifiHandler handles PUT /api/v1/wifi/guest.
func SetGuestWifiHandler(svc *services.WifiService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var cfg models.GuestWifiConfig
		if err := c.BodyParser(&cfg); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if cfg.Enabled {
			if strings.TrimSpace(cfg.SSID) == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ssid is required"})
			}
			validEnc := map[string]bool{"none": true, "psk2": true, "sae": true, "psk-mixed": true}
			if cfg.Encryption != "" && !validEnc[cfg.Encryption] {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "encryption must be one of: none, psk2, sae, psk-mixed"})
			}
			if cfg.Encryption != "" && cfg.Encryption != "none" && len(cfg.Key) < 8 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters"})
			}
		}
		if err := svc.SetGuestWifi(cfg); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
