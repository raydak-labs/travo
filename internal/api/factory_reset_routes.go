package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/factoryreset"
)

// FactoryResetter wipes the router (internal/factoryreset).
type FactoryResetter interface {
	Reset(ctx context.Context, phrase string) error
}

type factoryResetInput struct {
	Body struct {
		Confirm string `json:"confirm" maxLength:"64" doc:"Must be exactly \"factory reset\", typed by the operator"`
	}
}

func (s *server) registerFactoryReset(a huma.API) {
	o := op("postFactoryReset", http.MethodPost, "/api/v1/system/factory-reset", "Erase all settings and restart (OpenWrt firstboot)", "system", 400, 409, 500, 503)
	o.DefaultStatus = http.StatusAccepted
	// Wiping the router is never something an API token may do.
	registerSessionOnly(s, o, a, s.factoryReset)
}

func (s *server) factoryReset(ctx context.Context, in *factoryResetInput) (*struct{ Body okBody }, error) {
	if s.FactoryReset == nil {
		return nil, fail(http.StatusServiceUnavailable, "factory_reset_unavailable", "factory reset is unavailable")
	}
	err := s.FactoryReset.Reset(ctx, in.Body.Confirm)
	switch {
	case err == nil:
		out := &struct{ Body okBody }{}
		out.Body.Status = "ok"
		return out, nil
	case errors.Is(err, factoryreset.ErrConfirm):
		return nil, fail(http.StatusBadRequest, "confirmation_required", `type "factory reset" to confirm`)
	case errors.Is(err, factoryreset.ErrPending):
		return nil, fail(http.StatusConflict, "factory_reset_pending", "a factory reset was already requested")
	case errors.Is(err, factoryreset.ErrBusy):
		return nil, fail(http.StatusConflict, "operation_in_progress", "another change, restart or update is in progress")
	case errors.Is(err, factoryreset.ErrUnavailable):
		return nil, fail(http.StatusServiceUnavailable, "factory_reset_unavailable", "factory reset is unavailable")
	}
	slog.Warn("factory reset", "err", err)
	return nil, fail(http.StatusInternalServerError, "factory_reset_failed", "could not start the factory reset")
}
