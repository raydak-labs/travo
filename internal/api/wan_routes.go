package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wansettings"
)

// WANSettings is the wan uplink's protocol (internal/wansettings).
type WANSettings interface {
	Settings(context.Context) (wansettings.View, error)
	Set(context.Context, wansettings.Request) (txn.Status, error)
}

type wanOutput struct{ Body wansettings.View }

type wanInput struct{ Body wansettings.Request }

func (s *server) registerWAN(a huma.API) {
	register(s, false, op("getWanSettings", http.MethodGet, "/api/v1/network/wan",
		"WAN protocol and its settings (network.wan); the PPPoE password is never returned", "network", 409, 503), a, s.getWAN)
	put := op("setWanSettings", http.MethodPut, "/api/v1/network/wan",
		"Replace the WAN settings (guarded transaction the user confirms)", "network", 400, 409, 503)
	put.DefaultStatus = http.StatusAccepted
	put.MaxBodyBytes = 1024
	register(s, false, put, a, s.setWAN)
}

func wanUnavailable() error {
	return fail(http.StatusServiceUnavailable, "wan_unavailable", wansettings.ErrUnavailable.Error())
}

func (s *server) getWAN(ctx context.Context, _ *struct{}) (*wanOutput, error) {
	if s.WAN == nil {
		return nil, wanUnavailable()
	}
	v, err := s.WAN.Settings(ctx)
	if errors.Is(err, wansettings.ErrForeign) {
		return nil, wanForeign()
	}
	if err != nil {
		return nil, wanUnavailable()
	}
	return &wanOutput{Body: v}, nil
}

func wanForeign() error {
	return fail(http.StatusConflict, "wan_foreign_config", "network.wan is configured in a way Travo does not edit; use LuCI")
}

// setWAN never logs the request: it may carry the PPPoE password.
func (s *server) setWAN(ctx context.Context, in *wanInput) (*txnOutput, error) {
	if s.WAN == nil {
		return nil, wanUnavailable()
	}
	st, err := s.WAN.Set(ctx, in.Body)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	var fe *wansettings.FieldError
	switch {
	case errors.As(err, &fe):
		return nil, fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, wansettings.ErrOverlapsLAN):
		return nil, fail(http.StatusConflict, "wan_overlaps_lan", wansettings.ErrOverlapsLAN.Error())
	case errors.Is(err, wansettings.ErrForeign):
		return nil, wanForeign()
	case errors.Is(err, txn.ErrActive):
		return nil, fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(http.StatusConflict, "wan_no_change", "The WAN already has these settings")
	case errors.Is(err, txn.ErrForeignStaged):
		return nil, fail(http.StatusConflict, "uci_busy", "another change is staged; try again")
	}
	slog.Warn("wan settings", "err", err)
	return nil, wanUnavailable()
}
