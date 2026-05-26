---
title: "ADR 0008: WireGuard-family protocol coexistence and AmneziaWG support"
status: Accepted
date: 2026-05-26
tags: [adr, vpn, wireguard, amneziawg, openwrt]
---

# ADR 0008: WireGuard-family protocol coexistence and AmneziaWG support

## Status

Accepted.

## Context

Travo already ships a working **WireGuard** client path: profile import, activation, firewall plumbing, DNS forwarding, kill switch, split tunnel, verification, and speed testing. Travel-router users also need a practical way to survive **UDP filtering and DPI** in restrictive networks.

AmneziaWG is the strongest fit among the considered options because it is a **WireGuard-compatible config superset** with extra obfuscation parameters, while keeping a familiar tunnel model and low runtime overhead. The major complication is packaging: AmneziaWG uses a separate kernel module, userspace tool, and netifd proto helper, and those packages are **not** in official OpenWrt feeds.

## Decision

### 1. Product stance

- **WireGuard remains the default VPN protocol.**
- **AmneziaWG is additive, optional support** for the same user-facing VPN workflow.
- Travo keeps the existing **one active full-tunnel VPN path at a time** rule across WireGuard, AmneziaWG, and Tailscale exit-node routing.

### 2. Shared WireGuard-family surface

- Travo treats WireGuard and AmneziaWG as one **WireGuard-family** subsystem.
- The existing `/api/v1/vpn/wireguard*` surface remains the canonical config/status API and is extended with protocol metadata rather than cloned into a second full CRUD namespace.
- A dedicated **availability endpoint** is added for AWG capability probing because package readiness is firmware-specific.

### 3. Runtime ownership

- Travo continues to own a single app-managed VPN tunnel slot and shared firewall objects (`wg0`, `wg0_zone`, `wg0_fwd`) when device validation confirms the AWG netifd helper can back the same logical interface name.
- Protocol-specific differences are intentionally narrow: **proto name**, **userspace tool**, **status command**, and **AWG-specific UCI keys**.
- Firewall policy, kill switch, split tunnel, DNS forwarding, and profile activation stay shared.

### 4. Detection and persistence

- Imported configs containing `Jc`, `Jmin`, `Jmax`, `S1`, `S2`, or `H1-H4` are treated as **AmneziaWG** profiles.
- Raw imported config is preserved, and saved profile metadata includes derived protocol information so the UI can badge profiles without reparsing on every render.

### 5. Packaging rule

- AWG support is releaseable only when Travo can provide or document a **kernel-matched package set** for the target firmware.
- When packages are missing or incompatible, the product must **surface that state clearly** instead of silently downgrading or failing only during tunnel enable.

## Consequences

- Existing WireGuard users keep the same UX and API paths.
- Backend and frontend code should centralize protocol choice instead of duplicating WireGuard flows.
- Packaging/release work is part of the feature, not an afterthought, because the kernel module is ABI-sensitive.
- Real-device validation is mandatory before calling the feature done.

## References

- `docs/architecture.md` §6.4
- `docs/plans/amneziawg-integration.md`
- `backend/internal/services/vpn_service.go`
- `backend/internal/services/service_manager.go`
