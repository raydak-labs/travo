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
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
)

// startAdGuard builds the AdGuard Home supervisor, its reconcile worker and the liveness
// worker that owns the `adguard` DNS layer (ADR 0006, invariants §8).
// It returns nil when the device cannot drive procd services.
func startAdGuard(ctx context.Context, deps app.Deps, layers *dnslayer.Service, workers *lifecycle.Group, connection api.ConnectivityReader) *adguard.Service {
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
	svc.DNS = &adguard.DNS{
		Layers: layers, StatePath: filepath.Join(deps.Paths.State, "adguard-dns.json"),
		Probe: adguard.UDPProbe(adguard.ProbeAddr, 5*time.Second),
		HasRoute: func(ctx context.Context) (bool, error) {
			routes, ok := deps.Device.(device.DefaultRouteReader)
			if !ok {
				return false, nil
			}
			_, found, err := routes.DefaultIPv4Route(ctx)
			return found, err
		},
		Now: time.Now,
	}
	if deps.Mock { // no AGH listens in mock mode: answer like a healthy one
		svc.DNS.Probe = func(ctx context.Context) error {
			if running, err := services.ServiceRunning(ctx, adguard.Package); err != nil || !running {
				return errors.Join(adguard.ErrProbe, err)
			}
			return nil
		}
	}
	if err := svc.Recover(ctx); err != nil {
		slog.Warn("adguard: interrupted config change not recovered", "err", err)
	}
	workers.Go("adguard-reconcile", lifecycle.Every(time.Minute, func(ctx context.Context) {
		if err := svc.Reconcile(ctx); err != nil && !errors.Is(err, adguard.ErrOffline) && ctx.Err() == nil {
			slog.Warn("adguard", "err", err)
		}
	}))
	workers.Go("adguard-liveness", lifecycle.Every(15*time.Second, func(ctx context.Context) {
		if err := svc.DNS.Tick(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("adguard liveness", "err", err)
		}
	}))
	return svc
}
