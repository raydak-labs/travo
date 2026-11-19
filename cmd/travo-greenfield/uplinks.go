package main

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/ddns"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/failover"
	"github.com/blackdark-org/travo-greenfield/internal/ipv6settings"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
	"github.com/blackdark-org/travo-greenfield/internal/usbtether"
	"github.com/blackdark-org/travo-greenfield/internal/wansettings"
)

// newUSBTether wires the USB tethering uplink; nil when the device cannot list netdevs.
func newUSBTether(deps app.Deps) api.USBTether {
	links, ok := deps.Device.(device.NetLinkReader)
	if !ok {
		return nil
	}
	s := &usbtether.Service{UCI: deps.Device, Ubus: deps.Device, Links: links, Packages: deps.Packages, Guards: deps.Guards}
	if services, ok := deps.Device.(device.Services); ok {
		s.Services = services
	}
	s.FreeBytes = adguard.FreeBytes("/overlay", "/")
	if deps.Mock {
		s.FreeBytes = func() (uint64, error) { return 64 << 20, nil }
	}
	if deps.Txn != nil {
		s.Txns = deps.Txn
	}
	return s
}

// newWAN wires the wan settings; nil (503) without the password writer or route reader.
func newWAN(deps app.Deps) api.WANSettings {
	keys, hasKeys := deps.Device.(device.WANPasswordWriter)
	routes, hasRoutes := deps.Device.(device.DefaultRouteReader)
	if !hasKeys || !hasRoutes || deps.Txn == nil {
		return nil
	}
	return &wansettings.Service{UCI: deps.Device, Keys: keys, Ubus: deps.Device, Routes: routes, Txns: deps.Txn}
}

// newIPv6 wires the IPv6 switch; nil (503) without the route reader or transactions.
func newIPv6(deps app.Deps) api.IPv6Settings {
	routes, ok := deps.Device.(device.DefaultRouteReader)
	if !ok || deps.Txn == nil {
		return nil
	}
	return &ipv6settings.Service{UCI: deps.Device, Routes: routes, Txns: deps.Txn}
}

// newDDNS wires dynamic DNS; nil (503) without the token writer, services or transactions.
func newDDNS(deps app.Deps, connection api.ConnectivityReader) api.DDNS {
	keys, hasKeys := deps.Device.(device.DDNSPasswordWriter)
	services, hasServices := deps.Device.(device.Services)
	if !hasKeys || !hasServices || deps.Txn == nil {
		return nil
	}
	s := &ddns.Service{UCI: deps.Device, Keys: keys, Packages: deps.Packages, Services: services,
		Guards: deps.Guards, Txns: deps.Txn, FreeBytes: adguard.FreeBytes("/overlay", "/"),
		Online: func(ctx context.Context) bool {
			return connection != nil && connection.Status(ctx).Internet == connectivity.Online
		}}
	if deps.Mock {
		s.FreeBytes = func() (uint64, error) { return 64 << 20, nil }
	}
	return s
}

// startFailover wires multi-WAN failover and its mwan3 reconciler; nil without the ports
// it needs (reorder, services, the shared connectivity probe).
func startFailover(deps app.Deps, connection api.ConnectivityReader, workers *lifecycle.Group) api.Failover {
	reorder, hasReorder := deps.Device.(device.UCIReorderer)
	services, hasServices := deps.Device.(device.Services)
	runtime, hasRuntime := deps.Device.(device.MWAN3Reader)
	if !hasReorder || !hasServices || !hasRuntime || connection == nil || deps.Txn == nil {
		return nil
	}
	svc := &failover.Service{
		UCI: deps.Device, Reorder: reorder, Ubus: deps.Device, Services: services, Runtime: runtime,
		Packages: deps.Packages, Guards: deps.Guards, Txns: deps.Txn,
		Online: func(ctx context.Context) bool {
			return connection.Status(ctx).Internet == connectivity.Online
		},
		Path:    filepath.Join(deps.Paths.State, "failover.json"),
		Ordered: !deps.Mock, // the cli adapter keeps `uci export` order; the mock sorts by name
	}
	workers.Go("failover-mwan3", lifecycle.Every(failover.Interval, func(ctx context.Context) {
		if err := svc.Reconcile(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("failover reconcile", "err", err)
		}
	}))
	return svc
}
