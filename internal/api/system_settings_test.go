package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/syssettings"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeSettings struct {
	cur    syssettings.Settings
	setErr error
	sets   int
}

func (f *fakeSettings) Get(context.Context) (syssettings.Settings, error) { return f.cur, nil }

func (f *fakeSettings) Set(_ context.Context, v syssettings.Settings) (syssettings.Settings, error) {
	f.sets++
	if f.setErr != nil {
		return syssettings.Settings{}, f.setErr
	}
	if _, err := syssettings.Validate(v, f.cur); err != nil {
		return syssettings.Settings{}, err
	}
	f.cur = v
	return v, nil
}

func settingsEnv(t *testing.T, f *fakeSettings) (*env, string, string) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		if f != nil {
			d.SystemSettings = f
		}
	})
	cookie, csrf := e.login()
	return e, cookie, csrf
}

const settingsJSON = `{"hostname":"travo","zonename":"Europe/Berlin","timezone":"","ntp":{"enabled":true,"servers":["pool.ntp.org"]}}`

func TestSystemSettingsRoundTrip(t *testing.T) {
	f := &fakeSettings{cur: syssettings.Settings{Hostname: "OpenWrt", Zonename: "UTC", Timezone: "GMT0", NTP: syssettings.NTP{Servers: []string{}}}}
	e, cookie, csrf := settingsEnv(t, f)
	rec := e.do(req{method: "GET", path: "/api/v1/system/settings", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["hostname"] != "OpenWrt" || fmt.Sprint(got["ntp"]) != "map[enabled:false servers:[]]" {
		t.Fatalf("got %v", got)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/system/settings", body: settingsJSON, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || f.cur.Hostname != "travo" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestSystemSettingsPutIsStrict(t *testing.T) {
	f := &fakeSettings{cur: syssettings.Settings{Zonename: "UTC", Timezone: "GMT0"}}
	e, cookie, csrf := settingsEnv(t, f)
	for name, body := range map[string]string{
		"unknown field":    `{"hostname":"a","zonename":"UTC","timezone":"","ntp":{"enabled":false,"servers":[]},"x":1}`,
		"missing ntp":      `{"hostname":"a","zonename":"UTC","timezone":""}`,
		"null servers":     `{"hostname":"a","zonename":"UTC","timezone":"","ntp":{"enabled":false,"servers":null}}`,
		"bad hostname":     `{"hostname":"a b","zonename":"UTC","timezone":"","ntp":{"enabled":false,"servers":[]}}`,
		"unknown zone":     `{"hostname":"a","zonename":"Mars/X","timezone":"","ntp":{"enabled":false,"servers":[]}}`,
		"trailing data":    settingsJSON + `{}`,
		"case-variant key": `{"Hostname":"a","zonename":"UTC","timezone":"","ntp":{"enabled":false,"servers":[]}}`,
	} {
		rec := e.do(req{method: "PUT", path: "/api/v1/system/settings", body: body, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_request" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func TestSystemSettingsErrors(t *testing.T) {
	f := &fakeSettings{setErr: fmt.Errorf("txn: system: %w", txn.ErrForeignStaged)}
	e, cookie, csrf := settingsEnv(t, f)
	rec := e.do(req{method: "PUT", path: "/api/v1/system/settings", body: settingsJSON, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "uci_busy" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	e, cookie, csrf = settingsEnv(t, nil)
	rec = e.do(req{method: "GET", path: "/api/v1/system/settings", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "system_settings_unavailable" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestTimezonesListsUTCFirst(t *testing.T) {
	e, cookie, csrf := settingsEnv(t, nil)
	rec := e.do(req{method: "GET", path: "/api/v1/system/timezones", cookie: cookie, csrf: csrf})
	var got struct{ Zonenames []string }
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusOK || len(got.Zonenames) < 300 || got.Zonenames[0] != "UTC" {
		t.Fatalf("%d %d names", rec.Code, len(got.Zonenames))
	}
}
