package models

// ServiceInfo describes an installable/running service.
type ServiceInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	State       string  `json:"state"`
	Version     *string `json:"version,omitempty"`
	AutoStart   bool    `json:"auto_start"`
}

// AdGuardStatus contains AdGuard Home statistics.
type AdGuardStatus struct {
	Enabled         bool    `json:"enabled"`
	TotalQueries    int64   `json:"total_queries"`
	BlockedQueries  int64   `json:"blocked_queries"`
	BlockPercentage float64 `json:"block_percentage"`
	AvgResponseMS   float64 `json:"avg_response_ms"`
}

// AdGuardDNSStatus indicates whether AdGuard is configured as the LAN DNS.
type AdGuardDNSStatus struct {
	Enabled bool `json:"enabled"`
	DNSPort int  `json:"dns_port"`
}
