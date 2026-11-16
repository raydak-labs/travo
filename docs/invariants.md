# Invariants

MUST rules. Each came from a real incident or device finding (see [device-facts.md](device-facts.md)). A change that breaks one needs an ADR first. `[HW]` = proven on hardware only.

Paths: guards in `/etc/travo-greenfield-guards/`, state in `/etc/travo-greenfield/`, settings in UCI `/etc/config/travo-greenfield`. Guards never live in `/etc/travo-greenfield/`.

## 1. Live-state safety

- Any automated action that changes LIVE state (connectivity-affecting commit, apply, firewall, routes, radios, package ops, sysupgrade, factory reset, restore) writes a guard file FIRST and removes it ONLY after full success.
- Guard present → the feature SKIPS. Stale guards are reported at startup (a stuck guard is silent feature death).
- Manual redeploy/reinstall is the explicit retry path; it clears guards by NAME, never by glob.
- Guard write fails → abort the operation. Never degrade to unguarded.
- Confirmed reboot writes `reboot-in-progress` and its boot record before scheduling. After scheduling succeeds, keep both until the next daemon start proves a different kernel boot id; setup or scheduling failure releases the guard, and a same-boot guard becomes retryable only after `reboot.Grace`.
- Firmware-upgrade and factory-reset guards are never removed (interruption must stay visible).
- Every shell-out has an explicit timeout tier. Exceptions: terminal ops (reboot, poweroff, sysupgrade, firstboot) and long-lived listeners (stopped by lifecycle signal, not timeout).
- Every background worker is registered in one lifecycle. SIGTERM stops all; no UCI commit after stop.
- Handler panic → 500; the process survives.
- Wireless changes: snapshot → commit/apply → rollback watchdog → explicit user confirm, only after proof the device is still reachable. The backend NEVER self-confirms.
- Apply paths write UCI only. Never `wifi`, `wifi up/down/reload`. Only a bounded, documented `wifi up` exception exists (auto-reconnect, capped).

## 2. Lockout safety

- A mutation that would leave the caller with NO enabled AP while the caller is on WiFi → 409 `wifi_lockout_risk` BEFORE any write. Override only with `acknowledge_lockout: true`.
- Caller classes: ethernet / wifi-ap / wifi-client / unknown. Unknown FAILS CLOSED (refuse unless acknowledged).
- Affected mutations: mode→client, radio disable, role change to none/sta, AP disable, guest disable.
- Must NOT block: SSID change, key change, disabling one radio while another AP is up, disabling guest while another AP is up, anything over Ethernet.
- Verify on a device, not only in unit tests (a stranded operator once).

## 3. Auth and access

- Admin login = root password validated via rpcd `session.login`. Mock mode: local check.
- Cookie session (HttpOnly, SameSite=Strict) + CSRF header. Live push is SSE on the same cookie. No tokens in query strings.
- Session validity uses the MONOTONIC clock; the API returns only a relative `expires_in`; clients never compare server timestamps with their clock. Lifetime 24 h.
- Protected routes default-DENY. The public list is a small explicit set (spec, health, login, first-run, time-sync). Adding a public route is a security change.
- Routing is case-sensitive: `/API/v1/...` → 404 (a case-variant path once bypassed auth).
- Password change → revoke ALL sessions (persisted epoch file), return a replacement session. Min length 8. Logout/revocation survives restart; only hashes stored, never tokens.
- No fallback signing secret; entropy failure is fatal.
- Rate-limit login and pre-login endpoints; periodically sweep per-IP state.
- Optional IP allowlist (CIDR); health/login/time-sync bypass it.
- Pre-login time-sync only while the router clock is implausible; sticky once plausible (a 1970 rewind once reopened it); bounded window; rate-limited. Authenticated callers unbounded.
- Root SSH keys: write only dropbear `authorized_keys`; append exactly one validated line; reject `\n`/`\r`; strict key-type format; no options field (`command=`, `from=`); list/add/delete behind auth; not revoked by redeploy or password change.
- Admin reachable on `br-lan` only (wired + AP WiFi). Uplink zones and guest get no admin access. Enforced by firewall AND daemon source check (by `RemoteAddr`, never forwarded headers). No reverse proxy on loopback in front of the daemon: loopback is trusted. No default password, ever. The `Host` header must name the router (its lan IP, loopback, `localhost`, or an allowlisted name), else 421: DNS rebinding must not reach the API.

## 4. API contract

- REST JSON under `/api/v1`. Health `GET /api/health` public. OpenAPI 3.1 at `GET /api/openapi.json` (public), GENERATED from typed handlers, never hand-written; drift fails CI.
- Every operation has a unique id and summary. Every route is in the spec. No route outside the registration path.
- Whole-config PUT is STRICT: unknown field, null, trailing data → 400 naming the field. GET and PUT shapes are symmetric.
- Validate ranges/enums explicitly. Silent acceptance of a bad value is a defect.
- Errors: `{error, code?}`; stable `code` for client branching, never message text. Ack: `{status:"ok"}`. Empty collections encode `[]`/`{}`, never `null`.
- Missing optional package → 503 naming the package, not 500.
- Limits: body 64 MB, read timeout 5 min, write timeout 45 min (package streams), idle 60 s, graceful shutdown ~20 s; HTTP drains BEFORE services and persistence close.
- Long ops stream (NDJSON-like). Live status via one shared push subscription, never per page.
- GET never mutates.

## 5. Persistence

- UCI is the source of truth for device config; no shadow state. Derived JSON in `/etc/travo-greenfield/`; crash-critical records in `/etc/travo-greenfield-guards/`.
- Flash is NAND overlayfs: per-sample/per-request writes FORBIDDEN; batch; rare event-driven writes may write through. Keep ~20 MB free.
- Atomic writes for every state file (temp + fsync + rename). A torn file fails closed, never reads as "empty".
- Degraded persistence must be visible in the UI.
- Backup/restore = config tarball; restore over `/` is guarded. Factory reset is guarded.
- No embedded KV store in 0.1. Add one only with a retention rule for the data.

## 6. UCI write discipline

- ONE process-global staged-changes area. Concurrent writers of one config destroy each other's staged sections.
- All writes go through one helper: lock the whole config set ONCE, revert the staged delta on failure, commit once.
- Locks are keyed on CONFIG NAME (not service; `firewall`/`network` are shared). Global sorted lock order. Locks are NOT reentrant; a helper called inside a transaction must not lock.
- Read paths are unlocked.
- Known gap: external writers (cron/hotplug scripts) sit outside the model. Prefer helpers that call the daemon.

## 7. Network and routing

- Candidate uplinks: `wan`, `wwan` (STA), `usbtether`. Omitting one yields a false "No Internet".
- "Internet reachable" is a real probe; carrier-only is a separate signal and must not be labelled "Internet". Route checks read the KERNEL table, never netifd status (see device-facts §1).
- Throughput charts select interfaces by NAME, never index 0 (`br-lan`).
- Tunnel state has exactly 5 JSON values: `disabled` / `configured` / `enabled_not_up` / `no_handshake` / `connected`. Evaluate these rules in order:
  1. An absent `network.wg0` WireGuard interface or `disabled=1` means `disabled` without a selected profile, otherwise `configured`. Absent, empty or `0` enables the interface; any other `disabled` value is a read failure.
  2. For an enabled interface, an absent netifd object or `up=false` means `enabled_not_up`. An absent netifd object is expected for a disabled interface and is not an error.
  3. If netifd is up but the WireGuard device vanished, the state is `enabled_not_up`.
  4. With netifd up, no handshake or newest handshake age ≥180 s means `no_handshake`; `0 ≤ age < 180 s` means `connected`. The 180 s threshold is WireGuard's `REJECT_AFTER_TIME` from `drivers/net/wireguard/messages.h`. Negative age is a read failure.
  - The newest handshake across peers is the evidence; `up:true` alone is not. The kernel default route and `allowed_ips` are not state inputs (split tunnel may omit a default route).
  - A read failure never becomes `disabled`, `enabled_not_up` or `no_handshake`: keep the last good observation with its original timestamp marked stale for at most 10 minutes, then return 503 `vpn_status_unavailable`.
  - An idle tunnel without keepalive can legitimately read `no_handshake` even when its configuration is otherwise healthy.
- WAN zone is resolved by name, never by index.
- A new zone/forward/interface gets the full firewall surface (zone, masq, forwarding, input/output/forward policy) matching the `wan` pattern.
- Direct firewall commits outside the apply flow are named exceptions, each with guard + rollback.
- Wireless snapshot sets include firewall + dhcp.

## 8. DNS layering

dnsmasq on :53 is the single DNS authority. Features that change DNS push/pop a layer on ONE ordered stack.

- Layers: `captive`, `vpn`, `adguard` (reserved: tailscale). Base state is captured before the first layer. No per-feature snapshot of the same options (`server`, `noresolv`, rebind protection, WAN DNS fields).
- Enable is idempotent. Disabling a non-top layer removes only its own entry. Last layer out restores base and deletes the record.
- Restore never reports success while a bypass could still be in force. Missing/corrupt record → refuse + keep guard (retry).
- Status reads never mutate DNS. Only an explicit POST or a scheduler with its own guard mutates.
- VPN self-heal on status read only for terminal "disabled"; "enabled-not-up" needs debounce (N readings over T seconds). A broken probe is not a broken tunnel.
- The `vpn` layer is owned by a reconciler (every 15 s, `internal/vpndns`), never by the tunnel transaction. It pushes the selected profile's `DNS =` resolvers with `noresolv=1` (no leak to the uplink resolvers) only on `connected` and only while no transaction awaits confirmation; it pops on `disabled`/`configured` or when the profile has no resolvers, also during a pending transaction (popping returns to the uplink resolvers, the safe side). `enabled_not_up`, `no_handshake`, stale or unreadable inputs change nothing. A stack change that kept its guard stops the reconciler until the explicit retry.
- DNS verify (`POST /api/v1/vpn/dns/verify`) checks, in order: the layer stack has no unfinished change, the `vpn` layer is active and on top, dnsmasq's live `server` list equals the tunnel resolvers with `noresolv=1`, then resolves a fresh random label through `127.0.0.1:53`. NXDOMAIN counts as success (an upstream answered); timeout or SERVFAIL is `lookup_failed`.
- Known gap: resolvers come from the SELECTED profile; selecting another profile without re-applying the tunnel switches DNS to its resolvers on the next reconcile.
- Known gap: layer record survives but the tunnel config is deleted → heal never runs.
- Captive bypass is the `captive` layer: the active uplink's resolvers with rebind protection off. It never edits WAN DNS fields or the AdGuard upstream, so the stack's base is the only backup; never record the dnsmasq server list outside the stack. Auto-restore after 5 min (monotonic, memory-only deadline; a daemon restart re-arms a full 5 min, see [adr/0010](adr/0010-dns-layer-stack.md)).
- AdGuard: liveness probe resolves a fresh uncached name via `127.0.0.1:5353`. After M failures the `adguard` layer is popped (fail OPEN), a loud banner shows, and it does not re-enable until the user confirms or a probe passed for a hold-down period.
- AdGuard: forwarding written outside the stack (old Travo's base) is adopted as the `adguard` layer before any pop; turning AGH off pops the layer before stopping it.
- AdGuard binds loopback only, has no accounts, and is reached through the Travo-gated proxy. No first-run wizard is ever exposed on the LAN.

## 9. Multi-WAN failover (mwan3, IPv4 only; post-0.1)

- Source of truth is Travo's failover JSON, generated into `/etc/config/mwan3`. Generated sections are prefixed `travo_`.
- Ownership = PROVENANCE (fingerprint stamped by this build and still matching), never content. A non-namespaced section is NEVER deleted.
- Operator-edited `travo_` section: overwrite Travo-owned options with a LOUD warning.
- Back up EVERY section a save may write; restore on failure.
- Staged apply: apply with rollback → verify every managed section → confirm (not confirm-then-verify).
- Failback hold-down 30 s; walk the priority list, first uplink past hold-down wins (not "keep current first").
- Liveness from netifd interface status, not ping. `family: ipv4` explicit; no IPv6 policy until device-validated.
- Guard during the apply window. Mocks must match real UCI semantics (`set` = create-or-update).

## 10. WiFi model

- Roles: uplink STA (`mode=sta`, bound to `wwan`, in `wan` zone), downlink AP(s) (shared SSID/key default), guest AP (own subnet 192.168.2.0/24, own zone; refuse subnet overlap).
- Max ONE enabled STA bound to `wwan`. Multiple saved upstreams = ordered priority list, one active.
- Saved uplinks never write UCI on save, reorder or forget; select is a Join on a copy of the stored credentials and never rewrites the list. A saved uplink is connected only when the managed STA has carrier and `iwinfo info` reports its SSID (and BSSID when pinned); a configured SSID alone never counts. Keys never appear in list responses, errors or logs.
- With ≥2 radios, STA and AP go on DIFFERENT radios. Same-radio is an opt-in escape hatch, visible in health. Refuse STA enable on a radio carrying an enabled AP (409) unless single-radio/opt-in.
- Radio pick is deterministic (sorted by name, prefer no-AP). "Uplink radio" = radio of ANY enabled `mode=sta` iface.
- Layout reconcile happens BEFORE a single commit (stage STA, reconcile AP, commit once). Never commit AP+STA on one radio, even transiently.
- Health endpoint reports fragile layouts and invariant violations.
- Auto-reconnect has a failure counter so bad credentials are not retried forever; the counter is not a guard and is not cleared by redeploy.
- Band switching: persisted choice, guarded, a refused tick leaves no guard behind.
- Scan: one row per SSID, dual-band badge, per-AP disclosure. Scan must not commit the uplink onto an AP radio.
- Scan is read-only, one radio at a time, target = first interface ifname else radio name, concurrent requests join the in-flight scan.
- Wireless scan replies use `TierScan`, not `TierRead`.
- MAC clone/randomize; per-client MAC policies.
- Radio hardware status is CONFIGURED state, labelled as such.
- Post-apply probe proves each enabled AP/STA interface individually (see device-facts §2). Failure → rollback. "Could not read netifd" is distinct from "AP not up".
- Join replaces one STA on wwan in place. It chooses the first band-compatible radio by name with no enabled AP, returns `wifi_no_free_radio` otherwise, and requires STA carrier and a kernel default route via that STA before arming confirmation. The STA kernel link index must be absent from the snapshot taken under the transaction lock before staging and remain stable across the carrier and route checks.

## 11. VPN

- WireGuard: profiles (import, list, activate, delete), enable/disable, kill switch, split tunnel, DNS verify, DNS leak test, speed test. Profiles in `/etc/travo-greenfield/`, tunnel in UCI `network.wg0`.
- Enable: stage the selected profile in the WireGuard transaction and prove a new handshake from one configured peer before confirmation. The transaction owns the wg0 interface and wireguard_wg0 peer sections. Enable also stages the VPN zone `travo_vpn` and forwarding `travo_vpn_lan` (the kill switch's zone sections, `vpnkill.VPNZone`) in the SAME transaction (`network` + `firewall`, one snapshot, one confirm/restore): without a zone holding wg0, fw4 rejects LAN forwards into the tunnel and LAN clients lose internet while the router stays online (device-facts §1). Enabling an already matching tunnel whose zone is missing stages only the zone and does not restart wg0. Disable keeps the zone. The kill switch rules and DNS are separate operations.
- Disable must prove the wg0 link is absent and the kernel has a non-wg0 default route before confirmation. Recovery waits are bounded and never assume the uplink is `wan`.
- Split tunnel: the WireGuard transaction also owns `network.travo_split_<n>` route sections (`interface wg0`, `target` CIDR). With any present, every `wireguard_wg0` peer has `route_allowed_ips=0`, so only those IPv4 prefixes (≤32, no `0.0.0.0/0`, no IPv6) plus a `/32` for each uncovered profile `DNS =` resolver use the tunnel; the uplink keeps the default route. None present means a full tunnel (`route_allowed_ips=1`). Re-enabling or re-applying a profile keeps the selection. On an enabled tunnel the change restarts wg0, proves a new handshake, then proves the kernel default: via a non-wg0 uplink for split (bouncing the unique uplink if leaving full mode dropped it, device-facts §1), via wg0 for full. On a disabled tunnel it only stages UCI. With the kill switch on, LAN clients reach only the split prefixes.
- Kill switch (`internal/vpnkill`) is firewall state in a `firewall` transaction the user confirms. Travo owns sections by NAME only: zone `travo_vpn` (network `wg0`, masq, mtu_fix, input/forward REJECT, output ACCEPT), forwarding `travo_vpn_lan` (lan → travo_vpn), and one rule `travo_killswitch_<zone>` (src lan, proto all, REJECT, both families) per zone holding `wan`/`wan6`/`wwan`/`usbtether`. fw4 evaluates rules before zone forwardings, so the user's own lan → wan forwarding is never edited. A foreign section on an owned name, a missing `lan` zone or no uplink zone → 409 `vpn_killswitch_unsupported` before any write.
- Kill switch policy: enabling requires a fresh `connected` tunnel (409 `vpn_not_connected`) and its probe re-proves `connected` after the firewall reload. Once on it is STRICT: it holds across disconnects, `enabled_not_up`, a disabled tunnel and from boot (UCI, loaded by fw4 before wg0 is up); LAN clients then have no internet until the tunnel is back or the user disables it. Disabling is always allowed and removes only the rules; the VPN zone stays so LAN traffic keeps using the tunnel. The router's own traffic (WireGuard handshakes, dnsmasq upstream) is output, not forward, and is not blocked. The admin UI on br-lan is input and stays reachable. Only the `lan` zone is covered; a future guest zone needs its own rule (not covered in 0.1 until a guest zone lands).
- Tailscale (post-0.1): runs with `--accept-dns=false`; its resolver is a dnsmasq layer.

## 12. Captive portal

- Detection: HTTP probe to a known 204 endpoint + redirect inspection.
- Probe skipped when no default route (instant offline). Concurrent callers share one probe; cache ~2 s; a DNS change invalidates the cache. DNS-layer composition wires `dnslayer.Service.AfterApply` to the shared connectivity service's `Invalidate`, including when an apply fails after changing live DNS.
- Auto-accept (post-0.1) walks portal steps. Only step 0 may renew the wwan DHCP lease, only when wwan is the active uplink, guarded, bounded waits.
- Warning card only when an action exists (login, DNS bypass active, connected-but-no-internet).

## 13. Package ops

- Package manager (apk on 25.x, opkg older) is detected at runtime behind a `PackageManager` port; never hardcoded. Package lists live in `/tmp`: refresh before install, best effort.
- Install writes init scripts, `/etc/config`, kernel modules → guarded, with a flash-space warning.
- Missing optional package → 503 naming it. A failed presence check is not "absent".

## 14. Updates and releases

- Travo self-update: signed release (minisign public key embedded), download to `/tmp`, verify, keep previous as `.old`, atomic swap, procd restart, health check, automatic rollback on failure. Guarded. No background phone-home; on-demand and at most daily on page open. Release source configurable.
- After any upgrade verify the RUNNING exe path, not just file existence; restart, not start.
- Sysupgrade keep-list includes the init script, the boot link `/etc/rc.d/S99travo`, binary, and UCI enable.
- Build stamps version and build time into the binary; build time is the time-plausibility floor. A missing stamp is a release defect.

## 15. Footprint and ops

- Single self-contained binary per arch, stripped, no runtime deps. Budget ~12.8 MB stripped; measure the delta before adding a dependency. Keep ≥20 MB flash headroom.
- Cron log level 9 (keep an operator's own setting) so per-minute job starts don't flood the error log.
- Install requires an explicit `--password` or the first-run screen; the installer verifies the service answers on its configured port before reporting success, and refuses to move LuCI otherwise.
- Contract tests: scripts parse, guard paths under `/etc/travo-greenfield-guards/`, every route in the spec, auth covers every route (walk the live route table), strict PUT coverage, spec field names match models, no feature writes UCI outside the transaction module.
- Record a decision with evidence (firmware version + captured payload) for every device finding.

## Deliberate ceilings

Accept; do not "fix" silently.

- IPv6 failover deferred (IPv4 only). Needs a device matrix + decision record.
- Traffic history ~10 min in memory; resets on restart.
- Session registry is memory-only (cross-restart revocation via epoch only).
- Positional SSH key index (breaking to change).
- Dashboard rate = 2 s sample, not an average.
- Config locks are held across blocking reload/apply; long holds queue all writers of that config.
- Hardware button = detection only until actions are designed.
- External writers (cron/hotplug) outside the lock model.
- Reboot exclusion has a residual ~2 s window where `txn.Begin` can start after the second guard check; boot-time `txn.Recover` restores that transaction.
- AdGuard sees all queries from 127.0.0.1 (no per-client stats) under the default topology.
