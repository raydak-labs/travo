---
title: Completed tasks
description: Shipped milestones and done work; pair with tasks_open for current backlog.
updated: 2026-10-02
tags: [backlog, requirements, changelog]
---

# Completed Tasks

High-level **what shipped**, grouped by subsystem. For the old exhaustive checkbox export, see [`../_archive/requirements_done.md`](../_archive/requirements_done.md) (read-only).

When you finish something in [`tasks_open.md`](./tasks_open.md): remove it there, add a short bullet under the right heading here, and update [`../architecture.md`](../architecture.md) and the relevant [`../adr/`](../adr/) ADR if you introduced or changed a normative invariant.

> **Last updated:** 2026-10-02 (the `updated:` field in the frontmatter is the same date; keep them in step)

## Milestone checklist (compact)

Closed “Task N” items from earlier tracking — detail lives in the sections below.

- [x] Form pattern standardisation; network page **Status / Configuration / Advanced** grouping
- [x] Captive portal: auto-accept portal terms
- [x] Authentication: IP-based access control
- [x] VPN speed test; DDNS custom update URL; SQM / QoS (traffic shaping)
- [x] WiFi: setup wizard unified AP credentials; repeater-options `PUT` reconcile

## WiFi And Network Foundation

- Upstream WiFi scan, connect, disconnect, saved-network management, hidden networks, priority ordering.
- WiFi modes: AP, STA, repeater (wizard + health).
- Multi-radio detection; dual-band scan bundling and automatic band switching.
- AP: shared credentials, per-radio enable, guest WiFi, QR, MAC clone/policy, scheduling.
- Clients: aliases, block/kick, DHCP reservations, static IPs.
- Network: WAN status and config, DHCP, LAN DNS, DDNS, firewall summary, port forwarding, IPv6, WoL, traffic charts.
- Data usage tracking; USB tethering.

## VPN And Services

- WireGuard: import, toggle, status, peers, kill switch, split tunnel, verification, DNS leak checks, speed test.
- Tailscale: install, auth, peers, exit node, SSH toggle.
- Services: install/remove, start/stop, autostart, progress logs.
- AdGuard Home: install, auto-configure, DNS path, dashboard link, config editor, VPN interplay.
- Dynamic DNS including custom update URLs.

## System, Dashboard, And UX

- Dashboard: live stats, charts, quick actions, alerts, notification history, captive banners.
- System: reboot, shutdown, firmware upgrade, factory reset, hostname, backup/restore, LED, timezone, NTP, password, hardware buttons.
- Logs: system/kernel filters, search, export.
- UI: responsive layout, sidebar + mobile drawer, dark mode, skeletons, onboarding, grouped IA.

## Frontend Toolchain

- **ESLint replaced by oxlint, TypeScript 7 adopted.** The `TypeScript 7 — blocked by
  typescript-eslint` hold in [`tasks_open.md`](./tasks_open.md) §16 (and the matching
  Renovate `allowedVersions` rule) is resolved by the escape hatch it named: ESLint and
  `typescript-eslint` are gone, `.oxlintrc.json` replaces `eslint.config.js`, and
  `typescript` is pinned to `7.0.2` in the root, `frontend/`, and `shared/` packages.
  82 of the 85 previously-enabled ESLint rules are ported 1:1 at identical severity, plus
  `eslint/no-undef` added on top (the typescript-eslint baseline had it off) for 83 enabled; the
  three that oxlint does not implement (`eslint/no-octal`, `react-hooks/config`,
  `react-hooks/gating`) are documented as accepted losses in `.oxlintrc.json` and in
  [ADR 0011](../adr/0011-frontend-lint-toolchain-oxlint-and-typescript-7.md). Prettier is
  unchanged and remains the formatter (oxlint does not format). Type-aware linting via
  `oxlint-tsgolint` was evaluated and deliberately NOT enabled — it remains in the dependency
  graph only as an auto-installed optional peer of oxlint, so no type-aware rule runs, behaviour
  is unaffected, and CI does not depend on it. See [`docs/development.md`](../development.md).

## Reliability And Operational Fixes

- Wireless apply: LuCI-style rollback; confirm after reachability; no self-confirm while rollback pending.
- Apply failures surfaced; saved state not reported healthy when runtime is broken.
- Saved upstream WiFi persistence; UI refresh after WiFi/VPN actions.
- WireGuard disable restores connectivity; AdGuard install/DNS path fixes.
- OpenAPI at `GET /api/openapi.json`.
- Packaging, install tarball, uci-defaults, CI workflows.
- Connection failover: priority-based WAN source, health check, auto-switch, event notifications (mwan3).
- Captive portal: DNS bypass/restore for custom DNS blocking portal access; form-based auto-accept.
- Services: Watchcat (connection watchdog), Cloudflared (Cloudflare Tunnel) registered.
- Dashboard: system stats history (CPU/memory over time, 6h ring buffer, 30s interval).
- WiFi: multi-radio auto-discovery startup script, persists to /etc/travo/radios.json.
- Auth: clock-independent sessions (monotonic jti registry, relative `expires_in`, frontend countdown); hardened pre-login time-sync (build-time plausibility gate + rate limit). (2026-07-08)
- Reliability: all shell-outs bounded via `internal/execx` timeout tiers; WS hub write deadlines + dead-client removal; rate limiter sweep; stats-history stop; unified `appLifecycle` shutdown. (2026-07-08)
- Packages: opkg/apk handled via runtime-detected `PackageManager` everywhere (speedtest CLI included); best-effort index update before installs; speedtest-service routes actually registered. (2026-07-08)
- Frontend: WS `system_stats` feeds the React Query cache; HTTP polling only as disconnect fallback. Unknown `/api/*` GETs return JSON 404 instead of SPA HTML. (2026-07-08)

## Closed From The Open Backlog (2026-09-28)

Moved here from `tasks_open.md` because the code shipped; the items were still listed as open.

- **Connection failover (§2.7) — all four items implemented.** Priority-based WAN source with deterministic generation from `/etc/travo/failover.json`, mwan3 health checks (configurable target), automatic switching to the next source on failure, and event notifications. Service: `services/failover_service.go`; routes `GET`/`PUT /api/v1/network/failover` and `GET /api/v1/network/failover/events`, all present in the OpenAPI spec. Normative detail: [ADR 0005](../adr/0005-multi-wan-failover-mwan3.md). Device validation with two uplinks is still outstanding; see [`docs/tests/failover-verification.md`](../tests/failover-verification.md).
- **Persistence backend (§16) — decided and implemented as bbolt.** `/etc/travo/travo.db` (bbolt, 0600, 5 s open timeout) replaces the "is SQLite a better fit?" research item; measured at +247 KB on the stripped binary versus ~10 MB for `modernc.org/sqlite`. Degrades to memory-only on open failure. Buckets, retention and the flash-write batching rule: [ADR 0009](../adr/0009-persistent-store-bbolt.md).
- **Historical data (§6.2) — the 6 h ring buffer shipped.** 720 points at a 30 s interval via `GET /api/v1/system/stats/history`, persisted in the bbolt store and flushed every 20 collects. Extending the window is a separate, still-open product decision.

- **Guard directory unified on `/etc/trafo`.** Crash guards were split across `/etc/trafo/` and
  `/etc/travo/`, and one feature straddled both (`captive-dns-in-progress` vs
  `captive-wwan-bounce-in-progress`). A guard written to one directory was invisible to a check
  that only looked at the other, so a stale guard permanently disabled a feature with no log
  line, and `deploy-local.sh` could not clear all of them. All guards now live in `/etc/trafo/`,
  enforced by `internal/services/guards_test.go`; the deploy script clears both
  directories for devices upgraded from an older build. Non-guard state stays in `/etc/travo/`.
  Normative detail: [ADR 0003 §2](../adr/0003-crash-guards-and-live-state.md).

- **Failover guard skip is no longer silent.** A stuck `failover-in-progress` disabled the monitor
  with no log line at all, so "failover never fires" was undiagnosable from either the device log or
  the UI. `FailoverService.Start` now logs the guard once at startup, matching `BandSwitchingService`.
  Normative detail: [ADR 0003 §1.2](../adr/0003-crash-guards-and-live-state.md).

## Critical Review Remediation — 2026-09-26 Review (2026-09-28)

Recorded here because several of these change normative behavior and now have ADRs.

- P0 input-injection fixes: shared `services.ValidateHHMM` and `ValidateButtonName` at the handler *and* service boundary; single-line + key-format validation for SSH keys ([ADR 0008](../adr/0008-ssh-key-management.md)).
- Generated `/usr/libexec/travo-wireless-toggle.sh` replaces `wifi up` / `wifi down` in the WiFi schedule and the button hotplug script ([ADR 0002 §6.1](../adr/0002-wireless-model-and-luci-apply.md)).
- Sticky pre-login time-sync gate with a bounded client-clock window; released binaries must stamp `main.BuildTime` ([ADR 0007 §2a](../adr/0007-authentication-and-access-control.md)).
- Crash-guard catalog rewritten as an authoritative path → owner → protects table, with the redeploy recovery path pointing at it ([ADR 0003 §2](../adr/0003-crash-guards-and-live-state.md)).
- VPN verify-before-mutate with rollback and a guard; failover apply lock + staged mwan3 apply; USB-tether WAN zone resolved by name; captive auto-accept bounces `wwan` only when it is the active uplink ([ADR 0001 §4.1](../adr/0001-dns-vpn-captive-portal-architecture.md), [ADR 0004 §5](../adr/0004-firewall-zones-and-interface-policy.md), [ADR 0005 §4](../adr/0005-multi-wan-failover-mwan3.md)).
- Auth hardening: no fallback JWT secret, session revocation + token rotation on password change, persisted revocations; Fiber timeouts and a 64 MB body limit ([ADR 0006 §5](../adr/0006-application-platform-and-api-contract.md)).
- OpenAPI route↔spec drift gate, so the spec can no longer drift silently ([ADR 0006 §4](../adr/0006-application-platform-and-api-contract.md)).
- Two new ADRs: SSH key management (0008) and the bbolt store (0009).
