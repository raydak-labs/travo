package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/tailscale"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Tailscale is the slice of tailscale.Service the API needs.
type Tailscale interface {
	Status(context.Context) (tailscale.Status, error)
	// Install is explicit only; it clears the install guard by name (the retry path).
	Install(context.Context) error
	Login(context.Context) (tailscale.Status, error)
	Logout(context.Context) (tailscale.Status, error)
	SetSettings(context.Context, tailscale.Settings) (tailscale.Status, error)
	// ApplyFirewall begins the firewall transaction for the travo_ts zone.
	ApplyFirewall(context.Context) (txn.Status, error)
}

type tailscaleSettingsInput struct {
	Body tailscale.Settings
}

type tailscaleStatusOutput struct {
	Body tailscale.Status
}

func (s *server) registerTailscale(a huma.API) {
	register(s, false, op("getTailscale", http.MethodGet, "/api/v1/tailscale", "Tailscale package, daemon and tailnet status", "tailscale", 503), a, s.getTailscale)
	for _, o := range []struct {
		id, path, summary string
		h                 func(context.Context, *struct{}) (*tailscaleStatusOutput, error)
	}{
		{"installTailscale", "/api/v1/tailscale/install", "Install the tailscale package (explicit, guarded)", s.installTailscale},
		{"loginTailscale", "/api/v1/tailscale/login", "Bring Tailscale up; returns the auth URL until the login completes", s.loginTailscale},
		{"logoutTailscale", "/api/v1/tailscale/logout", "Sign the router out of the tailnet", s.logoutTailscale},
	} {
		x := op(o.id, http.MethodPost, o.path, o.summary, "tailscale", 409, 503)
		x.MaxBodyBytes = 1024
		register(s, false, x, a, o.h)
	}
	set := op("setTailscaleSettings", http.MethodPut, "/api/v1/tailscale/settings", "Set exit node, LAN advertisement and Tailscale SSH", "tailscale", 400, 409, 503)
	set.MaxBodyBytes = 1024
	register(s, false, set, a, s.setTailscaleSettings)
	fw := op("applyTailscaleFirewall", http.MethodPost, "/api/v1/tailscale/firewall", "Stage the Tailscale firewall zone in a transaction", "tailscale", 409, 503)
	fw.DefaultStatus = http.StatusAccepted
	fw.MaxBodyBytes = 1024
	register(s, false, fw, a, s.applyTailscaleFirewall)
}

func (s *server) setTailscaleSettings(ctx context.Context, in *tailscaleSettingsInput) (*tailscaleStatusOutput, error) {
	return s.tailscaleChange(ctx, "settings", func(ctx context.Context) (tailscale.Status, error) {
		return s.Tailscale.SetSettings(ctx, in.Body)
	})
}

func (s *server) applyTailscaleFirewall(ctx context.Context, _ *struct{}) (*txnOutput, error) {
	if s.Tailscale == nil {
		return nil, tailscaleUnavailable()
	}
	st, err := s.Tailscale.ApplyFirewall(ctx)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	switch {
	case err == nil:
		return txnResponse(st), nil
	case errors.Is(err, txn.ErrActive):
		return nil, fail(409, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(409, "tailscale_no_change", "The Tailscale firewall zone is already in place")
	case errors.Is(err, tailscale.ErrFirewallConflict):
		return nil, fail(409, "tailscale_firewall_conflict", err.Error())
	default:
		return nil, tailscaleError("firewall", err)
	}
}

func (s *server) getTailscale(ctx context.Context, _ *struct{}) (*tailscaleStatusOutput, error) {
	if s.Tailscale == nil {
		return nil, tailscaleUnavailable()
	}
	st, err := s.Tailscale.Status(ctx)
	if err != nil {
		slog.Warn("tailscale status", "err", err)
		return nil, tailscaleUnavailable()
	}
	return &tailscaleStatusOutput{Body: st}, nil
}

func (s *server) installTailscale(ctx context.Context, _ *struct{}) (*tailscaleStatusOutput, error) {
	if s.Tailscale == nil {
		return nil, tailscaleUnavailable()
	}
	if err := s.Tailscale.Install(ctx); err != nil {
		return nil, tailscaleError("install", err)
	}
	return s.getTailscale(ctx, nil)
}

func (s *server) loginTailscale(ctx context.Context, _ *struct{}) (*tailscaleStatusOutput, error) {
	return s.tailscaleChange(ctx, "login", s.tailscaleLogin)
}

func (s *server) logoutTailscale(ctx context.Context, _ *struct{}) (*tailscaleStatusOutput, error) {
	return s.tailscaleChange(ctx, "logout", s.tailscaleLogout)
}

func (s *server) tailscaleLogin(ctx context.Context) (tailscale.Status, error) {
	return s.Tailscale.Login(ctx)
}

func (s *server) tailscaleLogout(ctx context.Context) (tailscale.Status, error) {
	return s.Tailscale.Logout(ctx)
}

func (s *server) tailscaleChange(ctx context.Context, what string, do func(context.Context) (tailscale.Status, error)) (*tailscaleStatusOutput, error) {
	if s.Tailscale == nil {
		return nil, tailscaleUnavailable()
	}
	st, err := do(ctx)
	if err != nil {
		return nil, tailscaleError(what, err)
	}
	return &tailscaleStatusOutput{Body: st}, nil
}

// tailscaleError maps service errors to stable codes. A missing package is 503 naming it
// (invariants §13).
func tailscaleError(what string, err error) error {
	switch {
	case errors.Is(err, pkgmgr.ErrNotInstalled):
		return fail(503, "tailscale_not_installed", "The tailscale package is not installed")
	case errors.Is(err, tailscale.ErrNotRunning):
		return fail(503, "tailscale_not_running", "tailscaled is not running")
	case errors.Is(err, tailscale.ErrBusy), errors.Is(err, guard.ErrActive):
		return fail(409, "tailscale_busy", "A Tailscale install or change is in progress or was interrupted")
	case errors.Is(err, tailscale.ErrOffline):
		return fail(409, "tailscale_offline", "Tailscale installs once the router has Internet")
	case errors.Is(err, tailscale.ErrNoSpace):
		return fail(409, "tailscale_no_space", err.Error())
	case errors.Is(err, tailscale.ErrExitNodeUnknown):
		return fail(409, "tailscale_exit_node_unknown", tailscale.ErrExitNodeUnknown.Error())
	case errors.Is(err, tailscale.ErrExitNodeConflict):
		return fail(409, "tailscale_exit_node_conflict", tailscale.ErrExitNodeConflict.Error())
	case errors.Is(err, tailscale.ErrExitNodeOffline):
		return fail(409, "tailscale_exit_node_offline", tailscale.ErrExitNodeOffline.Error())
	case errors.Is(err, tailscale.ErrNoLAN):
		return fail(409, "tailscale_no_lan", tailscale.ErrNoLAN.Error())
	default:
		slog.Warn("tailscale "+what, "err", err)
		return fail(503, "tailscale_failed", "Tailscale "+what+" failed; see the system log")
	}
}

func tailscaleUnavailable() error {
	return fail(503, "tailscale_unavailable", "Tailscale status is unavailable")
}
