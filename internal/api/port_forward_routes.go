package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/portfwd"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// PortForwards is the firewall redirects from wan to lan hosts (internal/portfwd).
type PortForwards interface {
	Forwards(context.Context) ([]portfwd.Forward, error)
	Add(context.Context, portfwd.ForwardRequest) (txn.Status, error)
	Remove(ctx context.Context, id string) (txn.Status, error)
}

type portForwardsOutput struct {
	Body struct {
		Forwards []portfwd.Forward `json:"forwards" nullable:"false"`
	}
}

type portForwardInput struct{ Body portfwd.ForwardRequest }

type portForwardPath struct {
	ID string `path:"id" pattern:"^[A-Za-z0-9_]{1,64}$" doc:"UCI section of the forward"`
}

func (s *server) registerPortForwards(a huma.API) {
	register(s, false, op("listPortForwards", http.MethodGet, "/api/v1/network/port-forwards",
		"Port forwards (firewall redirect sections); only Travo's travo_fwd_<n> are editable", "network", 503), a, s.listPortForwards)
	add := op("addPortForward", http.MethodPost, "/api/v1/network/port-forwards",
		"Forward a wan port to a lan host (guarded transaction the user confirms)", "network", 400, 409, 503)
	add.DefaultStatus = http.StatusAccepted
	add.MaxBodyBytes = 512
	register(s, false, add, a, s.addPortForward)
	del := op("deletePortForward", http.MethodDelete, "/api/v1/network/port-forwards/{id}",
		"Remove a port forward Travo created (guarded transaction the user confirms)", "network", 404, 409, 503)
	del.DefaultStatus = http.StatusAccepted
	register(s, false, del, a, s.deletePortForward)
}

func portForwardsUnavailable() error {
	return fail(http.StatusServiceUnavailable, "port_forwards_unavailable", portfwd.ErrUnavailable.Error())
}

// portForwardResult answers a started or rolled-back transaction, else maps the refusal.
func portForwardResult(st txn.Status, err error) (*txnOutput, error) {
	if err == nil || (st.ID != "" && (st.State == txn.RolledBack || st.State == txn.Restoring || st.State == txn.RollbackFailed)) {
		return txnResponse(st), nil
	}
	var fe *portfwd.FieldError
	switch {
	case errors.As(err, &fe):
		return nil, fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, portfwd.ErrPortInUse):
		return nil, fail(http.StatusConflict, "port_forward_in_use", portfwd.ErrPortInUse.Error())
	case errors.Is(err, portfwd.ErrFull):
		return nil, fail(http.StatusConflict, "port_forwards_full", portfwd.ErrFull.Error())
	case errors.Is(err, portfwd.ErrNoZone):
		return nil, fail(http.StatusConflict, "port_forward_no_zone", portfwd.ErrNoZone.Error())
	case errors.Is(err, portfwd.ErrForeign):
		return nil, fail(http.StatusConflict, "port_forward_foreign", "the forward was not created by Travo; edit it in LuCI")
	case errors.Is(err, portfwd.ErrNotFound):
		return nil, fail(http.StatusNotFound, "not_found", portfwd.ErrNotFound.Error())
	case errors.Is(err, portfwd.ErrTransactionPending), errors.Is(err, txn.ErrActive):
		return nil, fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrForeignStaged):
		return nil, fail(http.StatusConflict, "uci_busy", "another change is staged; try again")
	}
	slog.Warn("port forwards", "err", err)
	return nil, portForwardsUnavailable()
}

func (s *server) listPortForwards(ctx context.Context, _ *struct{}) (*portForwardsOutput, error) {
	if s.PortForwards == nil {
		return nil, portForwardsUnavailable()
	}
	list, err := s.PortForwards.Forwards(ctx)
	if err != nil {
		return nil, portForwardsUnavailable()
	}
	out := &portForwardsOutput{}
	out.Body.Forwards = list
	if list == nil {
		out.Body.Forwards = []portfwd.Forward{}
	}
	return out, nil
}

func (s *server) addPortForward(ctx context.Context, in *portForwardInput) (*txnOutput, error) {
	if s.PortForwards == nil {
		return nil, portForwardsUnavailable()
	}
	return portForwardResult(s.PortForwards.Add(ctx, in.Body))
}

func (s *server) deletePortForward(ctx context.Context, in *portForwardPath) (*txnOutput, error) {
	if s.PortForwards == nil {
		return nil, portForwardsUnavailable()
	}
	return portForwardResult(s.PortForwards.Remove(ctx, in.ID))
}
