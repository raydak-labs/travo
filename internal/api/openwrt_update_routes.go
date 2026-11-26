package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/osupdate"
)

// OpenWrtUpdates reports newer OpenWrt releases and upgradable packages (internal/osupdate).
// It never flashes: LuCI attended sysupgrade does (device-facts §10).
type OpenWrtUpdates interface {
	Last() osupdate.OpenWrtCheck
	Check(ctx context.Context, refresh bool) (osupdate.OpenWrtCheck, error)
}

type openWrtUpdateOutput struct{ Body osupdate.OpenWrtCheck }

type openWrtCheckInput struct {
	Body struct {
		RefreshLists bool `json:"refresh_lists,omitempty" doc:"Refresh the package lists first (written to /tmp only)"`
	}
}

func (s *server) registerOpenWrtUpdates(a huma.API) {
	const path = "/api/v1/system/openwrt"
	register(s, false, op("getOpenWrtUpdates", http.MethodGet, path, "Last OpenWrt release and package check", "system", 503), a, s.getOpenWrtUpdates)
	check := op("checkOpenWrtUpdates", http.MethodPost, path+"/check", "Check for OpenWrt releases and upgradable packages now", "system", 503)
	check.MaxBodyBytes = 1024
	register(s, false, check, a, s.checkOpenWrtUpdates)
}

func (s *server) getOpenWrtUpdates(context.Context, *struct{}) (*openWrtUpdateOutput, error) {
	if s.OpenWrtUpdates == nil {
		return nil, fail(http.StatusServiceUnavailable, "update_check_unavailable", "the OpenWrt update check is unavailable")
	}
	return &openWrtUpdateOutput{Body: s.OpenWrtUpdates.Last()}, nil
}

func (s *server) checkOpenWrtUpdates(ctx context.Context, in *openWrtCheckInput) (*openWrtUpdateOutput, error) {
	if s.OpenWrtUpdates == nil {
		return nil, fail(http.StatusServiceUnavailable, "update_check_unavailable", "the OpenWrt update check is unavailable")
	}
	r, err := s.OpenWrtUpdates.Check(ctx, in.Body.RefreshLists)
	switch {
	case err == nil:
		return &openWrtUpdateOutput{Body: r}, nil
	case errors.Is(err, osupdate.ErrOffline):
		return nil, fail(http.StatusServiceUnavailable, "update_check_offline", "the router has no Internet connection to the release server")
	}
	slog.Warn("openwrt update check", "err", err)
	return nil, fail(http.StatusServiceUnavailable, "update_check_failed", "the OpenWrt update check failed")
}
