package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// Dependencies holds all service dependencies for API handlers.
type Dependencies struct {
	Auth           *auth.AuthService
	Blocklist      *auth.TokenBlocklist
	RateLimiter    *auth.RateLimiter
	System         *services.SystemService
	Network        *services.NetworkService
	Wifi           *services.WifiService
	Vpn            *services.VpnService
	ServiceManager *services.ServiceManager
	Captive        *services.CaptiveService
	AdGuard        *services.AdGuardService
}

// SetupRoutes registers all API routes under /api/v1/.
func SetupRoutes(app *fiber.App, deps *Dependencies) {
	v1 := app.Group("/api/v1")

	// Auth routes (login does not require auth)
	v1.Post("/auth/login", LoginHandler(deps.Auth, deps.RateLimiter))
	v1.Post("/auth/logout", LogoutHandler(deps.Auth, deps.Blocklist))
	v1.Get("/auth/session", SessionHandler(deps.Auth))
	v1.Put("/auth/password", ChangePasswordHandler(deps.Auth))

	// System routes
	v1.Get("/system/info", SystemInfoHandler(deps.System))
	v1.Get("/system/stats", SystemStatsHandler(deps.System))
	v1.Get("/system/logs", SystemLogsHandler(deps.System))
	v1.Get("/system/logs/kernel", SystemKernelLogsHandler(deps.System))
	v1.Post("/system/reboot", SystemRebootHandler(deps.System))
	v1.Post("/system/factory-reset", FactoryResetHandler(deps.System))
	v1.Put("/system/hostname", SetHostnameHandler(deps.System))
	v1.Get("/system/leds", GetLEDStatusHandler(deps.System))
	v1.Put("/system/leds", SetLEDStealthHandler(deps.System))
	v1.Get("/system/timezone", GetTimezoneHandler(deps.System))
	v1.Put("/system/timezone", SetTimezoneHandler(deps.System))
	v1.Get("/system/backup", BackupHandler(deps.System))
	v1.Post("/system/restore", RestoreHandler(deps.System))

	// Network routes
	v1.Get("/network/status", NetworkStatusHandler(deps.Network))
	v1.Get("/network/wan", GetWanConfigHandler(deps.Network))
	v1.Put("/network/wan", SetWanConfigHandler(deps.Network))
	v1.Get("/network/clients", GetClientsHandler(deps.Network))
	v1.Get("/network/dhcp", GetDHCPConfigHandler(deps.Network))
	v1.Put("/network/dhcp", SetDHCPConfigHandler(deps.Network))
	v1.Get("/network/dns", GetDNSConfigHandler(deps.Network))
	v1.Put("/network/dns", SetDNSConfigHandler(deps.Network))
	v1.Get("/network/dhcp/leases", GetDHCPLeasesHandler(deps.Network))

	// WiFi routes
	v1.Get("/wifi/scan", WifiScanHandler(deps.Wifi))
	v1.Post("/wifi/connect", WifiConnectHandler(deps.Wifi))
	v1.Post("/wifi/disconnect", WifiDisconnectHandler(deps.Wifi))
	v1.Get("/wifi/connection", WifiConnectionHandler(deps.Wifi))
	v1.Put("/wifi/mode", WifiSetModeHandler(deps.Wifi))
	v1.Get("/wifi/saved", WifiSavedHandler(deps.Wifi))
	v1.Delete("/wifi/saved/:section", WifiDeleteHandler(deps.Wifi))
	v1.Get("/wifi/ap", GetAPConfigHandler(deps.Wifi))
	v1.Put("/wifi/ap/:section", SetAPConfigHandler(deps.Wifi))
	v1.Get("/wifi/mac", GetMACHandler(deps.Wifi))
	v1.Put("/wifi/mac", SetMACHandler(deps.Wifi))

	// VPN routes
	v1.Get("/vpn/status", VpnStatusHandler(deps.Vpn))
	v1.Get("/vpn/wireguard", GetWireguardHandler(deps.Vpn))
	v1.Put("/vpn/wireguard", SetWireguardHandler(deps.Vpn))
	v1.Post("/vpn/wireguard/toggle", ToggleWireguardHandler(deps.Vpn))
	v1.Post("/vpn/wireguard/import", ImportWireguardHandler(deps.Vpn))
	v1.Get("/vpn/tailscale", GetTailscaleHandler(deps.Vpn))
	v1.Post("/vpn/tailscale/toggle", ToggleTailscaleHandler(deps.Vpn))

	// Services routes
	v1.Get("/services", ListServicesHandler(deps.ServiceManager))
	v1.Post("/services/:id/install", InstallServiceHandler(deps.ServiceManager))
	v1.Post("/services/:id/install/stream", InstallServiceStreamHandler(deps.ServiceManager))
	v1.Post("/services/:id/remove", RemoveServiceHandler(deps.ServiceManager))
	v1.Post("/services/:id/remove/stream", RemoveServiceStreamHandler(deps.ServiceManager))
	v1.Post("/services/:id/start", StartServiceHandler(deps.ServiceManager))
	v1.Post("/services/:id/stop", StopServiceHandler(deps.ServiceManager))
	v1.Post("/services/:id/autostart", SetAutoStartHandler(deps.ServiceManager))

	// AdGuard Home
	v1.Get("/services/adguardhome/status", AdGuardStatusHandler(deps.AdGuard))

	// Captive portal
	v1.Get("/captive/status", CaptiveStatusHandler(deps.Captive))
}
