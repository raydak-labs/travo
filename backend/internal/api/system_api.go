package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/system"
)

// maxRestoreUploadBytes bounds the uploaded restore archive. /tmp is a tmpfs
// on the router, so an unbounded upload is a way to fill RAM.
const maxRestoreUploadBytes = 32 << 20 // 32 MiB

// RebootRequired acknowledges a change that takes effect after a reboot.
type RebootRequired struct {
	Status         string `json:"status" example:"ok"`
	RebootRequired bool   `json:"reboot_required"`
}

var rebootRequired = &Body[RebootRequired]{Body: RebootRequired{Status: "ok", RebootRequired: true}}

// StatusMessage is an acknowledgement with a human-readable message.
type StatusMessage struct {
	Status  string `json:"status" example:"ok"`
	Message string `json:"message"`
}

// FirmwareUpgradeResult names the image that is being flashed.
type FirmwareUpgradeResult struct {
	Status           string   `json:"status" example:"ok"`
	Message          string   `json:"message"`
	Model            string   `json:"model"`
	SupportedDevices []string `json:"supported_devices"`
}

// TimeSyncRequest carries the browser clock.
type TimeSyncRequest struct {
	ClientTimeMs int64 `json:"client_time_ms"`
}

// TimeSyncResult reports whether the clock was moved.
type TimeSyncResult struct {
	Synced bool   `json:"synced"`
	Reason string `json:"reason,omitempty"`
	SetTo  string `json:"set_to,omitempty"`
}

type logsQuery struct {
	Service string `query:"service"`
	Level   string `query:"level"`
}

type sinceQuery struct {
	Since string `query:"since" doc:"Unix seconds; only newer points are returned"`
}

type indexParam struct {
	Index string `path:"index"`
}

func registerSystem(api *API, d *Dependencies) {
	svc := d.System

	get(api, "/api/v1/system/info", "GetSystemInfo", "Board, firmware and uptime",
		func(context.Context, *none) (*Body[models.SystemInfo], error) {
			return replyOrFail(svc.GetSystemInfo())
		})
	get(api, "/api/v1/system/stats", "GetSystemStats", "CPU, memory, load and storage",
		func(context.Context, *none) (*Body[models.SystemStats], error) {
			return replyOrFail(svc.GetSystemStats())
		})
	get(api, "/api/v1/system/stats/history", "GetStatsHistory", "Recent system stats samples",
		func(_ context.Context, in *sinceQuery) (*Body[[]system.StatsHistoryPoint], error) {
			if in.Since == "" {
				return reply(d.StatsHistory.GetHistory())
			}
			since, err := strconv.ParseInt(in.Since, 10, 64)
			if err != nil {
				return nil, badRequest("invalid since parameter")
			}
			return reply(d.StatsHistory.GetHistorySince(since))
		})
	get(api, "/api/v1/system/logs", "GetSystemLogs", "System log, filtered by service and level",
		func(_ context.Context, in *logsQuery) (*Body[models.LogResponse], error) {
			return replyOrFail(svc.GetLogs(in.Service, in.Level))
		})
	get(api, "/api/v1/system/logs/kernel", "GetKernelLogs", "Kernel log",
		func(context.Context, *none) (*Body[models.LogResponse], error) {
			return replyOrFail(svc.GetKernelLogs())
		})

	post(api, "/api/v1/system/reboot", "Reboot", "Reboot the router",
		func(context.Context, *none) (*Body[Status], error) { return okOrFail(svc.Reboot()) })
	post(api, "/api/v1/system/shutdown", "Shutdown", "Power the router off",
		func(context.Context, *none) (*Body[Status], error) { return okOrFail(svc.Shutdown()) })
	post(api, "/api/v1/system/factory-reset", "FactoryReset", "Erase the configuration and reboot",
		func(context.Context, *none) (*Body[Status], error) { return okOrFail(svc.FactoryReset()) })

	put(api, "/api/v1/system/hostname", "SetHostname", "Set the hostname (applies after reboot)",
		strict(func(_ context.Context, req models.SetHostnameRequest) (*Body[RebootRequired], error) {
			if req.Hostname == "" {
				return nil, badRequest("hostname is required")
			}
			if err := svc.SetHostname(req.Hostname); err != nil {
				return nil, serverError(err)
			}
			return rebootRequired, nil
		}))

	get(api, "/api/v1/system/leds", "GetLEDStatus", "LED stealth mode",
		func(context.Context, *none) (*Body[models.LEDStatus], error) { return reply(svc.GetLEDStatus()) })
	put(api, "/api/v1/system/leds", "SetLEDStealth", "Turn every LED off or back on",
		strict(func(_ context.Context, req models.SetLEDRequest) (*Body[models.LEDStatus], error) {
			if err := svc.SetLEDStealthMode(req.StealthMode); err != nil {
				return nil, serverError(err)
			}
			return reply(svc.GetLEDStatus())
		}))
	get(api, "/api/v1/system/leds/schedule", "GetLEDSchedule", "LED on/off schedule",
		func(context.Context, *none) (*Body[models.LEDSchedule], error) { return reply(svc.GetLEDSchedule()) })
	put(api, "/api/v1/system/leds/schedule", "SetLEDSchedule", "Set the LED on/off schedule",
		strict(func(_ context.Context, req models.LEDSchedule) (*Body[models.LEDSchedule], error) {
			// The times go into a root crontab line.
			if req.Enabled && req.OnTime != "" && req.OffTime != "" {
				if err := platform.ValidateHHMM(req.OnTime); err != nil {
					return nil, badRequest("on_time: " + err.Error())
				}
				if err := platform.ValidateHHMM(req.OffTime); err != nil {
					return nil, badRequest("off_time: " + err.Error())
				}
			}
			if err := svc.SetLEDSchedule(req); err != nil {
				return nil, serverError(err)
			}
			return reply(svc.GetLEDSchedule())
		}))

	get(api, "/api/v1/system/timezone", "GetTimezone", "Configured timezone",
		func(context.Context, *none) (*Body[models.TimezoneConfig], error) {
			return replyOrFail(svc.GetTimezone())
		})
	put(api, "/api/v1/system/timezone", "SetTimezone", "Set the timezone (applies after reboot)",
		strict(func(_ context.Context, cfg models.TimezoneConfig) (*Body[RebootRequired], error) {
			if cfg.Zonename == "" {
				return nil, badRequest("zonename is required")
			}
			if cfg.Timezone == "" {
				return nil, badRequest("timezone is required")
			}
			if err := svc.SetTimezone(cfg); err != nil {
				return nil, serverError(err)
			}
			return rebootRequired, nil
		}))
	get(api, "/api/v1/system/ntp", "GetNTPConfig", "NTP servers and client state",
		func(context.Context, *none) (*Body[models.NTPConfig], error) { return replyOrFail(svc.GetNTPConfig()) })
	put(api, "/api/v1/system/ntp", "SetNTPConfig", "Set the NTP configuration (applies after reboot)",
		strict(func(_ context.Context, cfg models.NTPConfig) (*Body[RebootRequired], error) {
			if err := svc.SetNTPConfig(cfg); err != nil {
				return nil, serverError(err)
			}
			return rebootRequired, nil
		}))
	post(api, "/api/v1/system/ntp/sync", "SyncNTP", "Sync the clock from NTP now",
		func(context.Context, *none) (*Body[Status], error) { return okOrFail(svc.SyncNTP()) })
	post(api, "/api/v1/system/time-sync", "SyncTime", "Set the clock from the browser",
		func(ctx context.Context, in *Body[TimeSyncRequest]) (*Body[TimeSyncResult], error) {
			return syncTime(ctx, d, in.Body)
		}, public, describe("Unauthenticated callers are rate limited and only allowed while the "+
			"router clock is implausible (before the build time); authenticated callers may always sync."))

	get(api, "/api/v1/system/setup-complete", "GetSetupComplete", "Whether the setup wizard finished",
		func(context.Context, *none) (*Body[models.SetupStatus], error) { return reply(svc.GetSetupComplete()) })
	post(api, "/api/v1/system/setup-complete", "SetSetupComplete", "Mark the setup wizard finished",
		func(context.Context, *none) (*Body[Status], error) { return okOrFail(svc.SetSetupComplete()) })

	get(api, "/api/v1/system/alerts", "GetAlerts", "Active system alerts",
		func(context.Context, *none) (*Body[models.AlertsResponse], error) {
			return reply(models.AlertsResponse{Alerts: d.Alerts.GetAlerts()})
		})
	get(api, "/api/v1/system/alert-thresholds", "GetAlertThresholds", "Alert thresholds",
		func(context.Context, *none) (*Body[models.AlertThresholds], error) {
			return reply(d.Alerts.GetAlertThresholds())
		})
	put(api, "/api/v1/system/alert-thresholds", "SetAlertThresholds", "Set the alert thresholds",
		strict(func(_ context.Context, t models.AlertThresholds) (*Body[Status], error) {
			if err := system.ValidateAlertThresholds(t); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			if err := d.Alerts.SetAlertThresholds(t); err != nil {
				return nil, serverError(err)
			}
			return statusOK, nil
		}))

	get(api, "/api/v1/system/ssh-keys", "GetSSHKeys", "Authorized SSH keys",
		func(context.Context, *none) (*Body[models.SSHKeysResponse], error) {
			return replyOrFail(svc.GetSSHKeys())
		})
	post(api, "/api/v1/system/ssh-keys", "AddSSHKey", "Authorize an SSH public key",
		func(_ context.Context, in *Body[models.AddSSHKeyRequest]) (*Body[Status], error) {
			if err := svc.AddSSHKey(in.Body.Key); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return statusOK, nil
		}, created)
	del(api, "/api/v1/system/ssh-keys/{index}", "DeleteSSHKey", "Remove an authorized SSH key",
		func(_ context.Context, in *indexParam) (*Body[Status], error) {
			index, err := strconv.Atoi(in.Index)
			if err != nil || index < 0 {
				return nil, badRequest("invalid index")
			}
			if err := svc.DeleteSSHKey(index); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return statusOK, nil
		})

	post(api, "/api/v1/system/speed-test", "RunSpeedTest", "Measure WAN throughput",
		func(context.Context, *none) (*Body[models.SpeedTestResult], error) {
			return replyOrFail(svc.RunSpeedTest())
		})
	get(api, "/api/v1/system/speedtest-service", "GetSpeedtestServiceStatus", "Ookla speedtest CLI state",
		func(context.Context, *none) (*Body[models.SpeedtestService], error) {
			return replyOrFail(d.Speedtest.GetSpeedtestServiceStatus())
		})
	post(api, "/api/v1/system/speedtest-service/install", "InstallSpeedtestCLI", "Install the speedtest CLI",
		func(context.Context, *none) (*Body[Status], error) {
			return okOrFail(d.Speedtest.InstallSpeedtestCLI())
		})
	post(api, "/api/v1/system/speedtest-service/uninstall", "UninstallSpeedtestCLI", "Remove the speedtest CLI",
		func(context.Context, *none) (*Body[Status], error) {
			return okOrFail(d.Speedtest.UninstallSpeedtestCLI())
		})
	post(api, "/api/v1/system/speedtest-service/run", "RunSpeedtestCLI", "Run the speedtest CLI",
		func(context.Context, *none) (*Body[models.SpeedTestResult], error) {
			return replyOrFail(d.Speedtest.RunSpeedtestCLI())
		})

	get(api, "/api/v1/system/buttons", "GetButtons", "Hardware buttons and their actions",
		func(context.Context, *none) (*Body[[]models.HardwareButton], error) {
			return replyOrFail(svc.GetHardwareButtons())
		})
	put(api, "/api/v1/system/button-actions", "SetButtonActions", "Set the hardware button actions",
		strict(func(_ context.Context, req models.ButtonActionsRequest) (*Body[Status], error) {
			if err := svc.SetButtonActions(req.Buttons); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return statusOK, nil
		}))

	get(api, "/api/v1/system/backup", "DownloadBackup", "Download a configuration backup (tar.gz)",
		func(ctx context.Context, _ *none) (*Stream, error) {
			path, err := svc.CreateBackup()
			if err != nil {
				return nil, serverError(err)
			}
			return &Stream{Write: func() {
				defer func() { _ = os.Remove(path) }()
				c := fiberCtx(ctx)
				c.Set("Content-Disposition", "attachment; filename=openwrt-backup.tar.gz")
				_ = c.SendFile(path)
			}}, nil
		})
	post(api, "/api/v1/system/restore", "RestoreBackup", "Restore a configuration backup",
		func(ctx context.Context, _ *none) (*Body[StatusMessage], error) {
			return restoreBackup(ctx, svc)
		}, upload("backup"))
	post(api, "/api/v1/system/firmware/upgrade", "UpgradeFirmware", "Flash a sysupgrade image",
		func(ctx context.Context, _ *none) (*Body[FirmwareUpgradeResult], error) {
			return upgradeFirmware(ctx, svc)
		}, upload("firmware"), formFlag("keep_settings", "keep the configuration across the flash", true),
		describe("The image's OpenWrt metadata is checked against this board before the flash starts."))
}

// restoreBackup saves the upload to a unique temp path (concurrent restores
// must not share a file) and hands it to RestoreBackup, which validates the
// archive before sysupgrade runs.
func restoreBackup(ctx context.Context, svc *system.Service) (*Body[StatusMessage], error) {
	c := fiberCtx(ctx)
	file, err := c.FormFile("backup")
	if err != nil {
		return nil, badRequest("backup file is required")
	}
	ct := file.Header.Get("Content-Type")
	if ct != "" && ct != "application/gzip" && ct != "application/x-gzip" && ct != "application/octet-stream" {
		return nil, badRequest("invalid file type, expected tar.gz")
	}
	if file.Size > maxRestoreUploadBytes {
		return nil, badRequest(fmt.Sprintf("backup file is too large (%d bytes, limit %d)", file.Size, maxRestoreUploadBytes))
	}
	f, err := os.CreateTemp("", "restore-upload-*.tar.gz")
	if err != nil {
		return nil, serverError(fmt.Errorf("creating temp file: %w", err))
	}
	tmpPath := f.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := f.Close(); err != nil {
		return nil, serverError(fmt.Errorf("creating temp file: %w", err))
	}
	if err := c.SaveFile(file, tmpPath); err != nil {
		return nil, serverError(fmt.Errorf("saving uploaded file: %w", err))
	}
	if err := svc.RestoreBackup(tmpPath); err != nil {
		if errors.Is(err, system.ErrInvalidBackupArchive) {
			return nil, badRequest(err.Error())
		}
		return nil, serverError(err)
	}
	return reply(StatusMessage{Status: "ok", Message: "Configuration restored. Reboot to apply changes."})
}

func upgradeFirmware(ctx context.Context, svc *system.Service) (*Body[FirmwareUpgradeResult], error) {
	c := fiberCtx(ctx)
	file, err := c.FormFile("firmware")
	if err != nil {
		return nil, badRequest("firmware file is required")
	}
	if !strings.HasSuffix(file.Filename, ".bin") {
		return nil, badRequest("invalid file type, expected .bin firmware image")
	}
	f, err := file.Open()
	if err != nil {
		return nil, errorf(http.StatusInternalServerError, "failed to read uploaded file")
	}
	defer func() { _ = f.Close() }()
	meta, err := svc.UpgradeFirmware(f, c.FormValue("keep_settings", "true") == "true")
	if err != nil {
		if errors.Is(err, system.ErrInvalidFirmware) {
			return nil, badRequest(err.Error())
		}
		return nil, serverError(err)
	}
	return reply(FirmwareUpgradeResult{
		Status:           "ok",
		Message:          "Firmware upgrade initiated. Device will reboot.",
		Model:            meta.Model,
		SupportedDevices: meta.SupportedDevices,
	})
}

// defaultSetSystemTime sets the system clock and persists it to the hardware
// clock (best-effort).
func defaultSetSystemTime(epochSec int64) error {
	if err := execx.Run(execx.Quick, "date", "-s", fmt.Sprintf("@%d", epochSec)); err != nil {
		return err
	}
	_ = execx.Run(execx.Quick, "hwclock", "-w")
	return nil
}

// syncTime applies the browser clock when it is more than a minute off. An
// unauthenticated caller is rate limited, refused once the clock is plausible
// (TimeSyncGate), and clamped to the plausible window.
func syncTime(ctx context.Context, d *Dependencies, req TimeSyncRequest) (*Body[TimeSyncResult], error) {
	if req.ClientTimeMs <= 0 {
		return nil, badRequest("client_time_ms is required")
	}
	ip := fiberCtx(ctx).IP()
	authorized := false
	if token := bearerToken(ctx); token != "" {
		authorized = d.Auth.ValidateToken(token) == nil
	}
	if !authorized {
		if d.TimeSyncLimiter != nil {
			if !d.TimeSyncLimiter.Allow(ip) {
				return nil, errorf(http.StatusTooManyRequests, "too many time-sync attempts")
			}
			d.TimeSyncLimiter.Record(ip)
		}
		plausible := !time.Now().Before(d.TimeSyncMinPlausible)
		if d.TimeSyncGate != nil {
			plausible = d.TimeSyncGate.UnauthBlocked(time.Now(), d.TimeSyncMinPlausible)
		}
		if plausible {
			return nil, errorf(http.StatusForbidden, "system clock is plausible; authentication required to change time")
		}
	}

	clientTime := time.UnixMilli(req.ClientTimeMs)
	if !authorized {
		if err := validateClientTimeWindow(clientTime, d.TimeSyncMinPlausible); err != nil {
			return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
		}
	}
	skew := time.Until(clientTime)
	if skew < 0 {
		skew = -skew
	}
	if skew < 60*time.Second {
		return reply(TimeSyncResult{Synced: false, Reason: "clock already accurate"})
	}
	setTime := d.TimeSyncSetTime
	if setTime == nil {
		setTime = defaultSetSystemTime
	}
	if err := setTime(clientTime.Unix()); err != nil {
		return nil, errorf(http.StatusInternalServerError, "failed to set system time")
	}
	if d.TimeSyncGate != nil {
		d.TimeSyncGate.NoteClockSet()
	}
	return reply(TimeSyncResult{Synced: true, SetTo: clientTime.UTC().Format(time.RFC3339)})
}
