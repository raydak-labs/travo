package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/reboot"
)

type rebootInput struct {
	Body struct {
		Confirm bool `json:"confirm" doc:"Must be true: the caller has shown the operator a confirmation"`
	}
}

type rebootBody struct {
	Status string `json:"status" enum:"ok"`
	BootID string `json:"boot_id" doc:"Opaque id of the boot that accepted the request; compare with GET /api/health after the restart"`
}

type rebootOutput struct{ Body rebootBody }

func (s *server) registerReboot(a huma.API) {
	operation := op("postSystemReboot", http.MethodPost, "/api/v1/system/reboot", "Restart the router", "system", 400, 409, 500, 503)
	operation.DefaultStatus = http.StatusAccepted
	register(s, false, operation, a, s.requestReboot)
}

func (s *server) requestReboot(ctx context.Context, in *rebootInput) (*rebootOutput, error) {
	if !in.Body.Confirm {
		return nil, fail(http.StatusBadRequest, "confirmation_required", "Confirm the router restart first")
	}
	if s.Rebooter == nil {
		return nil, fail(http.StatusServiceUnavailable, "reboot_unavailable", "Router restart is unavailable")
	}
	accepted, err := s.Rebooter.Reboot(ctx)
	if err != nil {
		return nil, rebootRequestError(err)
	}
	return &rebootOutput{Body: rebootBody{Status: "ok", BootID: accepted.BootID}}, nil
}

func rebootRequestError(err error) error {
	switch {
	case errors.Is(err, reboot.ErrPending):
		return fail(http.StatusConflict, "reboot_pending", "A router restart is already in progress")
	case errors.Is(err, reboot.ErrTxnPending):
		return fail(http.StatusConflict, "transaction_pending", "Confirm or roll back the pending change first")
	case errors.Is(err, reboot.ErrUpdateBusy):
		return fail(http.StatusConflict, "update_busy", "An update is in progress")
	case errors.Is(err, reboot.ErrUnavailable):
		return fail(http.StatusServiceUnavailable, "reboot_unavailable", "Router restart is unavailable")
	default:
		slog.Warn("reboot request failed", "err", err)
		return fail(http.StatusInternalServerError, "reboot_failed", "Could not restart the router")
	}
}
