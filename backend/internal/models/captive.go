package models

// CaptivePortalStatus represents captive portal detection results.
type CaptivePortalStatus struct {
	Detected         bool    `json:"detected"`
	PortalURL        *string `json:"portal_url,omitempty"`
	CanReachInternet bool    `json:"can_reach_internet"`
}
