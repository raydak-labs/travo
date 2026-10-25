package api

import (
	"context"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/system"
)

type packageUpgradeInput struct {
	Body *models.PackageUpgradeRequest
}

func registerUpdates(api *API, svc *system.UpdateService) {
	get(api, "/api/v1/system/updates", "GetUpdates", "Result of the last update check",
		func(context.Context, *none) (*Body[models.UpdateStatus], error) {
			return reply(svc.Status())
		}, describe("A check runs in the background shortly after start and daily; this does not "+
			"contact the internet."))
	post(api, "/api/v1/system/updates/check", "CheckUpdates", "Check for OpenWrt and package updates now",
		func(context.Context, *none) (*Body[models.UpdateStatus], error) {
			return reply(svc.Check())
		}, describe("Fetches the OpenWrt release list and refreshes the package index; takes a few "+
			"seconds."))
	post(api, "/api/v1/system/updates/packages/upgrade", "UpgradePackagesStream",
		"Upgrade packages, streaming the package manager's output",
		func(ctx context.Context, in *packageUpgradeInput) (*Stream, error) {
			var pkgs []string
			if in.Body != nil {
				pkgs = in.Body.Packages
			}
			return streamAction(ctx, func(logFn func(string)) error { return svc.Upgrade(pkgs, logFn) }), nil
		}, ndjson, describe("Without a body, or with an empty list, every package of the last check "+
			"is upgraded. Only names from that list are accepted."))
}
