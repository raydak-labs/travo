package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

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

// setupApp creates and configures the Fiber application with all routes.
func setupApp() *fiber.App {
	cfg := config.DefaultConfig()
	app, _, _ := setupAppWithConfig(cfg)
	return app
}

// setupAppWithConfig creates and configures the Fiber application with the given config.
// Returns the app, the WebSocket hub, and the alert service so the caller can manage their lifecycle.
func setupAppWithConfig(cfg config.Config) (*fiber.App, *ws.Hub, *services.AlertService) {
	app := fiber.New(fiber.Config{
		AppName: "openwrt-travel-gui",
	})

	// CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CorsOrigins,
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Authorization,Content-Type",
	}))

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

	// Create services
	authSvc := auth.NewAuthService(cfg.Password, cfg.JWTSecret)
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
	wifiSvc := services.NewWifiService(u, ub)

	// Ensure WiFi AP is up on every startup/reboot.
	// Runs in a background goroutine with a startup delay so the HTTP server is
	// immediately available and so the WiFi subsystem (netifd/hostapd) has time
	// to fully initialize before we call "wifi reload". Running it synchronously
	// at boot causes a race: the backend starts only ~2s after wifi-scripts and
	// an immediate reload interferes with ongoing AP/STA initialization.
	// Skipped in mock mode — mock UCI state is already correctly configured.
	if !cfg.MockMode {
		go func() {
			time.Sleep(30 * time.Second)
			if fixed, err := wifiSvc.EnsureAPRunning(); err != nil {
				log.Printf("WARNING: WiFi AP health check failed: %v", err)
			} else if fixed {
				log.Printf("WiFi AP health check: reset misconfigured APs to defaults (SSID: %q, password: %q)",
					services.DefaultAPSSID, services.DefaultAPKey)
			}
		}()
	}

	vpnSvc := services.NewVpnService(u)
	svcManager := services.NewServiceManager()
	captiveSvc := services.NewCaptiveService(captiveProber)
	adguardSvc := services.NewAdGuardService()
	alertSvc := services.NewAlertService(systemSvc)

	// Token blocklist with cleanup goroutine
	blocklist := auth.NewTokenBlocklist()
	authSvc.SetBlocklist(blocklist)
	stopCleanup := make(chan struct{})
	blocklist.StartCleanup(5*time.Minute, stopCleanup)

	// Rate limiter: 5 attempts per minute
	rateLimiter := auth.NewRateLimiter(5, time.Minute)

	// Auth middleware
	app.Use(authSvc.Middleware())

	// Health check endpoint (excluded from auth in middleware)
	app.Get("/api/health", func(c *fiber.Ctx) error {
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
		Wifi:           wifiSvc,
		Vpn:            vpnSvc,
		ServiceManager: svcManager,
		Captive:        captiveSvc,
		AdGuard:        adguardSvc,
		Alerts:         alertSvc,
	}
	api.SetupRoutes(app, deps)

	// WebSocket (with auth from query parameter)
	hub := ws.NewHub(systemSvc, alertSvc)
	app.Use("/api/v1/ws", ws.UpgradeMiddleware(authSvc))
	app.Get("/api/v1/ws", ws.Handler(hub, authSvc))
	hub.Start()
	alertSvc.Start()

	// Static files (if configured)
	if cfg.StaticDir != "" {
		app.Static("/", cfg.StaticDir)
		// SPA catch-all: serve index.html for non-API routes that don't match static files
		app.Get("/*", func(c *fiber.Ctx) error {
			return c.SendFile(cfg.StaticDir + "/index.html")
		})
	}

	return app, hub, alertSvc
}

func main() {
	cfg, showVersion, err := config.LoadConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	app, hub, alertSvc := setupAppWithConfig(cfg)

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		hub.Stop()
		alertSvc.Stop()
		if err := app.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		log.Println("Server stopped")
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting openwrt-travel-gui backend on %s (mock=%v)", addr, cfg.MockMode)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
