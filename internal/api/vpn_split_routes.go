package api

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
	"github.com/blackdark-org/travo-greenfield/internal/vpntunnel"
)

// VPNSplit reads and changes WireGuard split tunneling (internal/vpntunnel).
type VPNSplit interface {
	Split(context.Context) (vpntunnel.Split, error)
	SetSplit(context.Context, []netip.Prefix) (txn.Status, error)
}

type splitBody struct {
	// Routes are IPv4 prefixes that use the tunnel; [] means full tunnel.
	Routes []string `json:"routes" maxItems:"32" doc:"IPv4 prefixes routed through the tunnel; empty = full tunnel"`
}

type vpnSplitOutput struct{ Body splitBody }

type vpnSplitInput struct{ Body splitBody }

func (s *server) registerVPNSplit(a huma.API) {
	register(s, false, op("getVPNSplit", http.MethodGet, "/api/v1/vpn/split", "WireGuard split tunnel routes", "vpn", 503), a, s.getVPNSplit)
	change := op("setVPNSplit", http.MethodPost, "/api/v1/vpn/split", "Set WireGuard split tunnel routes", "vpn", 400, 409, 503)
	change.DefaultStatus = http.StatusAccepted
	change.MaxBodyBytes = 4096
	register(s, false, change, a, s.setVPNSplit)
}

func (s *server) getVPNSplit(ctx context.Context, _ *struct{}) (*vpnSplitOutput, error) {
	if s.VPNSplit == nil {
		return nil, vpnSplitUnavailable()
	}
	split, err := s.VPNSplit.Split(ctx)
	if err != nil {
		return nil, vpnSplitUnavailable()
	}
	out := &vpnSplitOutput{Body: splitBody{Routes: make([]string, 0, len(split.Routes))}}
	for _, p := range split.Routes {
		out.Body.Routes = append(out.Body.Routes, p.String())
	}
	return out, nil
}

func (s *server) setVPNSplit(ctx context.Context, in *vpnSplitInput) (*txnOutput, error) {
	routes := make([]netip.Prefix, 0, len(in.Body.Routes))
	for _, r := range in.Body.Routes {
		p, err := netip.ParsePrefix(r)
		if err != nil {
			return nil, fail(400, "validation", "routes: "+r+" is not an IPv4 prefix")
		}
		routes = append(routes, p)
	}
	if _, err := vpntunnel.ValidateSplit(routes); err != nil {
		return nil, fail(400, "validation", "routes: "+err.Error())
	}
	if s.VPNSplit == nil {
		return nil, vpnSplitUnavailable()
	}
	st, err := s.VPNSplit.SetSplit(ctx, routes)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	switch {
	case errors.Is(err, vpnprofiles.ErrNotFound):
		return nil, fail(409, "vpn_profile_required", "Select and apply a WireGuard profile first")
	case errors.Is(err, txn.ErrActive):
		return nil, fail(409, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(409, "vpn_no_change", "Split tunnel routes are unchanged")
	default:
		return nil, vpnSplitUnavailable()
	}
}

func vpnSplitUnavailable() error {
	return fail(503, "vpn_split_unavailable", "WireGuard split tunneling is unavailable")
}
