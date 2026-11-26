package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/updatestage"
)

type UpdateStager interface {
	Status() updatestage.StageStatus
	Stage(context.Context, string) (updatestage.StageStatus, error)
}

type stageInput struct {
	Body struct {
		Version string `json:"version"`
	}
}
type stageOutput struct{ Body updatestage.StageStatus }

const stagePath = "/api/v1/system/update/stage"

func (s *server) registerUpdateStage(a huma.API) {
	register(s, false, op("getSystemUpdateStage", http.MethodGet, stagePath, "Read release verification status", "system", 503), a, s.stageStatus)
	register(s, false, op("postSystemUpdateStage", http.MethodPost, stagePath, "Verify and stage a release", "system", 409, 502, 503), a, s.stageRelease)
}

func (s *server) stageStatus(context.Context, *struct{}) (*stageOutput, error) {
	if s.UpdateStage == nil {
		return nil, fail(http.StatusServiceUnavailable, "stage_unavailable", "Release verification is unavailable")
	}
	return &stageOutput{Body: s.UpdateStage.Status()}, nil
}

func (s *server) stageRelease(ctx context.Context, in *stageInput) (*stageOutput, error) {
	if s.UpdateStage == nil {
		return nil, fail(http.StatusServiceUnavailable, "stage_unavailable", "Release verification is unavailable")
	}
	out, err := s.UpdateStage.Stage(ctx, in.Body.Version)
	switch {
	case err == nil:
		return &stageOutput{Body: out}, nil
	case errors.Is(err, updatestage.ErrBusy):
		return nil, fail(http.StatusConflict, "stage_busy", "Release verification is already running")
	case errors.Is(err, updatestage.ErrChanged):
		return nil, fail(http.StatusConflict, "release_changed", "Check for updates before verifying a release")
	case errors.Is(err, updatestage.ErrUnavailable):
		return nil, fail(http.StatusServiceUnavailable, "stage_unavailable", "Release verification is unavailable")
	default:
		return nil, fail(http.StatusBadGateway, "stage_failed", "Could not verify release package")
	}
}
