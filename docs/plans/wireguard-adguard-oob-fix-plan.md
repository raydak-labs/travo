---
title: "WireGuard + AdGuard Out-Of-Box Fix Plan"
description: "Planning / design notes: WireGuard + AdGuard Out-Of-Box Fix Plan"
updated: 2026-04-13
tags: [adguard, maintenance, plan, traceability, wireguard]
---

# WireGuard + AdGuard Out-Of-Box Fix Plan

## Context

Real-device integration testing on OpenWrt `25.12` showed that both features do
not yet behave out-of-the-box as expected:

- WireGuard profile add/activate APIs return success, but no runtime `wg0`
  interface is created, and `wg show` remains empty.
- AdGuard can run as a process and UI can be reachable, but backend install
  detection and DNS integration can still fail functional DNS resolution.

This document defines a concrete backend-first implementation and test plan.

## Goals

1. WireGuard works end-to-end after install + profile import + enable:
   - Runtime interface exists (`wg0`)
   - Handshake/status visible
   - Routing behavior matches config
2. AdGuard works end-to-end after install + DNS enable:
   - Backend reports install/running correctly
   - DNS forwarding path is valid and resolvable
   - Optional filtering validation is possible
3. APIs fail loudly when runtime apply fails (no false "status: ok").

## Non-Goals

- Full policy-routing redesign for all VPN modes.
- Complete AdGuard auto-provisioning UX redesign.
- Multi-profile WireGuard advanced orchestration beyond current scope.

## Observed Backend Gaps

## A) WireGuard

- `ImportWireguardConfig()` writes keys/options but assumes `network.wg0` and
  peer sections exist and are correctly typed as WireGuard interface/peer.
- `ToggleWireguard()` commits `disabled` flag but does not ensure interface
  creation, proto/type correctness, or runtime bring-up.
- API success can occur without runtime success (`wg0` missing).

## B) AdGuard

- Service status can report `running=true` while `installed=false`.
- DNS toggle can update dnsmasq UCI (`127.0.0.1#5353`) without verifying AdGuard
  DNS listener/upstream readiness, causing resolution failures.

## Implementation Plan

## Phase 1 — WireGuard Runtime Correctness

1. **Normalize WireGuard UCI structure before writing values**
   - Ensure `network.wg0` exists as the correct interface/proto type.
   - Ensure peer sections exist and are bound to `wg0`.
   - Remove or migrate invalid stale sections.

2. **Make apply explicit and verifiable**
   - After config write: commit network, trigger interface reload/up for `wg0`.
   - Validate runtime with deterministic checks:
     - `ifstatus wg0` exists
     - `wg show wg0 dump` parses
   - If validation fails: return API error and include reason.

3. **Tighten API success criteria**
   - `activate profile` and `toggle enable` must only return success when runtime
     state is valid (not only UCI commit success).

4. **Status endpoint hardening**
   - Distinguish:
     - configured-only
     - enabled-not-up
     - up-no-handshake
     - connected (handshake + transfer)

## Phase 2 — AdGuard Install + DNS Path Correctness

1. **Unify install detection**
   - Determine installed state from package manager and binary/init presence.
   - Avoid reporting `installed=false` when process is already running.

2. **Safe DNS toggle sequencing**
   - On enable:
     - verify AdGuard service running
     - verify AdGuard DNS listener reachable on expected local socket/port
     - then apply dnsmasq forward/noresolv changes
   - On disable:
     - restore dnsmasq defaults safely
   - If any validation fails: rollback and return API error.

3. **Health checks exposed to API**
   - Add structured status fields:
     - `adguard_listener_ready`
     - `dnsmasq_forward_target`
     - `resolver_probe_ok`

## Phase 3 — Integration + Regression Safety

1. Add backend tests for:
   - missing/invalid `network.wg0` -> creation and runtime validation
   - WireGuard apply failure -> API returns error
   - AdGuard running-but-not-installed mismatch
   - DNS enable with failed listener check -> rollback + error

2. Extend manual integration script/tests:
   - include WireGuard and AdGuard suites with hard pass/fail checks
   - store artifacts under `tmp/integration-*`

3. Update docs:
   - `docs/testing.md` with final passing steps and expected outputs
   - requirements checkboxes/bug list status updates

## Detailed Test Plan (After Implementation)

## WireGuard acceptance

1. Install WireGuard service.
2. Import profile `test/integration/wireguard-profiles/privado.ams-033.conf`.
3. Activate profile + enable tunnel.
4. Verify:
   - `ifstatus wg0` exists
   - `wg show wg0` includes peer/public key
   - `/api/v1/vpn/wireguard/status` is non-empty
   - internet reachability still works or follows expected VPN path

Fail if any step returns success but runtime checks fail.

## AdGuard acceptance

1. Install + start AdGuard via API.
2. Verify API status reports `installed=true`, `running=true`.
3. Enable AdGuard DNS via API.
4. Verify:
   - dnsmasq config points to expected target
   - local resolver probe succeeds (`nslookup`/equivalent)
   - API health fields mark DNS path healthy

Fail if DNS toggle returns success but probe fails.

## Exit Criteria

Work is complete when all are true:

- WireGuard and AdGuard suites pass on real device with no manual patching.
- API responses are aligned with runtime truth (no false positive success).
- Requirements entries for these bugs are marked resolved.
