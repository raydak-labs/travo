package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
	"github.com/blackdark-org/travo-greenfield/internal/wifiprofiles"
)

const wifiProfileCreateOperation = "createWiFiProfile"

// WiFiProfileService stores saved uplinks; Select delegates to the guarded join.
type WiFiProfileService interface {
	List(context.Context) (wifiprofiles.List, error)
	Create(context.Context, wifi.JoinRequest) (wifiprofiles.Summary, error)
	Reorder(context.Context, []string) (wifiprofiles.List, error)
	Delete(context.Context, string) (wifiprofiles.List, error)
	Select(context.Context, string) (txn.Status, error)
}

type wifiProfileID struct {
	ID string `path:"id" pattern:"^[0-9a-f]{32}$"`
}
type wifiProfileOrderInput struct {
	Body struct {
		IDs []string `json:"ids" maxItems:"32" nullable:"false"`
	}
}
type wifiProfileListOutput struct{ Body wifiprofiles.List }
type wifiProfileOutput struct{ Body wifiprofiles.Summary }

func (s *server) registerWiFiProfiles(a huma.API) {
	const path = "/api/v1/wifi/profiles"
	register(s, false, op("getWiFiProfiles", http.MethodGet, path, "List saved uplink profiles", "wifi", 503), a, s.listWiFiProfiles)
	create := op(wifiProfileCreateOperation, http.MethodPost, path, "Save an uplink profile (no live change)", "wifi", 400, 409, 422, 503)
	create.DefaultStatus = http.StatusCreated
	create.MaxBodyBytes = 4096
	register(s, false, create, a, s.createWiFiProfile)
	register(s, false, op("reorderWiFiProfiles", http.MethodPut, path+"/order", "Reorder saved uplink profiles", "wifi", 400, 503), a, s.reorderWiFiProfiles)
	register(s, false, op("deleteWiFiProfile", http.MethodDelete, path+"/{id}", "Forget a saved uplink profile (no live change)", "wifi", 400, 404, 503), a, s.deleteWiFiProfile)
	selectOp := op("selectWiFiProfile", http.MethodPost, path+"/{id}/select", "Join a saved uplink profile (guarded transaction)", "wifi", 400, 404, 409, 422, 503)
	selectOp.DefaultStatus = http.StatusAccepted
	register(s, false, selectOp, a, s.selectWiFiProfile)
}

func wifiProfileError(err error) error {
	switch {
	case errors.Is(err, wifiprofiles.ErrNotFound):
		return fail(http.StatusNotFound, "wifi_profile_not_found", "Saved uplink not found")
	case errors.Is(err, wifiprofiles.ErrExists):
		return fail(http.StatusConflict, "wifi_profile_exists", "Uplink is already saved")
	case errors.Is(err, wifiprofiles.ErrLimit):
		return fail(http.StatusConflict, "wifi_profile_limit", "Saved uplink limit reached")
	case errors.Is(err, wifiprofiles.ErrOrderInvalid):
		return fail(http.StatusBadRequest, "wifi_profile_order_invalid", "Order must list every saved uplink once")
	default:
		slog.Warn("WiFi profiles unavailable")
		return fail(http.StatusServiceUnavailable, "wifi_profiles_unavailable", "Saved uplinks are unavailable")
	}
}

func (s *server) listWiFiProfiles(ctx context.Context, _ *struct{}) (*wifiProfileListOutput, error) {
	if s.WiFiProfiles == nil {
		return nil, wifiDeviceUnavailable()
	}
	return wifiProfileList(s.WiFiProfiles.List(ctx))
}

func wifiProfileList(list wifiprofiles.List, err error) (*wifiProfileListOutput, error) {
	if err != nil {
		return nil, wifiProfileError(err)
	}
	return &wifiProfileListOutput{Body: list}, nil
}

func (s *server) createWiFiProfile(ctx context.Context, in *joinWiFiInput) (*wifiProfileOutput, error) {
	if s.WiFiProfiles == nil {
		return nil, wifiDeviceUnavailable()
	}
	summary, err := s.WiFiProfiles.Create(ctx, in.request())
	var validation wifi.ValidationError
	switch {
	case err == nil:
		return &wifiProfileOutput{Body: summary}, nil
	case errors.As(err, &validation):
		return nil, fail(http.StatusBadRequest, validation.Code, "Invalid WiFi profile")
	case errors.Is(err, wifi.ErrSecurityUnsupported):
		return nil, fail(http.StatusUnprocessableEntity, "wifi_security_unsupported", "WiFi security is unsupported")
	default:
		return nil, wifiProfileError(err)
	}
}

func (s *server) reorderWiFiProfiles(ctx context.Context, in *wifiProfileOrderInput) (*wifiProfileListOutput, error) {
	if s.WiFiProfiles == nil {
		return nil, wifiDeviceUnavailable()
	}
	return wifiProfileList(s.WiFiProfiles.Reorder(ctx, in.Body.IDs))
}

func (s *server) deleteWiFiProfile(ctx context.Context, in *wifiProfileID) (*wifiProfileListOutput, error) {
	if s.WiFiProfiles == nil {
		return nil, wifiDeviceUnavailable()
	}
	return wifiProfileList(s.WiFiProfiles.Delete(ctx, in.ID))
}

func (s *server) selectWiFiProfile(ctx context.Context, in *wifiProfileID) (*txnOutput, error) {
	if s.WiFiProfiles == nil {
		return nil, wifiDeviceUnavailable()
	}
	st, err := s.WiFiProfiles.Select(ctx, in.ID)
	if errors.Is(err, wifiprofiles.ErrNotFound) || errors.Is(err, wifiprofiles.ErrUnavailable) {
		return nil, wifiProfileError(err)
	}
	return joinResult(st, err)
}
