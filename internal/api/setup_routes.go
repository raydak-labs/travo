package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/auth"
)

type setupInput struct {
	Body struct {
		Password string `json:"password" maxLength:"256" doc:"New root password, 8 to 128 bytes"`
	}
}

func (s *server) registerSetup(a huma.API) {
	register(s, true, op("getSetupState", http.MethodGet, "/api/v1/setup", "First-run state", "setup", 503), a, s.setupState)
	register(s, true, op("postSetupPassword", http.MethodPost, "/api/v1/setup/password", "Set the first root password (only while none is set)", "setup", 400, 409, 429, 503), a, s.setupPassword)
}

func (s *server) setupState(ctx context.Context, _ *struct{}) (*struct {
	Body struct {
		PasswordSet bool `json:"password_set"`
	}
}, error) {
	set, err := s.Accounts.RootPasswordSet(ctx)
	if err != nil { // unknown is not "unset": never advertise first-run on a read failure
		return nil, unavailable("shadow", err)
	}
	out := &struct {
		Body struct {
			PasswordSet bool `json:"password_set"`
		}
	}{}
	out.Body.PasswordSet = set
	return out, nil
}

func (s *server) setupPassword(ctx context.Context, in *setupInput) (*sessionOutput, error) {
	if err := s.limited("setup:" + infoOf(ctx).ip); err != nil {
		return nil, err
	}
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	set, err := s.Accounts.RootPasswordSet(ctx) // re-checked under the lock: exactly one caller wins
	if err != nil {
		return nil, unavailable("shadow", err)
	}
	if set {
		return nil, fail(http.StatusConflict, "already_set", "a root password already exists")
	}
	if err := auth.ValidatePassword(in.Body.Password); err != nil {
		return nil, fail(http.StatusBadRequest, "invalid_password", err.Error())
	}
	if err := s.Accounts.SetRootPassword(ctx, in.Body.Password); err != nil {
		return nil, unavailable("set password", err)
	}
	return s.issue(ctx)
}
