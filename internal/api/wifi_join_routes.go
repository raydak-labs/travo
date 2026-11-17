package api

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

const wifiJoinOperation = "joinWiFi"

type joinWiFiInput struct {
	Body struct {
		SSID     string        `json:"ssid"`
		BSSID    string        `json:"bssid,omitempty"`
		Key      string        `json:"key,omitempty" writeOnly:"true"`
		Security wifi.Security `json:"security" enum:"open,wpa2,wpa2_wpa3,other"`
		// Only a join that moves the AP off the uplink radio can need it (invariants §2).
		AcknowledgeLockout bool `json:"acknowledge_lockout,omitempty" doc:"Join even if moving the access point may disconnect the caller"`
	}
}

func (s *server) joinWiFi(ctx context.Context, in *joinWiFiInput) (*txnOutput, error) {
	if s.WiFiJoin == nil {
		return nil, wifiDeviceUnavailable()
	}
	req := in.request()
	req.Acknowledge = in.Body.AcknowledgeLockout
	req.Caller, _ = netip.ParseAddr(infoOf(ctx).ip)
	return joinResult(s.WiFiJoin.Join(ctx, req))
}

func (in *joinWiFiInput) request() wifi.JoinRequest {
	b := in.Body
	return wifi.JoinRequest{SSID: b.SSID, BSSID: b.BSSID, Key: b.Key, Security: b.Security}
}

// joinResult maps a join outcome. A returned rolled-back transaction is authoritative
// and answers 202 even with an error.
func joinResult(st txn.Status, err error) (*txnOutput, error) {
	if err == nil || (st.ID != "" && (st.State == txn.RolledBack || st.State == txn.Restoring || st.State == txn.RollbackFailed)) {
		return txnResponse(st), nil
	}
	var validation wifi.ValidationError
	switch {
	case errors.As(err, &validation):
		return nil, fail(http.StatusBadRequest, validation.Code, "Invalid WiFi join request")
	case errors.Is(err, wifi.ErrSecurityUnsupported):
		return nil, fail(http.StatusUnprocessableEntity, "wifi_security_unsupported", "WiFi security is unsupported")
	case errors.Is(err, wifi.ErrNetworkNotFound):
		return nil, fail(http.StatusConflict, "wifi_network_not_found", "Network was not found in the scan")
	case errors.Is(err, wifi.ErrNoFreeRadio):
		return nil, fail(http.StatusConflict, "wifi_no_free_radio", "No free band-compatible uplink radio")
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
