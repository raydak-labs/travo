// Package alerts derives the operator alerts on read from live sources; it stores nothing.
// Sources: stale guards (invariants §1: a stuck guard is silent feature death), low overlay
// space and degraded persistence (§5), AdGuard fail-open (§8), the exhausted uplink
// reconnect cap (ADR 0014), the data budget (internal/datausage), a firmware upgrade this
// daemon run saw complete (ADR 0030), low available memory (device-facts §11). Codes are stable; the UI owns the text.
package alerts

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/datausage"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/wifireconnect"
)

// Severity orders alerts; the shell shows a banner for danger.
type Severity string

const (
	Warning Severity = "warning"
	Danger  Severity = "danger"
	// Info reports a finished event; the shell shows it in the banner like danger.
	Info Severity = "info"
)

// Codes.
const (
	CodeStaleGuard           = "stale_guard"
	CodeOverlayLow           = "overlay_low"
	CodePersistenceDegraded  = "persistence_degraded"
	CodeAdGuardFailOpen      = "adguard_fail_open"
	CodeUplinkReconnectLimit = "uplink_reconnect_exhausted"
	CodeDataBudgetWarning    = "data_budget_warning"
	CodeDataBudgetExceeded   = "data_budget_exceeded"
	CodeFirmwareCompleted    = "firmware_upgrade_completed"
	CodeMemoryLow            = "memory_low"
)

// StaleGuardAfter separates a guard of a running operation (the longest, a package install,
// finishes well within it) from one left behind by an interrupted one.
const StaleGuardAfter = 15 * time.Minute

// OverlayMinFree is the flash headroom invariants §5 and §15 keep.
const OverlayMinFree = 20 << 20

// MemoryWarnBelow and MemoryDangerBelow bound MemAvailable. The AXT1800 drifts from
// ~75 MiB after boot to 20-30 MiB over hours with userspace flat (device-facts §11);
// below 8 MiB the OOM killer is close.
const (
	MemoryWarnBelow   = 15 << 20
	MemoryDangerBelow = 8 << 20
)

// Alert is one active condition. ID is stable while the condition lasts.
type Alert struct {
	ID       string     `json:"id" doc:"Stable while the condition lasts, e.g. stale_guard:wifi-apply-in-progress"`
	Severity Severity   `json:"severity" enum:"warning,danger,info"`
	Code     string     `json:"code" enum:"stale_guard,overlay_low,persistence_degraded,adguard_fail_open,uplink_reconnect_exhausted,data_budget_warning,data_budget_exceeded,firmware_upgrade_completed,memory_low"`
	Subject  string     `json:"subject,omitempty" doc:"What the alert is about, e.g. the guard name"`
	Since    *time.Time `json:"since" nullable:"true" doc:"When the condition started; null when unknown"`
}

// AlertList is the API body.
type AlertList struct {
	Alerts []Alert `json:"alerts" nullable:"false"`
}

// Inputs are one read of every source. A nil pointer means that source was unreadable
// or is absent and yields no alert: an unreadable source is not a healthy one, but an
// alert must not claim a condition it did not observe.
type Inputs struct {
	Now         time.Time
	Guards      []guard.Info
	OverlayFree *uint64
	Mounts      *string
	FailOpen    *time.Time
	Reconnect   *wifireconnect.Status
	DataBudget  datausage.State
	// FirmwareCompleted is when this daemon run released the guard of a finished upgrade.
	FirmwareCompleted *time.Time
	// MemAvailable is /proc/meminfo MemAvailable in bytes.
	MemAvailable *uint64
}

// Derive is the pure mapping from inputs to alerts, by severity rank, then by id.
func Derive(in Inputs) []Alert {
	out := []Alert{}
	for _, g := range in.Guards {
		if !g.Started.IsZero() && in.Now.Sub(g.Started) < StaleGuardAfter {
			continue
		}
		out = append(out, Alert{ID: CodeStaleGuard + ":" + g.Name, Severity: Danger, Code: CodeStaleGuard, Subject: g.Name, Since: timePtr(g.Started)})
	}
	if in.OverlayFree != nil && *in.OverlayFree < OverlayMinFree {
		out = append(out, Alert{ID: CodeOverlayLow, Severity: Warning, Code: CodeOverlayLow})
	}
	if a, ok := memoryLow(in.MemAvailable); ok {
		out = append(out, a)
	}
	if in.Mounts != nil && Degraded(*in.Mounts) {
		out = append(out, Alert{ID: CodePersistenceDegraded, Severity: Danger, Code: CodePersistenceDegraded})
	}
	if in.FailOpen != nil {
		out = append(out, Alert{ID: CodeAdGuardFailOpen, Severity: Danger, Code: CodeAdGuardFailOpen, Since: timePtr(*in.FailOpen)})
	}
	if in.Reconnect != nil && in.Reconnect.State == wifireconnect.StateExhausted {
		out = append(out, Alert{ID: CodeUplinkReconnectLimit, Severity: Warning, Code: CodeUplinkReconnectLimit})
	}
	switch in.DataBudget {
	case datausage.StateWarning:
		out = append(out, Alert{ID: CodeDataBudgetWarning, Severity: Warning, Code: CodeDataBudgetWarning})
	case datausage.StateExceeded:
		out = append(out, Alert{ID: CodeDataBudgetExceeded, Severity: Danger, Code: CodeDataBudgetExceeded})
	}
	if in.FirmwareCompleted != nil {
		out = append(out, Alert{ID: CodeFirmwareCompleted, Severity: Info, Code: CodeFirmwareCompleted, Since: timePtr(*in.FirmwareCompleted)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := rank(out[i].Severity), rank(out[j].Severity); ri != rj {
			return ri < rj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// memoryLow is the memory_low alert when MemAvailable is under a bound; same id at both
// severities, so a deepening shortage stays one alert.
func memoryLow(avail *uint64) (Alert, bool) {
	switch {
	case avail == nil || *avail >= MemoryWarnBelow:
		return Alert{}, false
	case *avail < MemoryDangerBelow:
		return Alert{ID: CodeMemoryLow, Severity: Danger, Code: CodeMemoryLow}, true
	}
	return Alert{ID: CodeMemoryLow, Severity: Warning, Code: CodeMemoryLow}, true
}

// rank orders danger, then info (both bannered), then warning.
func rank(s Severity) int {
	switch s {
	case Danger:
		return 0
	case Info:
		return 1
	}
	return 2
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

// Degraded reads /proc/mounts: settings survive a reboot only while /overlay is a
// writable flash file system. OpenWrt falls back to a tmpfs overlay when the flash
// overlay cannot mount, and every change is then lost on reboot.
func Degraded(mounts string) bool {
	for _, line := range strings.Split(mounts, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 || f[1] != "/overlay" {
			continue
		}
		if f[2] == "tmpfs" || f[2] == "ramfs" {
			return true
		}
		for _, opt := range strings.Split(f[3], ",") {
			if opt == "ro" {
				return true
			}
		}
		return false
	}
	return false // no overlay (x86 ext4 image, dev machine): nothing to degrade
}

// Sources read the live inputs. Each may be nil when the daemon lacks it.
type Sources struct {
	Guards      func() ([]guard.Info, error)
	OverlayFree func() (uint64, error)
	Mounts      func() (string, error)
	FailOpen    func() (*time.Time, error)
	Reconnect   func(context.Context) (wifireconnect.Status, error)
	DataBudget  func(context.Context) (datausage.State, error)
	// FirmwareCompleted never fails: the value lives in the daemon's memory.
	FirmwareCompleted func() *time.Time
	MemAvailable      func(context.Context) (uint64, error)
	Now               func() time.Time
}

// Service reads every source once per call.
type Service struct{ Sources Sources }

// List never mutates. A failed source is logged and contributes nothing.
func (s *Service) List(ctx context.Context) (AlertList, error) {
	src := s.Sources
	in := Inputs{Now: src.Now()}
	if src.Guards != nil {
		if g, err := src.Guards(); err != nil {
			slog.Warn("alerts: guards unreadable", "err", err)
		} else {
			in.Guards = g
		}
	}
	if src.OverlayFree != nil {
		if free, err := src.OverlayFree(); err == nil {
			in.OverlayFree = &free
		}
	}
	if src.Mounts != nil {
		if m, err := src.Mounts(); err == nil {
			in.Mounts = &m
		}
	}
	if src.FailOpen != nil {
		if t, err := src.FailOpen(); err == nil {
			in.FailOpen = t
		}
	}
	if src.Reconnect != nil {
		if st, err := src.Reconnect(ctx); err == nil {
			in.Reconnect = &st
		}
	}
	if src.DataBudget != nil {
		if st, err := src.DataBudget(ctx); err == nil {
			in.DataBudget = st
		}
	}
	if src.FirmwareCompleted != nil {
		in.FirmwareCompleted = src.FirmwareCompleted()
	}
	if src.MemAvailable != nil {
		if avail, err := src.MemAvailable(ctx); err == nil {
			in.MemAvailable = &avail
		}
	}
	return AlertList{Alerts: Derive(in)}, nil
}
