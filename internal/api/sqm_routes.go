package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/raydak-labs/travo/internal/pkgmgr"
	"github.com/raydak-labs/travo/internal/sqm"
	"github.com/raydak-labs/travo/internal/txn"
)

// SQM reads and writes essential SQM queue settings (internal/sqm).
type SQM interface {
	Get(context.Context) (sqm.Settings, error)
	Set(context.Context, sqm.Settings) (sqm.Settings, error)
}

type sqmOutput struct{ Body sqm.Settings }

type sqmInput struct{ Body sqm.Settings }

func (s *server) registerSQM(a huma.API) {
	const path = "/api/v1/sqm"
	register(s, false, op("getSqm", http.MethodGet, path,
		"Essential SQM queue settings (sqm-scripts)", "services", 503), a, s.getSQM)
	put := op("setSqm", http.MethodPut, path,
		"Replace essential SQM queue settings and reload sqm", "services", 400, 409, 503)
	put.MaxBodyBytes = 2048
	register(s, false, put, a, s.setSQM)
}

func sqmUnavailable() error {
	return fail(http.StatusServiceUnavailable, "sqm_unavailable", sqm.ErrUnavailable.Error())
}

func (s *server) getSQM(ctx context.Context, _ *struct{}) (*sqmOutput, error) {
	if s.SQM == nil {
		return nil, sqmUnavailable()
	}
	v, err := s.SQM.Get(ctx)
	if err != nil {
		return nil, sqmError(err)
	}
	return &sqmOutput{Body: v}, nil
}

func (s *server) setSQM(ctx context.Context, in *sqmInput) (*sqmOutput, error) {
	if s.SQM == nil {
		return nil, sqmUnavailable()
	}
	v, err := s.SQM.Set(ctx, in.Body)
	if err != nil {
		return nil, sqmError(err)
	}
	return &sqmOutput{Body: v}, nil
}

func sqmError(err error) error {
	var fe *sqm.FieldError
	switch {
	case errors.As(err, &fe):
		return fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, pkgmgr.ErrNotInstalled):
		return fail(http.StatusServiceUnavailable, "sqm_not_installed", sqm.Package+" is not installed")
	case errors.Is(err, txn.ErrActive):
		return fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrForeignStaged):
		return fail(http.StatusConflict, "uci_busy", "another change to the sqm config is staged; try again")
	}
	slog.Warn("sqm", "err", err)
	return sqmUnavailable()
}
