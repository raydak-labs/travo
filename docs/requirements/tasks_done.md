---
title: Completed tasks
description: Shipped milestones and done work; pair with tasks_open for current backlog.
updated: 2026-10-25
tags: [backlog, requirements, changelog]
---

# Completed Tasks

High-level **what shipped**, grouped by subsystem. For the old exhaustive checkbox export, see [`../_archive/requirements_done.md`](../_archive/requirements_done.md) (read-only).

When you finish something in [`tasks_open.md`](./tasks_open.md): remove it there, add a short bullet under the right heading here, and update [`../architecture/overview.md`](../architecture/overview.md) and the relevant [`../adr/`](../adr/) ADR if you introduced or changed a normative invariant.

> **Last updated:** 2026-10-25 (the `updated:` field in the frontmatter is the same date; keep them in step)

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
- UI consistency pass (2026-10-05): one card plane everywhere (dashboard source card and setup
  wizard normalized), one status language (semantic tokens + `StatusPill`), one group heading
  (`SectionHeading`), one load/error contract (`QueryCard`) across system, services, network, logs,
  vpn and the shared component layer ([ADR 0012](../adr/0012-ui-consistency-and-status-language.md)).
- WiFi summary band: always-visible connection / internet / mode tiles with per-tile degradation;
  the first page to use the band pattern (the #195 pass later made it two tiles).
- Network traffic history: `GET /api/v1/network/traffic-history` serves a bounded ~10-minute
  in-memory ring so the interface chart paints real history on page load instead of an empty
  axis.
- UI clarity pass (2026-10-21, issue #195): the dashboard shows one compact bit-rate
  field with a unit per direction, and the connected topology source stacks the same
  two rates; the WiFi summary band is two tiles (Connection and Internet) and WiFi / Advanced
  is one list of collapsible sections with one button size; the WiFi mode options sit directly
  below the band without an outer card; the current uplink is a flat status strip over one
  saved-network table in the same card; and the captive-portal warning only appears above the
  band when it has an action; traffic charts stop replaying their animation and state their
  retained window; the app shell owns the only page scroll container; Tailscale, Speedtest and
  SQM leave the nav while their package is absent
  ([ADR 0012](../adr/0012-ui-consistency-and-status-language.md)).
- **Not verified on device:** the 2026-10-05 pass was completed without access to 192.168.1.1
  (host has no route to it), so light/dark rendering, the new band, and history accumulation on
  real hardware still need the standing device checks.

## Code Structure

- **Oversized-file refactor (2026-10-20):** the largest files in the backend and the frontend
  were split into per-domain sibling modules in the same package or directory. The backend's
  `vpn_service.go` (2498 lines), `network_service.go` (2250), `system_service.go` (1653),
  `wifi_service.go` (1425) and `failover_service.go` (1353) became focused files per feature
  (WireGuard, Tailscale, split tunnel, profiles, the shared dnsmasq layer stack, the kill switch,
  WAN, DHCP, DNS, clients, DDNS, port forwards, firmware, backup, SSH keys, buttons, LED, logs,
  the mwan3 section management, the candidate discovery). The API handlers and the OpenAPI spec
  split the same way; the frontend's `use-network.ts`, `use-wifi.ts`, `use-system.ts`, the
  dashboard page, the failover card, the AP form and the MSW mocks split by domain, with the
  original module kept as a re-export barrel so no importer changed.
- The splits are pure moves: every moved declaration is byte-identical, and the served OpenAPI
  spec is unchanged. The VPN lock test now walks the whole package instead of one file, so a
  split can no longer hide a nested config lock, and a new docs gate resolves every `file:line`
  reference in `docs/` so a split can no longer leave a stale pointer behind.
- **Backend cleanup (2026-10-25):** crash guards go through one package, `internal/guard`
  (ADR 0003), replacing seven per-service copies and their silent temp-dir fallback. Legacy
  migrations are gone: the pre-stack dnsmasq snapshots, the pre-stack captive restore path, the
  `enable` / `exit_node` request aliases, `expires_at`, and `/etc/travo` guard clearing. Logging
  is `log/slog` with levels. golangci-lint adds errcheck, errorlint, gocritic, unparam,
  bodyclose and others. The docs gates live in `internal/docsgate`, and docs cite symbols
  instead of `file:line`.
- **Backend domain packages (2026-10-25):** `internal/services` splits into `network`, `wifi`,
  `system`, `uplink` and `failover`, with shared plumbing in `platform` and the wiring in
  `internal/app` ([backend packages](../reference/backend-packages.md)). Every service has one
  constructor; tests sit next to the code they exercise.
- **Generated OpenAPI (2026-10-25):** the hand-written spec (`openapi_paths_*.go`) and its
  1,000-line drift test are gone. Every operation is registered with a typed handler and its
  OpenAPI 3.1 entry is generated from the Go types through Huma's schema registry
  ([ADR 0006 §4](../adr/0006-application-platform-and-api-contract.md)); `go run ./cmd/openapi`
  writes the document. The binder is non-generic, so the binary shrank (15.35 → 15.12 MB) instead
  of growing 3.5 MB with `huma.Register`. The unused `GET /services/adguardhome/status` is removed.
- **Generated API types (2026-10-25):** `make generate` writes `shared/openapi.json` and the TS
  types (`openapi-typescript`, in `tools/openapi-gen`); 95 of the 105 hand-written shared types
  are now aliases of generated ones, and CI fails on stale output. String fields the backend
  only ever fills from a fixed set carry `enum` tags, so the generated unions are exact.
- **API follow-ups (2026-10-25):** every shared API type is now generated or derived from a
  generated one; unused type guards and their tests are gone. Acknowledgements are always
  `{"status":"ok"}`. `API_ROUTES` is type-checked against the generated `paths`.
- **Generated API client (2026-10-25):** the generator is now orval, which needs no TypeScript
  compiler API, so TypeScript 6 is gone from the repo. Every hook calls a generated request
  function; `apiClient` is removed. The port-forward request no longer requires an `id`, the
  WiFi connect request no longer requires `mode`, and the firmware upload documents
  `keep_settings`.
- **On-device contract check (2026-10-25):** on the GL-AXT1800 (OpenWrt 25.12.3) all 69
  parameterless GETs answered 200 and every UI page loaded without console errors, but four
  responses sent `null` where the schema promises an array. Responses are now encoded with
  `jsonx` (nil collections empty), and a mock-mode test validates every GET against its schema.
  Mock mode serves sample logs instead of failing on a missing `logread`.
- **Captive status and uplink display (2026-10-25):** captive status no longer probes without a
  default route (3 s → instant offline), shares one probe between concurrent polls, and treats
  a WireGuard `default dev` route as an upstream ([ADR 0001](../adr/0001-dns-vpn-captive-portal-architecture.md)).
  The uplink connection reports its band from the frequency (iwinfo on 25.12 has none), the
  saved-network row says "Not connected" for an enabled profile that is out of range, and one
  band label helper replaces six copies.
- **Quiet crond (2026-10-25):** the installer sets `cronloglevel=9`, so the per-minute WiFi
  reconnect check no longer writes a `cron.err` line every minute
  ([ADR 0013 §8](../adr/0013-operational-invariants-and-device-findings.md)).
- **Update notifications (2026-10-25):** the header and the System menu show when an OpenWrt
  release or package updates are available. System → Updates links a new release to LuCI's
  Attended Sysupgrade and upgrades packages (apk or opkg) with a streamed log
  ([ADR 0006 §2](../adr/0006-application-platform-and-api-contract.md)).
- **Upgrades replace the running Travo (2026-10-25):** installing v0.9.0 over a running v0.8
  build left the old process serving from the deleted binary (`start` is a no-op for a running
  procd service, and `/api/health` still answered). `install.sh` now restarts a running Travo and
  fails unless the process runs the installed `/usr/bin/travo`; the deployment guide's manual
  steps restart too. The Updates indicator now polls every 30 s until the first check has run.

## Frontend Toolchain

- **ESLint replaced by oxlint, TypeScript 7 adopted.** The `TypeScript 7 — blocked by
  typescript-eslint` hold in [`tasks_open.md`](./tasks_open.md) §17 (and the matching
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
  is unaffected, and CI does not depend on it. See [`docs/guides/development.md`](../guides/development.md).
- **MSW 3.0.2 alongside Vitest 5.0.3.** The §17 hold and the Renovate
  `allowedVersions: "<3"` rule for `msw` are gone. Vitest 5.0.3 is still the latest
  published release (no `@vitest/*` package is newer). `http` and `HttpResponse` import
  from `msw/http`; `setupServer` / `setupWorker` stay on `msw/node` and `msw/browser`.
  `server.listen` and `worker.start` take `onUnhandledFrame`. Published
  `@vitest/mocker@5.0.3` still optional-peers `msw ^2.4.9`. Vitest main widened that to
  `^2.4.9 || ^3.0.0` on 2026-10-05 (commit `fe025b2`) and has not published it. This
  repo does not load the browser-mode mocker that imports MSW; node and jsdom tests
  talk to MSW through `msw/node`, and that suite passes. jsdom 30.1 stays held.

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

- **Connection failover (§2.7) — all four items implemented.** Priority-based WAN source with deterministic generation from `/etc/travo/failover.json`, mwan3 health checks (configurable target), automatic switching to the next source on failure, and event notifications. Service: `failover/failover_service.go`; routes `GET`/`PUT /api/v1/network/failover` and `GET /api/v1/network/failover/events`, all present in the OpenAPI spec. Normative detail: [ADR 0005](../adr/0005-multi-wan-failover-mwan3.md). Device validation with two uplinks is still outstanding; see [`docs/tests/failover-verification.md`](../tests/failover-verification.md).
- **Persistence backend (§16) — decided and implemented as bbolt.** `/etc/travo/travo.db` (bbolt, 0600, 5 s open timeout) replaces the "is SQLite a better fit?" research item; measured at +247 KB on the stripped binary versus ~10 MB for `modernc.org/sqlite`. Degrades to memory-only on open failure. Buckets, retention and the flash-write batching rule: [ADR 0009](../adr/0009-persistent-store-bbolt.md).
- **Historical data (§6.2) — the 6 h ring buffer shipped.** 720 points at a 30 s interval via `GET /api/v1/system/stats/history`, persisted in the bbolt store and flushed every 20 collects. Extending the window is a separate, still-open product decision.

- **Guard directory unified on `/etc/trafo`.** Crash guards were split across `/etc/trafo/` and
  `/etc/travo/`, and one feature straddled both (`captive-dns-in-progress` vs
  `captive-wwan-bounce-in-progress`). A guard written to one directory was invisible to a check
  that only looked at the other, so a stale guard permanently disabled a feature with no log
  line, and `deploy-local.sh` could not clear all of them. All guards now live in `/etc/trafo/`,
  enforced by `internal/docsgate/guards_test.go`. Non-guard state stays in `/etc/travo/`.
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
