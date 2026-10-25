// Package app wires the services, routes and background workers of the
// backend into one Fiber application.
package app

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/openwrt-travel-gui/backend/internal/api"
	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/config"
	"github.com/openwrt-travel-gui/backend/internal/failover"
	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/jsonx"
	"github.com/openwrt-travel-gui/backend/internal/network"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/store"
	"github.com/openwrt-travel-gui/backend/internal/system"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
	"github.com/openwrt-travel-gui/backend/internal/uplink"
	"github.com/openwrt-travel-gui/backend/internal/wifi"
	"github.com/openwrt-travel-gui/backend/internal/ws"
)

// minPlausibleFloor is the fallback when no build time is stamped: any clock
// before this date is clearly broken (device booted without RTC/NTP).
const minPlausibleFloor = "2025-01-01T00:00:00Z"

// minPlausibleTime returns the later of the stamped build time and the floor.
func minPlausibleTime(buildTime string) time.Time {
	floor, _ := time.Parse(time.RFC3339, minPlausibleFloor)
	if bt, err := time.Parse(time.RFC3339, buildTime); err == nil && bt.After(floor) {
		return bt
	}
	return floor
}

// HTTP server limits. Fiber's zero-value config inherits fasthttp's
// effectively-unbounded read/write behaviour, which lets a handful of idle
// sockets hold every connection slot (Slowloris) and makes graceful shutdown
// wait on keep-alives that never send another byte.
const (
	// readTimeout bounds the time spent reading one request (headers + body).
	// It must stay well above a slow firmware/backup upload over a hotel uplink,
	// and far below "forever".
	readTimeout = 5 * time.Minute
	// writeTimeout bounds the response write. The package-install SSE endpoints
	// legitimately stream for 3 packages x execx.Package (10 min), so this is
	// set above that ceiling rather than to a tight value.
	writeTimeout = 45 * time.Minute
	// idleTimeout closes keep-alive connections that go quiet, so Shutdown
	// completes instead of blocking on an idle client.
	idleTimeout = 60 * time.Second
	// ShutdownTimeout bounds the graceful drain; after it elapses the process
	// stops anyway (a stuck handler must not prevent shutdown).
	ShutdownTimeout = 20 * time.Second
	// goroutineDrainTimeout bounds how long Stop() waits for tracked background
	// goroutines before giving up and closing the store anyway.
	goroutineDrainTimeout = 10 * time.Second
)

// bodyLimit replaces Fiber's 4 MB default. OpenWrt sysupgrade images and
// configuration backups routinely exceed that, and the firmware/restore
// upload endpoints would otherwise answer 413 for legitimate images.
const bodyLimit = 64 * 1024 * 1024

// splitCORSOrigins parses the configured origin allowlist. An unset (or
// blank) value yields an empty slice, which means "no cross-origin access
// allowed" (same-origin only) — see corsConfig.
func splitCORSOrigins(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// corsConfig builds the CORS middleware config. Fiber treats an empty
// AllowOrigins list as "allow everything", so same-origin-only has to be
// expressed with an explicit deny-all AllowOriginsFunc instead.
func corsConfig(rawOrigins string) cors.Config {
	cfg := cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}
	origins := splitCORSOrigins(rawOrigins)
	if len(origins) == 0 {
		// No Access-Control-Allow-Origin is emitted for any Origin header, so
		// the browser blocks the response. Same-origin requests are unaffected.
		cfg.AllowOriginsFunc = func(string) bool { return false }
		return cfg
	}
	cfg.AllowOrigins = origins
	return cfg
}

// Lifecycle bundles every component with a background goroutine so callers
// can shut all of them down together (repo rule: every background goroutine
// follows lifecycle rules).
type Lifecycle struct {
	hub             *ws.Hub
	alertSvc        *system.AlertService
	uptimeTracker   *network.UptimeTracker
	bandSwitchSvc   *wifi.BandSwitchingService
	failoverSvc     *failover.Service
	blocklist       *auth.TokenBlocklist
	netWatcher      network.EventWatcher
	rateLimiter     *auth.RateLimiter
	timeSyncLimiter *auth.RateLimiter
	statsHistory    *system.StatsHistoryService
	captiveSvc      *uplink.CaptiveService
	db              *store.Store // may be nil (memory-only fallback)

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	// live counts goroutines registered via Go that have not returned yet.
	live atomic.Int64
}

// newLifecycle returns a lifecycle with the shutdown signal channel ready.
func newLifecycle() *Lifecycle {
	return &Lifecycle{stopCh: make(chan struct{})}
}

// Go runs fn as a tracked background goroutine. fn receives a channel that is
// closed when Stop() begins, so work that waits (startup delays, retries) can
// bail out instead of committing UCI changes after SIGTERM.
func (l *Lifecycle) Go(fn func(stop <-chan struct{})) {
	l.wg.Add(1)
	l.live.Add(1)
	go func() {
		defer l.wg.Done()
		defer l.live.Add(-1)
		fn(l.stopCh)
	}()
}

// sleepOrStop waits for d and reports whether it completed (false = stop
// requested).
func sleepOrStop(stop <-chan struct{}, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-stop:
		return false
	}
}

// Stop signals every tracked goroutine, shuts the services down, then closes
// the store last (statsHistory.Stop flushes into it). It is safe to call
// multiple times and from multiple goroutines. Callers must have stopped
// accepting requests (app.Shutdown) first, otherwise in-flight handlers can
// still touch a closed store.
func (l *Lifecycle) Stop() {
	l.stopOnce.Do(func() {
		close(l.stopCh)

		l.blocklist.Stop()
		l.hub.Stop()
		l.netWatcher.Stop()
		l.alertSvc.Stop()
		l.uptimeTracker.Stop()
		l.bandSwitchSvc.Stop()
		l.failoverSvc.Stop()
		l.rateLimiter.Stop()
		l.timeSyncLimiter.Stop()
		l.statsHistory.Stop()
		if l.captiveSvc != nil {
			l.captiveSvc.Stop()
		}

		// Give tracked goroutines a bounded window to unwind. A worker stuck in
		// a long exec must not hold up shutdown, so we log and continue.
		drained := make(chan struct{})
		go func() {
			l.wg.Wait()
			close(drained)
		}()
		select {
		case <-drained:
		case <-time.After(goroutineDrainTimeout):
			slog.Warn("background goroutines still running after the drain timeout; continuing shutdown", "count", l.live.Load(), "timeout", goroutineDrainTimeout)
		}

		if l.db != nil {
			_ = l.db.Close()
		}
	})
}

// Build identifies the binary, from main's ldflags-stamped variables.
type Build struct {
	// Version is "0.9.0" for a release, the git description otherwise.
	Version string
	// Time is the RFC 3339 build stamp (may be empty), the floor of a
	// plausible clock for the time-sync gate.
	Time string
}

// New wires every service and route into a Fiber application. The returned
// Lifecycle owns every background goroutine.
func New(cfg config.Config, build Build) (*fiber.App, *Lifecycle, error) {
	lifecycle := newLifecycle()

	app := fiber.New(fiber.Config{
		AppName:      "travo",
		BodyLimit:    bodyLimit,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
		// Routing must be case-sensitive so that c.Path() and the registered
		// route table agree. With Fiber's default (case-insensitive) routing
		// the path in a request could differ in case from the registered route
		// while still matching it, which is what made the /api prefix check
		// bypassable and the whole admin API reachable without a token.
		CaseSensitive: true,
		// Nil slices encode as [], as the OpenAPI document promises.
		JSONEncoder: jsonx.Marshal,
	})

	// Recover middleware — registered FIRST, before CORS and the auth
	// middleware, so it wraps everything downstream.
	//
	// Without it a panic in any of the ~250 handlers takes the whole process
	// down: the router keeps routing traffic but the UI is gone until something
	// restarts travo, on a device that may have no supervisor. A nil service
	// dereference is the easy way to hit this (several handlers assume their
	// dependency is wired), and it is exactly what a contract test that exercises
	// every documented endpoint will stumble into.
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))

	// CORS middleware
	app.Use(cors.New(corsConfig(cfg.CorsOrigins)))

	nets, err := auth.ParseCIDRList(cfg.AllowedAdminCIDRs)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid ALLOWED_ADMIN_CIDRS: %w", err)
	}
	if len(nets) > 0 {
		app.Use(auth.IPAllowlistMiddleware(nets))
		slog.Info("admin IP allowlist enabled", "cidrs", len(nets))
	}

	// Create UCI and Ubus backends
	var u uci.UCI
	var ub ubus.Ubus
	if cfg.MockMode {
		u = uci.NewMockUCI()
		ub = ubus.NewMockUbus()
		// Crash guards sit next to the other mock-mode state, never in /etc/trafo.
		guard.SetDir(filepath.Join(filepath.Dir(cfg.AuthConfigPath), "guards"))
	} else {
		u = uci.NewRealUCI()
		ub = ubus.NewRealUbus()
	}

	// Persistent KV store next to auth.json (so tests with a temp auth path get
	// a temp store). Failure degrades to memory-only instead of blocking the UI.
	var db *store.Store
	if s, err := store.Open(filepath.Join(filepath.Dir(cfg.AuthConfigPath), "travo.db")); err == nil {
		db = s
	} else {
		slog.Warn("persistent store unavailable; running memory-only", "err", err)
	}

	// Create shared root password holder (written by auth after login, read by UCI apply).
	rootPassword := auth.NewRootPassword()

	// Create services
	authStore := auth.NewFileAuthStore(cfg.AuthConfigPath)
	authCfg, err := authStore.LoadOrInit()
	if err != nil {
		return nil, nil, fmt.Errorf("loading auth config: %w", err)
	}
	if !cfg.MockMode {
		if p := auth.LoadSealedRPCDPassword(cfg.AuthConfigPath, authCfg.JWTSecret); p != "" {
			rootPassword.Set(p)
			// Refresh the helper file for the generated wireless toggle script at
			// startup too, not only on login. It is written from the seal on a
			// reboot or an upgrade that did not involve a fresh login, and without
			// it every scheduled or button-driven toggle falls back to an empty
			// password that rpcd rejects.
			if err := auth.SaveRPCDLoginHelper(cfg.AuthConfigPath, p); err != nil {
				slog.Warn("could not refresh rpcd-login helper file", "err", err)
			}
		}
	}
	var authSvc *auth.AuthService
	if cfg.MockMode {
		authSvc = auth.NewAuthService("admin", authCfg.JWTSecret)
	} else {
		authSvc = auth.NewAuthServiceWithUbus(ub, authCfg.JWTSecret, rootPassword, cfg.AuthConfigPath)
	}
	// Monotonic session registry: session lifetime is immune to wall-clock
	// jumps (NTP, time-sync, timezone fixes). See ADR 0007.
	authSvc.SetSessionRegistry(auth.NewSessionRegistry(24 * time.Hour))
	var storage system.StorageProvider
	var captiveProber platform.HTTPProber
	if cfg.MockMode {
		storage = &system.MockStorageProvider{}
		captiveProber = &platform.MockHTTPProber{StatusCode: 200, Body: "success\n"}
	} else {
		storage = &system.RealStorageProvider{}
		captiveProber = platform.NewRealHTTPProber()
	}

	systemSvc := system.New(ub, u, storage)
	systemSvc.SetTravoVersion(build.Version)
	if cfg.MockMode {
		systemSvc.SetLogTail(system.MockLogTail)
	} else {
		// Surface crash guards left behind by an interrupted run (flash,
		// restore, factory reset) so the state is not silently forgotten.
		systemSvc.LogStaleCrashGuards()
	}
	networkSvc := network.New(network.Deps{UCI: u, Ubus: ub})
	sqmSvc := network.NewSQMService(u, nil)

	// Create event watcher (noop in mock mode to avoid running ubus on dev machines).
	var netWatcher network.EventWatcher
	if cfg.MockMode {
		netWatcher = network.NewNoopEventWatcher()
	} else {
		netWatcher = network.NewNetworkEventWatcher(networkSvc)
	}
	lifecycle.netWatcher = netWatcher
	lifecycle.Go(func(stop <-chan struct{}) { netWatcher.Start() })
	var wifiSvc *wifi.Service
	if cfg.MockMode {
		wifiSvc = wifi.New(wifi.Deps{UCI: u, Ubus: ub, Reloader: &wifi.NoopWifiReloader{}})
	} else {
		wifiSvc = wifi.New(wifi.Deps{UCI: u, Ubus: ub, Applier: platform.NewRealUCIApplyConfirm(ub, rootPassword)})
	}

	// The wireless configuration and its cron entries are reconciled on
	// startup; see startWifiReconciliation for what each repair does.
	if !cfg.MockMode {
		startWifiReconciliation(lifecycle, wifiSvc)
	}

	vpnSvc := uplink.NewVpnService(uplink.VpnDeps{UCI: u})
	svcManager := system.NewServiceManager(nil, nil)
	svcManager.RefreshCache()
	runningVersion := func() (string, error) {
		info, err := systemSvc.GetSystemInfo()
		return info.FirmwareVersion, err
	}
	updateDeps := system.UpdateDeps{FirmwareVersion: runningVersion, TravoVersion: build.Version}
	if cfg.MockMode {
		updateDeps.Packages = system.NewMockPackageManager()
		updateDeps.ReleaseFeed = system.MockVersionsFeed
		updateDeps.TravoRelease = system.MockTravoRelease
	}
	updateSvc := system.NewUpdateService(updateDeps)
	startUpdateChecks(lifecycle, updateSvc)
	captiveSvc := uplink.NewCaptiveService(uplink.CaptiveDeps{Prober: captiveProber, UCI: u, Cmd: &platform.RealCommandRunner{}})
	captiveSvc.StartAutoRestore()
	adguardSvc := uplink.NewAdGuardService(nil)
	dataUsageSvc := network.NewDataUsageService(nil)
	usbTetherSvc := network.NewUSBTetheringService(nil)
	bandSwitchSvc := wifi.NewBandSwitchingService(wifiSvc, "/etc/travo/band-switching.json")
	failoverSvc := failover.New(failover.Deps{
		UCI: u, Ubus: ub, Network: networkSvc,
		Applier: platform.NewRealUCIApplyConfirm(ub, rootPassword),
	})

	// Register post-install hook: auto-configure AdGuard Home after package install.
	if !cfg.MockMode {
		svcManager.SetPostInstallHook("adguardhome", adguardSvc.AutoConfigure)
		svcManager.SetPostInstallHook("vnstat", dataUsageSvc.AutoConfigureVnstat)
	}
	alertSvc := system.NewAlertService(systemSvc)
	if !cfg.MockMode {
		alertSvc.SetCarrierChecker(&system.RealCarrierChecker{})
	}
	failoverSvc.SetAlertService(alertSvc)
	uptimeTracker := network.NewUptimeTracker(captiveProber)

	// Stats history: collect every 30s, keep 720 points (~6 hours)
	var statsHistory *system.StatsHistoryService
	if db != nil {
		statsHistory = system.NewStatsHistoryService(systemSvc, 30*time.Second, 720, db)
	} else {
		statsHistory = system.NewStatsHistoryService(systemSvc, 30*time.Second, 720, nil)
	}
	statsHistory.Start()

	// Traffic history: sampled on every WebSocket broadcast tick (2s), ~10 min
	// per interface, so a freshly loaded dashboard chart is not empty.
	trafficHistory := network.NewTrafficHistoryService(network.TrafficHistoryDefaultPoints)

	// Token blocklist with cleanup goroutine
	var blocklist *auth.TokenBlocklist
	if db != nil {
		blocklist = auth.NewTokenBlocklistWithStore(db)
	} else {
		blocklist = auth.NewTokenBlocklist()
	}
	authSvc.SetBlocklist(blocklist)
	blocklist.StartCleanup(5 * time.Minute)

	// Rate limiters: login (5/min) and unauthenticated time-sync (3/min).
	// Periodic sweeps keep the per-IP maps bounded when many distinct source
	// IPs never return (see RateLimiter.Cleanup).
	rateLimiter := auth.NewRateLimiter(5, time.Minute)
	rateLimiter.StartCleanup(5 * time.Minute)
	timeSyncLimiter := auth.NewRateLimiter(3, time.Minute)
	timeSyncLimiter.StartCleanup(5 * time.Minute)

	// Auth middleware, mounted BEFORE any /api route is registered.
	//
	// It is deliberately app.Use, not a route-group middleware: Fiber v3 scopes
	// group middleware by PATH PREFIX rather than by which router the route was
	// registered on, so mounting it on the /api/v1 group also intercepted routes
	// registered directly on the app -- the WebSocket upgrade and the public
	// time-sync endpoint -- and broke both. The public exceptions are the exact
	// paths in auth.PublicPaths, not a prefix test.
	//
	// TestProductionAppRequiresAuthOnAPIRoutes drives the app this function
	// builds, because the composition of this file is otherwise untested: an
	// earlier commit moved the middleware onto a group and, when that was
	// reverted, this line was never restored -- leaving production with no auth
	// at all while every other test stayed green.
	app.Use(authSvc.Middleware())

	// Health check endpoint. Public, per auth.PublicPaths.
	app.Get("/api/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// API routes
	deps := &api.Dependencies{
		Auth:           authSvc,
		Blocklist:      blocklist,
		RateLimiter:    rateLimiter,
		System:         systemSvc,
		Network:        networkSvc,
		SQM:            sqmSvc,
		Wifi:           wifiSvc,
		Vpn:            vpnSvc,
		ServiceManager: svcManager,
		Captive:        captiveSvc,
		AdGuard:        adguardSvc,
		Alerts:         alertSvc,
		UptimeTracker:  uptimeTracker,
		DataUsage:      dataUsageSvc,
		USBTether:      usbTetherSvc,
		BandSwitching:  bandSwitchSvc,
		Failover:       failoverSvc,
		StatsHistory:   statsHistory,
		TrafficHistory: trafficHistory,
		Speedtest:      system.NewSpeedtestService(nil),
		Updates:        updateSvc,

		TimeSyncMinPlausible: minPlausibleTime(build.Time),
		TimeSyncLimiter:      timeSyncLimiter,
		TimeSyncGate:         api.NewTimeSyncGate(!time.Now().Before(minPlausibleTime(build.Time))),
	}
	api.SetupRoutes(app, deps)

	// WebSocket (with auth from query parameter)
	hub := ws.NewHub(systemSvc, alertSvc, netWatcher.Ch(), trafficHistory)
	app.Use("/api/v1/ws", ws.UpgradeMiddleware(authSvc, splitCORSOrigins(cfg.CorsOrigins)))
	app.Get("/api/v1/ws", ws.Handler(hub, authSvc, ws.HandlerOptions{}))
	hub.Start()
	alertSvc.Start()
	uptimeTracker.Start()
	bandSwitchSvc.Start()
	lifecycle.Go(func(stop <-chan struct{}) { failoverSvc.Start() })

	// Static files (if configured)
	if cfg.StaticDir != "" {
		app.Use("/", static.New(cfg.StaticDir))
		// SPA catch-all: serve index.html for non-API routes that don't match
		// static files. Unknown API paths get a JSON 404 — returning HTML with
		// status 200 breaks API consumers on typo'd endpoints.
		app.Get("/*", func(c fiber.Ctx) error {
			if strings.HasPrefix(strings.ToLower(c.Path()), "/api/") {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
			}
			return c.SendFile(cfg.StaticDir + "/index.html")
		})
	}

	lifecycle.hub = hub
	lifecycle.alertSvc = alertSvc
	lifecycle.uptimeTracker = uptimeTracker
	lifecycle.bandSwitchSvc = bandSwitchSvc
	lifecycle.failoverSvc = failoverSvc
	lifecycle.blocklist = blocklist
	lifecycle.rateLimiter = rateLimiter
	lifecycle.timeSyncLimiter = timeSyncLimiter
	lifecycle.statsHistory = statsHistory
	lifecycle.captiveSvc = captiveSvc
	lifecycle.db = db
	return app, lifecycle, nil
}
