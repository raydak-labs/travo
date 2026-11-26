package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/statshistory"
)

// StatsHistory is the persisted traffic, clients and uptime history (docs/adr/0026).
type StatsHistory interface {
	statshistory.Reader
	Reset() error
}

type statsHistoryInput struct {
	Hours int `query:"hours" minimum:"1" maximum:"168" default:"24" doc:"How far back, in hours of router wall clock"`
}

type statsHistoryBody struct {
	IntervalSeconds int64                    `json:"interval_seconds"`
	RetentionHours  int64                    `json:"retention_hours"`
	Persistence     statshistory.Persistence `json:"persistence" enum:"ok,degraded" doc:"degraded: the file was unreadable or a flush failed; history is RAM-only until reset"`
	Points          []statshistory.Point     `json:"points" nullable:"false"`
}

func (s *server) registerStatsHistory(a huma.API) {
	const path = "/api/v1/stats/history"
	register(s, false, op("getStatsHistory", http.MethodGet, path, "Traffic per uplink, clients and uptime history", "dashboard", 503), a, s.getStatsHistory)
	register(s, false, op("resetStatsHistory", http.MethodDelete, path, "Discard the stats history and its file", "dashboard", 503), a, s.resetStatsHistory)
}

func statsHistoryUnavailable() error {
	return fail(http.StatusServiceUnavailable, "stats_history_unavailable", "Stats history is unavailable")
}

func (s *server) getStatsHistory(_ context.Context, in *statsHistoryInput) (*struct{ Body statsHistoryBody }, error) {
	if s.StatsHistory == nil {
		return nil, statsHistoryUnavailable()
	}
	since := s.Clock.Now().Add(-time.Duration(in.Hours) * time.Hour)
	points, persistence := s.StatsHistory.Points(since)
	return &struct{ Body statsHistoryBody }{statsHistoryBody{
		IntervalSeconds: int64(statshistory.Interval / time.Second),
		RetentionHours:  int64(statshistory.Retention / time.Hour),
		Persistence:     persistence, Points: points,
	}}, nil
}

func (s *server) resetStatsHistory(context.Context, *struct{}) (*struct{ Body okBody }, error) {
	if s.StatsHistory == nil {
		return nil, statsHistoryUnavailable()
	}
	if err := s.StatsHistory.Reset(); err != nil {
		slog.Warn("stats history reset", "err", err)
		return nil, statsHistoryUnavailable()
	}
	out := &struct{ Body okBody }{}
	out.Body.Status = "ok"
	return out, nil
}
