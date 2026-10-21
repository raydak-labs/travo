package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// adguardDNSConfig is the JSON body for POST /control/dns_config.
type adguardDNSConfig struct {
	UpstreamDNS  []string `json:"upstream_dns"`
	BootstrapDNS []string `json:"bootstrap_dns,omitempty"`
	FallbackDNS  []string `json:"fallback_dns,omitempty"`
}

// adguardDNSInfoResp is the relevant subset of GET /control/dns_info.
type adguardDNSInfoResp struct {
	UpstreamDNS  []string `json:"upstream_dns"`
	BootstrapDNS []string `json:"bootstrap_dns"`
	FallbackDNS  []string `json:"fallback_dns"`
}

// isAdGuardUsingEncryptedDNS returns true when AdGuardHome is reachable and
// its upstream DNS entries use encrypted protocols (https://, tls://, quic://)
// that would prevent captive portal hostnames from resolving via hotel DNS.
// Returns false (non-blocking) when AdGuardHome is absent or unreachable.
func (c *CaptiveService) isAdGuardUsingEncryptedDNS() bool {
	upstream, _, _ := c.readAdGuardUpstream()
	for _, u := range upstream {
		if strings.HasPrefix(u, "https://") ||
			strings.HasPrefix(u, "tls://") ||
			strings.HasPrefix(u, "quic://") ||
			strings.HasPrefix(u, "sdns://") {
			return true
		}
	}
	return false
}

// agAPIBase returns the AdGuardHome API base URL, respecting test overrides.
func (c *CaptiveService) agAPIBase() string {
	if c.adguardAPIBaseOverride != "" {
		return c.adguardAPIBaseOverride
	}
	return adguardAPIBase
}

// readAdGuardUpstream fetches the current upstream DNS config from AdGuardHome.
// Returns empty slices if AdGuard is not running or not installed.
func (c *CaptiveService) readAdGuardUpstream() (upstream, bootstrap, fallback []string) {
	cl := &http.Client{Timeout: 2 * time.Second}
	resp, err := cl.Get(c.agAPIBase() + "/control/dns_info")
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	var info adguardDNSInfoResp
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return
	}
	return info.UpstreamDNS, info.BootstrapDNS, info.FallbackDNS
}

// setAdGuardUpstream calls AdGuardHome's /control/dns_config to update the upstream.
func (c *CaptiveService) setAdGuardUpstream(upstream, bootstrap, fallback []string) error {
	cfg := adguardDNSConfig{
		UpstreamDNS:  upstream,
		BootstrapDNS: bootstrap,
		FallbackDNS:  fallback,
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Post(c.agAPIBase()+"/control/dns_config", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AdGuardHome dns_config returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
