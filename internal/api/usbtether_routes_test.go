package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/usbtether"
)

type usbTetherFake struct {
	st  usbtether.Status
	err error
}

func (f *usbTetherFake) Status(context.Context) (usbtether.Status, error) { return f.st, f.err }

func TestUSBTetherStatusRoute(t *testing.T) {
	fake := &usbTetherFake{st: usbtether.Status{Links: []usbtether.Link{}, Drivers: []usbtether.Driver{}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.USBTether = fake })
	cookie, _ := e.login()
	got := e.do(req{path: "/api/v1/usbtether", cookie: cookie})
	want := `{"links":[],"drivers":[],"configured":false,"device":"","in_wan_zone":false,"up":false,"ipv4":""}`
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
