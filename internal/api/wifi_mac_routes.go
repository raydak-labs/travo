package api

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

type wifiMACOutput struct{ Body wifi.MACPolicy }

type setMACInput struct {
	Body struct {
		Mode    wifi.MACMode `json:"mode" enum:"default,random,custom,clone" doc:"clone copies the caller's MAC from the neighbour table"`
		MACAddr string       `json:"macaddr,omitempty" doc:"Unicast MAC for custom"`
	}
}

func (s *server) registerWiFiMAC(a huma.API) {
	register(s, false, op("getWiFiUplinkMAC", http.MethodGet, "/api/v1/wifi/uplink/mac", "Uplink MAC policy and effective MAC", "wifi", 409, 503), a, s.getWiFiMAC)
	set := op("setWiFiUplinkMAC", http.MethodPost, "/api/v1/wifi/uplink/mac", "Set the uplink MAC policy (guarded transaction)", "wifi", 400, 409, 503)
	set.DefaultStatus = http.StatusAccepted
	set.MaxBodyBytes = 1024
	register(s, false, set, a, s.setWiFiMAC)
}

func (s *server) getWiFiMAC(ctx context.Context, _ *struct{}) (*wifiMACOutput, error) {
	if s.WiFiMAC == nil {
		return nil, wifiDeviceUnavailable()
	}
	policy, err := s.WiFiMAC.UplinkMAC(ctx)
	if errors.Is(err, wifi.ErrNoUplink) {
		return nil, fail(http.StatusConflict, "wifi_no_uplink", "No uplink is configured")
	}
	if err != nil {
		return nil, wifiFailure(err)
	}
	return &wifiMACOutput{Body: policy}, nil
}

func (s *server) setWiFiMAC(ctx context.Context, in *setMACInput) (*txnOutput, error) {
	if s.WiFiMAC == nil {
		return nil, wifiDeviceUnavailable()
	}
	caller, _ := netip.ParseAddr(infoOf(ctx).ip)
	st, err := s.WiFiMAC.SetUplinkMAC(ctx, wifi.MACRequest{Mode: in.Body.Mode, MAC: in.Body.MACAddr, Caller: caller})
	if err == nil || (st.ID != "" && (st.State == txn.RolledBack || st.State == txn.Restoring || st.State == txn.RollbackFailed)) {
		return txnResponse(st), nil
	}
	var validation wifi.ValidationError
	switch {
	case errors.As(err, &validation):
		return nil, fail(http.StatusBadRequest, validation.Code, "Invalid MAC policy")
	case errors.Is(err, wifi.ErrNoUplink):
		return nil, fail(http.StatusConflict, "wifi_no_uplink", "No uplink is configured")
	case errors.Is(err, wifi.ErrMACCloneUnavailable):
		return nil, fail(http.StatusConflict, "wifi_mac_clone_unavailable", "This device's MAC is not known to the router")
	case errors.Is(err, wifi.ErrTransactionPending):
		return nil, fail(http.StatusConflict, "transaction_pending", "A transaction is already pending")
	case errors.Is(err, txn.ErrNoChanges):
		return nil, fail(http.StatusConflict, "wifi_no_change", "WiFi configuration is unchanged")
	default:
		return nil, wifiFailure(err)
	}
}
