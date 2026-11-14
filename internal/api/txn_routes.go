package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type txnBody struct {
	ID            string    `json:"id"`
	State         string    `json:"state" enum:"pending,confirmed,rolled_back,restoring,rollback_failed"`
	Summary       string    `json:"summary"`
	Configs       []string  `json:"configs"`
	Started       time.Time `json:"started"`
	WindowSeconds int       `json:"window_seconds" doc:"Confirm window, counted from when the change was armed"`
}

type txnOutput struct{ Body txnBody }

type txnInput struct {
	ID string `path:"id" pattern:"^[0-9a-f]{16}$" doc:"Transaction id"`
}

func (s *server) registerTxn(a huma.API) {
	register(s, false, op("getTransaction", http.MethodGet, "/api/v1/transactions/{id}", "Transaction status", "transactions", 404), a, s.txnCall(s.Txns.Get))
	register(s, false, op("confirmTransaction", http.MethodPost, "/api/v1/transactions/{id}/confirm", "Keep the applied change", "transactions", 404, 409), a, s.txnCall(s.Txns.Confirm))
	register(s, false, op("rollbackTransaction", http.MethodPost, "/api/v1/transactions/{id}/rollback", "Undo the applied change now", "transactions", 404, 409), a, s.txnCall(s.Txns.Rollback))
}

func (s *server) txnCall(f func(string) (txn.Status, error)) func(context.Context, *txnInput) (*txnOutput, error) {
	return func(_ context.Context, in *txnInput) (*txnOutput, error) {
		st, err := f(in.ID)
		switch {
		case errors.Is(err, txn.ErrNotFound):
			return nil, fail(http.StatusNotFound, "not_found", "no such transaction")
		case errors.Is(err, txn.ErrClosed):
			return nil, fail(http.StatusConflict, "closed", "transaction already finished")
		case err != nil && (st.State == txn.RollbackFailed || st.State == txn.Restoring):
			return nil, fail(http.StatusConflict, string(st.State), "rollback has not completed; the guard stays until it does")
		case err != nil:
			return nil, fail(http.StatusServiceUnavailable, "txn_unavailable", "transaction state unavailable")
		}
		return txnResponse(st), nil
	}
}

func txnResponse(st txn.Status) *txnOutput {
	cfgs := st.Configs
	if cfgs == nil {
		cfgs = []string{}
	}
	return &txnOutput{txnBody{ID: st.ID, State: string(st.State), Summary: st.Summary,
		Configs: cfgs, Started: st.Started, WindowSeconds: st.WindowSeconds}}
}
