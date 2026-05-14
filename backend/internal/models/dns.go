package models

// DNSMode describes the active DNS resolver configuration.
type DNSMode struct {
	Mode           string `json:"mode"`        // "default", "adguard-forwarding", "adguard-direct"
	Description    string `json:"description"`
	AdGuardRunning bool   `json:"adguard_running"`
	DNSBypassed    bool   `json:"dns_bypassed"`
}
