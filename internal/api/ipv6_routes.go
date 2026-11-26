package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/ipv6settings"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// IPv6Settings is the IPv6 switch for uplinks and LAN (internal/ipv6settings).
type IPv6Settings interface {
	Settings(context.Context) (ipv6settings.View, error)
	Set(context.Context, ipv6settings.Request) (txn.Status, error)
}

type ipv6Output struct{ Body ipv6settings.View }

type ipv6Input struct{ Body ipv6settings.Request }

func (s *server) registerIPv6(a huma.API) {
	register(s, false, op("getIpv6Settings", http.MethodGet, "/api/v1/network/ipv6",
		"Whether IPv6 is on for the uplinks (wan6, wwan6) and the LAN (odhcpd RA/DHCPv6)", "network", 409, 503), a, s.getIPv6)
	put := op("setIpv6Settings", http.MethodPut, "/api/v1/network/ipv6",
		"Turn IPv6 on or off (network + dhcp transaction the user confirms)", "network", 400, 409, 503)
	put.DefaultStatus = http.StatusAccepted
	put.MaxBodyBytes = 256
	register(s, false, put, a, s.setIPv6)
}

func ipv6Unavailable() error {
	return fail(http.StatusServiceUnavailable, "ipv6_unavailable", ipv6settings.ErrUnavailable.Error())
}

func ipv6Foreign() error {
	return fail(http.StatusConflict, "ipv6_foreign_config", ipv6settings.ErrForeign.Error())
}

func (s *server) getIPv6(ctx context.Context, _ *struct{}) (*ipv6Output, error) {
	if s.IPv6 == nil {
		return nil, ipv6Unavailable()
	}
	v, err := s.IPv6.Settings(ctx)
	if errors.Is(err, ipv6settings.ErrForeign) {
		return nil, ipv6Foreign()
	}
	if err != nil {
		return nil, ipv6Unavailable()
	}
	return &ipv6Output{Body: v}, nil
}

func (s *server) setIPv6(ctx context.Context, in *ipv6Input) (*txnOutput, error) {
	if s.IPv6 == nil {
		return nil, ipv6Unavailable()
	}
	st, err := s.IPv6.Set(ctx, in.Body)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	switch {
	case errors.Is(err, ipv6settings.ErrForeign):
		return nil, ipv6Foreign()
	case errors.Is(err, ipv6settings.ErrVPNLeak):
		return nil, fail(http.StatusConflict, "ipv6_vpn_leak", ipv6settings.ErrVPNLeak.Error())
	case errors.Is(err, txn.ErrActive):
		return nil, fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(http.StatusConflict, "ipv6_no_change", "IPv6 is already in this state")
	case errors.Is(err, txn.ErrForeignStaged):
		return nil, fail(http.StatusConflict, "uci_busy", "another change is staged; try again")
	}
	slog.Warn("ipv6 settings", "err", err)
	return nil, ipv6Unavailable()
}
