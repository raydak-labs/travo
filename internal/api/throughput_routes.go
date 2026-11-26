package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/throughput"
)

type throughputBody struct {
	Interfaces []throughput.Rate `json:"interfaces" nullable:"false"`
}

type throughputOutput struct {
	Body throughputBody
}

func (s *server) registerThroughput(a huma.API) {
	register(s, false, op("getDashboardThroughput", http.MethodGet, "/api/v1/dashboard/throughput", "Interface throughput", "dashboard", 503), a, s.getDashboardThroughput)
}

func (s *server) getDashboardThroughput(context.Context, *struct{}) (*throughputOutput, error) {
	if s.Throughput != nil {
		if rates, ok := s.Throughput.Latest(); ok {
			out := &throughputOutput{}
			out.Body.Interfaces = rates
			return out, nil
		}
	}
	return nil, fail(http.StatusServiceUnavailable, "throughput_unavailable", "Interface throughput is unavailable")
}
