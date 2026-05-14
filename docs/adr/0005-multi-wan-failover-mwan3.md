---
title: "ADR 0005: Multi-WAN connection failover (mwan3)"
status: Accepted
date: 2026-05-14
tags: [adr, mwan3, failover, routing, ipv4]
---

# ADR 0005: Multi-WAN connection failover (mwan3)

## Status

Accepted.

## Context

Travo implements **priority-based WAN failover** using OpenWrt’s **`mwan3`** package: health checks, policy routing, and interface tracking. Policy must be **deterministic**, **namespaced** to avoid colliding with user hand-edits, and **safe to apply** under crash-guard rules. IPv6 behavior on mwan3 across target OpenWrt versions was uncertain at productization time.

## Decision

### 1. Source of truth and generation

- Operator-facing failover configuration is stored in **`/etc/travo/failover.json`**.
- Travo **generates** UCI sections into **`/etc/config/mwan3`** (and coordinates **`network`** where required) from that JSON via **`FailoverService`**.
- App-owned generated sections use a **`travo_`** name prefix to reduce collision with manual mwan3 config.

### 2. IPv4-only phase

- Phase 1 emits mwan3 rules with **`family: ipv4`** only. **IPv6 failover is explicitly deferred** until validated on hardware (see `docs/plans/connection-failover.md` and `docs/architecture.md` §6.1).

### 3. Safety guards and backup

- Applying failover writes **`/etc/travo/failover-in-progress`** before live policy mutation and removes it only after successful verification (ADR 0003).
- A backup artifact **`/etc/travo/failover-mwan3-backup.json`** supports restore semantics implemented in `FailoverService` (paired with UCI operations).

### 4. Apply path

- Failover uses **`UCIApplyConfirm`** for **`network`** and **`mwan3`** configs (`mwan3UCIConfigs`) where the real applier is wired—aligning high-impact routing changes with the same **apply / confirm** philosophy as other dangerous UCI batches when applicable.
- **mwan3** service reload/restart is invoked after successful UCI commit paths as implemented in code.

### 5. Failback behavior

- **30-second hold-down** after failback to a higher-priority interface to reduce flapping (`docs/architecture.md` §6.1).

## Consequences

- Adding IPv6 must become **ADR amendment** or new ADR with device-tested matrices; do not silently extend rules to `ipv6` without that review.
- Manual edits inside **`travo_`** sections may be overwritten on next Travo apply—document for advanced users.

## References

- `backend/internal/services/failover_service.go`
- `backend/internal/models/failover.go`
- `docs/architecture.md` §6.1–6.2
- `docs/plans/connection-failover.md`
