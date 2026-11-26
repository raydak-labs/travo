package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/buttons"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// Buttons lists the hardware buttons and maps the slide switch to an action (internal/buttons).
type Buttons interface {
	Get(context.Context) (buttons.Status, error)
	Set(context.Context, string, buttons.Action) (buttons.Status, error)
}

type buttonActionInput struct {
	Name string `path:"name" maxLength:"64" doc:"Devicetree key node"`
	Body struct {
		Action buttons.Action `json:"action" enum:"none,vpn,guest_wifi" doc:"Pressed position turns it on, released turns it off"`
	}
}

type buttonsOutput struct{ Body buttons.Status }

func (s *server) registerButtons(a huma.API) {
	const path = "/api/v1/system/buttons"
	register(s, false, op("getButtons", http.MethodGet, path, "Hardware buttons and the slide switch action", "system", 503), a, s.getButtons)
	put := op("setButtonAction", http.MethodPut, path+"/{name}", "Map the slide switch to an action", "system", 400, 404, 409, 503)
	put.MaxBodyBytes = 256
	register(s, false, put, a, s.setButtonAction)
}

func buttonsError(err error) error {
	switch {
	case errors.Is(err, buttons.ErrInvalid):
		return fail(http.StatusBadRequest, "button_action_invalid", err.Error())
	case errors.Is(err, buttons.ErrNotConfigurable):
		return fail(http.StatusBadRequest, "button_not_configurable", "This button keeps its stock behaviour")
	case errors.Is(err, buttons.ErrNotFound):
		return fail(http.StatusNotFound, "button_not_found", err.Error())
	case errors.Is(err, txn.ErrForeignStaged):
		return fail(http.StatusConflict, "uci_busy", "another change to the travo-greenfield config is staged; try again")
	}
	slog.Warn("buttons", "err", err)
	return fail(http.StatusServiceUnavailable, "buttons_unavailable", "The buttons are unavailable")
}

func (s *server) getButtons(ctx context.Context, _ *struct{}) (*buttonsOutput, error) {
	if s.Buttons == nil {
		return nil, buttonsError(errors.New("no button service"))
	}
	v, err := s.Buttons.Get(ctx)
	if err != nil {
		return nil, buttonsError(err)
	}
	return &buttonsOutput{v}, nil
}

func (s *server) setButtonAction(ctx context.Context, in *buttonActionInput) (*buttonsOutput, error) {
	if s.Buttons == nil {
		return nil, buttonsError(errors.New("no button service"))
	}
	v, err := s.Buttons.Set(ctx, in.Name, in.Body.Action)
	if err != nil {
		return nil, buttonsError(err)
	}
	return &buttonsOutput{v}, nil
}
