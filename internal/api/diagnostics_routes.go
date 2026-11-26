package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/diagnostics"
)

// Diagnostics runs one ping, traceroute or DNS lookup from the router (internal/diagnostics).
type Diagnostics interface {
	Run(ctx context.Context, tool, target string) (diagnostics.Diagnosis, error)
}

type diagnosticsInput struct {
	Body struct {
		Tool   string `json:"tool" enum:"ping,traceroute,lookup"`
		Target string `json:"target" minLength:"1" maxLength:"253" doc:"Host name or IP address"`
	}
}

type diagnosticsOutput struct{ Body diagnostics.Diagnosis }

func (s *server) registerDiagnostics(a huma.API) {
	o := op("runNetworkDiagnostic", http.MethodPost, "/api/v1/network/diagnostics",
		"Run ping, traceroute or a DNS lookup (via the router's resolver) from the router", "network", 400, 409, 429, 503)
	o.MaxBodyBytes = 512
	register(s, false, o, a, s.runDiagnostic)
}

func (s *server) runDiagnostic(ctx context.Context, in *diagnosticsInput) (*diagnosticsOutput, error) {
	if s.Diagnostics == nil {
		return nil, fail(http.StatusServiceUnavailable, "diagnostics_unavailable", "diagnostics are unavailable")
	}
	// Each run sends packets and holds the single slot for up to 45 s: bounded per caller.
	if err := s.limited("diag:" + infoOf(ctx).ip); err != nil {
		return nil, err
	}
	res, err := s.Diagnostics.Run(ctx, in.Body.Tool, in.Body.Target)
	switch {
	case err == nil:
		return &diagnosticsOutput{Body: res}, nil
	case errors.Is(err, diagnostics.ErrInvalid):
		return nil, fail(http.StatusBadRequest, "invalid_target", "target must be a host name or IP address")
	case errors.Is(err, diagnostics.ErrBusy):
		return nil, fail(http.StatusConflict, "diagnostics_busy", "another diagnostic is running")
	}
	slog.Warn("diagnostics", "err", err)
	return nil, fail(http.StatusServiceUnavailable, "diagnostics_unavailable", "diagnostics are unavailable")
}
