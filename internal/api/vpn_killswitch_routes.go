package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
)

// VPNKillSwitch reads and changes the firewall kill switch (internal/vpnkill).
type VPNKillSwitch interface {
	Status(context.Context) (vpnkill.KillSwitchState, error)
	Set(context.Context, bool) (txn.Status, error)
}

type vpnKillSwitchOutput struct {
	Body vpnkill.KillSwitchState
}

type vpnKillSwitchInput struct {
	Body struct {
		Enabled *bool `json:"enabled" required:"true"`
	}
}

func (s *server) registerVPNKillSwitch(a huma.API) {
	register(s, false, op("getVPNKillSwitch", http.MethodGet, "/api/v1/vpn/killswitch", "VPN kill switch state", "vpn", 503), a, s.getVPNKillSwitch)
	change := op("setVPNKillSwitch", http.MethodPost, "/api/v1/vpn/killswitch", "Enable or disable the VPN kill switch", "vpn", 400, 409, 503)
	change.DefaultStatus = http.StatusAccepted
	change.MaxBodyBytes = 1024
	register(s, false, change, a, s.setVPNKillSwitch)
}

func (s *server) getVPNKillSwitch(ctx context.Context, _ *struct{}) (*vpnKillSwitchOutput, error) {
	if s.VPNKillSwitch == nil {
		return nil, vpnKillSwitchUnavailable()
	}
	st, err := s.VPNKillSwitch.Status(ctx)
	if err != nil {
		return nil, vpnKillSwitchUnavailable()
	}
	return &vpnKillSwitchOutput{Body: st}, nil
}

func (s *server) setVPNKillSwitch(ctx context.Context, in *vpnKillSwitchInput) (*txnOutput, error) {
	if in.Body.Enabled == nil {
		return nil, fail(400, "validation", "Enabled must be a boolean")
	}
	if s.VPNKillSwitch == nil {
		return nil, vpnKillSwitchUnavailable()
	}
	st, err := s.VPNKillSwitch.Set(ctx, *in.Body.Enabled)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	switch {
	case errors.Is(err, vpnkill.ErrNotConnected):
		return nil, fail(409, "vpn_not_connected", "Connect the tunnel before enabling the kill switch")
	case errors.Is(err, vpnkill.ErrUnsupported):
		return nil, fail(409, "vpn_killswitch_unsupported", vpnkill.ErrUnsupported.Error())
	case errors.Is(err, txn.ErrActive):
		return nil, fail(409, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(409, "vpn_no_change", "Kill switch is already in that state")
	default:
		return nil, vpnKillSwitchUnavailable()
	}
}

func vpnKillSwitchUnavailable() error {
	return fail(503, "vpn_killswitch_unavailable", vpnkill.ErrUnavailable.Error())
}
