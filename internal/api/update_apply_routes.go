package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/blackdark-org/travo-greenfield/internal/updateapply"
)

type UpdateApplier interface {
	Status() updateapply.ApplyStatus
	Begin(context.Context, string) (*updateapply.Accepted, error)
	Recover() (updateapply.ApplyStatus, error)
}

const applyPath = "/api/v1/system/update/apply"

type applyOutput struct{ Body updateapply.ApplyStatus }
type acceptedUpdate struct {
	ContentType string `header:"Content-Type"`
	Body        func(huma.Context)
}

func (s *server) registerUpdateApply(a huma.API) {
	register(s, false, op("getSystemUpdateApply", http.MethodGet, applyPath, "Read update operation status", "system", 503), a, s.updateApplyStatus)
	operation := op("postSystemUpdateApply", http.MethodPost, applyPath, "Activate a verified release", "system", 409, 503)
	operation.DefaultStatus = http.StatusAccepted
	operation.Responses = map[string]*huma.Response{"202": {
		Description: "Update accepted",
		Content:     map[string]*huma.MediaType{"application/json": {Schema: huma.SchemaFromType(a.OpenAPI().Components.Schemas, reflect.TypeFor[updateapply.ApplyStatus]())}},
	}}
	register(s, false, operation, a, s.beginUpdate)
	register(s, false, op("postSystemUpdateRecover", http.MethodPost, "/api/v1/system/update/recover", "Retry update recovery", "system", 409, 503), a, s.recoverUpdate)
}

func updateError(err error) error {
	switch {
	case errors.Is(err, updateapply.ErrBusy):
		return fail(http.StatusConflict, "update_busy", "An update or recovery is already in progress")
	case errors.Is(err, updateapply.ErrStage):
		return fail(http.StatusConflict, "stage_required", "Verify this release before applying it")
	default:
		return fail(http.StatusServiceUnavailable, "update_unavailable", "Could not start the update operation")
	}
}

func (s *server) updateApplyStatus(context.Context, *struct{}) (*applyOutput, error) {
	if s.UpdateApply == nil {
		return nil, updateError(updateapply.ErrUnavailable)
	}
	return &applyOutput{Body: s.UpdateApply.Status()}, nil
}

func (s *server) beginUpdate(ctx context.Context, in *stageInput) (*acceptedUpdate, error) {
	if s.UpdateApply == nil {
		return nil, updateError(updateapply.ErrUnavailable)
	}
	accepted, err := s.UpdateApply.Begin(ctx, in.Body.Version)
	if err != nil {
		return nil, updateError(err)
	}
	return &acceptedUpdate{ContentType: "application/json", Body: func(c huma.Context) {
		_, w := humago.Unwrap(c)
		flushed := false
		defer func() { accepted.Finish(flushed) }()
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(accepted.Status); err != nil {
			return
		}
		flushed = http.NewResponseController(w).Flush() == nil
	}}, nil
}

func (s *server) recoverUpdate(context.Context, *struct{}) (*applyOutput, error) {
	if s.UpdateApply == nil {
		return nil, updateError(updateapply.ErrUnavailable)
	}
	status, err := s.UpdateApply.Recover()
	if err != nil {
		return nil, updateError(err)
	}
	return &applyOutput{Body: status}, nil
}
