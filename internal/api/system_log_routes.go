package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/systemlog"
)

// SystemLog answers log queries from `logread` (internal/systemlog).
type SystemLog interface {
	Query(context.Context, systemlog.Query) (systemlog.LogPage, error)
}

type systemLogInput struct {
	Lines  int    `query:"lines" minimum:"1" maximum:"1000" default:"200" doc:"Newest matching lines to return"`
	Level  string `query:"level" enum:"emerg,alert,crit,err,warn,notice,info,debug" doc:"Keep this level and more severe ones; omit for all"`
	Filter string `query:"filter" maxLength:"128" doc:"Case-insensitive text in the program or message"`
}

func (s *server) registerSystemLog(a huma.API) {
	register(s, false, op("getSystemLog", http.MethodGet, "/api/v1/system/logs", "System log, newest first", "system", 503), a, s.getSystemLog)
}

func (s *server) getSystemLog(ctx context.Context, in *systemLogInput) (*struct{ Body systemlog.LogPage }, error) {
	if s.SystemLog == nil {
		return nil, fail(http.StatusServiceUnavailable, "logs_unavailable", "the system log is unavailable")
	}
	r, err := s.SystemLog.Query(ctx, systemlog.Query{Lines: in.Lines, MinLevel: in.Level, Contains: in.Filter})
	if err != nil {
		slog.Warn("system log", "err", err)
		return nil, fail(http.StatusServiceUnavailable, "logs_unavailable", "the system log is unavailable")
	}
	return &struct{ Body systemlog.LogPage }{r}, nil
}
