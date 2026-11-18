package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/wifischedule"
)

// WiFiSchedule is the AP on/off schedule (internal/wifischedule, docs/adr/0027).
type WiFiSchedule interface {
	Get(context.Context) (wifischedule.Rules, error)
	Put(context.Context, wifischedule.Rules) (wifischedule.Rules, error)
	Status(context.Context) wifischedule.Status
}

type wifiScheduleInput struct{ Body wifischedule.Rules }

func (s *server) registerWiFiSchedule(a huma.API) {
	const path = "/api/v1/wifi/schedule"
	register(s, false, op("getWiFiSchedule", http.MethodGet, path, "Access point on/off schedule", "wifi", 503), a, s.getWiFiSchedule)
	put := op("setWiFiSchedule", http.MethodPut, path, "Replace the access point schedule", "wifi", 400, 404, 409, 503)
	put.MaxBodyBytes = 16 << 10
	register(s, false, put, a, s.setWiFiSchedule)
	register(s, false, op("getWiFiScheduleStatus", http.MethodGet, path+"/status", "Cron state and the last scheduled toggles", "wifi", 503), a, s.getWiFiScheduleStatus)
}

func wifiScheduleError(err error) error {
	switch {
	case errors.Is(err, wifischedule.ErrInvalid):
		return fail(http.StatusBadRequest, "wifi_schedule_invalid", err.Error())
	case errors.Is(err, wifischedule.ErrAPNotFound):
		return fail(http.StatusNotFound, "wifi_ap_not_found", "Access point not found")
	case errors.Is(err, lockout.ErrLockoutRisk):
		return fail(http.StatusConflict, "wifi_lockout_risk", "The schedule turns off the last non-guest access point")
	}
	slog.Warn("wifi schedule", "err", err)
	return fail(http.StatusServiceUnavailable, "wifi_schedule_unavailable", "The WiFi schedule is unavailable")
}

func (s *server) getWiFiSchedule(ctx context.Context, _ *struct{}) (*struct{ Body wifischedule.Rules }, error) {
	if s.WiFiSchedule == nil {
		return nil, wifiScheduleError(errors.New("no schedule service"))
	}
	r, err := s.WiFiSchedule.Get(ctx)
	if err != nil {
		return nil, wifiScheduleError(err)
	}
	return &struct{ Body wifischedule.Rules }{r}, nil
}

func (s *server) setWiFiSchedule(ctx context.Context, in *wifiScheduleInput) (*struct{ Body wifischedule.Rules }, error) {
	if s.WiFiSchedule == nil {
		return nil, wifiScheduleError(errors.New("no schedule service"))
	}
	r, err := s.WiFiSchedule.Put(ctx, in.Body)
	if err != nil {
		return nil, wifiScheduleError(err)
	}
	return &struct{ Body wifischedule.Rules }{r}, nil
}

func (s *server) getWiFiScheduleStatus(ctx context.Context, _ *struct{}) (*struct{ Body wifischedule.Status }, error) {
	if s.WiFiSchedule == nil {
		return nil, wifiScheduleError(errors.New("no schedule service"))
	}
	return &struct{ Body wifischedule.Status }{s.WiFiSchedule.Status(ctx)}, nil
}
