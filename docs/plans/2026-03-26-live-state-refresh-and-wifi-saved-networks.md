---
title: "Plan: Live state refresh + WiFi saved network persistence (2026-03-26)"
description: "Planning / design notes: Plan: Live state refresh + WiFi saved network persistence (2026-03-26)"
updated: 2026-04-13
tags: [plan, traceability, wifi]
---

# Plan: Live state refresh + WiFi saved network persistence (2026-03-26)

## Context / symptoms

- After connecting to an upstream AP, the **WiFi tab UI does not update** until a manual page reload.
- Similar “stale UI” behavior also appears in **VPN backend/UI**, suggesting a **cross-cutting state invalidation / subscription issue**, not a single-page bug.
- When connecting to a new AP, **previously saved networks disappear** from the saved list (regression/bug).

## Goals

- Ensure the UI reflects changes **immediately** after actions that change router state (connect/disconnect, enable/disable, apply).
- Ensure saved upstream WiFi networks **persist** across connects and are not unintentionally deleted/overwritten.
- Avoid excessive polling; prefer **event-driven** updates with bounded fallback polling where necessary.

## Non-goals

- Redesign of WiFi UI layout or scan UX (unless required to support correct state flow).
- Changing the underlying OpenWrt apply/confirm safety model (must keep rollback semantics).

## Likely root causes (hypotheses to validate)

- **Frontend cache not invalidated** after mutations (React Query / SWR / custom hooks).
- Mutations do not update a **global store** or shared query keys, so other cards still render old data.
- **WebSocket eventing** exists for some domains (system stats) but not for WiFi/VPN, or events are not routed into invalidation.
- Backend connect flow might **rewrite** UCI `wireless` sections (e.g., reusing a single `wifi-iface` section) and accidentally **drops other STA profiles**.
- Backend “list saved networks” endpoint might be derived from a runtime-only view rather than UCI config, causing it to “lose” networks during/after apply.

## Implementation approach

### Phase 0 — Reproduce + instrument (fast, high-signal)

- Reproduce stale UI update:
  - Connect to upstream AP from UI
  - Observe which cards remain stale (current connection card, saved networks list, status badges, etc.)
  - Repeat for VPN enable/disable and note the stale pieces
- Add temporary structured logs (backend) around:
  - connect/disconnect handlers
  - “list saved networks” / “wifi status” handlers
  - UCI apply/confirm steps for wireless/VPN
  - Ensure logs are removable or kept behind debug flag if needed

### Phase 1 — Frontend state model: make mutations invalidate the right queries

- Identify the state mechanism used:
  - If React Query: ensure each mutation has `onSuccess` that invalidates *all* dependent query keys:
    - WiFi: current connection, saved networks, wifi radios/AP config, connectivity indicators
    - VPN: status, profile list, routing/DNS status summary, dashboard VPN card
  - If custom fetch + local component state: migrate to a shared query layer for router state slices
- Create a small “router state invalidation” helper:
  - `invalidateWifiState()`, `invalidateVpnState()`, and `invalidateGlobalConnectivity()`
  - Used by all relevant mutations (connect/disconnect, enable/disable, import profile, apply)
- Add a bounded fallback:
  - After an action that triggers a delayed backend apply/confirm, poll status for a short window
  - Stop early when state becomes consistent
  - Do not add unbounded or permanent polling

### Phase 2 — Event-driven refresh: push “state changed” notifications

- Extend existing WebSocket channel (if present) or add a lightweight event stream:
  - Backend emits events like:
    - `wifi:changed` (connect/disconnect, saved profiles changed, apply started/finished)
    - `vpn:changed` (enable/disable, profile changed, DNS/routing updated)
    - `connectivity:changed` (internet reachability, default route changes)
- Frontend subscribes and triggers the same invalidation helpers.
- Ensure events are coarse enough to avoid flooding; payload can be minimal (just type + timestamp).

### Phase 3 — Fix “saved networks disappear” bug (backend + contract)

- Define the persistence contract:
  - “Saved networks” == UCI persistent profiles, not scan results
  - Connecting to one profile must not delete other saved profiles
- Audit connect flow:
  - Ensure “connect” only:
    - selects/enables one STA profile (or sets priority)
    - updates credentials for the selected profile
    - attaches to `network=wwan` correctly
  - It must **not**:
    - `uci delete` other STA sections
    - re-create wireless config from scratch
- Add regression tests (backend):
  - Given two saved STA profiles, connect to one, verify both remain present afterward
  - Verify only the selected one is active/enabled (as intended by the design)

### Phase 4 — Consistency across pages

- Ensure the same “source of truth” hooks are used on:
  - WiFi page, Dashboard quick actions, VPN page, Services toggles
- Remove duplicated local state that can drift from server truth.

## Acceptance criteria

- After connecting/disconnecting upstream WiFi, the WiFi page updates without manual reload:
  - current SSID/status updates
  - saved networks list remains intact and accurate
  - connectivity badge updates when relevant
- After enabling/disabling VPN, VPN page + dashboard update without manual reload.
- Saved networks persist across connects and across browser reloads.
- No long-running background polling loops introduced.

## Test plan

- Unit/integration (backend):
  - connect with existing saved profiles does not delete others
  - list saved profiles returns stable results across connect/disconnect
- Frontend:
  - connect → UI updates (no reload)
  - VPN toggle → UI updates (no reload)
- On real device:
  - connect to at least two different upstream APs in sequence and verify both remain saved
  - confirm no unexpected wireless apply regressions (rollback semantics unchanged)

