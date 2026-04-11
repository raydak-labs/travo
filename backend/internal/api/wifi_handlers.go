package api

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

func wifiMutationResponse(apply *services.WirelessApplyResult) fiber.Map {
	resp := fiber.Map{"status": "ok"}
	if apply != nil {
		resp["apply"] = fiber.Map{
			"pending":                  true,
			"token":                    apply.Token,
			"rollback_timeout_seconds": apply.RollbackTimeoutSeconds,
		}
	}
	return resp
}

// WifiScanHandler handles GET /api/v1/wifi/scan.
func WifiScanHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		results, err := svc.Scan()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(results)
	}
}

// WifiConnectHandler handles POST /api/v1/wifi/connect.
func WifiConnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.WifiConfig
		if err := c.Bind().Body(&config); err != nil {
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

		apply, err := svc.Connect(config)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// WifiDisconnectHandler handles POST /api/v1/wifi/disconnect.
func WifiDisconnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		apply, err := svc.Disconnect()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// WifiConnectionHandler handles GET /api/v1/wifi/connection.
func WifiConnectionHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		conn, err := svc.GetConnection()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(conn)
	}
}

// WifiHealthHandler handles GET /api/v1/wifi/health.
func WifiHealthHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		health, err := svc.GetHealth()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(health)
	}
}

// WifiSetModeHandler handles PUT /api/v1/wifi/mode.
func WifiSetModeHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Mode string `json:"mode"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if strings.TrimSpace(body.Mode) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mode is required"})
		}
		apply, err := svc.SetMode(body.Mode)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// WifiSavedHandler handles GET /api/v1/wifi/saved.
func WifiSavedHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		networks, err := svc.GetSavedNetworks()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "section parameter is required"})
		}
		apply, err := svc.DeleteNetwork(section)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetRadioStatusHandler handles GET /api/v1/wifi/radio.
func GetRadioStatusHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		enabled, err := svc.GetRadioStatus()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"enabled": enabled})
	}
}

// SetRadioEnabledHandler handles PUT /api/v1/wifi/radio.
func SetRadioEnabledHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		apply, err := svc.SetRadioEnabled(body.Enabled)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetAPConfigHandler handles GET /api/v1/wifi/ap.
func GetAPConfigHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		configs, err := svc.GetAPConfigs()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(configs)
	}
}

// SetAPConfigHandler handles PUT /api/v1/wifi/ap/:section.
func SetAPConfigHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		section := c.Params("section")
		if section == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "section parameter is required"})
		}
		var update models.APConfigUpdate
		if err := c.Bind().Body(&update); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if update.SSID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ssid is required"})
		}
		validEnc := map[string]bool{"none": true, "psk2": true, "sae": true, "psk-mixed": true}
		if update.Encryption != "" && !validEnc[update.Encryption] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "encryption must be one of: none, psk2, sae, psk-mixed"})
		}
		if update.Encryption != "" && update.Encryption != "none" && len(update.Key) < 8 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters"})
		}
		apply, err := svc.SetAPConfig(section, update)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetRepeaterOptionsHandler handles GET /api/v1/wifi/repeater-options.
func GetRepeaterOptionsHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		opts, err := svc.GetRepeaterOptions()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(opts)
	}
}

// SetRepeaterOptionsHandler handles PUT /api/v1/wifi/repeater-options.
func SetRepeaterOptionsHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body models.RepeaterOptions
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		apply, err := svc.SetRepeaterOptions(body)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		resp := wifiMutationResponse(apply)
		resp["allow_ap_on_sta_radio"] = body.AllowAPOnSTARadio
		return c.JSON(resp)
	}
}

// ReconcileRepeaterAPLayoutHandler handles POST /api/v1/wifi/repeater/reconcile.
func ReconcileRepeaterAPLayoutHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		apply, err := svc.ReconcileRepeaterAPLayout()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetMACHandler handles GET /api/v1/wifi/mac.
func GetMACHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		configs, err := svc.GetMACAddresses()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(configs)
	}
}

// SetMACHandler handles PUT /api/v1/wifi/mac.
func SetMACHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.SetMACRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		// Validate MAC format if provided (empty means reset)
		if req.MAC != "" && !isValidMAC(req.MAC) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid MAC address format (expected XX:XX:XX:XX:XX:XX)"})
		}
		apply, err := svc.SetMACAddress(req.MAC)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// RandomizeMACHandler handles POST /api/v1/wifi/mac/randomize.
func RandomizeMACHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		mac, apply, err := svc.RandomizeMAC()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		resp := wifiMutationResponse(apply)
		resp["mac"] = mac
		return c.JSON(resp)
	}
}

// WifiSetPriorityHandler handles PUT /api/v1/wifi/saved/priority.
func WifiSetPriorityHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			SSIDs []string `json:"ssids"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if len(body.SSIDs) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ssids list must not be empty"})
		}
		if err := svc.ReorderNetworks(body.SSIDs); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetRadiosHandler handles GET /api/v1/wifi/radios.
func GetRadiosHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		radios, err := svc.GetRadios()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(radios)
	}
}

// SetRadioRoleHandler handles PUT /api/v1/wifi/radios/:name/role.
func SetRadioRoleHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		radioName := c.Params("name")
		var req models.RadioRoleRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		result, err := svc.SetRadioRole(radioName, req.Role)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	}
}

// GetGuestWifiHandler handles GET /api/v1/wifi/guest.
func GetGuestWifiHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetGuestWifi()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(cfg)
	}
}

// SetGuestWifiHandler handles PUT /api/v1/wifi/guest.
func SetGuestWifiHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.GuestWifiConfig
		if err := c.Bind().Body(&cfg); err != nil {
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
		apply, err := svc.SetGuestWifi(cfg)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(wifiMutationResponse(apply))
	}
}

// GetAutoReconnectHandler handles GET /api/v1/wifi/autoreconnect.
func GetAutoReconnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		enabled, err := svc.GetAutoReconnect()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"enabled": enabled})
	}
}

// SetAutoReconnectHandler handles PUT /api/v1/wifi/autoreconnect.
func SetAutoReconnectHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetAutoReconnect(body.Enabled); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// ConfirmWifiApplyHandler handles POST /api/v1/wifi/apply/confirm.
func ConfirmWifiApplyHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Token string `json:"token"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if strings.TrimSpace(body.Token) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token is required"})
		}
		if err := svc.ConfirmApply(body.Token); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetWiFiScheduleHandler handles GET /api/v1/wifi/schedule.
func GetWiFiScheduleHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		schedule, err := svc.GetWiFiSchedule()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(schedule)
	}
}

// SetWiFiScheduleHandler handles PUT /api/v1/wifi/schedule.
func SetWiFiScheduleHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var schedule models.WiFiSchedule
		if err := c.Bind().Body(&schedule); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetWiFiSchedule(schedule); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}

// GetMACPoliciesHandler handles GET /api/v1/wifi/mac-policies.
func GetMACPoliciesHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		policies, err := svc.GetMACPolicies()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(policies)
	}
}

// SetMACPoliciesHandler handles PUT /api/v1/wifi/mac-policies.
func SetMACPoliciesHandler(svc *services.WifiService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var policies models.MACPolicies
		if err := c.Bind().Body(&policies); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if err := svc.SetMACPolicies(policies); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	}
}
