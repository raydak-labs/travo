package facts

import (
	"encoding/json"
	"fmt"
	"strings"
)

// IwinfoInfo is the association identity from `ubus call iwinfo info` on a STA.
// A disconnected STA still reports its last BSSID but no SSID.
type IwinfoInfo struct {
	SSID  string
	BSSID string
}

// ParseIwinfoInfo keeps only the fields that identify an association.
func ParseIwinfoInfo(b []byte) (IwinfoInfo, error) {
	var raw *struct {
		Mode  *string `json:"mode"`
		SSID  string  `json:"ssid"`
		BSSID string  `json:"bssid"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return IwinfoInfo{}, fmt.Errorf("iwinfo info: %w", err)
	}
	if raw == nil || raw.Mode == nil {
		return IwinfoInfo{}, fmt.Errorf("iwinfo info: %w: need mode", ErrShape)
	}
	return IwinfoInfo{SSID: raw.SSID, BSSID: strings.ToLower(raw.BSSID)}, nil
}
