---
title: "ADR 0003: Crash guards and automated live-state changes"
status: Accepted
date: 2026-05-14
tags: [adr, safety, travo, guards, operations]
---

# ADR 0003: Crash guards and automated live-state changes

## Status

Accepted.

## Context

The router runs Travo as a **long-lived process**. Automated jobs (failover apply, background WiFi recovery, band switching, etc.) can **commit UCI**, **restart services**, or **change routes** while the process could crash mid-flight. Without a durable marker, the next boot might repeat a half-finished operation or leave the system in an ambiguous state.

This ADR is distinct from **transaction snapshots** (e.g. VPN DNS JSON, captive DNS JSON) which store *configuration deltas* for restore. Crash guards answer: **“Should this feature refuse to start because a previous run died?”**

## Decision

### 1. Guard file contract

Any automated action that **materially changes live system state** and is **unsafe to blindly retry** after a crash must:

1. **Write** a guard file under **`/etc/travo/<feature>-in-progress`** (or a clearly named variant) **before** the dangerous segment.
2. On **startup**, if the guard exists, **skip** repeating the operation and **log** a warning until an operator clears it.
3. **Remove** the guard file **only after** the operation completes successfully end-to-end.
4. Treat **`deploy-local.sh`** / manual redeploy as the **explicit recovery** path that may clear stuck guards (see `docs/architecture.md` §4).

### 2. Implemented guard files (non-exhaustive)

| Path | Feature area |
| ---- | ------------- |
| `/etc/travo/failover-in-progress` | mwan3 failover generation and apply (`FailoverService`) |
| `/etc/travo/captive-dns-in-progress` | Captive DNS bypass active state + serialized backup (also used as bypass marker; see ADR 0001) |
| `/etc/travo/band-switch-in-progress` | Automated dual-band / band-switching workflow |

Other `/etc/travo/*.json` files store **config** or **snapshots** (VPN DNS, failover backup) without necessarily being crash guards—distinguish by whether startup logic consults them as **“abort if present”** signals.

### 3. Related patterns

- **Wireless** safety is primarily **LuCI-style rollback** (ADR 0002), not the same as a crash guard, but overlaps philosophically.
- **Autoreconnect** shell logic uses **`/etc/travo/autoreconnect-crash-guard`** and fail-count files (`wifi_service.go` generated script)—bounded recovery, documented in code comments.

## Consequences

- New **goroutines**, **cron hooks**, or **init**-triggered Travo paths that mutate connectivity must either use this guard pattern or be proven safe to retry idempotently without user intervention.
- Documentation and on-call playbooks should list **known guard paths** when debugging “feature won’t run” reports.

## References

- `docs/architecture.md` §4
- `backend/internal/services/failover_service.go` — `failover-in-progress`
- `backend/internal/services/band_switching_service.go` — `band-switch-in-progress`
- `backend/internal/services/captive_service.go` — `captive-dns-in-progress`
