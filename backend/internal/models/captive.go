package models

// CaptivePortalStatus represents captive portal detection results.
type CaptivePortalStatus struct {
	Detected         bool    `json:"detected"`
	PortalURL        *string `json:"portal_url,omitempty"`
	CanReachInternet bool    `json:"can_reach_internet"`
}

type CaptiveAutoAcceptResult struct {
	OK               bool    `json:"ok"`
	Message          string  `json:"message,omitempty"`
	Detected         bool    `json:"detected"`
	CanReachInternet bool    `json:"can_reach_internet"`
	PortalURL        *string `json:"portal_url,omitempty"`
	Attempts         int     `json:"attempts,omitempty"`
}
