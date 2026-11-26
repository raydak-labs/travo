package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/updatecheck"
)

type UpdateChecker interface {
	Check(context.Context, bool) updatecheck.Result
}

type updateCheckInput struct {
	Body struct {
		Mode string `json:"mode" enum:"automatic,manual"`
	}
}

type updateCheckOutput struct{ Body updatecheck.Result }

func (s *server) registerUpdateCheck(a huma.API) {
	register(s, false, op("postSystemUpdateCheck", http.MethodPost, "/api/v1/system/update/check", "Check for a newer stable release", "system"), a, s.checkUpdate)
}

func (s *server) checkUpdate(ctx context.Context, in *updateCheckInput) (*updateCheckOutput, error) {
	if s.Updates == nil {
		return &updateCheckOutput{Body: updatecheck.Result{RunningVersion: s.Version, Status: "disabled"}}, nil
	}
	return &updateCheckOutput{Body: s.Updates.Check(ctx, in.Body.Mode == "manual")}, nil
}
