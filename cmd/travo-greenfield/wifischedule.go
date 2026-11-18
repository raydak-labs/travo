package main

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/crontab"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/hook"
	"github.com/blackdark-org/travo-greenfield/internal/wifischedule"
)

// cronService is the one crontab writer (ADR 0024); nil when the device lacks a port.
func cronService(deps app.Deps) *crontab.Service {
	file, hasFile := deps.Device.(crontab.File)
	services, hasServices := deps.Device.(device.Services)
	if !hasFile || !hasServices || deps.Txn == nil {
		return nil
	}
	return &crontab.Service{File: file, UCI: deps.Device, Apply: deps.Txn, Services: services}
}

// startWiFiSchedule registers the stored rules' hook actions and re-renders their crontab
// block (docs/adr/0027). nil when there is no crontab writer, hook server or AP editor.
func startWiFiSchedule(ctx context.Context, deps app.Deps, hooks *hook.Server, aps api.WiFiAPEditor) *wifischedule.Service {
	cron := cronService(deps)
	if cron == nil || hooks == nil || aps == nil {
		return nil
	}
	services, _ := deps.Device.(device.Services)
	svc := &wifischedule.Service{Path: filepath.Join(deps.Paths.State, "wifi-schedule.json"), Cron: cron,
		Services: services, APs: aps, Txns: deps.Txn, Guards: deps.Guards, Register: hooks.Register}
	if err := svc.Sync(ctx); err != nil {
		slog.Warn("wifi schedule: crontab not synced", "err", err)
	}
	return svc
}
