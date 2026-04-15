---
title: Architecture decisions
description: Stable runtime invariants, safety rules, subsystem contracts, deployment assumptions, footprint constraints.
updated: 2026-04-15
tags: [architecture, safety, invariants, travo]
---

# Architecture Decisions

This file keeps **stable architecture and operational decisions**. It is the
first place to update when we decide or change:

- runtime invariants
- safety rules
- subsystem contracts
- deployment assumptions
- performance or footprint constraints

For active backlog use [`docs/requirements/tasks_open.md`](./requirements/tasks_open.md).  
For completed work use [`docs/requirements/tasks_done.md`](./requirements/tasks_done.md).

## 1. System Shape

- Monorepo layout:
  - `frontend/` React + TypeScript + Vite SPA
  - `backend/` Go + Fiber API and device orchestration
  - `shared/` shared TypeScript API contracts
  - `docs/plans/` historical design notes (not runtime truth; see `docs/README.md`)
- Travo coexists with LuCI instead of replacing it outright.
- Backend is the source of truth for OpenWRT mutations; frontend drives API calls and renders device state.
- The backend exposes a machine-readable OpenAPI contract at `GET /api/openapi.json` for automation and integration work.

## 2. Wireless Model And Invariants

These rules are stable product behavior, not backlog notes.

### 2.1 STA / WWAN ownership

- At most **one enabled STA** (`mode=sta`) may be bound to `network=wwan`.
- If multiple saved upstream networks exist, only one may be active at runtime.
- Any workflow that reuses or creates a STA section must ensure `network=wwan` exists and is attached to the firewall `wan` zone. Failing that is an error, not a soft warning.

### 2.2 Repeater layout on multi-radio hardware

- In repeater mode on hardware with two or more radios, STA and downlink AP should live on different radios by default.
- Same-radio STA+AP is treated as fragile on ath11k/IPQ6018 because AP channel follows STA channel and upstream failure can drag down local AP.
- `allow_ap_on_sta_radio` is the explicit escape hatch when same-radio layout is required.
- Repeater reconciliation is a first-class operation: changing AP settings or repeater options must not silently leave the router in a fragile layout when a better radio split exists.

### 2.3 AP credential model

- Default UX is one shared SSID/password across enabled downlink AP sections.
- Per-radio enable switches stay visible.
- A toggle may expose separate per-radio forms, but shared credentials are the default because it matches typical travel-router use.

### 2.4 Health and recovery signals

- `GET /api/v1/wifi/health` is the place where frontend learns about wireless invariant violations and fragile layouts.
- Repeater same-radio AP/STA situations must be surfaced as a warning and have a reconcile action.
- Auto-reconnect logic must have a failure-count guard so broken config is not replayed forever.

## 3. Wireless Apply And Rollback Flow

Wireless mutation safety is intentionally modeled after LuCI.

- Backend wireless changes use rpcd session login, copy config into session state, `uci apply` with rollback timeout, then explicit `uci confirm`.
- Confirmation must happen only after the caller proves the router is still reachable.
- Backend must **not** self-confirm immediately after starting rollback apply.
- Scripts and SSH setup flows must **not** run `wifi`, `wifi up`, or `wifi reload` as part of applying user wireless changes.
- Setup scripts only write UCI. User applies via LuCI "Save & Apply" or by rebooting.
- `wifi reload` is avoided on ath11k/IPQ6018. Where `wifi up` exists for bounded recovery paths, that exception must stay narrow and documented.

## 4. Crash Guards For Automated Live-State Changes

Any automated action that can change live system state must use a crash guard:

1. Write guard file to persistent storage under `/etc/travo/` before dangerous operation.
2. On next startup, if guard file exists, skip operation and log warning.
3. Remove guard file only after successful completion.
4. Manual redeploy (`deploy-local.sh`) clears guard files and is the explicit retry signal.

Guard naming convention:

```text
/etc/travo/<feature>-in-progress
```

This rule applies to:

- UCI commits that alter connectivity
- wireless apply or recovery flows
- firewall or route mutation
- background repair jobs
- scheduled tasks and goroutines that touch live system state

## 5. Firewall And Interface Policy

- New zones, forwarding paths, or interfaces must include the full required firewall changes.
- Follow existing default `wan` patterns instead of inventing a separate one-off policy model.
- WWAN, WAN, VPN, guest, and future interfaces should be treated as explicit routing and firewall topology decisions, not UI-only toggles.

## 6. Networking Invariants

### 6.1 Multi-WAN failover (mwan3)

- Connection failover uses OpenWRT's mwan3 package for health tracking and route management.
- Failover policy is generated deterministically from `/etc/travo/failover.json` into `/etc/config/mwan3`.
- App-owned UCI sections are namespaced with `travo_` prefix.
- Failback to higher-priority interfaces includes a 30-second hold-down period to prevent interface flapping.
- Phase 1 implementation is **IPv4-only**: failover rules use `family: 'ipv4'` only.
- IPv6 failover is deferred until device testing confirms mwan3 IPv6 reliability on OpenWRT 23.05+.

**Decision rationale (IPv6 deferral):**

- Generated failover rules specify `family: 'ipv4'` explicitly.
- No IPv6 failover policies or tracking rules are created in Phase 1.
- Future IPv6 support requires device verification of mwan3's IPv6 stability.
- See [`docs/plans/connection-failover.md`](./plans/connection-failover.md) Phase 0 decision track for detailed rationale.

### 6.2 Failover safety guards

- Applying failover configuration writes live routing policy and requires an explicit guard: `/etc/travo/failover-in-progress`.
- Guard file is only removed after verification succeeds; manual redeploy via `deploy-local.sh` clears stuck guards.
- Wireless changes during failover apply must preserve LuCI-style rollback semantics.
- Any new routes, zones, or firewall changes added for failover must include complete firewall zone configuration.

## 7. Device Constraints

Router hardware is constrained. Every feature must justify its footprint.

- Go backend: single static binary, no CGO, stripped symbols
- Frontend: tree-shaken bundles, code-splitting per route, minimal dependencies
- Avoid heavy JavaScript frameworks beyond React core
- Prefer SVG assets over raster assets
- Keep API payloads small; avoid polling where a realtime channel already exists
- Warn before installing packages that meaningfully consume flash storage

## 7. Documentation Rules

- Put stable rules here, not in backlog files.
- Put open work in `tasks_open.md`, completed work in `tasks_done.md`.
- Use [`docs/README.md`](./README.md) as the documentation map; plans live under [`docs/plans/`](./plans/) with a searchable [`docs/plans/README.md`](./plans/README.md).
- When a plan graduates into a durable rule, copy the essential decision here and link back to the plan for rationale if useful.
