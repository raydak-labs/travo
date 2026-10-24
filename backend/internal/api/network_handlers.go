package api

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// NetworkStatusHandler handles GET /api/v1/network/status.
func NetworkStatusHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetNetworkStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// GetWanConfigHandler handles GET /api/v1/network/wan.
func GetWanConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetWanConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(config)
	}
}

// DetectWanTypeHandler handles GET /api/v1/network/wan/detect.
func DetectWanTypeHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		result, err := svc.DetectWanType()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(result)
	}
}

// SetWanConfigHandler handles PUT /api/v1/network/wan.
func SetWanConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.WanConfig
		if err := BindStrictBodyConfig(c, &config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}

		// Validate WAN type
		if !isValidWanType(config.Type) {
			return RespondWithError(c, fiber.StatusBadRequest, "type must be one of: dhcp, static, pppoe")
		}

		// For static config, validate IP fields
		if config.Type == "static" {
			if config.IPAddress != "" && !isValidIPv4(config.IPAddress) {
				return RespondWithError(c, fiber.StatusBadRequest, "invalid IP address")
			}
			if config.Gateway != "" && !isValidIPv4(config.Gateway) {
				return RespondWithError(c, fiber.StatusBadRequest, "invalid gateway address")
			}
			if config.Netmask != "" && !isValidNetmask(config.Netmask) {
				return RespondWithError(c, fiber.StatusBadRequest, "invalid netmask")
			}
		}

		// Validate MTU if provided
		if config.MTU != 0 && !isValidMTU(config.MTU) {
			return RespondWithError(c, fiber.StatusBadRequest, fmt.Sprintf("MTU must be between 68 and 9000, got %d", config.MTU))
		}

		// Validate DNS servers if provided
		for _, dns := range config.DNSServers {
			if !isValidIPv4(dns) {
				return RespondWithError(c, fiber.StatusBadRequest, fmt.Sprintf("invalid DNS server IP: %s", dns))
			}
		}

		if err := svc.SetWanConfig(config); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// SetInterfaceStateHandler handles POST /api/v1/network/interfaces/:name/state.
func SetInterfaceStateHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		name := c.Params("name")
		if name == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "interface name is required")
		}
		var req models.SetInterfaceStateRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetInterfaceState(name, req.Up); err != nil {
			if err.Error() == fmt.Sprintf("unknown interface: %s", name) {
				return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
			}
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetDDNSConfigHandler handles GET /api/v1/network/ddns.
func GetDDNSConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetDDNSConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		// Report availability alongside the config. Without ddns-scripts there is
		// nothing that can act on this config, and ddns is not in the service
		// catalog, so the UI cannot offer an install — it can only explain. It
		// needs this flag to do that instead of offering a form whose only
		// possible outcome is an error.
		return c.JSON(fiber.Map{
			"config":    config,
			"available": svc.DDNSAvailable(),
		})
	}
}

// SetDDNSConfigHandler handles PUT /api/v1/network/ddns.
func SetDDNSConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.DDNSConfig
		// Strict: the handler persists the whole DDNS config, so an
		// unrecognised field must not silently zero it.
		if err := BindStrictBodyConfig(c, &config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if config.Enabled {
			if config.Domain == "" {
				return RespondWithError(c, fiber.StatusBadRequest, "domain is required when DDNS is enabled")
			}
			serviceName := strings.TrimSpace(config.Service)
			if strings.EqualFold(serviceName, "custom") {
				raw := strings.TrimSpace(config.UpdateURL)
				if raw == "" {
					return RespondWithError(c, fiber.StatusBadRequest, "update_url is required for custom DDNS")
				}
				u, err := url.Parse(raw)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
					return RespondWithError(c, fiber.StatusBadRequest, "update_url must be a valid http(s) URL")
				}
			} else if serviceName == "" {
				return RespondWithError(c, fiber.StatusBadRequest, "service is required when DDNS is enabled")
			}
		}
		if err := svc.SetDDNSConfig(config); err != nil {
			// A missing ddns-scripts package is a capability gap, not a bug in
			// the request and not a server fault. 503 plus the reason lets the UI
			// say "install ddns-scripts" instead of showing a bare 500.
			if errors.Is(err, services.ErrDDNSNotAvailable) {
				return RespondWithError(c, fiber.StatusServiceUnavailable, err.Error())
			}
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetUptimeLogHandler handles GET /api/v1/network/uptime-log.
func GetUptimeLogHandler(tracker *services.UptimeTracker) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(tracker.GetUptimeLog())
	}
}

// GetDDNSStatusHandler handles GET /api/v1/network/ddns/status.
func GetDDNSStatusHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetDDNSStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// RunDiagnosticsHandler handles POST /api/v1/network/diagnostics.
//
// The type and the target are validated here rather than in the service: a
// target that a diagnostic tool would read as an option (a leading dash) or
// that is not a host at all must be a 400, not a 200 carrying the tool's
// complaint in an "error" field.
func RunDiagnosticsHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.DiagnosticsRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		switch req.Type {
		case "ping", "traceroute", "dns":
		default:
			return RespondWithError(c, fiber.StatusBadRequest, "type must be one of: ping, traceroute, dns")
		}
		if !isValidDiagnosticsTarget(req.Target) {
			return RespondWithError(c, fiber.StatusBadRequest,
				"target must be a hostname or IPv4 address, and must not start with '-'")
		}
		result := svc.RunDiagnostics(req)
		return c.JSON(result)
	}
}

// GetDoHConfigHandler handles GET /api/v1/network/doh.
func GetDoHConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetDoHConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(cfg)
	}
}

// SetDoHConfigHandler handles PUT /api/v1/network/doh.
func SetDoHConfigHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.DoHConfig
		// Strict: SetDoHConfig persists the whole config, so an unrecognised
		// field must not silently zero it.
		if err := BindStrictBodyConfig(c, &cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetDoHConfig(cfg); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// GetIPv6StatusHandler handles GET /api/v1/network/ipv6.
func GetIPv6StatusHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetIPv6Status()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// SetIPv6EnabledHandler handles PUT /api/v1/network/ipv6.
func SetIPv6EnabledHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SetIPv6Enabled(req.Enabled); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// SendWoLHandler handles POST /api/v1/network/wol.
func SendWoLHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req models.WoLRequest
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody+": "+err.Error())
		}
		if err := svc.SendWoL(req.MAC, req.Interface); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// ConnectionMethodHandler handles GET /api/v1/network/connection-method.
func ConnectionMethodHandler(svc *services.NetworkService) fiber.Handler {
	return func(c fiber.Ctx) error {
		clientIP := c.IP()
		if clientIP == "" {
			return RespondWithError(c, fiber.StatusInternalServerError, "could not determine client IP")
		}

		resp, err := svc.GetConnectionMethod(clientIP)
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(resp)
	}
}

// GetTrafficHistoryHandler handles GET /api/v1/network/traffic-history.
//
// It lets the dashboard chart paint the last minutes of per-interface traffic on
// page load instead of starting empty and waiting for WebSocket pushes. The
// point list is empty right after boot, which is a valid response.
func GetTrafficHistoryHandler(svc *services.TrafficHistoryService) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"points":           svc.History(),
			"retained_seconds": svc.RetainedSeconds(),
		})
	}
}
