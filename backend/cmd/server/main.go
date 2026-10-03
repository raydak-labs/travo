package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/openwrt-travel-gui/backend/internal/api"
	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/config"
	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/store"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
	"github.com/openwrt-travel-gui/backend/internal/ws"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "dev"

// BuildTime is set at build time via -ldflags "-X main.BuildTime=..." (RFC3339).
// Used as the "minimum plausible clock" for the unauthenticated time-sync gate.
var BuildTime = ""

// minPlausibleFloor is the fallback when no BuildTime is stamped: any clock
// before this date is clearly broken (device booted without RTC/NTP).
const minPlausibleFloor = "2025-01-01T00:00:00Z"

// minPlausibleTime returns the later of the stamped build time and the floor.
func minPlausibleTime() time.Time {
	floor, _ := time.Parse(time.RFC3339, minPlausibleFloor)
	if bt, err := time.Parse(time.RFC3339, BuildTime); err == nil && bt.After(floor) {
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
	// shutdownTimeout bounds the graceful drain; after it elapses the process
	// stops anyway (a stuck handler must not prevent shutdown).
	shutdownTimeout = 20 * time.Second
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

// appLifecycle bundles every component with a background goroutine so callers
// can shut all of them down together (repo rule: every background goroutine
// follows lifecycle rules).
type appLifecycle struct {
	hub             *ws.Hub
	alertSvc        *services.AlertService
	uptimeTracker   *services.UptimeTracker
	bandSwitchSvc   *services.BandSwitchingService
	failoverSvc     *services.FailoverService
	blocklist       *auth.TokenBlocklist
	netWatcher      services.EventWatcher
	rateLimiter     *auth.RateLimiter
	timeSyncLimiter *auth.RateLimiter
	statsHistory    *services.StatsHistoryService
	captiveSvc      *services.CaptiveService
	db              *store.Store // may be nil (memory-only fallback)

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	// live counts goroutines registered via Go that have not returned yet.
	live atomic.Int64
}

// newAppLifecycle returns a lifecycle with the shutdown signal channel ready.
func newAppLifecycle() *appLifecycle {
	return &appLifecycle{stopCh: make(chan struct{})}
}

// Go runs fn as a tracked background goroutine. fn receives a channel that is
// closed when Stop() begins, so work that waits (startup delays, retries) can
// bail out instead of committing UCI changes after SIGTERM. Untracked
// goroutines are the reason a detached worker used to be able to flash or
// reconfigure the device after the HTTP server was already gone.
func (l *appLifecycle) Go(fn func(stop <-chan struct{})) {
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
func (l *appLifecycle) Stop() {
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
			log.Printf("WARNING: %d background goroutine(s) still running after %v; continuing shutdown", l.live.Load(), goroutineDrainTimeout)
		}

		if l.db != nil {
			_ = l.db.Close()
		}
	})
}

// setupApp creates and configures the Fiber application with all routes.
func setupApp() *fiber.App {
	cfg := config.DefaultConfig()
	if tmpDir, err := os.MkdirTemp("", "travo-auth-*"); err == nil {
		cfg.AuthConfigPath = tmpDir + "/auth.json"
	}
	app, lifecycle := setupAppWithConfig(cfg)
	// Stop the background goroutines immediately — setupApp is only used in tests.
	lifecycle.Stop()
	return app
}

// setupAppWithConfig creates and configures the Fiber application with the given config.
// The returned lifecycle owns every background goroutine started here.
func setupAppWithConfig(cfg config.Config) (*fiber.App, *appLifecycle) {
	lifecycle := newAppLifecycle()

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
	log.Printf("Panic recovery enabled: a handler panic returns 500 instead of killing the process")

	// CORS middleware
	app.Use(cors.New(corsConfig(cfg.CorsOrigins)))

	nets, err := auth.ParseCIDRList(cfg.AllowedAdminCIDRs)
	if err != nil {
		log.Fatalf("invalid ALLOWED_ADMIN_CIDRS: %v", err)
	}
	if len(nets) > 0 {
		app.Use(auth.IPAllowlistMiddleware(nets))
		log.Printf("Admin IP allowlist enabled (%d CIDR(s))", len(nets))
	}

	// Create UCI and Ubus backends
	var u uci.UCI
	var ub ubus.Ubus
	if cfg.MockMode {
		u = uci.NewMockUCI()
		ub = ubus.NewMockUbus()
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
		log.Printf("WARNING: persistent store unavailable (%v); running memory-only", err)
	}

	// Create shared root password holder (written by auth after login, read by UCI apply).
	rootPassword := auth.NewRootPassword()

	// Create services
	authStore := auth.NewFileAuthStore(cfg.AuthConfigPath)
	authCfg, err := authStore.LoadOrInit()
	if err != nil {
		log.Fatalf("failed to load auth config: %v", err)
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
				log.Printf("WARNING: could not refresh rpcd-login helper file: %v", err)
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
	var storage services.StorageProvider
	var captiveProber services.HTTPProber
	if cfg.MockMode {
		storage = &services.MockStorageProvider{}
		captiveProber = &services.MockHTTPProber{StatusCode: 200, Body: "success\n"}
	} else {
		storage = &services.RealStorageProvider{}
		captiveProber = services.NewRealHTTPProber()
	}

	systemSvc := services.NewSystemService(ub, u, storage)
	if !cfg.MockMode {
		// Surface crash guards left behind by an interrupted run (flash,
		// restore, factory reset) so the state is not silently forgotten.
		systemSvc.LogStaleCrashGuards()
	}
	networkSvc := services.NewNetworkService(u, ub)
	sqmSvc := services.NewSQMService(u)

	// Create event watcher (noop in mock mode to avoid running ubus on dev machines).
	var netWatcher services.EventWatcher
	if cfg.MockMode {
		netWatcher = services.NewNoopEventWatcher()
	} else {
		netWatcher = services.NewNetworkEventWatcher(networkSvc)
	}
	lifecycle.netWatcher = netWatcher
	lifecycle.Go(func(stop <-chan struct{}) { netWatcher.Start() })
	var wifiSvc *services.WifiService
	if cfg.MockMode {
		wifiSvc = services.NewWifiServiceWithReloader(u, ub, &services.NoopWifiReloader{})
	} else {
		wifiSvc = services.NewWifiService(u, ub, rootPassword) // uses apply+confirm instead of wifi up
	}

	// Fix wireless UCI on startup (country/channel, missing SSID/key on existing APs,
	// enable radios when AP enabled). Do not auto-apply on startup: there is no
	// browser in the loop to confirm rpcd rollback safely, so we commit the repair
	// and require LuCI Save & Apply or reboot for runtime activation.
	if !cfg.MockMode {
		lifecycle.Go(func(stop <-chan struct{}) {
			if !sleepOrStop(stop, 30*time.Second) {
				return
			}
			fixed, needApply, err := wifiSvc.EnsureAPRunning()
			if err != nil {
				log.Printf("WARNING: WiFi AP health check failed: %v", err)
				return
			}
			if fixed && needApply {
				log.Printf("WiFi AP health: UCI fixes committed. Runtime apply skipped on startup to preserve LuCI-style rollback safety; use LuCI Save & Apply or reboot.")
			} else if fixed {
				log.Printf("WiFi AP health: UCI fixes committed (SSID/key only, no apply needed).")
			}
		})
		// Ensure auto-reconnect script is present and up-to-date when enabled.
		// Uses SetAutoReconnect to recreate a missing script (e.g. after a crash or
		// accidental deletion) and to upgrade any old "wifi reload" script to the safe
		// "wifi up" version. Safe to call idempotently: it rewrites the cron entry and
		// script atomically, which is the same state SetAutoReconnect(true) produces.
		lifecycle.Go(func(stop <-chan struct{}) {
			if !sleepOrStop(stop, 5*time.Second) {
				return
			}
			// Repair in BOTH directions, not just when the feature is on.
			// A device carrying a pre-rename cron entry or script keeps running
			// `wifi reload` / `wifi up` on a timer even though auto-reconnect is
			// disabled, and `wifi reload` is the documented ath11k crash trigger.
			// SetAutoReconnect rewrites the script and the cron entry to match the
			// configured state, so calling it unconditionally converges the device
			// onto the safe script in both cases.
			enabled, _ := wifiSvc.GetAutoReconnect()
			if err := wifiSvc.SetAutoReconnect(enabled); err != nil {
				log.Printf("WARNING: could not reconcile auto-reconnect script: %v", err)
			}
		})
		// Same for the WiFi on/off schedule: an existing /etc/cron.d entry still
		// holds the pre-fix `/sbin/wifi up` / `/sbin/wifi down` lines until the
		// user re-saves the schedule. Re-writing it from the stored config points
		// the cron entries at the generated UCI toggle helper instead.
		lifecycle.Go(func(stop <-chan struct{}) {
			if !sleepOrStop(stop, 6*time.Second) {
				return
			}
			sched, err := wifiSvc.GetWiFiSchedule()
			if err != nil || !sched.Enabled || sched.OnTime == "" || sched.OffTime == "" {
				return
			}
			if err := wifiSvc.SetWiFiSchedule(sched); err != nil {
				log.Printf("WARNING: could not reconcile WiFi schedule cron entry: %v", err)
			}
		})
		// Auto-discover radio hardware and persist config on first boot.
		lifecycle.Go(func(stop <-chan struct{}) {
			if !sleepOrStop(stop, 10*time.Second) {
				return
			}
			if discovered, err := wifiSvc.DiscoverAndPersistRadios(); err != nil {
				log.Printf("WARNING: radio discovery failed: %v", err)
			} else if discovered {
				log.Printf("Radio auto-discovery completed on first boot.")
			}
		})
	}

	vpnSvc := services.NewVpnService(u)
	svcManager := services.NewServiceManager()
	captiveSvc := services.NewCaptiveServiceWithUCI(captiveProber, u, &services.RealCommandRunner{})
	adguardSvc := services.NewAdGuardService()
	dataUsageSvc := services.NewDataUsageService()
	usbTetherSvc := services.NewUSBTetheringService()
	bandSwitchSvc := services.NewBandSwitchingService(wifiSvc, "/etc/travo/band-switching.json")
	failoverSvc := services.NewFailoverService(u, ub, networkSvc, rootPassword)

	// Register post-install hook: auto-configure AdGuard Home after package install.
	if !cfg.MockMode {
		svcManager.SetPostInstallHook("adguardhome", adguardSvc.AutoConfigure)
		svcManager.SetPostInstallHook("vnstat", dataUsageSvc.AutoConfigureVnstat)
	}
	alertSvc := services.NewAlertService(systemSvc)
	if !cfg.MockMode {
		alertSvc.SetCarrierChecker(&services.RealCarrierChecker{})
	}
	failoverSvc.SetAlertService(alertSvc)
	uptimeTracker := services.NewUptimeTracker(captiveProber)

	// Stats history: collect every 30s, keep 720 points (~6 hours)
	var statsHistory *services.StatsHistoryService
	if db != nil {
		statsHistory = services.NewStatsHistoryServiceWithStore(systemSvc, 30*time.Second, 720, db)
	} else {
		statsHistory = services.NewStatsHistoryService(systemSvc, 30*time.Second, 720)
	}
	statsHistory.Start()

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

	// Health check endpoint. Registered on the app rather than the
	// authenticated /api/v1 group, so it stays reachable without a token.
	app.Get("/api/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// API routes
	deps := &api.Dependencies{
		Auth:           authSvc,
		AuthStore:      authStore,
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
		Speedtest:      services.NewSpeedtestService(),

		TimeSyncMinPlausible: minPlausibleTime(),
		TimeSyncLimiter:      timeSyncLimiter,
		TimeSyncGate:         api.NewTimeSyncGate(!time.Now().Before(minPlausibleTime())),
	}
	api.SetupRoutes(app, deps)

	// WebSocket (with auth from query parameter)
	hub := ws.NewHub(systemSvc, alertSvc, netWatcher.Ch())
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
	return app, lifecycle
}

func main() {
	log.SetOutput(os.Stdout)
	cfg, showVersion, err := config.LoadConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	app, lifecycle := setupAppWithConfig(cfg)

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		// Order matters: stop accepting requests and let in-flight handlers
		// finish BEFORE the services (and the store) are torn down. Doing it the
		// other way round made every in-flight DB read fail against a closed
		// bbolt handle, with the error swallowed inside the store.
		if err := app.ShutdownWithTimeout(shutdownTimeout); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		lifecycle.Stop()
		log.Println("Server stopped")
	}()

	// If TLS is enabled, start HTTPS listener concurrently.
	if cfg.TLSEnabled {
		if err := config.EnsureTLSCert(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil {
			log.Printf("WARNING: could not generate TLS certificate: %v", err)
		} else {
			tlsAddr := fmt.Sprintf(":%d", cfg.TLSPort)
			log.Printf("Starting HTTPS listener on %s", tlsAddr)
			go func() {
				if err := app.Listen(tlsAddr, fiber.ListenConfig{
					CertFile:    cfg.TLSCertFile,
					CertKeyFile: cfg.TLSKeyFile,
				}); err != nil {
					log.Printf("HTTPS listener stopped: %v", err)
				}
			}()
		}
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting travo backend on %s (mock=%v, tls=%v)", addr, cfg.MockMode, cfg.TLSEnabled)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
