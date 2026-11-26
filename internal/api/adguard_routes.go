package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

// AdGuard is the slice of adguard.Service the API needs.
type AdGuard interface {
	Status(context.Context) (adguard.Status, error)
	// Install is the user's explicit retry: it clears the install guard by name.
	Install(ctx context.Context, explicit bool) error
	SetDefaultDNS(ctx context.Context, on bool) error
	SetEnabled(ctx context.Context, on bool) error
}

type adguardToggleInput struct {
	Body struct {
		Enabled *bool `json:"enabled" required:"true"`
	}
}

type adguardStatusOutput struct {
	Body adguard.Status
}

func (s *server) registerAdGuard(a huma.API) {
	register(s, false, op("getAdGuard", http.MethodGet, "/api/v1/adguard", "AdGuard Home package and service status", "adguard", 503), a, s.getAdGuard)
	install := op("installAdGuard", http.MethodPost, "/api/v1/adguard/install", "Install AdGuard Home now (explicit retry)", "adguard", 409, 503)
	install.MaxBodyBytes = 1024
	register(s, false, install, a, s.installAdGuard)
	dns := op("setAdGuardDNS", http.MethodPost, "/api/v1/adguard/dns", "Turn AdGuard Home as the default DNS on or off", "adguard", 400, 409, 503)
	dns.MaxBodyBytes = 1024
	register(s, false, dns, a, s.setAdGuardDNS)
	svc := op("setAdGuardService", http.MethodPost, "/api/v1/adguard/service", "Start or stop AdGuard Home", "adguard", 400, 409, 503)
	svc.MaxBodyBytes = 1024
	register(s, false, svc, a, s.setAdGuardService)
}

func (s *server) setAdGuardDNS(ctx context.Context, in *adguardToggleInput) (*adguardStatusOutput, error) {
	return s.adguardToggle(ctx, in, func(ctx context.Context, on bool) error { return s.AdGuard.SetDefaultDNS(ctx, on) })
}

func (s *server) setAdGuardService(ctx context.Context, in *adguardToggleInput) (*adguardStatusOutput, error) {
	return s.adguardToggle(ctx, in, func(ctx context.Context, on bool) error { return s.AdGuard.SetEnabled(ctx, on) })
}

func (s *server) adguardToggle(ctx context.Context, in *adguardToggleInput, set func(context.Context, bool) error) (*adguardStatusOutput, error) {
	if in.Body.Enabled == nil {
		return nil, fail(400, "validation", "Enabled must be a boolean")
	}
	if s.AdGuard == nil {
		return nil, adguardUnavailable()
	}
	err := set(ctx, *in.Body.Enabled)
	switch {
	case err == nil:
		return s.getAdGuard(ctx, nil)
	case errors.Is(err, pkgmgr.ErrNotInstalled):
		return nil, fail(409, "adguard_not_installed", "adguardhome is not installed")
	case errors.Is(err, adguard.ErrExpertTopology):
		return nil, fail(409, "adguard_expert_topology", adguard.ErrExpertTopology.Error())
	case errors.Is(err, adguard.ErrNotAnswering):
		return nil, fail(409, "adguard_not_answering", adguard.ErrNotAnswering.Error())
	case errors.Is(err, adguard.ErrBusy):
		return nil, fail(409, "adguard_busy", "An AdGuard Home change is in progress or was interrupted")
	case errors.Is(err, guard.ErrActive), errors.Is(err, dnslayer.ErrForeignStaged):
		return nil, fail(409, "dns_busy", "Another DNS change is in progress or unfinished")
	default:
		slog.Warn("adguard toggle", "err", err)
		return nil, fail(503, "adguard_unavailable", "Changing adguardhome failed; see the system log")
	}
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
