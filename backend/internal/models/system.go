package models

// SystemInfo identifies the router hardware and firmware.
type SystemInfo struct {
	Hostname        string `json:"hostname"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	KernelVersion   string `json:"kernel_version"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
}

// SetHostnameRequest is the payload for changing the device hostname.
type SetHostnameRequest struct {
	Hostname string `json:"hostname"`
}

// LEDStatus represents the current LED state.
type LEDStatus struct {
	StealthMode bool `json:"stealth_mode"`
	LEDCount    int  `json:"led_count"`
}

// SetLEDRequest is the payload for toggling LED stealth mode.
type SetLEDRequest struct {
	StealthMode bool `json:"stealth_mode"`
}

// CpuStats contains CPU usage information.
type CpuStats struct {
	UsagePercent       float64    `json:"usage_percent"`
	Cores              int        `json:"cores"`
	TemperatureCelsius *float64   `json:"temperature_celsius,omitempty"`
	LoadAverage        [3]float64 `json:"load_average"`
}

// MemoryStats contains memory usage information.
type MemoryStats struct {
	TotalBytes   int64   `json:"total_bytes"`
	UsedBytes    int64   `json:"used_bytes"`
	FreeBytes    int64   `json:"free_bytes"`
	CachedBytes  int64   `json:"cached_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// StorageStats contains storage usage information.
type StorageStats struct {
	TotalBytes   int64   `json:"total_bytes"`
	UsedBytes    int64   `json:"used_bytes"`
	FreeBytes    int64   `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// SystemStats aggregates CPU, memory, and storage statistics.
type SystemStats struct {
	CPU     CpuStats     `json:"cpu"`
	Memory  MemoryStats  `json:"memory"`
	Storage StorageStats `json:"storage"`
}

// TimezoneConfig holds timezone configuration.
type TimezoneConfig struct {
	Zonename string `json:"zonename"` // e.g. "Europe/Berlin", "UTC"
	Timezone string `json:"timezone"` // POSIX TZ string e.g. "CET-1CEST,M3.5.0,M10.5.0/3"
}

// LogEntry represents a single log line from logread or dmesg.
type LogEntry struct {
	Line string `json:"line"`
}

// LogResponse is the response for log retrieval endpoints.
type LogResponse struct {
	Source string     `json:"source"`
	Lines  []LogEntry `json:"lines"`
	Total  int        `json:"total"`
}
