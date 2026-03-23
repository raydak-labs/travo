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

// LEDInfo represents a single LED on the device.
type LEDInfo struct {
	Name       string `json:"name"`
	Brightness int    `json:"brightness"`
}

// LEDStatus represents the current LED state.
type LEDStatus struct {
	StealthMode bool      `json:"stealth_mode"`
	LEDCount    int       `json:"led_count"`
	LEDs        []LEDInfo `json:"leds"`
}

// SetLEDRequest is the payload for toggling LED stealth mode.
type SetLEDRequest struct {
	StealthMode bool `json:"stealth_mode"`
}

// LEDSchedule represents a cron-based schedule for LED stealth mode.
type LEDSchedule struct {
	Enabled bool   `json:"enabled"`
	OnTime  string `json:"on_time"`  // HH:MM format
	OffTime string `json:"off_time"` // HH:MM format
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

// NetworkInterfaceStats contains network traffic counters for a single interface.
type NetworkInterfaceStats struct {
	Interface string `json:"interface"`
	RxBytes   int64  `json:"rx_bytes"`
	TxBytes   int64  `json:"tx_bytes"`
}

// SystemStats aggregates CPU, memory, storage, and network statistics.
type SystemStats struct {
	CPU     CpuStats                `json:"cpu"`
	Memory  MemoryStats             `json:"memory"`
	Storage StorageStats            `json:"storage"`
	Network []NetworkInterfaceStats `json:"network"`
}

// TimezoneConfig holds timezone configuration.
type TimezoneConfig struct {
	Zonename string `json:"zonename"` // e.g. "Europe/Berlin", "UTC"
	Timezone string `json:"timezone"` // POSIX TZ string e.g. "CET-1CEST,M3.5.0,M10.5.0/3"
}

// FirmwareUpgradeRequest holds options for a firmware upgrade.
type FirmwareUpgradeRequest struct {
	KeepSettings bool `json:"keep_settings"`
}

// NTPConfig holds NTP time synchronization configuration.
type NTPConfig struct {
	Enabled bool     `json:"enabled"`
	Servers []string `json:"servers"`
}

// LogEntry represents a single log line from logread or dmesg.
type LogEntry struct {
	Line  string `json:"line"`
	Level string `json:"level"`
}

// LogResponse is the response for log retrieval endpoints.
type LogResponse struct {
	Source string     `json:"source"`
	Lines  []LogEntry `json:"lines"`
	Total  int        `json:"total"`
}

// SetupStatus represents whether first-run setup has been completed.
type SetupStatus struct {
	Complete bool `json:"complete"`
}

// Alert represents a system alert notification.
type Alert struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // wifi_disconnect, storage_low, high_cpu, high_memory
	Message   string `json:"message"`
	Severity  string `json:"severity"`  // info, warning, critical
	Timestamp int64  `json:"timestamp"` // unix millis
}

// AlertsResponse is the response for GET /api/v1/system/alerts.
type AlertsResponse struct {
	Alerts []Alert `json:"alerts"`
}

// ButtonAction is the action to perform when a hardware button is pressed.
// Valid values: "none", "vpn_toggle", "wifi_toggle", "led_toggle", "reboot".
type ButtonAction string

const (
	ButtonActionNone       ButtonAction = "none"
	ButtonActionVPNToggle  ButtonAction = "vpn_toggle"
	ButtonActionWifiToggle ButtonAction = "wifi_toggle"
	ButtonActionLEDToggle  ButtonAction = "led_toggle"
	ButtonActionReboot     ButtonAction = "reboot"
)

// HardwareButton describes a detected hardware button and its configured action.
type HardwareButton struct {
	Name   string       `json:"name"`   // button identifier, e.g. "reset", "wps"
	Action ButtonAction `json:"action"` // configured action
}

// ButtonActionsRequest is the payload for PUT /api/v1/system/button-actions.
type ButtonActionsRequest struct {
	Buttons []HardwareButton `json:"buttons"`
}
