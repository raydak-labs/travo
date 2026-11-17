package models

// ServiceInfo describes an installable/running service.
type ServiceInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	State       string  `json:"state" enum:"not_installed,installed,running,stopped"`
	Version     *string `json:"version,omitempty"`
	AutoStart   bool    `json:"auto_start"`
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
	// ConfigPresent reports whether either supported init layout has a config file.
	ConfigPresent bool `json:"config_present"`
	// RepairNeeded is true when dnsmasq forwards here but config or the listener is missing.
	RepairNeeded bool `json:"repair_needed"`
}
