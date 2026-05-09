package api

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// parseToggleEnabled extracts the boolean "enable/enabled" field from a request
// body, handling the legacy "enable" key for backward-compatibility.
func parseToggleEnabled(c fiber.Ctx) (bool, error) {
	var body struct {
		Enabled *bool `json:"enabled"`
		Enable  *bool `json:"enable"` // backward-compat for older clients
	}
	if err := c.Bind().Body(&body); err != nil {
		return false, err
	}
	if body.Enabled != nil {
		return *body.Enabled, nil
	}
	if body.Enable != nil {
		return *body.Enable, nil
	}
	return false, nil
}

// VpnStatusHandler handles GET /api/v1/vpn/status.
func VpnStatusHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		statuses, err := svc.GetVpnStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(statuses)
	}
}

// GetWireguardHandler handles GET /api/v1/vpn/wireguard.
func GetWireguardHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		config, err := svc.GetWireguardConfig()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(config)
	}
}

// SetWireguardHandler handles PUT /api/v1/vpn/wireguard.
func SetWireguardHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var config models.WireguardConfig
		if err := c.Bind().Body(&config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}

		// Validate private key
		if config.PrivateKey != "" && !isValidBase64Key(config.PrivateKey) {
			return RespondWithError(c, fiber.StatusBadRequest, "private key must be a valid base64-encoded 32-byte key")
		}

		// Validate peers
		for i, peer := range config.Peers {
			if peer.Endpoint != "" && !isValidEndpoint(peer.Endpoint) {
				return RespondWithError(c, fiber.StatusBadRequest, fmt.Sprintf("peer %d: endpoint must be in host:port format", i))
			}
			for _, cidr := range peer.AllowedIPs {
				if !isValidCIDR(cidr) {
					return RespondWithError(c, fiber.StatusBadRequest, fmt.Sprintf("peer %d: invalid CIDR in allowed_ips: %s", i, cidr))
				}
			}
		}

		if err := svc.SetWireguardConfig(config); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// ToggleWireguardHandler handles POST /api/v1/vpn/wireguard/toggle.
func ToggleWireguardHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		enabled, err := parseToggleEnabled(c)
		if err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if err := svc.ToggleWireguard(enabled); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetTailscaleHandler handles GET /api/v1/vpn/tailscale.
func GetTailscaleHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetTailscaleStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// ToggleTailscaleHandler handles POST /api/v1/vpn/tailscale/toggle.
func ToggleTailscaleHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		enabled, err := parseToggleEnabled(c)
		if err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if err := svc.ToggleTailscale(enabled); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// TailscaleAuthHandler handles POST /api/v1/vpn/tailscale/auth.
// Starts `tailscale up` and returns the browser auth URL if login is required.
func TailscaleAuthHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			AuthKey string `json:"auth_key"`
		}
		_ = c.Bind().Body(&body)
		authURL, err := svc.StartTailscaleAuth(body.AuthKey)
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"auth_url": authURL})
	}
}

// SetTailscaleExitNodeHandler handles POST /api/v1/vpn/tailscale/exit-node.
func SetTailscaleExitNodeHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			NodeIP   string `json:"node_ip"`
			ExitNode string `json:"exit_node"` // legacy / alternate key from older clients
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		ip := body.NodeIP
		if ip == "" {
			ip = body.ExitNode
		}
		if err := svc.SetTailscaleExitNode(ip); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// ImportWireguardHandler handles POST /api/v1/vpn/wireguard/import.
func ImportWireguardHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Config string `json:"config"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if body.Config == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "config field is required")
		}
		if err := svc.ImportWireguardConfig(body.Config); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return RespondOK(c)
	}
}

// GetWireguardStatusHandler handles GET /api/v1/vpn/wireguard/status.
func GetWireguardStatusHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetWireGuardStatus()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// GetWireguardProfilesHandler handles GET /api/v1/vpn/wireguard/profiles.
func GetWireguardProfilesHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		profiles, err := svc.GetProfiles()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(profiles)
	}
}

// AddWireguardProfileHandler handles POST /api/v1/vpn/wireguard/profiles.
func AddWireguardProfileHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Name   string `json:"name"`
			Config string `json:"config"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if body.Name == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "name is required")
		}
		if body.Config == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "config is required")
		}
		profile, err := svc.AddProfile(body.Name, body.Config)
		if err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(profile)
	}
}

// DeleteWireguardProfileHandler handles DELETE /api/v1/vpn/wireguard/profiles/:id.
func DeleteWireguardProfileHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "profile id is required")
		}
		if err := svc.DeleteProfile(id); err != nil {
			return RespondWithError(c, fiber.StatusNotFound, err.Error())
		}
		return RespondOK(c)
	}
}

// ActivateWireguardProfileHandler handles POST /api/v1/vpn/wireguard/profiles/:id/activate.
func ActivateWireguardProfileHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return RespondWithError(c, fiber.StatusBadRequest, "profile id is required")
		}
		if err := svc.ActivateProfile(id); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return RespondOK(c)
	}
}

// GetKillSwitchHandler handles GET /api/v1/vpn/killswitch.
func GetKillSwitchHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := svc.GetKillSwitch()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(status)
	}
}

// SetKillSwitchHandler handles PUT /api/v1/vpn/killswitch.
func SetKillSwitchHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if err := svc.SetKillSwitch(body.Enabled); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// DNSLeakTestHandler handles GET /api/v1/vpn/dns-leak-test.
func DNSLeakTestHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		result := svc.RunDNSLeakTest()
		return c.JSON(result)
	}
}

// RunWireGuardSpeedTestHandler handles POST /api/v1/vpn/speed-test.
func RunWireGuardSpeedTestHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		result, err := svc.RunWireGuardSpeedTest()
		if err != nil {
			// All current errors are preconditions (tunnel not usable for the test).
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		return c.JSON(result)
	}
}

// VerifyWireguardHandler handles GET /api/v1/vpn/wireguard/verify.
// Returns interface status, handshake recency, route check, and firewall plumbing state.
func VerifyWireguardHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		result := svc.VerifyWireGuard()
		return c.JSON(result)
	}
}

// GetSplitTunnelHandler handles GET /api/v1/vpn/split-tunnel.
func GetSplitTunnelHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := svc.GetSplitTunnel()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(cfg)
	}
}

// SetSplitTunnelHandler handles PUT /api/v1/vpn/split-tunnel.
func SetSplitTunnelHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg models.SplitTunnelConfig
		if err := c.Bind().Body(&cfg); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, ErrInvalidRequestBody)
		}
		if cfg.Mode != "all" && cfg.Mode != "custom" {
			return RespondWithError(c, fiber.StatusBadRequest, "mode must be 'all' or 'custom'")
		}
		if err := svc.SetSplitTunnel(cfg); err != nil {
			return RespondWithServerError(c, err)
		}
		return RespondOK(c)
	}
}

// GetTailscaleSSHHandler handles GET /api/v1/vpn/tailscale/ssh.
func GetTailscaleSSHHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		enabled, err := svc.GetTailscaleSSHEnabled()
		if err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"enabled": enabled})
	}
}

// SetTailscaleSSHHandler handles PUT /api/v1/vpn/tailscale/ssh.
func SetTailscaleSSHHandler(svc *services.VpnService) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return RespondWithError(c, fiber.StatusBadRequest, err.Error())
		}
		if err := svc.SetTailscaleSSHEnabled(req.Enabled); err != nil {
			return RespondWithServerError(c, err)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}
