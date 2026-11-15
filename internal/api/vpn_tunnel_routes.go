package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

type vpnTunnelInput struct {
	Body struct {
		Enabled *bool `json:"enabled" required:"true"`
	}
}

func (s *server) registerVPNTunnel(a huma.API) {
	change := op("setVPNTunnel", http.MethodPost, "/api/v1/vpn/tunnel", "Enable or disable the WireGuard tunnel", "vpn", 400, 409, 503)
	change.DefaultStatus = http.StatusAccepted
	change.MaxBodyBytes = 1024
	register(s, false, change, a, s.setVPNTunnel)
}

func (s *server) setVPNTunnel(ctx context.Context, in *vpnTunnelInput) (*txnOutput, error) {
	if in.Body.Enabled == nil {
		return nil, fail(400, "validation", "Enabled must be a boolean")
	}
	if s.VPNTunnel == nil {
		return nil, vpnTunnelUnavailable()
	}
	st, err := s.VPNTunnel.Set(ctx, *in.Body.Enabled)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	switch {
	case errors.Is(err, vpnprofiles.ErrNotFound):
		return nil, fail(409, "vpn_profile_required", "Select a WireGuard profile first")
	case errors.Is(err, txn.ErrActive):
		return nil, fail(409, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(409, "vpn_no_change", "WireGuard configuration is unchanged")
	default:
		return nil, vpnTunnelUnavailable()
	}
}

func vpnTunnelUnavailable() error {
	return fail(503, "vpn_tunnel_unavailable", "WireGuard tunnel changes are unavailable")
}
