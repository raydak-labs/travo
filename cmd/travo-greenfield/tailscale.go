package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
	"github.com/blackdark-org/travo-greenfield/internal/tailscale"
)

// startTailscale builds the Tailscale supervisor and the reconciler that owns the
// `tailscale` DNS layer (docs/adr/0017). It returns nil when the device cannot drive procd services.
func startTailscale(deps app.Deps, layers *dnslayer.Service, workers *lifecycle.Group, connection api.ConnectivityReader, tunnel api.VPNStatusReader) *tailscale.Service {
	services, ok := deps.Device.(device.Services)
	if !ok || deps.Tailscale == nil {
		return nil
	}
	svc := &tailscale.Service{
		CLI: deps.Tailscale, Packages: deps.Packages, Services: services, Guards: deps.Guards,
		UCI:       deps.Device,
		FreeBytes: adguard.FreeBytes("/overlay", "/"),
		Online: func(ctx context.Context) bool {
			return connection != nil && connection.Status(ctx).Internet == connectivity.Online
		},
	}
	if deps.Txn != nil {
		svc.Txns = deps.Txn
	}
	if tunnel != nil {
		svc.Tunnel = tunnel
	}
	if deps.Mock {
		svc.FreeBytes = func() (uint64, error) { return 64 << 20, nil }
	}
	dns := &tailscale.DNS{Service: svc, Layers: layers}
	workers.Go("tailscale-dns-layer", lifecycle.Every(15*time.Second, func(ctx context.Context) {
		if _, err := dns.Tick(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("tailscale dns layer", "err", err)
		}
	}))
	return svc
}
