package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/openwrt-travel-gui/backend/internal/api"
	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/config"
	"github.com/openwrt-travel-gui/backend/internal/services"
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

func splitCORSOrigins(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{"*"}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
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
}

// Stop shuts down all background goroutines.
func (l *appLifecycle) Stop() {
	l.blocklist.Stop()
	l.hub.Stop()
	l.netWatcher.Stop()
	l.alertSvc.Stop()
	l.uptimeTracker.Stop()
	l.bandSwitchSvc.Stop()
	l.failoverSvc.Stop()
	l.rateLimiter.Stop()
	l.timeSyncLimiter.Stop()
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
	app := fiber.New(fiber.Config{AppName: "travo"})

	// CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: splitCORSOrigins(cfg.CorsOrigins),
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

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
	networkSvc := services.NewNetworkService(u, ub)
	sqmSvc := services.NewSQMService(u)

	// Create event watcher (noop in mock mode to avoid running ubus on dev machines).
	var netWatcher services.EventWatcher
	if cfg.MockMode {
		netWatcher = services.NewNoopEventWatcher()
	} else {
		netWatcher = services.NewNetworkEventWatcher(networkSvc)
	}
	go netWatcher.Start()
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
		go func() {
			time.Sleep(30 * time.Second)
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
		}()
		// Ensure auto-reconnect script is present and up-to-date when enabled.
		// Uses SetAutoReconnect to recreate a missing script (e.g. after a crash or
		// accidental deletion) and to upgrade any old "wifi reload" script to the safe
		// "wifi up" version. Safe to call idempotently: it rewrites the cron entry and
		// script atomically, which is the same state SetAutoReconnect(true) produces.
		go func() {
			time.Sleep(5 * time.Second)
			if enabled, _ := wifiSvc.GetAutoReconnect(); enabled {
				if err := wifiSvc.SetAutoReconnect(true); err != nil {
					log.Printf("WARNING: could not refresh auto-reconnect script: %v", err)
				}
			}
		}()
		// Auto-discover radio hardware and persist config on first boot.
		go func() {
			time.Sleep(10 * time.Second)
			if discovered, err := wifiSvc.DiscoverAndPersistRadios(); err != nil {
				log.Printf("WARNING: radio discovery failed: %v", err)
			} else if discovered {
				log.Printf("Radio auto-discovery completed on first boot.")
			}
		}()
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
	statsHistory := services.NewStatsHistoryService(systemSvc, 30*time.Second, 720)
	statsHistory.Start()

	// Token blocklist with cleanup goroutine
	blocklist := auth.NewTokenBlocklist()
	authSvc.SetBlocklist(blocklist)
	blocklist.StartCleanup(5 * time.Minute)

	// Rate limiters: login (5/min) and unauthenticated time-sync (3/min).
	// Periodic sweeps keep the per-IP maps bounded when many distinct source
	// IPs never return (see RateLimiter.Cleanup).
	rateLimiter := auth.NewRateLimiter(5, time.Minute)
	rateLimiter.StartCleanup(5 * time.Minute)
	timeSyncLimiter := auth.NewRateLimiter(3, time.Minute)
	timeSyncLimiter.StartCleanup(5 * time.Minute)

	// Auth middleware
	app.Use(authSvc.Middleware())

	// Health check endpoint (excluded from auth in middleware)
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

		TimeSyncMinPlausible: minPlausibleTime(),
		TimeSyncLimiter:      timeSyncLimiter,
	}
	api.SetupRoutes(app, deps)

	// WebSocket (with auth from query parameter)
	hub := ws.NewHub(systemSvc, alertSvc, netWatcher.Ch())
	app.Use("/api/v1/ws", ws.UpgradeMiddleware(authSvc))
	app.Get("/api/v1/ws", ws.Handler(hub, authSvc))
	hub.Start()
	alertSvc.Start()
	uptimeTracker.Start()
	bandSwitchSvc.Start()
	go failoverSvc.Start()

	// Static files (if configured)
	if cfg.StaticDir != "" {
		app.Use("/", static.New(cfg.StaticDir))
		// SPA catch-all: serve index.html for non-API routes that don't match static files
		app.Get("/*", func(c fiber.Ctx) error {
			return c.SendFile(cfg.StaticDir + "/index.html")
		})
	}

	return app, &appLifecycle{
		hub:             hub,
		alertSvc:        alertSvc,
		uptimeTracker:   uptimeTracker,
		bandSwitchSvc:   bandSwitchSvc,
		failoverSvc:     failoverSvc,
		blocklist:       blocklist,
		netWatcher:      netWatcher,
		rateLimiter:     rateLimiter,
		timeSyncLimiter: timeSyncLimiter,
	}
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
		lifecycle.Stop()
		if err := app.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
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
