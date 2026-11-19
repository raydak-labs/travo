package facts

import (
	"encoding/json"
	"fmt"
	"time"
)

// Board is `ubus call system board`: what the router is.
type Board struct {
	Hostname, Model, Kernel, BoardName, Release, Revision, Target string // Release = release.version
}

// ParseBoard parses `ubus call system board`. Model, kernel and release.version must be present.
func ParseBoard(b []byte) (Board, error) {
	var raw struct {
		Hostname  string `json:"hostname"`
		Model     string `json:"model"`
		Kernel    string `json:"kernel"`
		BoardName string `json:"board_name"`
		Release   struct {
			Version  string `json:"version"`
			Revision string `json:"revision"`
			Target   string `json:"target"`
		} `json:"release"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Board{}, fmt.Errorf("system board: %w", err)
	}
	if raw.Model == "" || raw.Kernel == "" || raw.Release.Version == "" {
		return Board{}, fmt.Errorf("system board: %w: model, kernel or release.version missing", ErrShape)
	}
	return Board{Hostname: raw.Hostname, Model: raw.Model, Kernel: raw.Kernel, BoardName: raw.BoardName,
		Release: raw.Release.Version, Revision: raw.Release.Revision, Target: raw.Release.Target}, nil
}

// FirmwareURL is `release.firmware_url` of `ubus call system board`, the release server.
func FirmwareURL(b []byte) (string, error) {
	var raw struct {
		Release struct {
			FirmwareURL string `json:"firmware_url"`
		} `json:"release"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", fmt.Errorf("system board: %w", err)
	}
	if raw.Release.FirmwareURL == "" {
		return "", fmt.Errorf("system board: %w: release.firmware_url missing", ErrShape)
	}
	return raw.Release.FirmwareURL, nil
}

// SystemInfo is `ubus call system info`. The router's localtime is ignored: no RTC, so it is
// not truth (device-facts §13).
type SystemInfo struct {
	Uptime                 time.Duration
	Load                   [3]float64 // 1, 5, 15 minutes; already divided by 65536
	MemTotal, MemAvailable uint64     // bytes
	RootTotal, RootAvail   uint64     // bytes (ubus reports KiB)
	TmpAvail               uint64     // bytes free on the /tmp tmpfs; 0 when the reply has no "tmp"
}

// ubusLoadScale is the fixed-point divisor of the load averages (SI_LOAD_SHIFT 16).
const ubusLoadScale = 65536

// ParseSystemInfo parses `ubus call system info`.
func ParseSystemInfo(b []byte) (SystemInfo, error) {
	var raw struct {
		Uptime *int64    `json:"uptime"`
		Load   []float64 `json:"load"`
		Memory struct {
			Total     *uint64 `json:"total"`
			Available *uint64 `json:"available"`
		} `json:"memory"`
		Root struct {
			Total *uint64 `json:"total"`
			Avail *uint64 `json:"avail"`
		} `json:"root"`
		Tmp struct {
			Avail uint64 `json:"avail"`
		} `json:"tmp"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return SystemInfo{}, fmt.Errorf("system info: %w", err)
	}
	switch {
	case raw.Uptime == nil || *raw.Uptime < 0:
		return SystemInfo{}, fmt.Errorf("system info: %w: uptime", ErrShape)
	case len(raw.Load) != 3:
		return SystemInfo{}, fmt.Errorf("system info: %w: load needs 3 values", ErrShape)
	case raw.Memory.Total == nil || raw.Memory.Available == nil:
		return SystemInfo{}, fmt.Errorf("system info: %w: memory.total or memory.available", ErrShape)
	case *raw.Memory.Available > *raw.Memory.Total:
		return SystemInfo{}, fmt.Errorf("system info: %w: memory available exceeds total", ErrShape)
	case raw.Root.Total == nil || raw.Root.Avail == nil:
		return SystemInfo{}, fmt.Errorf("system info: %w: root.total or root.avail", ErrShape)
	case *raw.Root.Avail > *raw.Root.Total:
		return SystemInfo{}, fmt.Errorf("system info: %w: root avail exceeds total", ErrShape)
	}
	si := SystemInfo{
		Uptime:   time.Duration(*raw.Uptime) * time.Second,
		MemTotal: *raw.Memory.Total, MemAvailable: *raw.Memory.Available,
		RootTotal: *raw.Root.Total * 1024, RootAvail: *raw.Root.Avail * 1024,
		TmpAvail: raw.Tmp.Avail * 1024,
	}
	for i, l := range raw.Load {
		si.Load[i] = l / ubusLoadScale
	}
	return si, nil
}
