package api

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

type wifiGuestOutput struct{ Body wifi.GuestConfig }

type setGuestInput struct {
	Body struct {
		SSID               string `json:"ssid"`
		Key                string `json:"key,omitempty" writeOnly:"true" doc:"WPA key; required on first setup, omitted keeps the current key"`
		Enabled            bool   `json:"enabled"`
		Radio              string `json:"radio,omitempty" doc:"Radio for the guest AP; omitted keeps the current one or picks a radio without the uplink"`
		AcknowledgeLockout bool   `json:"acknowledge_lockout,omitempty" doc:"Apply even if the caller may lose its access point"`
	}
}

func (s *server) registerWiFiGuest(a huma.API) {
	register(s, false, op("getWiFiGuest", http.MethodGet, "/api/v1/wifi/guest", "Guest access point", "wifi", 503), a, s.getWiFiGuest)
	set := op("setWiFiGuest", http.MethodPost, "/api/v1/wifi/guest", "Configure the guest access point, subnet and zone (guarded transaction)", "wifi", 400, 409, 503)
	set.DefaultStatus = http.StatusAccepted
	set.MaxBodyBytes = 4096
	register(s, false, set, a, s.setWiFiGuest)
}

func (s *server) getWiFiGuest(ctx context.Context, _ *struct{}) (*wifiGuestOutput, error) {
	if s.WiFiGuest == nil {
		return nil, wifiDeviceUnavailable()
	}
	guest, err := s.WiFiGuest.Guest(ctx)
	if err != nil {
		return nil, wifiFailure(err)
	}
	return &wifiGuestOutput{Body: guest}, nil
}

func (s *server) setWiFiGuest(ctx context.Context, in *setGuestInput) (*txnOutput, error) {
	if s.WiFiGuest == nil {
		return nil, wifiDeviceUnavailable()
	}
	caller, _ := netip.ParseAddr(infoOf(ctx).ip)
	b := in.Body
	st, err := s.WiFiGuest.SetGuest(ctx, wifi.GuestRequest{SSID: b.SSID, Key: b.Key, Radio: b.Radio, Enabled: b.Enabled, Acknowledge: b.AcknowledgeLockout, Caller: caller})
	if err == nil || (st.ID != "" && (st.State == txn.RolledBack || st.State == txn.Restoring || st.State == txn.RollbackFailed)) {
		return txnResponse(st), nil
	}
	var validation wifi.ValidationError
	switch {
	case errors.As(err, &validation):
		return nil, fail(http.StatusBadRequest, validation.Code, "Invalid guest access point settings")
	case errors.Is(err, wifi.ErrGuestOverlap):
		return nil, fail(http.StatusConflict, "wifi_guest_subnet_overlap", "The guest subnet overlaps another network")
	case errors.Is(err, wifi.ErrGuestConflict):
		return nil, fail(http.StatusConflict, "wifi_guest_conflict", "An existing guest configuration is not managed here")
	case errors.Is(err, vpnkill.ErrUnsupported): // the kill switch is on but cannot cover the guest zone
		return nil, fail(http.StatusConflict, "vpn_killswitch_unsupported", vpnkill.ErrUnsupported.Error())
	case errors.Is(err, wifi.ErrNoGuestRadio):
		return nil, fail(http.StatusConflict, "wifi_no_free_radio", "No radio can carry the guest access point")
	case errors.Is(err, wifi.ErrRadioInUse):
		return nil, fail(http.StatusConflict, "wifi_radio_in_use", "The radio carries the uplink")
	case errors.Is(err, wifi.ErrTransactionPending):
		return nil, fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, lockout.ErrLockoutRisk):
		return nil, fail(http.StatusConflict, "wifi_lockout_risk", "Change would disconnect the caller")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(http.StatusConflict, "wifi_no_change", "WiFi configuration is unchanged")
	default:
		return nil, wifiFailure(err)
	}
}
