package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

type vpnStatusOutput struct {
	Body vpnstatus.TunnelStatus
}

func (s *server) registerVPNStatus(a huma.API) {
	register(s, false, op("getVPNStatus", http.MethodGet, "/api/v1/vpn/status", "WireGuard tunnel status", "vpn", 503), a, s.getVPNStatus)
}

func (s *server) getVPNStatus(ctx context.Context, _ *struct{}) (*vpnStatusOutput, error) {
	if s.VPNStatus == nil {
		return nil, fail(503, "vpn_status_unavailable", vpnstatus.ErrUnavailable.Error())
	}
	status, err := s.VPNStatus.Status(ctx)
	if err != nil {
		return nil, fail(503, "vpn_status_unavailable", vpnstatus.ErrUnavailable.Error())
	}
	return &vpnStatusOutput{Body: status}, nil
}
