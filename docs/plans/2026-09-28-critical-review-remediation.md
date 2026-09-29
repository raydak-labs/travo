---
title: Critical code review remediation — 2026-09-26
date: 2026-09-28
source: 2026-09-26-critical-code-review.md (repo root)
status: in progress
---

# Critical code review remediation

Work derived from `2026-09-26-critical-code-review.md`. Findings are addressed in the
document's suggested order, in dependency-ordered waves. P2 bulk (styling, low-risk
correctness nits) is out of scope for this pass; only P2 items that are security- or
data-loss-relevant are pulled into the relevant lane.

## Ground rules for every lane

- One writer per file. The file ownership lists below are exclusive; touching a file
  owned by another lane is a defect.
- No `wifi` / `wifi up` / `wifi down` / `wifi reload` from any generated script or
  service path (AGENTS.md; ADR 0002 §6). UCI writes + rpcd apply/confirm only.
- Every live-state mutation needs a `/etc/trafo/<feature>-in-progress` crash guard
  (ADR 0003), removed only after success.
- Shell-outs go through `internal/execx` with an explicit timeout tier.
- TDD: add/extend a test per behaviour change; `cd backend && go test ./...` and the
  frontend/shared suites must stay green.
- Do not reformat unrelated files; `make format` is run once at the end.

## Wave 0 — build unblock (parent)

- [x] P0.0: `.mise.toml` pins `go = "1.27.0"` (matches `backend/go.mod` `go 1.27.0`
      and CI's `go-version-file`); the 1.27.1 `klauspost/compress` zstd breakage is no
      longer reachable. An explicit `toolchain` directive is *not* added: `go mod tidy`
      strips it while the toolchain is newer than the `go` directive.

## Wave 1 — P0 injection + clock control (parent, done)

- [x] P0.1 shared `services.ValidateHHMM` (`^([01]\d|2[0-3]):([0-5]\d)$`, no
      whitespace/newline), enforced at the handler boundary (400) *and* re-checked in
      `SetLEDSchedule` / `SetWiFiSchedule`; crontab values can no longer inject a second
      root cron entry.
- [x] P0.2 `services.ValidateButtonName`: allowlist against `detectButtonNames()`
      devicetree labels plus `[A-Za-z0-9_-]`, checked in `SetButtonActions` before the
      0755 root hotplug script is written.
- [x] P0.3 `api.TimeSyncGate`: sticky "clock was plausible" latch +
      `validateClientTimeWindow` (floor-30d … floor+400d) for unauthenticated syncs.
- [x] P0.4 (schedule + button halves) new generated helper
      `/usr/libexec/travo-wireless-toggle.sh` — UCI `wireless.*.disabled` + rpcd
      apply/confirm behind the `wifi-toggle-in-progress` guard. Cron and the button
      hotplug script call it; neither runs `wifi up/down` any more.
- [x] Extra (same file, same boundary): `AddSSHKey` rejects multi-line/garbage keys.

## Wave 2 — parallel backend + frontend lanes

Dispatched as isolated per-lane workflows (one top-level call each). A single fan-out was abandoned:
when one child fails, the runner tears down its siblings mid-edit, which twice left the working tree
not compiling. Every lane got exclusive file ownership plus a resume preamble ("git diff first, finish
only what is missing"), because several lanes were killed mid-flight by provider rate limits.

| Lane | Files (exclusive) | Outcome |
|---|---|---|
| L1 wireless/UCI | `uci/real.go`, `wifi_*.go`, `network_service.go` | `GetSections` returns its error; `SetRadioRole` validates before commit, calls `disableOtherSTASections`, generates a random WPA key; `ensureNamedSection` re-types; `Connect`/`BlockClient` lock + revert staged deltas; `SetMACAddress` guards/verifies/confirms; guest teardown; `KickClient` reports failure; `interfaceType()` emits the five discriminators |
| L2 VPN/failover/captive | `failover_service.go`, `vpn_service.go`, `usb_tethering_service.go`, `captive_autoaccept.go`, `uci_apply.go` | VPN verifies the tunnel before firewall/DNS and rolls back; kill-switch ownership; split-tunnel validation; failover serializes saves, verifies every enabled candidate, staged mwan3 apply; USB-tether WAN zone resolved by name; captive bounce gated on an active `wwan`; `uci_apply` made testable |
| L3 platform/system | `main.go`, `config.go`, `system_service.go`, `service_manager.go`, lifecycle services | Fiber timeouts, 64 MB body limit, deny-all CORS default, shutdown ordering, `sync.Once` lifecycle, tracked goroutines, crash guards for restore/upgrade/factory-reset, bounded log tailing, and **a data race it found in its own refactor** (`refreshOne` wrote the package cache with no lock) |
| L4 auth/WS | `auth/**`, `api/auth_handlers.go`, `router.go`, `ws/**` | No fallback JWT secret; mutex-guarded `passwordHash`; session revocation + token rotation on password change; WS read limit, pong, re-validation, Origin check; `Hub.Stop` closes clients |
| L5a frontend | `frontend/src/**`, `shared/src/**` | WS connects after login; finite `staleTime`; logout revocation; 401-aware transfer endpoints; terminal-status retry stop; repeater rollback; URL/path encoding; external-URL allow-list; stream abort; setup-status cache |
| L5b theming/a11y | frontend components/hooks | `--chart-*` tokens, ARIA labels, menu keyboard semantics, focus-within reveal, `use-session-timeout` reset |

## Wave 3 — gates, tooling, docs

- OpenAPI drift gate (`internal/api/openapi_drift_test.go`): compares every registered route against
  the served spec in both directions and requires a summary plus a 200 response per operation. It
  normalises Fiber's `:param` to OpenAPI's `{param}` — that alone exposed 14 spec entries whose paths
  could never have matched. 41 operations were undocumented; the spec lane fills them and fixes the
  two known-wrong entries (`/adguard/config` body field, `/auth/login` `username`).
- CI: `workflow_call` so a tagged release re-runs CI first; `go mod tidy -diff`; a `-race` job on PRs;
  `format-check`, `shellcheck` and `openwrt-package` jobs; toolchain versions read from `.mise.toml`
  instead of duplicated in CI; `main.BuildTime` stamped on releases.
- Makefile: `-count=1` in `make test`; new `format-check`, `shellcheck`, `integration`, `coverage`,
  `test-race`, `toolchain-versions` targets.
- Shell: the crash-guard list in `deploy-local.sh` rebuilt from the code (it cleared a guard no code
  writes and missed failover/band-switch/captive — a stuck guard permanently disables those features);
  the deploy now fails loudly instead of printing `OK Done` over a dead service; SSH host-key
  verification is on by default (`TRAVO_INSECURE_SSH=1` opts out) in `deploy-local.sh` and all three
  `test/integration/*.sh`; `build.sh` verifies tidiness instead of mutating `go.mod`.
- `packaging/openwrt/Makefile` deleted: it predates the rename to `travo` and referenced a binary, init
  script and config file that do not exist. `packaging/openwrt/files/` is live (used by
  `scripts/package-tarball.sh`) and was kept.
- A tracked build artifact (`shared/tsconfig.tsbuildinfo`) is now gitignored and untracked.

## Wave 4 — verification (parent + reviewer)

`make lint && make test && make build`, `make format-check`, `make shellcheck`, `go test -race`, the
frontend/shared suites, then an independent read-only review of the full diff.

## Known limitations carried out of this pass

- The repeater wizard cannot undo the upstream STA connection: there is no disconnect endpoint in the
  shared contract. It now names the failing step and whether rollback succeeded.
- The WebSocket token still travels in the query string (`/api/v1/ws?token=`), where it lands in
  access logs and browser history. Moving it to a header or `Sec-WebSocket-Protocol` is follow-up work.
- The frontend hook files (`use-network.ts`, `use-wifi.ts`, `use-system.ts`) remain flat lists of
  single-use query/mutation pairs; a `createMutation` factory would remove ~40 % and make the
  missing-`onError` class structurally impossible. Deliberately not done here.
- A WireGuard private key in `test/integration/wireguard-profiles/privado.ams-033.conf` is gitignored
  but present in the working copy: rotate it.
- `install.sh` only installs from a GitHub release and has no local-tarball input, so the
  `git describe`-derived local tarball name is not a real failure path; left as is.

## Documentation record (docs lane, 2026-09-28)

Documentation-as-code pass over the same diff. What changed in the docs:

- **Two new ADRs.** [ADR 0008](../adr/0008-ssh-key-management.md) — SSH key management
  as a root-credential surface (what the endpoint may write, the single-line +
  key-format validation boundary, why it is default-deny behind JWT auth). [ADR 0009](../adr/0009-persistent-store-bbolt.md) —
  the bbolt store at `/etc/trafo/travo.db` (NAND/overlayfs write discipline, persisted vs
  in-memory, memory-only degradation, per-bucket retention). Both indexed in
  [`docs/adr/README.md`](../adr/README.md) and linked from `docs/architecture.md`.
- **ADR 0003 rewritten around an authoritative guard table** (path → owning file → what it
  protects), because the old partial list is what made the documented redeploy recovery
  path unverifiable. It also corrects §1.2: the failover guard-present skip is **silent**
  (`FailoverService.Start` contains no `log` calls at all), so the ADR now says "check the
  filesystem, not the log".
- **ADR 0002 §6.1** documents the generated `/usr/libexec/travo-wireless-toggle.sh` and
  states explicitly that the WiFi schedule *and* the button hotplug toggle both go through
  it. **ADR 0007** drops "blocklist persistence" from the remaining-work list (it shipped),
  adds `travo.db`, session revocation on password change, the removal of the fallback JWT
  secret, and the `main.BuildTime` release invariant. **ADR 0006** records the OpenAPI
  route↔spec drift test as the contract gate plus the 64 MB body limit and the Fiber
  read/write/idle timeouts. **ADR 0004 §5** and **ADR 0001 §4.1** add the two direct
  `commit firewall` exceptions (with their guards) and the conditional `wwan` bounce.
- **Backlog reconciled.** `tasks_open.md` §2.7 (four failover items), §16 (SQLite?) and
  §6.2 (historical data) were implemented or decided; they moved to `tasks_done.md` with
  pointers to the ADRs. The two competing "last updated" markers are now one.
- **Hygiene.** `CLAUDE.md` reduced to a pointer at `AGENTS.md`; `README.md`'s command
  table corrected against the `Makefile` (`make lint` is ESLint + `golangci-lint`, not
  `go vet`; `make format` is Prettier + `goimports`); `CONTRIBUTING.md` now clones
  `raydak-labs/travo` and uses `make install` instead of `pnpm install` + `go mod tidy`.

What this pass **deliberately left open** (see also *Known limitations* above, and
`docs/requirements/tasks_open.md` §15, which is new):

- The entire P2 bulk from the review — styling, contrast, ARIA, oversized-component
  refactors, the P2 correctness nits. Out of scope by the plan's own ground rules.
- **The WebSocket token still travels in the query string** (`/api/v1/ws?token=`). The
  handler now re-validates the session, sets a read limit, requires pongs and checks
  `Origin`, but the token still lands in access logs, browser history and `Referer`.
  Moving it to a header or `Sec-WebSocket-Protocol` is follow-up work.
- **The frontend hook-factory refactor** (`use-network.ts`, `use-wifi.ts`,
  `use-system.ts` as flat single-use mutation lists) is not done. A `createMutation`
  factory would remove ~40 % and make the missing-`onError` class structurally
  impossible, but it is a large mechanical frontend change and was not in any lane.
- **The local OpenWrt `ipk` packaging question.** `packaging/openwrt/Makefile` (dead, it
  predates the rename to `travo`) was deleted; `packaging/openwrt/files/` is live and
  kept. Whether a real ipk should be produced locally, or whether
  `package-tarball.sh` is the only packaging path, is undecided.
- **The untracked WireGuard private key** in
  `test/integration/wireguard-profiles/privado.ams-033.conf` must be **rotated**. It is
  gitignored and unreferenced, but a live VPN identity is sitting in working copies.
- **Two doc↔code gaps the docs now record instead of hiding:** the crash-guard directory
  is spelled `/etc/trafo/` in some services and `/etc/travo/` in others (and
  `deploy-local.sh` clears only the former), and the failover guard skip is silent. Both
  are code fixes, recorded in ADR 0003 and `tasks_open.md` §15.
