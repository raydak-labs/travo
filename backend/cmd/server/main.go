// Command server runs the Travo backend.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/app"
	"github.com/openwrt-travel-gui/backend/internal/config"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "dev"

// BuildTime is set at build time via -ldflags "-X main.BuildTime=..." (RFC3339).
// Used as the "minimum plausible clock" for the unauthenticated time-sync gate.
var BuildTime = ""

// fatal logs err and exits; startup cannot continue.
func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

func main() {
	// procd forwards stdout to logread, which adds its own timestamp.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})))
	cfg, showVersion, err := config.LoadConfig(os.Args[1:])
	if err != nil {
		fatal("failed to load config", err)
	}

	if showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	srv, lifecycle, err := app.New(cfg, BuildTime)
	if err != nil {
		fatal("startup", err)
	}

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		slog.Info("shutting down")
		// Order matters: stop accepting requests and let in-flight handlers
		// finish BEFORE the services (and the store) are torn down. Doing it the
		// other way round made every in-flight DB read fail against a closed
		// bbolt handle, with the error swallowed inside the store.
		if err := srv.ShutdownWithTimeout(app.ShutdownTimeout); err != nil {
			slog.Error("shutdown", "err", err)
		}
		lifecycle.Stop()
		slog.Info("server stopped")
	}()

	// If TLS is enabled, start HTTPS listener concurrently.
	if cfg.TLSEnabled {
		if err := config.EnsureTLSCert(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil {
			slog.Warn("could not generate TLS certificate", "err", err)
		} else {
			tlsAddr := fmt.Sprintf(":%d", cfg.TLSPort)
			slog.Info("starting HTTPS listener", "addr", tlsAddr)
			go func() {
				if err := srv.Listen(tlsAddr, fiber.ListenConfig{
					CertFile:    cfg.TLSCertFile,
					CertKeyFile: cfg.TLSKeyFile,
				}); err != nil {
					slog.Warn("HTTPS listener stopped", "err", err)
				}
			}()
		}
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("starting travo backend", "addr", addr, "mock", cfg.MockMode, "tls", cfg.TLSEnabled)
	if err := srv.Listen(addr); err != nil {
		fatal("failed to start server", err)
	}
}
