package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/alerts"
)

// Alerts derives the operator alerts on read (internal/alerts).
type Alerts interface {
	List(context.Context) (alerts.AlertList, error)
}

type systemAlertsOutput struct{ Body alerts.AlertList }

func (s *server) registerSystemAlerts(a huma.API) {
	register(s, false, op("getSystemAlerts", http.MethodGet, "/api/v1/system/alerts", "Active operator alerts", "system", 503), a, s.getSystemAlerts)
}

func (s *server) getSystemAlerts(ctx context.Context, _ *struct{}) (*systemAlertsOutput, error) {
	if s.Alerts == nil {
		return nil, fail(http.StatusServiceUnavailable, "alerts_unavailable", "alerts are unavailable")
	}
	l, err := s.Alerts.List(ctx)
	if err != nil {
		return nil, fail(http.StatusServiceUnavailable, "alerts_unavailable", "alerts are unavailable")
	}
	return &systemAlertsOutput{Body: l}, nil
}
