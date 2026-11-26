package main

import (
	"context"
	"os"
	"time"

	"github.com/raydak-labs/travo/internal/adguard"
	"github.com/raydak-labs/travo/internal/alerts"
	"github.com/raydak-labs/travo/internal/api"
	"github.com/raydak-labs/travo/internal/app"
	"github.com/raydak-labs/travo/internal/datausage"
	"github.com/raydak-labs/travo/internal/device/facts"
	"github.com/raydak-labs/travo/internal/firmware"
	"github.com/raydak-labs/travo/internal/wifireconnect"
)

// newAlerts wires the alert sources. Mock mode has no flash or mount table to read.
func newAlerts(deps app.Deps, agh *adguard.Service, reconnect api.WiFiReconnectReader, usage *datausage.Service, fw *firmware.Service) *alerts.Service {
	src := alerts.Sources{Now: time.Now, Guards: deps.Guards.Stale}
	if !deps.Mock {
		src.OverlayFree = adguard.FreeBytes("/overlay")
		src.Mounts = func() (string, error) {
			b, err := os.ReadFile("/proc/mounts")
			return string(b), err
		}
	}
	if deps.Device != nil {
		// `system info` memory.available is /proc/meminfo MemAvailable (system/proc-meminfo.txt).
		src.MemAvailable = func(ctx context.Context) (uint64, error) {
			raw, err := deps.Device.Call(ctx, "system", "info", nil)
			if err != nil {
				return 0, err
			}
			info, err := facts.ParseSystemInfo(raw)
			return info.MemAvailable, err
		}
	}
	if agh != nil && agh.DNS != nil {
		src.FailOpen = func() (*time.Time, error) {
			st, err := agh.DNS.Status()
			return st.FailOpenSince, err
		}
	}
	if reconnect != nil {
		src.Reconnect = func(ctx context.Context) (wifireconnect.Status, error) { return reconnect.Status(ctx) }
	}
	if usage != nil {
		src.DataBudget = usage.BudgetState
	}
	if fw != nil {
		src.FirmwareCompleted = fw.Completed
	}
	return &alerts.Service{Sources: src}
}
