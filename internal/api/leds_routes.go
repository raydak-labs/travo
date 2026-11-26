package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/leds"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// LEDs reads the router LEDs, holds them off and owns their night schedule (internal/leds).
type LEDs interface {
	Get(context.Context) (leds.Status, error)
	SetOff(context.Context, bool) (leds.Status, error)
	SetLED(context.Context, string, bool) (leds.Status, error)
	SetSchedule(context.Context, leds.Schedule) (leds.Status, error)
}

type ledsOffInput struct {
	Body struct {
		Off bool `json:"off" doc:"true holds every LED off; false returns them to the firmware default"`
	}
}

type ledInput struct {
	Body struct {
		Name string `json:"name" maxLength:"64" doc:"Kernel LED name from GET /system/leds"`
		Off  bool   `json:"off" doc:"true holds this LED off; false returns it to the firmware default"`
	}
}

type ledsScheduleInput struct{ Body leds.Schedule }

type ledsOutput struct{ Body leds.Status }

func (s *server) registerLEDs(a huma.API) {
	const path = "/api/v1/system/leds"
	register(s, false, op("getLEDs", http.MethodGet, path, "Router LEDs and their night schedule", "system", 503), a, s.getLEDs)
	put := op("setLEDs", http.MethodPut, path, "Hold the LEDs off or return them to default", "system", 409, 503)
	put.MaxBodyBytes = 1024
	register(s, false, put, a, s.setLEDs)
	one := op("setLED", http.MethodPut, path+"/led", "Hold one LED off or return it to default", "system", 400, 409, 503)
	one.MaxBodyBytes = 1024
	register(s, false, one, a, s.setLED)
	sch := op("setLEDSchedule", http.MethodPut, path+"/schedule", "Replace the LED night schedule", "system", 400, 503)
	sch.MaxBodyBytes = 1024
	register(s, false, sch, a, s.setLEDSchedule)
}

func ledsError(err error) error {
	switch {
	case errors.Is(err, leds.ErrInvalid):
		return fail(http.StatusBadRequest, "led_schedule_invalid", err.Error())
	case errors.Is(err, leds.ErrUnknownLED):
		return fail(http.StatusBadRequest, "led_unknown", "No controllable LED by that name")
	case errors.Is(err, txn.ErrForeignStaged):
		return fail(http.StatusConflict, "uci_busy", "another change to the system config is staged; try again")
	}
	slog.Warn("leds", "err", err)
	return fail(http.StatusServiceUnavailable, "leds_unavailable", "The LEDs are unavailable")
}

func (s *server) getLEDs(ctx context.Context, _ *struct{}) (*ledsOutput, error) {
	if s.LEDs == nil {
		return nil, ledsError(errors.New("no LED service"))
	}
	v, err := s.LEDs.Get(ctx)
	if err != nil {
		return nil, ledsError(err)
	}
	return &ledsOutput{v}, nil
}

func (s *server) setLEDs(ctx context.Context, in *ledsOffInput) (*ledsOutput, error) {
	if s.LEDs == nil {
		return nil, ledsError(errors.New("no LED service"))
	}
	v, err := s.LEDs.SetOff(ctx, in.Body.Off)
	if err != nil {
		return nil, ledsError(err)
	}
	return &ledsOutput{v}, nil
}

func (s *server) setLED(ctx context.Context, in *ledInput) (*ledsOutput, error) {
	if s.LEDs == nil {
		return nil, ledsError(errors.New("no LED service"))
	}
	v, err := s.LEDs.SetLED(ctx, in.Body.Name, in.Body.Off)
	if err != nil {
		return nil, ledsError(err)
	}
	return &ledsOutput{v}, nil
}

func (s *server) setLEDSchedule(ctx context.Context, in *ledsScheduleInput) (*ledsOutput, error) {
	if s.LEDs == nil {
		return nil, ledsError(errors.New("no LED service"))
	}
	v, err := s.LEDs.SetSchedule(ctx, in.Body)
	if err != nil {
		return nil, ledsError(err)
	}
	return &ledsOutput{v}, nil
}
