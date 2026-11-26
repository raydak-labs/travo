package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/apitokens"
)

// APITokens is the automation token store (internal/apitokens).
type APITokens interface {
	Authenticate(raw string) (apitokens.Token, bool)
	List() ([]apitokens.Token, error)
	Create(name string, scope apitokens.Scope) (apitokens.Token, string, error)
	Delete(id string) error
}

type apiTokenList struct {
	Tokens []apitokens.Token `json:"tokens" nullable:"false"`
}

type apiTokenCreateInput struct {
	Body struct {
		Name  string          `json:"name" minLength:"1" maxLength:"256" doc:"Label shown in the token list"`
		Scope apitokens.Scope `json:"scope" enum:"read,admin" doc:"read = GET only; admin = every route except session-only ones"`
	}
}

type apiTokenCreated struct {
	apitokens.Token
	Secret string `json:"token" doc:"The bearer token; shown once, only its hash is stored"`
}

type apiTokenIDInput struct {
	ID string `path:"id" pattern:"^[0-9a-f]{32}$"`
}

func (s *server) registerAPITokens(a huma.API) {
	const path = "/api/v1/auth/tokens"
	registerSessionOnly(s, op("getAPITokens", http.MethodGet, path, "List API tokens", "auth", 503), a, s.listAPITokens)
	create := op("createAPIToken", http.MethodPost, path, "Create an API token (the secret is returned once)", "auth", 400, 409, 503)
	create.DefaultStatus = http.StatusCreated
	registerSessionOnly(s, create, a, s.createAPIToken)
	registerSessionOnly(s, op("deleteAPIToken", http.MethodDelete, path+"/{id}", "Revoke an API token", "auth", 404, 503), a, s.deleteAPIToken)
}

func apiTokenError(err error) error {
	switch {
	case errors.Is(err, apitokens.ErrInvalidName), errors.Is(err, apitokens.ErrInvalidScope):
		return fail(http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, apitokens.ErrLimit):
		return fail(http.StatusConflict, "token_limit", "token limit reached; revoke one first")
	case errors.Is(err, apitokens.ErrNotFound):
		return fail(http.StatusNotFound, "not_found", "no such API token")
	}
	return fail(http.StatusServiceUnavailable, "tokens_unavailable", "API token store is unavailable")
}

func (s *server) listAPITokens(_ context.Context, _ *struct{}) (*struct{ Body apiTokenList }, error) {
	if s.Tokens == nil {
		return nil, apiTokenError(apitokens.ErrUnavailable)
	}
	list, err := s.Tokens.List()
	if err != nil {
		return nil, apiTokenError(err)
	}
	return &struct{ Body apiTokenList }{apiTokenList{Tokens: list}}, nil
}

func (s *server) createAPIToken(_ context.Context, in *apiTokenCreateInput) (*struct{ Body apiTokenCreated }, error) {
	if s.Tokens == nil {
		return nil, apiTokenError(apitokens.ErrUnavailable)
	}
	tok, secret, err := s.Tokens.Create(in.Body.Name, in.Body.Scope)
	if err != nil {
		return nil, apiTokenError(err)
	}
	return &struct{ Body apiTokenCreated }{apiTokenCreated{Token: tok, Secret: secret}}, nil
}

func (s *server) deleteAPIToken(_ context.Context, in *apiTokenIDInput) (*struct{ Body okBody }, error) {
	if s.Tokens == nil {
		return nil, apiTokenError(apitokens.ErrUnavailable)
	}
	if err := s.Tokens.Delete(in.ID); err != nil {
		return nil, apiTokenError(err)
	}
	out := &struct{ Body okBody }{}
	out.Body.Status = "ok"
	return out, nil
}
