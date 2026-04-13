---
title: "Plan: VPN disable latency + VPN DNS forwarding behavior (2026-03-26)"
description: "Planning / design notes: Plan: VPN disable latency + VPN DNS forwarding behavior (2026-03-26)"
updated: 2026-04-13
tags: [plan, traceability, vpn]
---

# Plan: VPN disable latency + VPN DNS forwarding behavior (2026-03-26)

## Context / questions

- Disabling VPN from the UI appears to take “a long time”.
- Desired behavior: when VPN is enabled, DNS forwarding should use **VPN-configured DNS servers only**; when VPN is disabled, that forwarding should be **removed** (restore prior DNS path).

## Goals

- Determine whether the disable latency is:
  - expected (kernel/netifd, route convergence, uci apply window), or
  - avoidable (blocking waits, slow commands, unnecessary verification loops)
- Implement deterministic DNS forwarding behavior that:
  - uses VPN DNS servers when VPN is enabled (and only then)
  - restores the previous DNS configuration cleanly on disable
  - avoids DNS leaks and avoids breaking LAN name resolution

## Constraints / safety

- Preserve OpenWrt rollback safety model: use rpcd `uci apply` + confirm flow; avoid direct `wifi` operations.
- Keep changes compatible with existing WireGuard + AdGuard integration (if enabled).

## Phase 0 — Measure disable time and identify the bottleneck

- Add timing spans around the disable endpoint steps:
  - UCI writes
  - apply initiation
  - confirm / waiting loops
  - verification checks (ping, route checks, wg status reads)
- On real device, compare:
  - UI disable
  - LuCI disable (if applicable)
  - direct CLI stop (for diagnosis only)
- Determine where the time is spent:
  - waiting for rollback window?
  - waiting for connectivity checks?
  - command timeouts (e.g. `wg show`, `ip route`, dnsmasq reload)?

## Phase 1 — Define desired DNS behavior matrix

Establish explicit rules for:

- **VPN enabled, AdGuard disabled**
  - LAN clients should use router DNS as usual (dnsmasq)
  - dnsmasq upstream forwarders should be set to VPN DNS servers (from WireGuard config)
- **VPN enabled, AdGuard enabled**
  - Decide whether:
    - AdGuard forwards to VPN DNS, or
    - AdGuard remains primary and uses its own upstreams, but those upstreams route via VPN
  - Align with existing “VPN + AdGuard interplay” plan and implementation
- **VPN disabled**
  - Restore upstream DNS forwarders to:
    - WAN/WWAN-provided DNS, or
    - user-configured custom DNS (if set), or
    - previous snapshot taken before enabling VPN

## Phase 2 — Implement DNS forwarding as a reversible transaction

- Introduce a “pre-VPN DNS snapshot” persisted in the app config:
  - stores the previous dnsmasq forwarding config and any related flags touched
  - stored before enabling VPN
  - used to restore on disable
- Apply changes via UCI only:
  - write new forwarders when enabling VPN
  - restore from snapshot when disabling VPN
- Ensure idempotency:
  - enabling twice doesn’t keep stacking changes
  - disabling when already disabled is a no-op

## Phase 3 — Make disable path faster (if possible) without losing correctness

Potential improvements once bottleneck is known:

- Avoid slow, synchronous “verification loops” on disable; instead:
  - return quickly after apply start
  - UI shows “disabling…” with live status
  - final confirmation via state refresh/events
- If a hard wait is required, bound it with:
  - short intervals
  - clear timeout and helpful error
- Reduce expensive calls:
  - avoid repeated `wg show` if wg interface is already down
  - avoid redundant route scans if apply hasn’t finished

## Acceptance criteria

- We can explain with measurements whether disable latency is required and what dominates it.
- When VPN is enabled, DNS forwarding uses VPN DNS servers (according to the defined matrix).
- When VPN is disabled, DNS forwarding is restored to the correct previous state.
- No regressions to LAN DNS resolution, AdGuard mode, or connectivity.

## Test plan

- Automated (backend):
  - enable VPN → DNS config written as expected
  - disable VPN → DNS config restored exactly
- On device:
  - verify dnsmasq/adguard upstreams change as expected
  - run DNS leak checks before/after disable/enable
  - confirm browsing still works immediately after disable

