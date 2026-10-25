package api

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/failover"
	"github.com/openwrt-travel-gui/backend/internal/network"
	"github.com/openwrt-travel-gui/backend/internal/system"
	"github.com/openwrt-travel-gui/backend/internal/uplink"
	"github.com/openwrt-travel-gui/backend/internal/wifi"
)

// Dependencies holds all service dependencies for API handlers.
type Dependencies struct {
	Auth           *auth.AuthService
	Blocklist      *auth.TokenBlocklist
	RateLimiter    *auth.RateLimiter
	System         *system.Service
	Network        *network.Service
	SQM            *network.SQMService
	Wifi           *wifi.Service
	Vpn            *uplink.VpnService
	ServiceManager *system.ServiceManager
	Captive        *uplink.CaptiveService
	AdGuard        *uplink.AdGuardService
	Alerts         *system.AlertService
	UptimeTracker  *network.UptimeTracker
	DataUsage      *network.DataUsageService
	USBTether      *network.USBTetheringService
	BandSwitching  *wifi.BandSwitchingService
	Failover       *failover.Service
	StatsHistory   *system.StatsHistoryService
	TrafficHistory *network.TrafficHistoryService
	Speedtest      *system.SpeedtestService

	// Time-sync policy (see SyncTimeHandler). MinPlausible is typically the
	// binary build time: a clock before it is clearly broken. SetTime is
	// injectable for tests; nil uses the real system clock setter.
	TimeSyncMinPlausible time.Time
	TimeSyncSetTime      func(epochSec int64) error
	TimeSyncLimiter      *auth.RateLimiter
	// TimeSyncGate latches the "clock was plausible" decision so an
	// unauthenticated caller cannot rewind the clock to reopen the endpoint.
	TimeSyncGate *TimeSyncGate
}

// PublicRoutes re-exports the auth package's public path set, the single
// source of truth for "reachable without a token" (see auth.Middleware).
var PublicRoutes = auth.PublicPaths

// SetupRoutes registers every API operation on app and serves the generated
// OpenAPI document at OpenAPIPath. Authentication is enforced by
// auth.Middleware, mounted by the caller, against auth.PublicPaths.
func SetupRoutes(app *fiber.App, deps *Dependencies) *API {
	api := NewAPI(app)
	registerAuth(api, deps)
	registerSystem(api, deps)
	registerServices(api, deps.ServiceManager)
	registerNetwork(api, deps)
	registerWifi(api, deps)
	registerVPN(api, deps.Vpn)
	registerAdGuard(api, deps.AdGuard, deps.Captive)
	registerCaptive(api, deps.Captive)
	return api
}
