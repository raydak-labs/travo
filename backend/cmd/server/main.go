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

// setupApp creates and configures the Fiber application with all routes.
func setupApp() *fiber.App {
	cfg := config.DefaultConfig()
	if tmpDir, err := os.MkdirTemp("", "travo-auth-*"); err == nil {
		cfg.AuthConfigPath = tmpDir + "/auth.json"
	}
	app, _, _, _, _, _, netWatcher := setupAppWithConfig(cfg)
	// Stop the watcher goroutine immediately — setupApp is only used in tests.
	netWatcher.Stop()
	return app
}

// setupAppWithConfig creates and configures the Fiber application with the given config.
// Returns the app, WebSocket hub, alert service, uptime tracker, band switching service, blocklist, and event watcher so the caller can manage their lifecycle.
func setupAppWithConfig(cfg config.Config) (*fiber.App, *ws.Hub, *services.AlertService, *services.UptimeTracker, *services.BandSwitchingService, *auth.TokenBlocklist, services.EventWatcher) {
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
	var authSvc *auth.AuthService
	if cfg.MockMode {
		authSvc = auth.NewAuthService("admin", authCfg.JWTSecret)
	} else {
		authSvc = auth.NewAuthServiceWithUbus(ub, authCfg.JWTSecret, rootPassword)
	}
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
		// Replace any old auto-reconnect script that still had "wifi reload" with
		// the safe "wifi up" version, so cron does not crash the device every minute.
		go func() {
			time.Sleep(5 * time.Second)
			if enabled, _ := wifiSvc.GetAutoReconnect(); enabled {
				wifiSvc.WriteReconnectScriptSafe()
			}
		}()
	}

	vpnSvc := services.NewVpnService(u)
	svcManager := services.NewServiceManager()
	captiveSvc := services.NewCaptiveService(captiveProber)
	adguardSvc := services.NewAdGuardService()
	dataUsageSvc := services.NewDataUsageService()
	usbTetherSvc := services.NewUSBTetheringService()
	bandSwitchSvc := services.NewBandSwitchingService(wifiSvc, "/etc/travo/band-switching.json")

	// Register post-install hook: auto-configure AdGuard Home after package install.
	if !cfg.MockMode {
		svcManager.SetPostInstallHook("adguardhome", adguardSvc.AutoConfigure)
		svcManager.SetPostInstallHook("vnstat", dataUsageSvc.AutoConfigureVnstat)
	}
	alertSvc := services.NewAlertService(systemSvc)
	if !cfg.MockMode {
		alertSvc.SetCarrierChecker(&services.RealCarrierChecker{})
	}
	uptimeTracker := services.NewUptimeTracker(captiveProber)

	// Token blocklist with cleanup goroutine
	blocklist := auth.NewTokenBlocklist()
	authSvc.SetBlocklist(blocklist)
	blocklist.StartCleanup(5 * time.Minute)

	// Rate limiter: 5 attempts per minute
	rateLimiter := auth.NewRateLimiter(5, time.Minute)

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

	// Static files (if configured)
	if cfg.StaticDir != "" {
		app.Use("/", static.New(cfg.StaticDir))
		// SPA catch-all: serve index.html for non-API routes that don't match static files
		app.Get("/*", func(c fiber.Ctx) error {
			return c.SendFile(cfg.StaticDir + "/index.html")
		})
	}

	return app, hub, alertSvc, uptimeTracker, bandSwitchSvc, blocklist, netWatcher
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

	app, hub, alertSvc, uptimeTracker, bandSwitchSvc, blocklist, netWatcher := setupAppWithConfig(cfg)

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		blocklist.Stop()
		hub.Stop()
		netWatcher.Stop()
		alertSvc.Stop()
		uptimeTracker.Stop()
		bandSwitchSvc.Stop()
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
