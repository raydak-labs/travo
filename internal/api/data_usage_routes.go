package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/datausage"
)

// DataUsage is usage per uplink, the monthly budget and the uptime log (internal/datausage).
type DataUsage interface {
	View(context.Context) (datausage.View, error)
	SetBudget(context.Context, datausage.Budget) (datausage.View, error)
	ResetBudget(context.Context) (datausage.View, error)
}

type dataUsageOutput struct{ Body datausage.View }

type dataBudgetInput struct{ Body datausage.Budget }

func (s *server) registerDataUsage(a huma.API) {
	const path = "/api/v1/network/usage"
	register(s, false, op("getDataUsage", http.MethodGet, path,
		"Data usage per uplink (today, billing period, days), budget state and uptime log", "network", 503), a, s.getDataUsage)
	put := op("setDataBudget", http.MethodPut, path+"/budget",
		"Replace the monthly data budget (warns only, never cuts off)", "network", 400, 409, 503)
	put.MaxBodyBytes = 1024
	register(s, false, put, a, s.setDataBudget)
	register(s, false, op("resetDataBudget", http.MethodDelete, path+"/budget",
		"Remove the data budget file (the explicit way out of an unreadable one)", "network", 503), a, s.resetDataBudget)
}

func dataUsageUnavailable() error {
	return fail(http.StatusServiceUnavailable, "data_usage_unavailable", datausage.ErrUnavailable.Error())
}

func dataUsageError(err error) error {
	var fe *datausage.FieldError
	switch {
	case errors.As(err, &fe):
		return fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, datausage.ErrUnreadable):
		return fail(http.StatusConflict, "data_budget_unreadable", datausage.ErrUnreadable.Error())
	}
	return dataUsageUnavailable()
}

func (s *server) getDataUsage(ctx context.Context, _ *struct{}) (*dataUsageOutput, error) {
	if s.DataUsage == nil {
		return nil, dataUsageUnavailable()
	}
	return dataUsageResult(s.DataUsage.View(ctx))
}

func (s *server) setDataBudget(ctx context.Context, in *dataBudgetInput) (*dataUsageOutput, error) {
	if s.DataUsage == nil {
		return nil, dataUsageUnavailable()
	}
	return dataUsageResult(s.DataUsage.SetBudget(ctx, in.Body))
}

func (s *server) resetDataBudget(ctx context.Context, _ *struct{}) (*dataUsageOutput, error) {
	if s.DataUsage == nil {
		return nil, dataUsageUnavailable()
	}
	return dataUsageResult(s.DataUsage.ResetBudget(ctx))
}

func dataUsageResult(v datausage.View, err error) (*dataUsageOutput, error) {
	if err != nil {
		return nil, dataUsageError(err)
	}
	return &dataUsageOutput{Body: v}, nil
}
