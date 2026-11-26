package api

import (
	"context"
	"log/slog"
	"math"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/system"
	"github.com/blackdark-org/travo-greenfield/internal/version"
)

type memoryBody struct {
	TotalBytes     uint64 `json:"total_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
}

type appBody struct {
	Version   string `json:"version" doc:"travo-greenfield build version"`
	BuildTime string `json:"build_time" doc:"Build time, RFC 3339; empty for unstamped builds"`
}

type systemInfoBody struct {
	Hostname      string     `json:"hostname"`
	Model         string     `json:"model"`
	BoardName     string     `json:"board_name"`
	OpenWrt       string     `json:"openwrt" doc:"OpenWrt release version"`
	Revision      string     `json:"revision"`
	Target        string     `json:"target"`
	Kernel        string     `json:"kernel"`
	UptimeSeconds int64      `json:"uptime_seconds"`
	Load          [3]float64 `json:"load" nullable:"false" doc:"Load averages over 1, 5 and 15 minutes (not CPU percent)"`
	Memory        memoryBody `json:"memory"`
	Storage       memoryBody `json:"storage" doc:"The overlay (root) filesystem"`
	App           appBody    `json:"app"`
}

type systemInfoOutput struct{ Body systemInfoBody }

func (s *server) registerSystem(a huma.API) {
	register(s, false, op("getSystemInfo", http.MethodGet, "/api/v1/system/info", "Device and firmware information", "system", 503), a, s.getSystemInfo)
}

func (s *server) getSystemInfo(ctx context.Context, _ *struct{}) (*systemInfoOutput, error) {
	if s.Ubus == nil {
		return nil, fail(http.StatusServiceUnavailable, "device_unavailable", "device not available")
	}
	b, i, err := system.Read(ctx, s.Ubus)
	if err != nil {
		slog.Warn("system info", "err", err)
		return nil, fail(http.StatusServiceUnavailable, "device_unavailable", "device not available")
	}
	load := i.Load
	for k := range load {
		load[k] = math.Round(load[k]*100) / 100
	}
	return &systemInfoOutput{systemInfoBody{
		Hostname: b.Hostname, Model: b.Model, BoardName: b.BoardName, OpenWrt: b.Release,
		Revision: b.Revision, Target: b.Target, Kernel: b.Kernel,
		UptimeSeconds: int64(i.Uptime.Seconds()), Load: load,
		Memory:  memoryBody{TotalBytes: i.MemTotal, AvailableBytes: i.MemAvailable},
		Storage: memoryBody{TotalBytes: i.RootTotal, AvailableBytes: i.RootAvail},
		App:     appBody{Version: version.Version, BuildTime: version.BuildTime},
	}}, nil
}
