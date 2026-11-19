package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/buttons"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type fakeButtons struct {
	st     buttons.Status
	setErr error
}

func (f *fakeButtons) Get(context.Context) (buttons.Status, error) { return f.st, nil }

func (f *fakeButtons) Set(_ context.Context, name string, a buttons.Action) (buttons.Status, error) {
	if f.setErr != nil {
		return buttons.Status{}, f.setErr
	}
	if name != "switch" {
		return buttons.Status{}, buttons.ErrNotConfigurable
	}
	f.st.Buttons[1].Action = a
	return f.st, nil
}

func TestButtonRoutes(t *testing.T) {
	f := &fakeButtons{st: buttons.Status{Buttons: []buttons.Button{
		{Name: "reset", Label: "reset", Hotplug: "reset", Action: buttons.None},
		{Name: "switch", Label: "switch", Hotplug: "BTN_0", Configurable: true, Action: buttons.None},
	}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Buttons = f })
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/system/buttons", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hotplug":"BTN_0"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/system/buttons/switch", body: `{"action":"vpn"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || f.st.Buttons[1].Action != buttons.VPN {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/system/buttons/switch", body: `{"action":"factory_reset"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown action: %d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: "PUT", path: "/api/v1/system/buttons/reset", body: `{"action":"vpn"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "button_not_configurable") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	f.setErr = txn.ErrForeignStaged
	rec = e.do(req{method: "PUT", path: "/api/v1/system/buttons/switch", body: `{"action":"none"}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "uci_busy") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestButtonRoutesUnavailable(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil, func(*Deps) {})
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/system/buttons", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "buttons_unavailable") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
