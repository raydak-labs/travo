package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/lanaddress"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// LANAddress is the router's own lan address (internal/lanaddress, ADR 0028).
type LANAddress interface {
	Settings(context.Context) (lanaddress.View, error)
	Set(context.Context, lanaddress.Request) (txn.Status, error)
}

type lanOutput struct{ Body lanaddress.View }

type lanInput struct{ Body lanaddress.Request }

func (s *server) registerLAN(a huma.API) {
	register(s, false, op("getLanAddress", http.MethodGet, "/api/v1/network/lan",
		"Router lan address and a change waiting for its confirm from the new address", "network", 409, 503), a, s.getLAN)
	put := op("setLanAddress", http.MethodPut, "/api/v1/network/lan",
		"Move the router to a new lan address (guarded transaction confirmed from the new address)", "network", 400, 409, 503)
	put.DefaultStatus = http.StatusAccepted
	put.MaxBodyBytes = 256
	register(s, false, put, a, s.setLAN)
}

func lanUnavailable() error {
	return fail(http.StatusServiceUnavailable, "lan_unavailable", lanaddress.ErrUnavailable.Error())
}

func lanForeign() error {
	return fail(http.StatusConflict, "lan_foreign_config", lanaddress.ErrForeign.Error()+"; use LuCI")
}

func (s *server) getLAN(ctx context.Context, _ *struct{}) (*lanOutput, error) {
	if s.LAN == nil {
		return nil, lanUnavailable()
	}
	v, err := s.LAN.Settings(ctx)
	if errors.Is(err, lanaddress.ErrForeign) {
		return nil, lanForeign()
	}
	if err != nil {
		return nil, lanUnavailable()
	}
	return &lanOutput{Body: v}, nil
}

func (s *server) setLAN(ctx context.Context, in *lanInput) (*txnOutput, error) {
	if s.LAN == nil {
		return nil, lanUnavailable()
	}
	st, err := s.LAN.Set(ctx, in.Body)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	var fe *lanaddress.FieldError
	switch {
	case errors.As(err, &fe):
		return nil, fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, lanaddress.ErrOverlap):
		return nil, fail(http.StatusConflict, "lan_subnet_overlap", err.Error())
	case errors.Is(err, lanaddress.ErrPoolOutside):
		return nil, fail(http.StatusConflict, "lan_dhcp_pool_outside", err.Error())
	case errors.Is(err, lanaddress.ErrDependents):
		return nil, fail(http.StatusConflict, "lan_dependents_outside", err.Error())
	case errors.Is(err, lanaddress.ErrAdGuardExpert):
		return nil, fail(http.StatusConflict, "lan_adguard_expert", err.Error())
	case errors.Is(err, lanaddress.ErrForeign):
		return nil, lanForeign()
	case errors.Is(err, txn.ErrActive):
		return nil, fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(http.StatusConflict, "lan_no_change", "The router already has this lan address")
	case errors.Is(err, txn.ErrForeignStaged):
		return nil, fail(http.StatusConflict, "uci_busy", "another change is staged; try again")
	}
	slog.Warn("lan address", "err", err)
	return nil, lanUnavailable()
}
