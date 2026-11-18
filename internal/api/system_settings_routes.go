package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/syssettings"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// SystemSettings reads and writes hostname, timezone and NTP (internal/syssettings).
type SystemSettings interface {
	Get(context.Context) (syssettings.Settings, error)
	Set(context.Context, syssettings.Settings) (syssettings.Settings, error)
}

type ntpBody struct {
	Enabled bool     `json:"enabled" doc:"Keep the clock in sync with the servers (sysntpd)"`
	Servers []string `json:"servers" nullable:"false" maxItems:"8" doc:"NTP servers: host names or IP addresses, in order"`
}

type systemSettingsBody struct {
	Hostname string  `json:"hostname" maxLength:"63" doc:"Router host name (one DNS label)"`
	Zonename string  `json:"zonename" maxLength:"64" doc:"IANA time zone, e.g. Europe/Berlin; UTC is the OpenWrt default"`
	Timezone string  `json:"timezone" maxLength:"128" doc:"POSIX TZ string derived from zonename; send it back unchanged or empty"`
	NTP      ntpBody `json:"ntp"`
}

type systemSettingsOutput struct{ Body systemSettingsBody }

type systemSettingsInput struct{ Body systemSettingsBody }

type timezonesOutput struct {
	Body struct {
		Zonenames []string `json:"zonenames" nullable:"false" doc:"Selectable IANA zones, UTC first"`
	}
}

func (s *server) registerSystemSettings(a huma.API) {
	const path = "/api/v1/system/settings"
	register(s, false, op("getSystemSettings", http.MethodGet, path, "Hostname, time zone and NTP", "system", 503), a, s.getSystemSettings)
	put := op("setSystemSettings", http.MethodPut, path, "Change hostname, time zone and NTP", "system", 400, 409, 503)
	put.MaxBodyBytes = 4096
	register(s, false, put, a, s.setSystemSettings)
	register(s, false, op("getTimezones", http.MethodGet, "/api/v1/system/timezones", "Selectable time zones", "system"), a, s.getTimezones)
}

func settingsBody(v syssettings.Settings) systemSettingsBody {
	servers := v.NTP.Servers
	if servers == nil {
		servers = []string{}
	}
	return systemSettingsBody{Hostname: v.Hostname, Zonename: v.Zonename, Timezone: v.Timezone,
		NTP: ntpBody{Enabled: v.NTP.Enabled, Servers: servers}}
}

func settingsUnavailable() error {
	return fail(http.StatusServiceUnavailable, "system_settings_unavailable", "system settings are unavailable")
}

func (s *server) getSystemSettings(ctx context.Context, _ *struct{}) (*systemSettingsOutput, error) {
	if s.SystemSettings == nil {
		return nil, settingsUnavailable()
	}
	v, err := s.SystemSettings.Get(ctx)
	if err != nil {
		slog.Warn("system settings", "err", err)
		return nil, settingsUnavailable()
	}
	return &systemSettingsOutput{settingsBody(v)}, nil
}

func (s *server) setSystemSettings(ctx context.Context, in *systemSettingsInput) (*systemSettingsOutput, error) {
	if s.SystemSettings == nil {
		return nil, settingsUnavailable()
	}
	b := in.Body
	v, err := s.SystemSettings.Set(ctx, syssettings.Settings{
		Hostname: b.Hostname, Zonename: b.Zonename, Timezone: b.Timezone,
		NTP: syssettings.NTP{Enabled: b.NTP.Enabled, Servers: b.NTP.Servers},
	})
	var fe *syssettings.FieldError
	switch {
	case err == nil:
		return &systemSettingsOutput{settingsBody(v)}, nil
	case errors.As(err, &fe):
		return nil, fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, txn.ErrForeignStaged):
		return nil, fail(http.StatusConflict, "uci_busy", "another change to the system config is staged; try again")
	}
	slog.Warn("system settings write", "err", err)
	return nil, settingsUnavailable()
}

func (s *server) getTimezones(context.Context, *struct{}) (*timezonesOutput, error) {
	out := &timezonesOutput{}
	out.Body.Zonenames = syssettings.Zonenames()
	return out, nil
}
