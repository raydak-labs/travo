package main

import (
	"context"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/catalog"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
)

// newCatalog wires the services catalog; nil (503) without the services port.
func newCatalog(deps app.Deps, connection api.ConnectivityReader) api.Catalog {
	services, ok := deps.Device.(catalog.Services)
	if !ok {
		return nil
	}
	worlds, _ := deps.Device.(catalog.Worlds)
	s := &catalog.Service{Packages: deps.Packages, Services: services, Worlds: worlds, Guards: deps.Guards,
		FreeBytes: adguard.FreeBytes("/overlay", "/"),
		Online: func(ctx context.Context) bool {
			return connection != nil && connection.Status(ctx).Internet == connectivity.Online
		}}
	if deps.Mock {
		s.FreeBytes = func() (uint64, error) { return 64 << 20, nil }
		seedMockCatalog(deps, services)
	}
	return s
}

// seedMockCatalog mirrors the 25.12 image in mock mode: sqm-scripts ships enabled, and
// every catalog init script exists so autostart works after a mock install.
func seedMockCatalog(deps app.Deps, services catalog.Services) {
	scripted, ok := services.(interface{ SetService(string, bool, bool) })
	if !ok {
		return
	}
	for _, e := range catalog.Entries {
		if e.InitScript != "" {
			scripted.SetService(e.InitScript, e.ID == "sqm", false)
		}
	}
	for _, p := range []string{"sqm-scripts", "luci-app-sqm"} {
		_ = deps.Packages.Install(context.Background(), p)
	}
}
