package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
)

// AdGuardDoH is AdGuard Home's DNS over HTTPS upstream (adguard.Upstreams, ADR 0010).
type AdGuardDoH interface {
	Get(context.Context) (adguard.DoHView, error)
	Set(context.Context, adguard.DoHRequest) (adguard.DoHView, error)
}

type dohOutput struct{ Body adguard.DoHView }

type dohInput struct{ Body adguard.DoHRequest }

func (s *server) registerAdGuardDoH(a huma.API) {
	register(s, false, op("getAdGuardDoh", http.MethodGet, "/api/v1/adguard/doh",
		"AdGuard Home's upstream resolvers and whether they are encrypted", "adguard", 503), a, s.getDoH)
	put := op("setAdGuardDoh", http.MethodPut, "/api/v1/adguard/doh",
		"Use one DNS over HTTPS server as AdGuard Home's upstream after AdGuard tested it", "adguard", 400, 409, 503)
	put.MaxBodyBytes = 1024
	register(s, false, put, a, s.setDoH)
}

func dohUnavailable() error {
	return fail(http.StatusServiceUnavailable, "adguard_unavailable", "AdGuard Home is not running or its admin API did not answer")
}

func (s *server) getDoH(ctx context.Context, _ *struct{}) (*dohOutput, error) {
	if s.AdGuardDoH == nil {
		return nil, dohUnavailable()
	}
	v, err := s.AdGuardDoH.Get(ctx)
	if err != nil {
		slog.Warn("adguard doh", "err", err)
		return nil, dohUnavailable()
	}
	return &dohOutput{Body: v}, nil
}

func (s *server) setDoH(ctx context.Context, in *dohInput) (*dohOutput, error) {
	if s.AdGuardDoH == nil {
		return nil, dohUnavailable()
	}
	v, err := s.AdGuardDoH.Set(ctx, in.Body)
	switch {
	case err == nil:
		return &dohOutput{Body: v}, nil
	case errors.Is(err, adguard.ErrDoHInvalid):
		return nil, fail(http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, adguard.ErrDoHUnreachable):
		return nil, fail(http.StatusConflict, "doh_upstream_unreachable", err.Error())
	case errors.Is(err, adguard.ErrExpertTopology):
		return nil, fail(http.StatusConflict, "adguard_expert_topology", err.Error())
	case errors.Is(err, adguard.ErrDoHNoChange):
		return nil, fail(http.StatusConflict, "doh_no_change", err.Error())
	case errors.Is(err, adguard.ErrBusy):
		return nil, fail(http.StatusConflict, "adguard_busy", "An AdGuard Home change is in progress or was interrupted")
	}
	slog.Warn("adguard doh", "err", err)
	return nil, dohUnavailable()
}
