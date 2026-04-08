package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// Dependencies holds all service dependencies for API handlers.
type Dependencies struct {
	Auth           *auth.AuthService
	AuthStore      *auth.FileAuthStore
	Blocklist      *auth.TokenBlocklist
	RateLimiter    *auth.RateLimiter
	System         *services.SystemService
	Network        *services.NetworkService
	SQM            *services.SQMService
	Wifi           *services.WifiService
	Vpn            *services.VpnService
	ServiceManager *services.ServiceManager
	Captive        *services.CaptiveService
	AdGuard        *services.AdGuardService
	Alerts         *services.AlertService
	UptimeTracker  *services.UptimeTracker
	DataUsage      *services.DataUsageService
	USBTether      *services.USBTetheringService
	BandSwitching  *services.BandSwitchingService
	Failover       *services.FailoverService
	Snapshots      *services.SnapshotService
}

// SetupRoutes registers all API routes under /api/v1/.
func SetupRoutes(app *fiber.App, deps *Dependencies) {
	// OpenAPI spec — served without auth for agent/automation use.
	app.Get("/api/openapi.json", OpenAPIHandler())

	v1 := app.Group("/api/v1")

	// Auth routes (login does not require auth)
	v1.Post("/auth/login", LoginHandler(deps.Auth, deps.RateLimiter))
	v1.Post("/auth/logout", LogoutHandler(deps.Auth, deps.Blocklist))
	v1.Get("/auth/session", SessionHandler(deps.Auth))
	v1.Put("/auth/password", ChangePasswordHandler(deps.Auth, deps.AuthStore))

	// System routes
	v1.Get("/system/info", SystemInfoHandler(deps.System))
	v1.Get("/system/stats", SystemStatsHandler(deps.System))
	v1.Get("/system/logs", SystemLogsHandler(deps.System))
	v1.Get("/system/logs/kernel", SystemKernelLogsHandler(deps.System))
	v1.Post("/system/reboot", SystemRebootHandler(deps.System))
	v1.Post("/system/shutdown", SystemShutdownHandler(deps.System))
	v1.Post("/system/factory-reset", FactoryResetHandler(deps.System))
	v1.Put("/system/hostname", SetHostnameHandler(deps.System))
	v1.Get("/system/leds", GetLEDStatusHandler(deps.System))
	v1.Put("/system/leds", SetLEDStealthHandler(deps.System))
	v1.Get("/system/leds/schedule", GetLEDScheduleHandler(deps.System))
	v1.Put("/system/leds/schedule", SetLEDScheduleHandler(deps.System))
	v1.Get("/system/timezone", GetTimezoneHandler(deps.System))
	v1.Put("/system/timezone", SetTimezoneHandler(deps.System))
	v1.Get("/system/backup", BackupHandler(deps.System))
	v1.Post("/system/restore", RestoreHandler(deps.System))
	v1.Post("/system/firmware/upgrade", FirmwareUpgradeHandler(deps.System))
	v1.Get("/system/ntp", GetNTPConfigHandler(deps.System))
	v1.Put("/system/ntp", SetNTPConfigHandler(deps.System))
	v1.Post("/system/ntp/sync", NTPSyncHandler(deps.System))
	v1.Get("/system/setup-complete", GetSetupCompleteHandler(deps.System))
	v1.Post("/system/setup-complete", SetSetupCompleteHandler(deps.System))
	v1.Post("/system/time-sync", SyncTimeHandler())
	v1.Get("/system/alerts", SystemAlertsHandler(deps.Alerts))
	v1.Get("/system/alert-thresholds", GetAlertThresholdsHandler(deps.Alerts))
	v1.Put("/system/alert-thresholds", SetAlertThresholdsHandler(deps.Alerts))
	v1.Get("/system/ssh-keys", GetSSHKeysHandler(deps.System))
	v1.Post("/system/ssh-keys", AddSSHKeyHandler(deps.System))
	v1.Delete("/system/ssh-keys/:index", DeleteSSHKeyHandler(deps.System))
	v1.Post("/system/speed-test", RunSpeedTestHandler(deps.System))
	v1.Get("/system/buttons", GetButtonsHandler(deps.System))
	v1.Put("/system/button-actions", SetButtonActionsHandler(deps.System))

	// Network routes
	v1.Get("/network/status", NetworkStatusHandler(deps.Network))
	v1.Get("/network/wan", GetWanConfigHandler(deps.Network))
	v1.Get("/network/wan/detect", DetectWanTypeHandler(deps.Network))
	v1.Put("/network/wan", SetWanConfigHandler(deps.Network))
	v1.Get("/network/clients", GetClientsHandler(deps.Network))
	v1.Get("/network/dhcp", GetDHCPConfigHandler(deps.Network))
	v1.Put("/network/dhcp", SetDHCPConfigHandler(deps.Network))
	v1.Get("/network/dns", GetDNSConfigHandler(deps.Network))
	v1.Put("/network/dns", SetDNSConfigHandler(deps.Network))
	v1.Get("/network/dhcp/leases", GetDHCPLeasesHandler(deps.Network))
	v1.Put("/network/clients/alias", SetClientAliasHandler(deps.Network))
	v1.Get("/network/dns/entries", GetDNSEntriesHandler(deps.Network))
	v1.Post("/network/dns/entries", AddDNSEntryHandler(deps.Network))
	v1.Delete("/network/dns/entries/:section", DeleteDNSEntryHandler(deps.Network))
	v1.Get("/network/dhcp/reservations", GetDHCPReservationsHandler(deps.Network))
	v1.Post("/network/dhcp/reservations", AddDHCPReservationHandler(deps.Network))
	v1.Delete("/network/dhcp/reservations/:section", DeleteDHCPReservationHandler(deps.Network))
	v1.Post("/network/interfaces/:name/state", SetInterfaceStateHandler(deps.Network))
	v1.Post("/network/clients/kick", KickClientHandler(deps.Network))
	v1.Post("/network/clients/block", BlockClientHandler(deps.Network))
	v1.Post("/network/clients/unblock", UnblockClientHandler(deps.Network))
	v1.Get("/network/clients/blocked", GetBlockedClientsHandler(deps.Network))
	v1.Get("/network/ddns", GetDDNSConfigHandler(deps.Network))
	v1.Put("/network/ddns", SetDDNSConfigHandler(deps.Network))
	v1.Get("/network/ddns/status", GetDDNSStatusHandler(deps.Network))
	v1.Get("/network/uptime-log", GetUptimeLogHandler(deps.UptimeTracker))
	v1.Get("/network/failover", GetFailoverConfigHandler(deps.Failover))
	v1.Put("/network/failover", SetFailoverConfigHandler(deps.Failover))
	v1.Get("/network/failover/events", GetFailoverEventsHandler(deps.Failover))
	v1.Get("/network/firewall/zones", GetFirewallZonesHandler(deps.Network))
	v1.Get("/network/firewall/port-forwards", GetPortForwardsHandler(deps.Network))
	v1.Post("/network/firewall/port-forwards", AddPortForwardHandler(deps.Network))
	v1.Delete("/network/firewall/port-forwards/:id", DeletePortForwardHandler(deps.Network))
	v1.Post("/network/diagnostics", RunDiagnosticsHandler(deps.Network))
	v1.Get("/network/doh", GetDoHConfigHandler(deps.Network))
	v1.Put("/network/doh", SetDoHConfigHandler(deps.Network))
	v1.Get("/network/ipv6", GetIPv6StatusHandler(deps.Network))
	v1.Put("/network/ipv6", SetIPv6EnabledHandler(deps.Network))
	v1.Post("/network/wol", SendWoLHandler(deps.Network))

	// SQM routes
	v1.Get("/sqm/config", GetSQMConfigHandler(deps.SQM))
	v1.Put("/sqm/config", SetSQMConfigHandler(deps.SQM))
	v1.Post("/sqm/apply", ApplySQMHandler(deps.SQM))

	// WiFi routes
	v1.Get("/wifi/scan", WifiScanHandler(deps.Wifi))
	v1.Post("/wifi/connect", WifiConnectHandler(deps.Wifi))
	v1.Post("/wifi/disconnect", WifiDisconnectHandler(deps.Wifi))
	v1.Get("/wifi/connection", WifiConnectionHandler(deps.Wifi))
	v1.Put("/wifi/mode", WifiSetModeHandler(deps.Wifi))
	v1.Get("/wifi/saved", WifiSavedHandler(deps.Wifi))
	v1.Delete("/wifi/saved/:section", WifiDeleteHandler(deps.Wifi))
	v1.Put("/wifi/saved/priority", WifiSetPriorityHandler(deps.Wifi))
	v1.Get("/wifi/radio", GetRadioStatusHandler(deps.Wifi))
	v1.Put("/wifi/radio", SetRadioEnabledHandler(deps.Wifi))
	v1.Get("/wifi/ap", GetAPConfigHandler(deps.Wifi))
	v1.Put("/wifi/ap/:section", SetAPConfigHandler(deps.Wifi))
	v1.Get("/wifi/radios", GetRadiosHandler(deps.Wifi))
	v1.Put("/wifi/radios/:name/role", SetRadioRoleHandler(deps.Wifi))
	v1.Get("/wifi/band-switching", GetBandSwitchingHandler(deps.BandSwitching))
	v1.Put("/wifi/band-switching", SetBandSwitchingHandler(deps.BandSwitching))
	v1.Get("/wifi/mac", GetMACHandler(deps.Wifi))
	v1.Put("/wifi/mac", SetMACHandler(deps.Wifi))
	v1.Post("/wifi/mac/randomize", RandomizeMACHandler(deps.Wifi))
	v1.Get("/wifi/guest", GetGuestWifiHandler(deps.Wifi))
	v1.Put("/wifi/guest", SetGuestWifiHandler(deps.Wifi))
	v1.Get("/wifi/autoreconnect", GetAutoReconnectHandler(deps.Wifi))
	v1.Put("/wifi/autoreconnect", SetAutoReconnectHandler(deps.Wifi))
	v1.Post("/wifi/apply/confirm", ConfirmWifiApplyHandler(deps.Wifi))
	v1.Get("/wifi/schedule", GetWiFiScheduleHandler(deps.Wifi))
	v1.Put("/wifi/schedule", SetWiFiScheduleHandler(deps.Wifi))
	v1.Get("/wifi/mac-policies", GetMACPoliciesHandler(deps.Wifi))
	v1.Put("/wifi/mac-policies", SetMACPoliciesHandler(deps.Wifi))

	// VPN routes
	v1.Get("/vpn/status", VpnStatusHandler(deps.Vpn))
	v1.Get("/vpn/wireguard", GetWireguardHandler(deps.Vpn))
	v1.Put("/vpn/wireguard", SetWireguardHandler(deps.Vpn))
	v1.Post("/vpn/wireguard/toggle", ToggleWireguardHandler(deps.Vpn))
	v1.Post("/vpn/wireguard/import", ImportWireguardHandler(deps.Vpn))
	v1.Get("/vpn/wireguard/status", GetWireguardStatusHandler(deps.Vpn))
	v1.Get("/vpn/wireguard/profiles", GetWireguardProfilesHandler(deps.Vpn))
	v1.Post("/vpn/wireguard/profiles", AddWireguardProfileHandler(deps.Vpn))
	v1.Delete("/vpn/wireguard/profiles/:id", DeleteWireguardProfileHandler(deps.Vpn))
	v1.Post("/vpn/wireguard/profiles/:id/activate", ActivateWireguardProfileHandler(deps.Vpn))
	v1.Get("/vpn/killswitch", GetKillSwitchHandler(deps.Vpn))
	v1.Put("/vpn/killswitch", SetKillSwitchHandler(deps.Vpn))
	v1.Get("/vpn/tailscale", GetTailscaleHandler(deps.Vpn))
	v1.Post("/vpn/tailscale/toggle", ToggleTailscaleHandler(deps.Vpn))
	v1.Post("/vpn/tailscale/auth", TailscaleAuthHandler(deps.Vpn))
	v1.Post("/vpn/tailscale/exit-node", SetTailscaleExitNodeHandler(deps.Vpn))
	v1.Get("/vpn/tailscale/ssh", GetTailscaleSSHHandler(deps.Vpn))
	v1.Put("/vpn/tailscale/ssh", SetTailscaleSSHHandler(deps.Vpn))
	v1.Get("/vpn/dns-leak-test", DNSLeakTestHandler(deps.Vpn))
	v1.Post("/vpn/speed-test", RunWireGuardSpeedTestHandler(deps.Vpn))
	v1.Get("/vpn/wireguard/verify", VerifyWireguardHandler(deps.Vpn))
	v1.Get("/vpn/split-tunnel", GetSplitTunnelHandler(deps.Vpn))
	v1.Put("/vpn/split-tunnel", SetSplitTunnelHandler(deps.Vpn))

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
	v1.Get("/adguard/dns", AdGuardDNSStatusHandler(deps.AdGuard))
	v1.Put("/adguard/dns", SetAdGuardDNSHandler(deps.AdGuard))
	v1.Get("/adguard/config", GetAdGuardConfigHandler(deps.AdGuard))
	v1.Put("/adguard/config", SetAdGuardConfigHandler(deps.AdGuard))
	v1.Put("/adguard/password", SetAdGuardPasswordHandler(deps.AdGuard))

	// Data usage tracking (requires vnstat)
	v1.Get("/network/data-usage", GetDataUsageHandler(deps.DataUsage))
	v1.Post("/network/data-usage/reset", ResetDataUsageHandler(deps.DataUsage))
	v1.Get("/network/data-usage/budget", GetDataBudgetHandler(deps.DataUsage))
	v1.Put("/network/data-usage/budget", SetDataBudgetHandler(deps.DataUsage))

	// USB Tethering
	v1.Get("/network/usb-tethering", GetUSBTetherStatusHandler(deps.USBTether))
	v1.Post("/network/usb-tethering/configure", ConfigureUSBTetherHandler(deps.USBTether))
	v1.Post("/network/usb-tethering/unconfigure", UnconfigureUSBTetherHandler(deps.USBTether))

	// Captive portal
	v1.Get("/captive/status", CaptiveStatusHandler(deps.Captive))
	v1.Post("/captive/auto-accept", CaptiveAutoAcceptHandler(deps.Captive))

	// Configuration snapshots
	v1.Post("/snapshots", CreateSnapshotHandler(deps.Snapshots))
	v1.Get("/snapshots", ListSnapshotsHandler(deps.Snapshots))
	v1.Get("/snapshots/:id", GetSnapshotHandler(deps.Snapshots))
	v1.Post("/snapshots/:id/restore", RestoreSnapshotHandler(deps.Snapshots, deps.System.Applier))
	v1.Delete("/snapshots/:id", DeleteSnapshotHandler(deps.Snapshots))
	v1.Post("/snapshots/clean", CleanSnapshotsHandler(deps.Snapshots))
}
