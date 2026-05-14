---
title: "ADR 0004: Firewall zones, forwarding, and interface topology"
status: Accepted
date: 2026-05-14
tags: [adr, firewall, zones, wireguard, wwan, openwrt]
---

# ADR 0004: Firewall zones, forwarding, and interface topology

## Status

Accepted.

## Context

OpenWrt **firewall4** / UCI **`firewall`** defines zones, forwards, and rules. Travo adds **WWAN**, **WireGuard (`wg0`)**, **guest** WiFi, and **failover**-related interfaces. Half-complete zone definitions (zone without masq, forward, or input policy) cause subtle breakage. LuCI users expect Travo’s changes to remain **inspectable and editable** in the same UCI model.

## Decision

### 1. Full-zone rule

- Any **new zone**, **forwarding path**, or **interface assignment** must ship the **complete** UCI surface needed for that zone to behave like existing **`wan`** patterns: zone definition, **masq** where appropriate, **forwarding** to/from `lan` (or documented exceptions), and **input**/policy consistency with product security stance.
- Do **not** introduce parallel “shadow” firewall state outside UCI for core routing—**UCI remains the source of truth** on device.

### 2. Naming and mental model

- **`wan`** continues to represent untrusted upstream(s): physical WAN, WWAN (`wwan`), and similar.
- **VPN** (`wg0` zone) is a separate topology decision: kill switch, **lan → wg** forwarding, and split-tunnel JSON (`/etc/travo/split-tunnel.json`) interact with **`VpnService`** and firewall generation—changes must preserve coherent **routing + firewall** together.
- **Guest** wireless maps to its own zone/firewall path as implemented in `WifiService` / network helpers—extend symmetrically when adding guest features.

### 3. Wireless staged apply includes firewall and DHCP

- **`uciApplyConfigs`** for wireless LuCI-style apply includes **`firewall`** and **`dhcp`** alongside **`wireless`**, **`network`**, and **`system`** so rollback snapshots capture **cross-package** wireless-related edits (ADR 0002).

### 4. Failover and mwan3

- Failover-generated policy must not create **orphan interfaces** in `mwan3` without matching **network** and **firewall** context (see ADR 0005).

## Consequences

- UI-only toggles that imply a new interface **must** be implemented with the full firewall story or rejected.
- Code reviews for `network`/`firewall` commits should explicitly ask: **“What zone and forwardings did we add or change?”**

## References

- `docs/architecture.md` §5
- `backend/internal/services/vpn_service.go` — `setupWireGuardFirewall` and related
- `backend/internal/services/wifi_service.go` — `uciApplyConfigs`, WWAN / guest paths
- `backend/internal/services/failover_service.go` — mwan3 + network interaction
