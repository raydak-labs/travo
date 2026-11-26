package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/usbtether"
)

type usbTetherFake struct {
	st       usbtether.Status
	err      error
	tx       txn.Status
	setErr   error
	req      usbtether.Request
	phone    usbtether.Phone
	did      bool
	driveErr error
}

func (f *usbTetherFake) Status(context.Context) (usbtether.Status, error) { return f.st, f.err }
func (f *usbTetherFake) Set(_ context.Context, r usbtether.Request) (txn.Status, error) {
	f.req = r
	return f.tx, f.setErr
}
func (f *usbTetherFake) InstallDrivers(_ context.Context, p usbtether.Phone) (bool, error) {
	f.phone = p
	return f.did, f.driveErr
}

func TestUSBTetherStatusRoute(t *testing.T) {
	fake := &usbTetherFake{st: usbtether.Status{Links: []usbtether.Link{}, Drivers: []usbtether.Driver{}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.USBTether = fake })
	cookie, _ := e.login()
	got := e.do(req{path: "/api/v1/usbtether", cookie: cookie})
	want := `{"links":[],"drivers":[],"configured":false,"device":"","in_wan_zone":false,"up":false,"ipv4":"","install_blocked":false}`
	if got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != want {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	fake.err = errors.New("boom")
	if got := e.do(req{path: "/api/v1/usbtether", cookie: cookie}); got.Code != http.StatusServiceUnavailable || errCode(t, got) != "usbtether_unavailable" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if got := e.do(req{path: "/api/v1/usbtether"}); got.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", got.Code)
	}
}

func TestUSBTetherUnwiredIs503(t *testing.T) {
	e := newEnvWithDashboard(t, nil, nil, nil)
	cookie, _ := e.login()
	if got := e.do(req{path: "/api/v1/usbtether", cookie: cookie}); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}

func TestUSBTetherSetRoute(t *testing.T) {
	fake := &usbTetherFake{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.USBTether = fake })
	cookie, csrf := e.login()
	post := func(body string) *httptest.ResponseRecorder {
		return e.do(req{method: "POST", path: "/api/v1/usbtether", body: body, cookie: cookie, csrf: csrf})
	}
	if got := post(`{}`); got.Code != http.StatusBadRequest {
		t.Fatalf("missing enabled: %d %s", got.Code, got.Body)
	}
	if got := post(`{"enabled":true,"device":"usb0;reboot"}`); got.Code != http.StatusBadRequest {
		t.Fatalf("bad device: %d %s", got.Code, got.Body)
	}
	fake.tx = txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", Started: time.Unix(0, 0).UTC(), Summary: "Enable USB tethering uplink on usb0", WindowSeconds: 60}, State: txn.Pending}
	if got := post(`{"enabled":true,"device":"usb0"}`); got.Code != http.StatusAccepted || !strings.Contains(got.Body.String(), `"state":"pending"`) || fake.req != (usbtether.Request{Enabled: true, Device: "usb0"}) {
		t.Fatalf("%d %s %+v", got.Code, got.Body, fake.req)
	}
	fake.tx = txn.Status{}
	for err, code := range map[error]string{
		usbtether.ErrNoDevice:      "usbtether_no_device",
		usbtether.ErrAmbiguous:     "usbtether_ambiguous",
		usbtether.ErrUnsupported:   "usbtether_unsupported",
		txn.ErrActive:              "transaction_pending",
		txn.ErrNoChanges:           "usbtether_no_change",
		errors.New("uci exploded"): "usbtether_unavailable",
	} {
		fake.setErr = err
		if got := post(`{"enabled":false}`); errCode(t, got) != code {
			t.Fatalf("%v: %d %s", err, got.Code, got.Body)
		}
	}
}

func TestUSBTetherDriversRoute(t *testing.T) {
	fake := &usbTetherFake{did: true}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.USBTether = fake })
	cookie, csrf := e.login()
	post := func(body string) *httptest.ResponseRecorder {
		return e.do(req{method: "POST", path: "/api/v1/usbtether/drivers", body: body, cookie: cookie, csrf: csrf})
	}
	if got := post(`{"phone":"nokia"}`); got.Code != http.StatusBadRequest {
		t.Fatalf("bad phone: %d %s", got.Code, got.Body)
	}
	if got := post(`{"phone":"ios"}`); got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != `{"installed":true}` || fake.phone != usbtether.IOS {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	for err, code := range map[error]string{
		usbtether.ErrBusy:        "usbtether_install_busy",
		errors.New("apk failed"): "usbtether_install_failed",
	} {
		fake.driveErr = err
		if got := post(`{"phone":"android"}`); errCode(t, got) != code {
			t.Fatalf("%v: %d %s", err, got.Code, got.Body)
		}
	}
}
