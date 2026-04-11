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

// WifiHealthStatus enumerates the health check result.
//
//	"ok"      — STA is associated, wwan has an IP, or user is in AP mode (no STA expected).
//	"warning" — STA is associated but wwan has no lease yet.
//	"error"   — wwan is bound to a different device than the associated STA (config mismatch).
type WifiHealth struct {
	Status string `json:"status"`
	Issues []string `json:"issues"`
	// RepeaterSameRadioAPSTA is true when repeater mode, multi-radio, allow_ap_on_sta is off,
	// and an enabled AP shares the STA wifi-device (fragile on many chipsets).
	RepeaterSameRadioAPSTA bool `json:"repeater_same_radio_ap_sta"`
	STA    *struct {
		Ifname     string `json:"ifname"`
		SSID       string `json:"ssid"`
		Associated bool   `json:"associated"`
	} `json:"sta,omitempty"`
	Wwan *struct {
		Device    string `json:"device"`
		Up        bool   `json:"up"`
		IPAddress string `json:"ip_address"`
	} `json:"wwan,omitempty"`
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

// APConfigUpdate is the request body for PUT /wifi/ap/:section.
// When Enabled is nil, UCI disabled is left unchanged (repeater credential sync after radio split).
type APConfigUpdate struct {
	SSID       string `json:"ssid"`
	Encryption string `json:"encryption"`
	Key        string `json:"key"`
	Enabled    *bool  `json:"enabled,omitempty"`
}

// RepeaterOptions is stored in /etc/travo/repeater-options.json.
type RepeaterOptions struct {
	AllowAPOnSTARadio bool `json:"allow_ap_on_sta_radio"`
}

// BoolPtr returns a pointer to b (optional JSON fields).
func BoolPtr(b bool) *bool { p := b; return &p }

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
	// Role is the current active role of this radio: "ap", "sta", "both", or "none".
	Role string `json:"role"`
}

// RadioRoleRequest is the request body for setting a radio's role.
type RadioRoleRequest struct {
	Role string `json:"role"`
}
