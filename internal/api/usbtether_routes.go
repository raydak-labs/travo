package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/usbtether"
)

// USBTether reads the USB tethering uplink (internal/usbtether).
type USBTether interface {
	Status(context.Context) (usbtether.Status, error)
}

type usbTetherOutput struct {
	Body usbtether.Status
}

func (s *server) registerUSBTether(a huma.API) {
	register(s, false, op("getUSBTether", http.MethodGet, "/api/v1/usbtether", "Tethered phones, drivers and the USB uplink state", "uplinks", 503), a, s.getUSBTether)
}

func (s *server) getUSBTether(ctx context.Context, _ *struct{}) (*usbTetherOutput, error) {
	if s.USBTether == nil {
		return nil, usbTetherUnavailable()
	}
	st, err := s.USBTether.Status(ctx)
	if err != nil {
		return nil, usbTetherUnavailable()
	}
	return &usbTetherOutput{Body: st}, nil
}

func usbTetherUnavailable() error {
	return fail(http.StatusServiceUnavailable, "usbtether_unavailable", usbtether.ErrUnavailable.Error())
}
