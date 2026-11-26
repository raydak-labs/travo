package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/catalog"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// Catalog is the services catalog (internal/catalog).
type Catalog interface {
	List(context.Context) (catalog.View, error)
	Install(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	SetAutostart(ctx context.Context, id string, on bool) error
}

type catalogOutput struct{ Body catalog.View }

type catalogIDInput struct {
	ID string `path:"id" pattern:"^[a-z0-9-]{1,32}$" doc:"Catalog entry id"`
}

type catalogAutostartInput struct {
	ID   string `path:"id" pattern:"^[a-z0-9-]{1,32}$" doc:"Catalog entry id"`
	Body struct {
		Enabled bool `json:"enabled" doc:"Enable and start, or stop and disable, the init script"`
	}
}

func (s *server) registerCatalog(a huma.API) {
	register(s, false, op("getServicesCatalog", http.MethodGet, "/api/v1/services/catalog",
		"Optional services: installed, autostart, running", "services", 503), a, s.getCatalog)
	inst := op("installCatalogService", http.MethodPost, "/api/v1/services/catalog/{id}/install",
		"Install a catalog service and its LuCI app (guarded, needs Internet and free flash)", "services", 404, 409, 503)
	inst.MaxBodyBytes = 1024
	register(s, false, inst, a, s.installCatalog)
	rm := op("removeCatalogService", http.MethodPost, "/api/v1/services/catalog/{id}/remove",
		"Remove a catalog service and its LuCI app (guarded; firmware packages refused)", "services", 404, 409, 503)
	rm.MaxBodyBytes = 1024
	register(s, false, rm, a, s.removeCatalog)
	auto := op("setCatalogAutostart", http.MethodPut, "/api/v1/services/catalog/{id}/autostart",
		"Turn a catalog service on (enable, start) or off (stop, disable)", "services", 400, 404, 409, 503)
	auto.MaxBodyBytes = 1024
	register(s, false, auto, a, s.setCatalogAutostart)
}

func (s *server) getCatalog(ctx context.Context, _ *struct{}) (*catalogOutput, error) {
	if s.Catalog == nil {
		return nil, catalogError(catalog.ErrUnavailable)
	}
	v, err := s.Catalog.List(ctx)
	if err != nil {
		return nil, catalogError(err)
	}
	return &catalogOutput{Body: v}, nil
}

func (s *server) installCatalog(ctx context.Context, in *catalogIDInput) (*catalogOutput, error) {
	return s.changeCatalog(ctx, func() error { return s.Catalog.Install(ctx, in.ID) })
}

func (s *server) removeCatalog(ctx context.Context, in *catalogIDInput) (*catalogOutput, error) {
	return s.changeCatalog(ctx, func() error { return s.Catalog.Remove(ctx, in.ID) })
}

func (s *server) setCatalogAutostart(ctx context.Context, in *catalogAutostartInput) (*catalogOutput, error) {
	return s.changeCatalog(ctx, func() error { return s.Catalog.SetAutostart(ctx, in.ID, in.Body.Enabled) })
}

func (s *server) changeCatalog(ctx context.Context, change func() error) (*catalogOutput, error) {
	if s.Catalog == nil {
		return nil, catalogError(catalog.ErrUnavailable)
	}
	if err := change(); err != nil {
		return nil, catalogError(err)
	}
	return s.getCatalog(ctx, nil)
}

func catalogError(err error) error {
	conflicts := []struct {
		err  error
		code string
	}{
		{catalog.ErrBusy, "catalog_busy"}, {guard.ErrActive, "catalog_busy"},
		{catalog.ErrOffline, "catalog_offline"}, {catalog.ErrNoSpace, "catalog_no_space"},
		{catalog.ErrBuiltIn, "catalog_built_in"}, {catalog.ErrInUse, "catalog_in_use"},
		{catalog.ErrNotInstalled, "catalog_not_installed"}, {catalog.ErrNoAutostart, "catalog_no_autostart"},
	}
	if errors.Is(err, catalog.ErrUnknown) {
		return fail(http.StatusNotFound, "not_found", err.Error())
	}
	for _, c := range conflicts {
		if errors.Is(err, c.err) {
			return fail(http.StatusConflict, c.code, err.Error())
		}
	}
	slog.Warn("services catalog", "err", err)
	return fail(http.StatusServiceUnavailable, "catalog_unavailable", catalog.ErrUnavailable.Error())
}
