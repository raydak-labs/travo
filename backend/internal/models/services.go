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
	AdminURL        string  `json:"admin_url,omitempty"`
	ConfigYAMLPath  string  `json:"config_yaml_path,omitempty"`
}

// AdGuardDNSStatus indicates whether AdGuard is configured as the LAN DNS,
// plus health information about the AdGuard DNS listener path.
type AdGuardDNSStatus struct {
	Enabled bool `json:"enabled"`
	DNSPort int  `json:"dns_port"`
	// AdguardListenerReady is true when a TCP connection to AdGuard's DNS port succeeds.
	AdguardListenerReady bool `json:"adguard_listener_ready"`
	// DnsmasqForwardTarget is the current dnsmasq server option (e.g., "127.0.0.1#5353").
	DnsmasqForwardTarget string `json:"dnsmasq_forward_target"`
	// ResolverProbeOk is true when a DNS lookup via the AdGuard listener succeeds.
	ResolverProbeOk bool `json:"resolver_probe_ok"`
}
