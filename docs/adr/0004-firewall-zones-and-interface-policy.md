---
title: "ADR 0004: Firewall zones, forwarding, and interface topology"
status: Accepted
date: 2026-05-14
updated: 2026-09-28
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

- **`wan`** continues to represent untrusted upstream(s): physical WAN, WWAN (`wwan`), USB tethering (`usbtether`, added to the **`wan`** zone's `network` list) and similar.
- **VPN** (`wg0` zone) is a separate topology decision: kill switch, **lan → wg** forwarding, and split-tunnel JSON (`/etc/trafo/split-tunnel.json`) interact with **`VpnService`** and firewall generation—changes must preserve coherent **routing + firewall** together. The `wg0` zone is written with the full §1 surface: `input=DROP`, `output=ACCEPT`, `forward=DROP`, `masq=1`, `mtu_fix=1`, plus a `lan` → `wg0` forwarding.
- **Guest** wireless maps to its own zone/firewall path as implemented in `WifiService` / network helpers—extend symmetrically when adding guest features.
- **Zone lookup is by name, never by index.** Code that needs the WAN zone resolves the section whose `name` option is `wan` (`USBTetheringService.wanZoneSection`); a hard-coded `@zone[1]` breaks the moment a zone is added, removed or reordered.

### 3. Wireless staged apply includes firewall and DHCP

- **`uciApplyConfigs`** for wireless LuCI-style apply includes **`firewall`** and **`dhcp`** alongside **`wireless`**, **`network`**, and **`system`** so rollback snapshots capture **cross-package** wireless-related edits (ADR 0002).

### 4. Failover and mwan3

- Failover-generated policy must not create **orphan interfaces** in `mwan3` without matching **network** and **firewall** context (see ADR 0005).

### 5. Firewall mutations that bypass rpcd apply/confirm

Most UCI changes go through the rpcd apply / rollback / confirm window (ADR 0002 §5). Two
firewall mutations deliberately do **not**, and are recorded here as explicit exceptions:

| Path | Owner | What it commits | Guard |
| ---- | ----- | --------------- | ----- |
| `wg0` zone + `lan → wg0` forwarding | `vpn_service.go` — `setupWireGuardFirewall` / `teardownWireGuardFirewall` | `uci commit firewall` + firewall reload | `/etc/trafo/vpn-in-progress` |
| `usbtether` added to / removed from the `wan` zone | `usb_tethering_service.go` — `Configure` / `Unconfigure` | `uci add_list` / `del_list`, `uci commit firewall` + reload | `/etc/trafo/usbtether-in-progress` |

(Guard paths and their owners are catalogued in [ADR 0003 §2](./0003-crash-guards-and-live-state.md).)

Why: both run inside a larger operation that already owns an explicit crash guard and a
hand-written rollback, and both must be visible to the **running** firewall immediately —
a rollback window that silently expires on a routing change is worse than a direct commit
that is guarded and reversible. The compensation is not optional:

- the guard is written before the first mutation and removed only after success;
- the VPN path commits the tunnel **and verifies it is up** before touching the firewall,
  and tears the firewall and DNS changes back down if anything fails;
- the USB-tether path resolves the zone by name and **returns** the `add_list` / `del_list`
  and `commit` errors instead of discarding them.

**Rule:** a new direct `commit firewall` + reload path must either be brought onto the
rpcd apply/confirm flow or be added to this table with a named guard and a rollback.
Unlisted direct firewall commits are a defect.

## Consequences

- UI-only toggles that imply a new interface **must** be implemented with the full firewall story or rejected.
- Code reviews for `network`/`firewall` commits should explicitly ask: **“What zone and forwardings did we add or change?”** and **“Is this in §5, and does it have a guard?”**

## References

- `docs/architecture/overview.md` §5
- `backend/internal/services/vpn_service.go` — `setupWireGuardFirewall` and related
- `backend/internal/services/usb_tethering_service.go` — `wanZoneSection`, `Configure`, `Unconfigure`
- `backend/internal/services/wifi_service.go` — `uciApplyConfigs`, WWAN / guest paths
- `backend/internal/services/failover_service.go` — mwan3 + network interaction
- [ADR 0003](./0003-crash-guards-and-live-state.md) — the guards this table depends on

## Device findings: USB tethering

Tethering devices present differently depending on the phone, and the firewall
change is the same either way: `usbtether` joins the `wan` zone.

- Android typically enumerates as RNDIS on `usb0` and needs `kmod-usb-net-rndis`;
  some devices enumerate as CDC-NCM on `usb0` and need `kmod-usb-net-cdc-ncm`.
- iOS needs `usbmuxd` plus `libimobiledevice`; it does not appear as a plain
  RNDIS or NCM device.

Detection therefore probes for both drivers rather than assuming one, and the
uplink medium is the discriminator for the interface — not the address (see
`NetworkMedium` in `docs/architecture/overview.md`).
