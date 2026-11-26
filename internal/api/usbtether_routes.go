package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/usbtether"
)

// USBTether reads and changes the USB tethering uplink (internal/usbtether).
type USBTether interface {
	Status(context.Context) (usbtether.Status, error)
	InstallDrivers(context.Context, usbtether.Phone) (bool, error)
	Set(context.Context, usbtether.Request) (txn.Status, error)
}

type usbTetherOutput struct {
	Body usbtether.Status
}

type usbTetherSetInput struct {
	Body struct {
		Enabled *bool  `json:"enabled" required:"true"`
		Device  string `json:"device,omitempty" maxLength:"15" pattern:"^[A-Za-z0-9_.-]*$" doc:"Tether link to bind; omitted picks the only detected one"`
	}
}

type usbTetherDriversInput struct {
	Body struct {
		Phone string `json:"phone" enum:"android,ios"`
	}
}

type usbTetherDriversOutput struct {
	Body struct {
		Installed bool `json:"installed" doc:"Packages were installed; false when all were present"`
	}
}

func (s *server) registerUSBTether(a huma.API) {
	register(s, false, op("getUSBTether", http.MethodGet, "/api/v1/usbtether", "Tethered phones, drivers and the USB uplink state", "uplinks", 503), a, s.getUSBTether)
	set := op("setUSBTether", http.MethodPost, "/api/v1/usbtether", "Enable or disable the USB tethering uplink (guarded transaction)", "uplinks", 400, 409, 503)
	set.DefaultStatus = http.StatusAccepted
	set.MaxBodyBytes = 1024
	register(s, false, set, a, s.setUSBTether)
	drivers := op("installUSBTetherDrivers", http.MethodPost, "/api/v1/usbtether/drivers", "Install the tethering drivers for a phone type (guarded)", "uplinks", 400, 409, 503)
	drivers.MaxBodyBytes = 1024
	register(s, false, drivers, a, s.installUSBTetherDrivers)
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

func (s *server) setUSBTether(ctx context.Context, in *usbTetherSetInput) (*txnOutput, error) {
	if in.Body.Enabled == nil {
		return nil, fail(http.StatusBadRequest, "validation", "Enabled must be a boolean")
	}
	if s.USBTether == nil {
		return nil, usbTetherUnavailable()
	}
	st, err := s.USBTether.Set(ctx, usbtether.Request{Enabled: *in.Body.Enabled, Device: in.Body.Device})
	if st.ID != "" {
		switch st.State {
		case txn.Pending, txn.RolledBack, txn.Restoring, txn.RollbackFailed:
			return txnResponse(st), nil
		}
	}
	switch {
	case errors.Is(err, usbtether.ErrNoDevice):
		return nil, fail(http.StatusConflict, "usbtether_no_device", "No tethered phone was detected")
	case errors.Is(err, usbtether.ErrAmbiguous):
		return nil, fail(http.StatusConflict, "usbtether_ambiguous", "More than one tethered phone; choose one")
	case errors.Is(err, usbtether.ErrUnsupported):
		return nil, fail(http.StatusConflict, "usbtether_unsupported", usbtether.ErrUnsupported.Error())
	case errors.Is(err, txn.ErrActive):
		return nil, fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(http.StatusConflict, "usbtether_no_change", "The USB uplink is already in that state")
	default:
		return nil, usbTetherUnavailable()
	}
}

func (s *server) installUSBTetherDrivers(ctx context.Context, in *usbTetherDriversInput) (*usbTetherDriversOutput, error) {
	if s.USBTether == nil {
		return nil, usbTetherUnavailable()
	}
	did, err := s.USBTether.InstallDrivers(ctx, usbtether.Phone(in.Body.Phone))
	switch {
	case err == nil:
		out := &usbTetherDriversOutput{}
		out.Body.Installed = did
		return out, nil
	case errors.Is(err, usbtether.ErrUnknownPhone):
		return nil, fail(http.StatusBadRequest, "validation", "Unknown phone type")
	case errors.Is(err, usbtether.ErrNoSpace):
		return nil, fail(http.StatusConflict, "usbtether_no_space", usbtether.ErrNoSpace.Error())
	case errors.Is(err, usbtether.ErrBusy), errors.Is(err, guard.ErrActive):
		return nil, fail(http.StatusConflict, "usbtether_install_busy", usbtether.ErrBusy.Error())
	default:
		return nil, fail(http.StatusServiceUnavailable, "usbtether_install_failed", "The driver install did not finish; installing again retries it")
	}
}

func usbTetherUnavailable() error {
	return fail(http.StatusServiceUnavailable, "usbtether_unavailable", usbtether.ErrUnavailable.Error())
}
