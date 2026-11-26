package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/captive"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
)

// DashboardConnectivity is the probe result plus the captive assist state the card acts on.
type DashboardConnectivity struct {
	connectivity.Status
	Captive CaptiveState `json:"captive"`
}

// CaptiveState is what the hotel card can offer: the DNS bypass and the wwan DHCP renew.
type CaptiveState struct {
	captive.BypassStatus
	DHCPRenew captive.RenewState `json:"dhcp_renew" enum:"available,not_applicable,blocked" doc:"blocked: an earlier renew is unproven; renewing again is its retry"`
}

type connectivityOutput struct {
	Body DashboardConnectivity
}

func (s *server) registerConnectivity(a huma.API) {
	register(s, false, op("getDashboardConnectivity", http.MethodGet, "/api/v1/dashboard/connectivity", "Uplink and internet status", "dashboard"), a, s.getDashboardConnectivity)
}

// getDashboardConnectivity never mutates: the captive state is read from the layer stack.
func (s *server) getDashboardConnectivity(ctx context.Context, _ *struct{}) (*connectivityOutput, error) {
	out := DashboardConnectivity{Status: connectivity.Status{Internet: connectivity.Unknown}}
	if s.Connectivity != nil {
		out.Status = s.Connectivity.Status(ctx)
	}
	if s.Captive != nil {
		out.Captive.BypassStatus = s.Captive.Status()
	}
	out.Captive.DHCPRenew = captive.RenewNotApplicable
	if s.CaptiveRenew != nil {
		out.Captive.DHCPRenew = s.CaptiveRenew.Availability(ctx, out.Status)
	}
	return &connectivityOutput{Body: out}, nil
}
