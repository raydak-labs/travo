package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/wol"
)

// WakeOnLAN broadcasts a magic packet on the LAN bridges (internal/wol).
type WakeOnLAN interface {
	Wake(ctx context.Context, mac, iface string) (wol.Wake, error)
}

type wolInput struct {
	Body struct {
		MAC       string `json:"mac" minLength:"17" maxLength:"17" doc:"Unicast MAC, colon or hyphen separated"`
		Interface string `json:"interface,omitempty" maxLength:"15" doc:"netifd LAN interface (lan, guest); all up LAN bridges when absent"`
	}
}

type wolOutput struct{ Body wol.Wake }

func (s *server) registerWakeOnLAN(a huma.API) {
	o := op("wakeOnLan", http.MethodPost, "/api/v1/network/wol",
		"Send a Wake-on-LAN magic packet as a UDP broadcast on the LAN bridges", "network", 400, 409, 429, 503)
	o.MaxBodyBytes = 256
	register(s, false, o, a, s.wakeOnLAN)
}

func (s *server) wakeOnLAN(ctx context.Context, in *wolInput) (*wolOutput, error) {
	if s.WakeOnLAN == nil {
		return nil, fail(http.StatusServiceUnavailable, "wol_unavailable", "wake-on-lan is unavailable")
	}
	// Each request broadcasts on every LAN bridge: bounded per caller.
	if err := s.limited("wol:" + infoOf(ctx).ip); err != nil {
		return nil, err
	}
	res, err := s.WakeOnLAN.Wake(ctx, in.Body.MAC, in.Body.Interface)
	switch {
	case err == nil:
		return &wolOutput{Body: res}, nil
	case errors.Is(err, wol.ErrInvalidMAC):
		return nil, fail(http.StatusBadRequest, "invalid_mac", "mac must be a unicast Ethernet address")
	case errors.Is(err, wol.ErrInvalidInterface):
		return nil, fail(http.StatusBadRequest, "invalid_interface", "interface is not an up LAN bridge")
	case errors.Is(err, wol.ErrBusy):
		return nil, fail(http.StatusConflict, "wol_busy", "another wake request is running")
	}
	slog.Warn("wake-on-lan", "err", err)
	return nil, fail(http.StatusServiceUnavailable, "wol_unavailable", "wake-on-lan is unavailable")
}
