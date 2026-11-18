package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/failover"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Failover reads and changes multi-WAN failover (internal/failover).
type Failover interface {
	Status(context.Context) (failover.Status, error)
	Enable(context.Context, []string) (txn.Status, error)
	Disable(context.Context) error
}

type failoverOutput struct {
	Body failover.Status
}

type failoverEnableInput struct {
	Body struct {
		Uplinks []string `json:"uplinks" minItems:"2" maxItems:"3" enum:"wan,wwan,usbtether" doc:"Uplinks in priority order (wan, wwan, usbtether)"`
	}
}

func (s *server) registerFailover(a huma.API) {
	register(s, false, op("getFailover", http.MethodGet, "/api/v1/failover", "Multi-WAN failover settings and state", "uplinks", 503), a, s.getFailover)
	enable := op("enableFailover", http.MethodPost, "/api/v1/failover", "Enable failover with an uplink priority list (guarded transaction)", "uplinks", 400, 409, 503)
	enable.DefaultStatus = http.StatusAccepted
	enable.MaxBodyBytes = 1024
	register(s, false, enable, a, s.enableFailover)
	register(s, false, op("disableFailover", http.MethodDelete, "/api/v1/failover", "Disable failover (stops mwan3, keeps its config)", "uplinks", 409, 503), a, s.disableFailover)
}

func (s *server) getFailover(ctx context.Context, _ *struct{}) (*failoverOutput, error) {
	if s.Failover == nil {
		return nil, failoverUnavailable()
	}
	st, err := s.Failover.Status(ctx)
	if err != nil {
		return nil, failoverUnavailable()
	}
	return &failoverOutput{Body: st}, nil
}

func (s *server) enableFailover(ctx context.Context, in *failoverEnableInput) (*txnOutput, error) {
	if s.Failover == nil {
		return nil, failoverUnavailable()
	}
	st, err := s.Failover.Enable(ctx, in.Body.Uplinks)
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	switch {
	case errors.Is(err, failover.ErrInvalid):
		return nil, fail(http.StatusBadRequest, "validation", failover.ErrInvalid.Error())
	case errors.Is(err, failover.ErrOffline):
		return nil, fail(http.StatusConflict, "failover_offline", failover.ErrOffline.Error())
	case errors.Is(err, failover.ErrConflict):
		return nil, fail(http.StatusConflict, "failover_conflict", failover.ErrConflict.Error())
	case errors.Is(err, txn.ErrActive):
		return nil, fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(http.StatusConflict, "failover_no_change", "Failover already uses these settings")
	default:
		return nil, fail(http.StatusServiceUnavailable, "failover_failed", "Failover could not be enabled; enabling again retries it")
	}
}

func (s *server) disableFailover(ctx context.Context, _ *struct{}) (*struct{ Body okBody }, error) {
	if s.Failover == nil {
		return nil, failoverUnavailable()
	}
	if err := s.Failover.Disable(ctx); err != nil {
		if errors.Is(err, failover.ErrBusy) {
			return nil, fail(http.StatusConflict, "failover_busy", failover.ErrBusy.Error())
		}
		return nil, fail(http.StatusServiceUnavailable, "failover_failed", "Failover could not be disabled; disabling again retries it")
	}
	out := &struct{ Body okBody }{}
	out.Body.Status = "ok"
	return out, nil
}

func failoverUnavailable() error {
	return fail(http.StatusServiceUnavailable, "failover_unavailable", failover.ErrUnavailable.Error())
}
