package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/ifrestart"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

// InterfaceRestarter restarts one uplink interface (internal/ifrestart).
type InterfaceRestarter interface {
	Restart(context.Context, ifrestart.Request) (ifrestart.Outcome, error)
}

type interfaceRestartInput struct {
	Name string `path:"name" enum:"wan,wwan,usbtether" doc:"Uplink interface"`
	Body *struct {
		AcknowledgeLockout bool `json:"acknowledge_lockout,omitempty" doc:"Restart the WiFi uplink even if the caller's access point shares its radio"`
	} `required:"false"`
}

type interfaceRestartOutput struct{ Body ifrestart.Outcome }

func (s *server) registerInterfaceRestart(a huma.API) {
	o := op("restartNetworkInterface", http.MethodPost, "/api/v1/network/interfaces/{name}/restart",
		"Restart an uplink (ifup) and wait up to 30 s for it to come back", "network", 400, 404, 409, 503)
	o.MaxBodyBytes = 256
	register(s, false, o, a, s.restartInterface)
}

func (s *server) restartInterface(ctx context.Context, in *interfaceRestartInput) (*interfaceRestartOutput, error) {
	if s.InterfaceRestart == nil {
		return nil, fail(http.StatusServiceUnavailable, "interface_restart_unavailable", "interface restart is unavailable")
	}
	caller, _ := netip.ParseAddr(infoOf(ctx).ip)
	req := ifrestart.Request{Name: in.Name, Caller: caller}
	if in.Body != nil {
		req.AcknowledgeLockout = in.Body.AcknowledgeLockout
	}
	res, err := s.InterfaceRestart.Restart(ctx, req)
	if err != nil {
		return nil, interfaceRestartFailure(err)
	}
	return &interfaceRestartOutput{Body: res}, nil
}

func interfaceRestartFailure(err error) error {
	switch {
	case errors.Is(err, ifrestart.ErrNotRestartable):
		return fail(http.StatusBadRequest, "interface_not_restartable", "only wan, wwan and usbtether can be restarted")
	case errors.Is(err, ifrestart.ErrNotFound):
		return fail(http.StatusNotFound, "interface_not_found", "the interface is not configured")
	case errors.Is(err, ifrestart.ErrTxnPending):
		return fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, ifrestart.ErrBusy):
		return fail(http.StatusConflict, "interface_restart_busy", "another restart or live change is in progress")
	case errors.Is(err, lockout.ErrLockoutRisk):
		return fail(http.StatusConflict, "wifi_lockout_risk", "Change would disconnect the caller")
	case errors.Is(err, ifrestart.ErrFailed):
		slog.Warn("interface restart", "err", err)
		return fail(http.StatusServiceUnavailable, "interface_restart_failed", "the restart failed; retrying clears the guard")
	case errors.Is(err, wifi.ErrUnavailable):
		return fail(http.StatusServiceUnavailable, "interface_restart_unavailable", "WiFi state could not be read for the lockout check")
	}
	slog.Warn("interface restart", "err", err)
	return fail(http.StatusServiceUnavailable, "interface_restart_unavailable", "interface restart is unavailable")
}
