---
title: "Plan: AdGuard Auto-Configure After Install"
description: "Planning / design notes: Plan: AdGuard Auto-Configure After Install"
updated: 2026-04-13
tags: [adguard, plan, traceability]
---

# Plan: AdGuard Auto-Configure After Install

**Status:** Not implemented
**Priority:** High
**Related requirements:** [4.2 AdGuard Home](../requirements/tasks_done.md#vpn-and-services)

---

## Goal

After installing AdGuard Home via the Services page, automatically configure it with sensible defaults: correct port bindings, interfaces, and DNS forwarding integration with dnsmasq. Provide two modes: "DNS forwarding" (default, safer) and "Primary DNS" (advanced, with warning).

---

## Background

AdGuard Home configuration lives in `/opt/AdGuardHome/AdGuardHome.yaml`. Key settings:

```yaml
bind_host: 0.0.0.0     # Web UI bind address
bind_port: 3000         # Web UI port
dns:
  bind_hosts:
    - 0.0.0.0
  port: 5353            # DNS listen port (in forwarding mode)
  # port: 53            # DNS listen port (in primary mode)
```

**Two DNS modes:**

1. **DNS Forwarding (default):** AdGuard listens on port 5353. dnsmasq is configured to forward all DNS queries to `127.0.0.1#5353`. dnsmasq stays as the primary DNS server on port 53. This is safer because other services (DHCP, local DNS) continue working through dnsmasq.

2. **Primary DNS (advanced):** AdGuard listens on port 53 directly. dnsmasq DNS is disabled (or moved to a different port). AdGuard handles all DNS directly. ⚠️ Warning: may break VPN DNS, DHCP hostname resolution, and local DNS entries.

---

## Phases

### Phase 1 — Post-Install Auto-Configuration

**When:** AdGuard Home is installed via Services page
**What:** Generate a default `AdGuardHome.yaml` and configure dnsmasq forwarding.

**Steps:**
1. Write default config to `/opt/AdGuardHome/AdGuardHome.yaml`:
   ```yaml
   bind_host: 0.0.0.0
   bind_port: 3000
   users: []           # No auth initially (user configures via AdGuard UI)
   dns:
     bind_hosts:
       - 0.0.0.0
     port: 5353
     upstream_dns:
       - https://dns.cloudflare.com/dns-query
       - https://dns.google/dns-query
     bootstrap_dns:
       - 1.1.1.1
       - 8.8.8.8
   filtering:
     enabled: true
   ```
2. Start AdGuard Home service
3. Configure dnsmasq forwarding (already exists as `SetDNS(true)`)
4. Verify AdGuard is responding

**Files:**
- `backend/internal/services/adguard_service.go` — `AutoConfigure()` method
- Called from service install handler after successful package install

### Phase 2 — DNS Mode Toggle

**API:** `PUT /api/v1/adguard/dns-mode`
**Request:** `{ "mode": "forwarding" | "primary" }`

**Forwarding mode (default):**
- AdGuard DNS port → 5353
- dnsmasq: `server=127.0.0.1#5353`, `noresolv=1`
- AdGuard handles filtering; dnsmasq handles DHCP, local DNS

**Primary mode (advanced):**
- AdGuard DNS port → 53
- dnsmasq: `port=0` (disable DNS), keep DHCP running
- AdGuard handles everything DNS
- ⚠️ Warning: local DNS entries and DHCP hostname resolution may break

**Frontend:**
- Toggle in Services > AdGuard section
- "Forwarding" mode = default, checkmark
- "Primary" mode = with warning tooltip: "⚠️ This makes AdGuard the sole DNS server. Local DNS entries and some VPN configurations may stop working. Use only if you need advanced AdGuard features that require port 53."

### Phase 3 — VPN + AdGuard Interplay

**When VPN is active + AdGuard in forwarding mode:**
- dnsmasq forwards to AdGuard (127.0.0.1#5353)
- AdGuard uses upstream DNS that may go through VPN tunnel
- This "just works" because AdGuard's upstream DNS queries route through the VPN like any other traffic

**When VPN is active + AdGuard in primary mode:**
- Need to ensure AdGuard uses DNS servers reachable via VPN
- If VPN has its own DNS server, configure it as AdGuard's upstream
- This needs manual configuration or a "VPN DNS" field in AdGuard settings

**Implementation:** Add a note in the VPN page when both are active, suggesting the correct DNS configuration.

---

## Frontend Changes

**Services page — AdGuard section:**
- After install: show "Configured ✓" badge
- DNS mode toggle with forwarding/primary options
- Warning tooltip on primary mode
- Link to AdGuard Web UI (already exists)
- Config editor (already exists)

**System page — Quick Links:**
- AdGuard dashboard link (already exists)

---

## Testing Strategy

- **Unit tests:** Test config file generation, dnsmasq configuration
- **Integration:** Install AdGuard on device, verify DNS resolution works through forwarding
- **Mode switching:** Test switching between forwarding and primary modes

---

## Notes

- AdGuard Home first-run wizard: on first access to port 3000, AdGuard shows a setup wizard. Our auto-config should bypass this by writing the config file before first start.
- Default upstream DNS uses DoH (DNS over HTTPS) — works even when ISP DNS is unreliable
- Storage impact: AdGuard Home binary is ~20MB + query log database growing over time
