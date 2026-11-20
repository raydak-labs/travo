package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/uplinkview"
)

// UplinkReader is the dashboard uplink snapshot (internal/uplinkview).
type UplinkReader interface {
	Uplinks(context.Context) (uplinkview.Uplinks, error)
}

type dashboardUplinksOutput struct {
	Body uplinkview.Uplinks
}

func (s *server) registerDashboardUplinks(a huma.API) {
	register(s, false, op("getDashboardUplinks", http.MethodGet, "/api/v1/dashboard/uplinks", "Uplink state for the topology diagram", "dashboard", 503), a, s.getDashboardUplinks)
}

func (s *server) getDashboardUplinks(ctx context.Context, _ *struct{}) (*dashboardUplinksOutput, error) {
	if s.Uplinks == nil {
		return nil, networkUnavailable(nil)
	}
	body, err := s.Uplinks.Uplinks(ctx)
	if err != nil {
		slog.Debug("dashboard uplinks", "err", err)
		return nil, networkUnavailable(nil)
	}
	return &dashboardUplinksOutput{Body: body}, nil
}
