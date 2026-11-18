package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
	"github.com/blackdark-org/travo-greenfield/internal/tlscert"
)

// adguardProxyAddr picks the gated proxy's listener: the explicit flag, else the daemon's
// host on port 3001 (ADR 0006). Mock mode has no AGH and runs beside dev servers: off
// unless asked for.
func adguardProxyAddr(flagValue, listen string, mock bool) string {
	switch {
	case flagValue == "off":
		return ""
	case flagValue != "":
		return flagValue
	case mock:
		return ""
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return ""
	}
	return net.JoinHostPort(host, "3001")
}

// serveAdGuardProxy runs the gated AGH proxy, plain and (with certs) TLS, until the returned
// stop is called. A listen failure is logged, not fatal: the admin UI must not depend on AGH.
func serveAdGuardProxy(addr, tlsAddr string, certs *tlscert.Source, deps api.Deps) (stop func()) {
	if addr == "" {
		return func() {}
	}
	target, _ := url.Parse(adguard.AdminURL)
	h := api.NewAdGuardProxy(deps, target)
	stopPlain := serveBackground("adguard proxy", newServer(addr, h), nil)
	stopTLS := func() {}
	if tlsAddr != "" && certs != nil {
		stopTLS = serveBackground("adguard proxy", newServer(tlsAddr, h), certs)
	}
	return func() { stopPlain(); stopTLS() }
}

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
		Ready:  adguard.HTTPReady(&http.Client{Timeout: 2 * time.Second}, adguard.AdminURL, 500*time.Millisecond),
		Bundle: adguard.Bundle{Path: adguard.BundlePath, SHA256: adguard.BundleSHA256},
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
