package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
)

// startAdGuard builds the AdGuard Home supervisor and its reconcile worker (ADR 0006).
// It returns nil when the device cannot drive procd services.
func startAdGuard(ctx context.Context, deps app.Deps, workers *lifecycle.Group, connection api.ConnectivityReader) *adguard.Service {
	services, ok := deps.Device.(device.Services)
	if !ok {
		return nil
	}
	svc := &adguard.Service{
		Packages: deps.Packages, Services: services, Guards: deps.Guards,
		Config:    adguard.ConfigPath,
		Backup:    filepath.Join(deps.Paths.State, "adguardhome.yaml.old"),
		Marker:    filepath.Join(deps.Paths.State, "adguard-install-attempted"),
		FreeBytes: adguard.FreeBytes("/overlay", "/"),
		Online: func(ctx context.Context) bool {
			return connection != nil && connection.Status(ctx).Internet == connectivity.Online
		},
		Ready: adguard.HTTPReady(&http.Client{Timeout: 2 * time.Second}, adguard.AdminURL, 500*time.Millisecond),
	}
	if deps.Mock { // the mock seeds the service; keep the config off the real path
		svc.Config = filepath.Join(deps.Paths.State, "adguardhome", "adguardhome.yaml")
		svc.FreeBytes = func() (uint64, error) { return 64 << 20, nil }
		svc.Ready = func(ctx context.Context) error { return nil }
	}
	if err := svc.Recover(ctx); err != nil {
		slog.Warn("adguard: interrupted config change not recovered", "err", err)
	}
	workers.Go("adguard-reconcile", lifecycle.Every(time.Minute, func(ctx context.Context) {
		if err := svc.Reconcile(ctx); err != nil && !errors.Is(err, adguard.ErrOffline) && ctx.Err() == nil {
			slog.Warn("adguard", "err", err)
		}
	}))
	return svc
}
