package api

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/danielgtaylor/huma/v2"

	"github.com/raydak-labs/travo/internal/lockout"
	"github.com/raydak-labs/travo/internal/txn"
	"github.com/raydak-labs/travo/internal/wifi"
)

type wifiAPsOutput struct {
	Body struct {
		APs []wifi.APConfig `json:"aps" nullable:"false"`
	}
}

type updateAPInput struct {
	Section string `path:"section" doc:"UCI wifi-iface section of the access point"`
	Body    struct {
		SSID               string `json:"ssid"`
		Key                string `json:"key,omitempty" writeOnly:"true" doc:"New WPA key; omitted keeps the current key"`
		Enabled            bool   `json:"enabled"`
		AcknowledgeLockout bool   `json:"acknowledge_lockout,omitempty" doc:"Apply even if the caller may lose its access point"`
	}
}

type setupAPsInput struct {
	Body struct {
		SSID string `json:"ssid" doc:"Name of the router's WiFi on every band"`
		Key  string `json:"key" writeOnly:"true" doc:"WPA key, 8 to 63 printable ASCII characters"`
	}
}

func (s *server) registerWiFiAPs(a huma.API) {
	register(s, false, op("getWiFiAccessPoints", http.MethodGet, "/api/v1/wifi/aps", "Configured access points", "wifi", 503), a, s.getWiFiAPs)
	update := op("updateWiFiAccessPoint", http.MethodPost, "/api/v1/wifi/aps/{section}", "Edit one access point (guarded transaction)", "wifi", 400, 404, 409, 503)
	update.DefaultStatus = http.StatusAccepted
	update.MaxBodyBytes = 4096
	register(s, false, update, a, s.updateWiFiAP)
	setup := op("setupWiFiAccessPoints", http.MethodPost, "/api/v1/wifi/setup", "Turn on the router's WiFi: every main access point with one name and key (guarded transaction)", "wifi", 400, 404, 409, 503)
	setup.DefaultStatus = http.StatusAccepted
	setup.MaxBodyBytes = 4096
	register(s, false, setup, a, s.setupWiFiAPs)
}

func (s *server) setupWiFiAPs(ctx context.Context, in *setupAPsInput) (*txnOutput, error) {
	if s.WiFiAPs == nil {
		return nil, wifiDeviceUnavailable()
	}
	st, err := s.WiFiAPs.SetupAPs(ctx, wifi.SetupRequest{SSID: in.Body.SSID, Key: in.Body.Key})
	return apTxnResult(st, err)
}

func (s *server) getWiFiAPs(ctx context.Context, _ *struct{}) (*wifiAPsOutput, error) {
	if s.WiFiAPs == nil {
		return nil, wifiDeviceUnavailable()
	}
	aps, err := s.WiFiAPs.AccessPoints(ctx)
	if err != nil {
		return nil, wifiFailure(err)
	}
	out := &wifiAPsOutput{}
	out.Body.APs = aps
	return out, nil
}

func (s *server) updateWiFiAP(ctx context.Context, in *updateAPInput) (*txnOutput, error) {
	if s.WiFiAPs == nil {
		return nil, wifiDeviceUnavailable()
	}
	caller, _ := netip.ParseAddr(infoOf(ctx).ip)
	b := in.Body
	st, err := s.WiFiAPs.UpdateAP(ctx, wifi.APRequest{Section: in.Section, SSID: b.SSID, Key: b.Key, Enabled: b.Enabled, Acknowledge: b.AcknowledgeLockout, Caller: caller})
	return apTxnResult(st, err)
}

// apTxnResult maps an AP transaction's outcome; a rolled-back transaction is a 202 with its state.
func apTxnResult(st txn.Status, err error) (*txnOutput, error) {
	if err == nil || (st.ID != "" && (st.State == txn.RolledBack || st.State == txn.Restoring || st.State == txn.RollbackFailed)) {
		return txnResponse(st), nil
	}
	var validation wifi.ValidationError
	switch {
	case errors.As(err, &validation):
		return nil, fail(http.StatusBadRequest, validation.Code, "Invalid access point settings")
	case errors.Is(err, wifi.ErrAPNotFound):
		return nil, fail(http.StatusNotFound, "wifi_ap_not_found", "Access point not found")
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
