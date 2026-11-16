package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
)

// AdGuard is the slice of adguard.Service the API needs.
type AdGuard interface {
	Status(context.Context) (adguard.Status, error)
	// Install is the user's explicit retry: it clears the install guard by name.
	Install(ctx context.Context, explicit bool) error
}

type adguardStatusOutput struct {
	Body adguard.Status
}

func (s *server) registerAdGuard(a huma.API) {
	register(s, false, op("getAdGuard", http.MethodGet, "/api/v1/adguard", "AdGuard Home package and service status", "adguard", 503), a, s.getAdGuard)
	install := op("installAdGuard", http.MethodPost, "/api/v1/adguard/install", "Install AdGuard Home now (explicit retry)", "adguard", 409, 503)
	install.MaxBodyBytes = 1024
	register(s, false, install, a, s.installAdGuard)
}

func (s *server) getAdGuard(ctx context.Context, _ *struct{}) (*adguardStatusOutput, error) {
	if s.AdGuard == nil {
		return nil, adguardUnavailable()
	}
	st, err := s.AdGuard.Status(ctx)
	if err != nil {
		slog.Warn("adguard status", "err", err)
		return nil, adguardUnavailable()
	}
	return &adguardStatusOutput{Body: st}, nil
}

func (s *server) installAdGuard(ctx context.Context, _ *struct{}) (*adguardStatusOutput, error) {
	if s.AdGuard == nil {
		return nil, adguardUnavailable()
	}
	err := s.AdGuard.Install(ctx, true)
	switch {
	case errors.Is(err, adguard.ErrBusy):
		return nil, fail(409, "adguard_busy", "An AdGuard Home change is in progress or was interrupted")
	case errors.Is(err, adguard.ErrOffline):
		return nil, fail(409, "adguard_offline", "AdGuard Home installs once the router has Internet")
	case errors.Is(err, adguard.ErrNoSpace):
		return nil, fail(409, "adguard_no_space", err.Error())
	case errors.Is(err, adguard.ErrUnknownShape):
		return nil, fail(409, "adguard_config_unknown", err.Error())
	case err != nil:
		slog.Warn("adguard install", "err", err)
		return nil, fail(503, "adguard_install_failed", "Installing adguardhome failed; see the system log")
	}
	return s.getAdGuard(ctx, nil)
}

func adguardUnavailable() error {
	return fail(503, "adguard_unavailable", "adguardhome status is unavailable")
}
