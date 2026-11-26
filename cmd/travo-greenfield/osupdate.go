package main

import (
	"context"
	"net/http"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/osupdate"
)

// newOpenWrtUpdates wires the report-only OpenWrt check; mock mode answers the captured index.
func newOpenWrtUpdates(deps app.Deps) api.OpenWrtUpdates {
	pkgs, ok := deps.Packages.(osupdate.Packages)
	if !ok {
		return nil
	}
	s := &osupdate.Service{Ubus: deps.Device, Routes: deps.Device, Packages: pkgs, Now: time.Now,
		Fetch: osupdate.HTTPFetch(&http.Client{Timeout: 15 * time.Second})}
	if deps.Mock {
		s.Fetch = func(context.Context, string) ([]byte, error) { return mock.VersionsIndex, nil }
	}
	return s
}
