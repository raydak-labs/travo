package models

// SystemInfo identifies the router hardware and firmware.
type SystemInfo struct {
	Hostname        string `json:"hostname"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	KernelVersion   string `json:"kernel_version"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
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
