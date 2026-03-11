package models

// WifiScanResult represents a detected WiFi network.
type WifiScanResult struct {
	SSID          string `json:"ssid"`
	BSSID         string `json:"bssid"`
	Channel       int    `json:"channel"`
	SignalDBM     int    `json:"signal_dbm"`
	SignalPercent int    `json:"signal_percent"`
	Encryption    string `json:"encryption"`
	Band          string `json:"band"`
}

// WifiConnection represents the active WiFi connection.
type WifiConnection struct {
	SSID          string `json:"ssid"`
	BSSID         string `json:"bssid"`
	Mode          string `json:"mode"`
	SignalDBM     int    `json:"signal_dbm"`
	SignalPercent int    `json:"signal_percent"`
	Channel       int    `json:"channel"`
	Encryption    string `json:"encryption"`
	Band          string `json:"band"`
	IPAddress     string `json:"ip_address"`
	Connected     bool   `json:"connected"`
}

// WifiConfig is the configuration for connecting to a WiFi network.
type WifiConfig struct {
	SSID       string `json:"ssid"`
	Password   string `json:"password"`
	Encryption string `json:"encryption"`
	Mode       string `json:"mode"`
	Band       string `json:"band"`
	Hidden     bool   `json:"hidden"`
	Channel    *int   `json:"channel,omitempty"`
}

// SavedNetwork represents a saved WiFi network.
type SavedNetwork struct {
	SSID        string `json:"ssid"`
	Section     string `json:"section"`
	Encryption  string `json:"encryption"`
	Mode        string `json:"mode"`
	AutoConnect bool   `json:"auto_connect"`
	Priority    int    `json:"priority"`
}

// APConfig holds access point configuration for a radio.
type APConfig struct {
	Radio      string `json:"radio"`
	Band       string `json:"band"`
	SSID       string `json:"ssid"`
	Encryption string `json:"encryption"`
	Key        string `json:"key"`
	Enabled    bool   `json:"enabled"`
	Channel    int    `json:"channel"`
	Section    string `json:"section"`
}

// GuestWifiConfig holds the guest WiFi network configuration.
type GuestWifiConfig struct {
	Enabled    bool   `json:"enabled"`
	SSID       string `json:"ssid"`
	Encryption string `json:"encryption"`
	Key        string `json:"key"`
}

// RadioInfo describes a WiFi radio hardware device.
type RadioInfo struct {
	Name     string `json:"name"`
	Band     string `json:"band"`
	Channel  int    `json:"channel"`
	HTMode   string `json:"htmode"`
	Type     string `json:"type"`
	Disabled bool   `json:"disabled"`
}
