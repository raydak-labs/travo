package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/ddns"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// DDNS is dynamic DNS through ddns-scripts (internal/ddns).
type DDNS interface {
	Settings(context.Context) (ddns.View, error)
	Set(context.Context, ddns.Request) error
	Install(context.Context) error
}

type ddnsOutput struct{ Body ddns.View }

type ddnsInput struct{ Body ddns.Request }

func (s *server) registerDDNS(a huma.API) {
	register(s, false, op("getDdns", http.MethodGet, "/api/v1/network/ddns",
		"Dynamic DNS (ddns-scripts section ddns.travo); the token is never returned", "network", 409, 503), a, s.getDDNS)
	put := op("setDdns", http.MethodPut, "/api/v1/network/ddns",
		"Replace the dynamic DNS settings and restart ddns-scripts", "network", 400, 409, 503)
	put.MaxBodyBytes = 2048
	register(s, false, put, a, s.setDDNS)
	inst := op("installDdns", http.MethodPost, "/api/v1/network/ddns/install",
		"Install ddns-scripts (explicit, guarded, needs free flash)", "network", 409, 503)
	inst.MaxBodyBytes = 1024
	register(s, false, inst, a, s.installDDNS)
}

func ddnsUnavailable() error {
	return fail(http.StatusServiceUnavailable, "ddns_unavailable", ddns.ErrUnavailable.Error())
}

func (s *server) getDDNS(ctx context.Context, _ *struct{}) (*ddnsOutput, error) {
	if s.DDNS == nil {
		return nil, ddnsUnavailable()
	}
	v, err := s.DDNS.Settings(ctx)
	if err != nil {
		return nil, ddnsError(err)
	}
	return &ddnsOutput{Body: v}, nil
}

// setDDNS never logs the request: it may carry the token.
func (s *server) setDDNS(ctx context.Context, in *ddnsInput) (*ddnsOutput, error) {
	if s.DDNS == nil {
		return nil, ddnsUnavailable()
	}
	if err := s.DDNS.Set(ctx, in.Body); err != nil {
		return nil, ddnsError(err)
	}
	return s.getDDNS(ctx, nil)
}

func (s *server) installDDNS(ctx context.Context, _ *struct{}) (*ddnsOutput, error) {
	if s.DDNS == nil {
		return nil, ddnsUnavailable()
	}
	if err := s.DDNS.Install(ctx); err != nil {
		return nil, ddnsError(err)
	}
	return s.getDDNS(ctx, nil)
}

func ddnsError(err error) error {
	var fe *ddns.FieldError
	switch {
	case errors.As(err, &fe):
		return fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, pkgmgr.ErrNotInstalled):
		return fail(http.StatusServiceUnavailable, "ddns_not_installed", ddns.Package+" is not installed")
	case errors.Is(err, ddns.ErrForeign):
		return fail(http.StatusConflict, "ddns_foreign_config", ddns.ErrForeign.Error())
	case errors.Is(err, ddns.ErrNoChange):
		return fail(http.StatusConflict, "ddns_no_change", ddns.ErrNoChange.Error())
	case errors.Is(err, ddns.ErrBusy), errors.Is(err, guard.ErrActive):
		return fail(http.StatusConflict, "ddns_busy", ddns.ErrBusy.Error())
	case errors.Is(err, ddns.ErrOffline):
		return fail(http.StatusConflict, "ddns_offline", "ddns-scripts installs once the router has Internet")
	case errors.Is(err, ddns.ErrNoSpace):
		return fail(http.StatusConflict, "ddns_no_space", err.Error())
	case errors.Is(err, txn.ErrActive):
		return fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrForeignStaged):
		return fail(http.StatusConflict, "uci_busy", "another change is staged; try again")
	}
	slog.Warn("ddns", "err", err)
	return ddnsUnavailable()
}
