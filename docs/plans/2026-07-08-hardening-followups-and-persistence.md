---
title: Hardening follow-ups and persistence
description: Work log + queue for the 2026-07-08 hardening branch — completed items, cleanup queue (vpn sleeps, wifi_service split, WS-fed queries), and the SQLite/persistence decision.
updated: 2026-07-08
tags: [reliability, auth, persistence, refactor, plan]
---

# Hardening follow-ups and persistence

Branch: `feat/hardening-reliability`. This doc is the running work log: **Done**,
**Queue** (one item at a time, committed separately), and **Findings** recorded
along the way.

## Done (validated on GL-AXT1800, OpenWrt 25.12, apk-only)

1. **Clock-independent sessions** — jti + monotonic `SessionRegistry`; relative
   `expires_in` in login/session; frontend counts down via `performance.now()`;
   time-sync gated by build-time plausibility + rate limit. (ADR 0007)
2. **Bounded exec everywhere** — `internal/execx` timeout tiers; converted ubus,
   uci, CommandRunner, package managers, init.d probes, logread/dmesg, ntpd,
   sysupgrade -b/-r, vnstat, USB tether. Exceptions: reboot-like fire-and-forget
   paths and the `iw event` listener (stopCh-bounded).
3. **opkg/apk unification** — runtime-detected `PackageManager` everywhere incl.
   speedtest CLI; best-effort index `Update()` before installs; speedtest-service
   routes were never registered — now they are (frontend called dead endpoints).
4. **WS hub** — write deadlines, snapshot writes outside the lock, dead-client
   removal; rate limiter sweeps; `StatsHistoryService.Stop`; unified
   `appLifecycle`; JSON 404 for unknown `/api/*`.
5. **Frontend** — WS `system_stats` feeds the React Query cache; HTTP polling
   only while disconnected.
6. **Round 2 from device validation** — random TLS serial (fixed serial 1 broke
   regenerated certs in browsers); tarball root ownership (uid-501 leak onto
   `/etc/travo`); apk/opkg note in testing docs.

## Queue

- [ ] **Q1: vpn_service settle-sleeps → observable waits.** ~12 bare
  `time.Sleep` calls. Introduce a `waitFor(cond, timeout, step)` helper and use
  it where a poll condition exists (interface up, wg handshake, route present).
  Keep genuinely condition-less settle delays, but name them as constants.
- [ ] **Q2: split wifi_service.go (2,276 lines)** into focused files by concern
  (scan/connect, AP config, repeater, auto-reconnect/cron, apply/rollback,
  health). Pure file move — no behavior change; tests must stay green untouched.
- [ ] **Q3: WS-feed remaining query caches.** The hub already pushes
  `network_status` and `alert` messages; wire them into the React Query cache
  like `system_stats`, and make the corresponding hooks fallback-poll only.
  Hooks without a WS source (wifi, vpn, services…) keep their intervals.
- [x] **Q4: persistence layer (backlog research item 16) — DECIDED: bbolt.**
  Measured on this repo: `go.etcd.io/bbolt` adds **247 KB** to the stripped
  binary (12.57 → 12.83 MB); `modernc.org/sqlite` would add ~10 MB — rejected.
  Implemented `internal/store` (bucket KV at `<auth dir>/travo.db`, 0600, open
  falls back to memory-only with a warning). First consumers:
  - token blocklist: sha256(token)→expiry persisted; revocations survive
    restarts (closes auth-hardening plan item 1).
  - stats history: ring buffer flushed every 20 collects (~10 min) and on
    Stop; restored at startup capped to maxLen. Flash-write discipline held.
  Future consumers: data usage budgets, alert history.

## Findings along the way

Record anything new here before deciding whether to fix inline or queue it.

- (2026-07-08) `docs/tests/failover-verification.md` playbook is ready but
  `tasks_open.md` failover items still say "verify on device" — needs a lab
  session with two uplinks.
- (2026-07-08, fixed in Q3) The `network_status` WS→query-cache wiring existed
  but only inside `useTopologyData` — live network data stopped flowing the
  moment the dashboard unmounted. Moved into `useNetworkStatus` itself.
- (2026-07-08) Frontend `use-wifi`/`use-vpn` poll 10–15s; no WS source exists
  for them today. A later idea: broadcast a generic `state_changed` WS event
  from the network event watcher to trigger query invalidation instead of
  adding more push payloads.
