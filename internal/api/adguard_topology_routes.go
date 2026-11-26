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
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// AdGuardTopology is the expert switch of ADR 0021 (adguard.Service.SetTopology).
type AdGuardTopology interface {
	SetTopology(ctx context.Context, mode string) (txn.Status, error)
}

type adguardTopologyInput struct {
	Body struct {
		Mode string `json:"mode" enum:"default,expert" doc:"expert: AdGuard Home answers on port 53 and forwards to dnsmasq on 127.0.0.1:5354"`
	}
}

func (s *server) registerAdGuardTopology(a huma.API) {
	put := op("setAdGuardTopology", http.MethodPut, "/api/v1/adguard/topology",
		"Move AdGuard Home onto port 53 in front of dnsmasq, or back (dhcp transaction the user confirms)", "adguard", 400, 409, 503)
	put.DefaultStatus = http.StatusAccepted
	put.MaxBodyBytes = 256
	register(s, false, put, a, s.setAdGuardTopology)
}

func (s *server) setAdGuardTopology(ctx context.Context, in *adguardTopologyInput) (*txnOutput, error) {
	if s.AdGuardTopology == nil {
		return nil, adguardUnavailable()
	}
	st, err := s.AdGuardTopology.SetTopology(ctx, in.Body.Mode)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			if err != nil {
				slog.Warn("adguard topology", "err", err)
			}
			return txnResponse(st), nil
		}
	}
	return nil, topologyError(err)
}

func topologyError(err error) error {
	conflict := func(code, msg string) error { return fail(http.StatusConflict, code, msg) }
	switch {
	case errors.Is(err, adguard.ErrTopologyInvalid):
		return fail(http.StatusBadRequest, "validation", err.Error())
	case errors.Is(err, adguard.ErrTopologyNotReady):
		return conflict("adguard_topology_not_ready", adguard.ErrTopologyNotReady.Error())
	case errors.Is(err, adguard.ErrTopologyForeign):
		return conflict("adguard_topology_foreign", adguard.ErrTopologyForeign.Error())
	case errors.Is(err, adguard.ErrUnknownShape):
		return conflict("adguard_config_unknown", err.Error())
	case errors.Is(err, adguard.ErrBusy):
		return conflict("adguard_busy", "An AdGuard Home change is in progress or was interrupted")
	case errors.Is(err, txn.ErrNoChanges):
		return conflict("adguard_topology_no_change", "AdGuard Home already uses this topology")
	case errors.Is(err, txn.ErrActive):
		return conflict("transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrForeignStaged), errors.Is(err, dnslayer.ErrForeignStaged), errors.Is(err, guard.ErrActive):
		return conflict("dns_busy", "Another DNS change is in progress or unfinished")
	}
	slog.Warn("adguard topology", "err", err)
	return adguardUnavailable()
}
