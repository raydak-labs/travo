---
title: "ADR 0013: Operational invariants and device findings"
description: Cross-cutting operational invariants and the device findings behind them: VPN disable route recovery, push wiring, store cost, band switching, buttons, AdGuard.
status: Accepted
date: 2026-10-05
updated: 2026-11-08
tags: [adr, operations, invariants, vpn, wifi, hardware, findings]
---

# ADR 0013: Operational invariants and device findings

Rules that cut across subsystems, plus the device findings that are expensive to
rediscover and cheap to lose. The subsystem ADRs own *what* each area does; this
one owns the cross-cutting invariants and the evidence behind them.

It exists because those facts used to live only in executed plans under
planning documents, which were deleted once their work shipped. Deleting the
document must never delete the rule.

## Status

Accepted.

## Context

A documentation audit of the planning documents found eight decisions or findings that no
ADR and no section of the architecture overview recorded, all of them written down
exactly once, in a plan scheduled for deletion. Each is recorded here instead.

## Decision

### 1. VPN disable restores the default route or fails

Disabling WireGuard leaves the device without a default route more often than the
API shape suggests. The shipped recovery is in `VpnService.restoreDefaultRouteAfterWireGuardDisable`
(`backend/internal/uplink/vpn_service.go`):

1. If the kernel already has a default route, touch nothing.
2. Otherwise renew every uplink netifd reports as up — discovered from a ubus dump,
   falling back to `[wwan, wan]` — for both the interface and its `6` variant, then
   reload and wait up to 2 s.
3. Still nothing: force a down/up cycle on those uplinks, reload, wait up to 5 s.
4. Still nothing: last resort, run the netifd helper scripts (`ifdown`/`ifup`) on
   those uplinks, reload, and wait up to 8 s. On some devices the helper scripts
   re-trigger proto handlers where a ubus down/up alone does not.
5. The function itself returns nothing. `disableWireguard` re-checks
   `hasKernelDefaultRoute` and, if the kernel still has no default route, returns
   `WireGuard disabled but no default route was restored; internet may be down` and
   keeps the crash guard (ADR 0003). Success is never reported without a kernel
   default route.

Worst-case disable latency is therefore bounded by those three waits (2 s + 5 s +
8 s) plus the apply/confirm cycle — do not read step 4 as optional.

Two rules follow. Disable recovery must not assume the uplink is `wan` — on a
travel router the internet often arrives over a WiFi STA link. And the check must
read the *kernel* routing table, not the interface status.

**Device finding (OpenWrt 25.12.1, IPQ6018):** after disabling WireGuard,
`ubus call network.interface.wwan status` still reported a default route obtained
over DHCP while `ip -4 route show table main` contained none, and `wget` exited 4.
netifd's reported state is not evidence that the kernel has a route.

**Related platform rule:** the toggle endpoint once read `enable` while the
OpenAPI contract and the frontend sent `enabled`, so a client asking to *enable*
silently disabled the tunnel and received `{"status":"ok"}`. Request bodies are
tolerated with a documented fallback (`parseToggleEnabled`); see ADR 0010 for the
request-contract rules this follows from.

### 2. Push wiring belongs in a shared hook, never in a page

WebSocket → query-cache wiring once lived inside `useTopologyData`, a
dashboard-scoped hook. Live network state therefore stopped flowing the moment
the dashboard unmounted, and every other page fell back to slower polling without
anything looking broken.

A `subscribe(...)` call belongs in the hook the pages already share
(`use-network.ts` subscribes to `network_status` today), not in a component or a
page-scoped hook. A page that needs a push must consume a shared hook; adding a
second subscription in a page is a defect even when it appears to work.

### 3. The persistent store's cost is measured, not assumed

`go.etcd.io/bbolt` adds **247 KB** to the stripped binary (12.57 → 12.83 MB);
`modernc.org/sqlite` would add roughly **10 MB** on a device whose whole budget is
a ~12.8 MB stripped binary and NAND-backed flash. That measurement is what
selected bbolt (ADR 0009) and it has not been re-taken since; re-measure before
adding any dependency that is not already in the module graph.

### 4. WebSocket token transport is designed, not built

The browser cannot set headers on a WebSocket handshake, so the token currently
travels in the query string. The designed replacement is a single-use,
short-lived (~30 s) ticket from an authenticated `POST /api/v1/auth/ws-ticket`,
passed as the ticket in the query string; accepting the token via
`Sec-WebSocket-Protocol` is the smaller alternative. Neither exists in code, and
moving the token to an `HttpOnly; SameSite=Strict; Secure` cookie is deliberately
bundled with it — it changes CORS setup, WebSocket auth, and every
`Authorization`-header consumer of the OpenAPI document, so it needs an
API-versioning decision of its own (ADR 0007 tracks the auth contract).

### 5. Band switching costs seconds, and scan results are grouped per SSID

Measured on IPQ6018: scanning for both bands while a STA link is up works, and a
band switch is `uci set wireless.sta0.device=radioX` followed by apply/confirm,
with a **2–5 s** connectivity gap. That gap is the reason band switching carries
its own crash guard in `/etc/trafo/` (ADR 0003).

The scan list shows **one row per SSID**, badged *Dual-band* when more than one
radio can see it, with the individual APs behind a disclosure. The band choice is
per connection and persisted; a user switching a STA link to 5 GHz expects it to
stay there across reconnects.

### 6. Hardware buttons are detected from devicetree

Button detection reads `/sys/firmware/devicetree/base/keys` — each sub-directory is
a physical key and its `label` property is what OpenWrt exports as `$BUTTON`.
`/etc/rc.button` ships generic stock scripts for every common button type on every
device and is deliberately not used as a detection source.

Any future persisted button→action mapping must live under `/etc/trafo/` with the
rest of the ordinary state, and must not re-create the pre-unification
`/etc/openwrt-travel-gui/` spelling (ADR 0003).

### 7. AdGuard DNS defaults to forwarding mode

AdGuard's default mode is **dnsmasq forwarding** to AdGuard Home (its listener on
5353). Making AdGuard the primary resolver for the LAN is the advanced mode and
must warn the operator, because it changes the resolution path for every client
and composes with the VPN DNS layer stack in ADR 0001.

### 8. crond logs at level 9

BusyBox crond logs every job start (`USER root pid … cmd …`) as `cron.err` at
OpenWrt's default `-l 7`. With the WiFi reconnect check in cron every minute, the
system log gained an error line per minute (GL-AXT1800, OpenWrt 25.12.3: 18 since
boot), which buried real errors under the Logs page's error filter. The installer's
`98-travo-cron-loglevel` sets `system.@system[0].cronloglevel=9` unless the operator
set one. On the same device, after the change, the job still ran each minute and no
`cron.err` line followed. Level 9 still logs cron's own failures.

### 9. One retired question, recorded so it stops resurfacing

A plan in March 2026 asked for a measurement explaining VPN disable latency and
what dominates it. No measurement was ever taken and none of the follow-ups depend
on the answer: disable is now a single bounded apply/confirm cycle with a bounded
uplink recovery (rule 1) and returns success only once a default route exists. The
question is retired, not answered.

### 10. Preserve Travo's boot enablement across sysupgrade

Sysupgrade must preserve `/etc/rc.d/S99travo` alongside the init script and binary. Preserving
`/etc/init.d/travo` alone is insufficient: on GL-AXT1800 after upgrading from OpenWrt 25.12.3 to
25.12.6, the binary still reported version 0.10.0 and the init script and enabled UCI config were
present, but `/etc/rc.d/S99travo` was missing. procd reported the service inactive and no API
process answered. Running `/etc/init.d/travo start` restored the service (`/api/health` returned
`{"status":"ok"}`); running `enable` recreated both `S99travo` and `K10travo` links.

The release keep list therefore includes `/etc/rc.d/S99travo`. This preserves the established
boot-enabled state when firmware upgrade does not retain the generated enablement symlink.

### 11. rpcd applies only what is pending in the calling session

`ubus call uci apply` acts on the configs that have a pending change in the calling session's
delta. It ignores files copied into `/var/run/rpcd/uci-<session id>/`, which older rpcd read.
rpcd 2026.10.04 (OpenWrt 25.12.6) names that directory after a private random value
(`rpc_session_new` in `session.c`) that no ubus method returns, so a session id cannot be mapped
to it. With nothing pending, apply answers `UBUS_STATUS_NO_DATA` and the `ubus` CLI prints
`Command failed: ubus call uci apply {...} (No response)` (#220).

Observed on the device: a session created with `ubus call session create` and a `uci set` through
it produced `/var/run/rpcd/uci-87ae2895...` while the session id was `180ae843...`; copying
`wireless` into `uci-<session id>` by hand and applying gave exactly the reported error.

Travo commits before it applies (ADR 0002 §5.0), so the real edit is never in the session.
`StartApply`, the generated toggle helper and `scripts/setup-wireless-ap.sh` therefore add and
delete a `travo_apply` marker section per config through the session. The delta is non-empty and
its net effect on the config is nothing. Do not copy files into rpcd's run dir to stage an apply.

Two consequences of rpcd committing every staged config, both checked in rpcd's `uci.c` and on a
scratch copy, not on the device's live files: the commit re-exports each config through libuci,
so hand-written comments in `system`, `firewall` or `dhcp` are dropped (the device's files hold
none, so their checksums did not change), and each commit fires a `config.change` event, so the
services behind all five configs reload on every wireless apply. A second `StartApply` while a
first apply is unconfirmed reuses that pending session (`session login` reclaims it) and its
`uci apply` is refused. Failover guards against this with its pending-session check; the wireless
write lock is released before confirm, so the wireless path has no such guard.
Writing the real edits through rpcd's own `uci set/add/delete` is the cleaner design and is
tracked separately.

### 12. `GetSections` returns list options comma-joined

`parseShowOutput` (`backend/internal/uci/real.go`) joins list elements with `,`, so the wan
zone's `network` list reads `wan,wan6,wwan`. `MockUCI` joins with a space. Membership tests on
a list read through `uci.UCI` go through `uci.SplitListValue`, which accepts both; a bare
`strings.Fields` never matched on a device, and `ensureWwanFirewall` appended `wwan` to the
wan zone on every connect (16 copies, #262). It now also collapses existing duplicates once.

## Why

Hard to reverse: each of these cost a real device incident or a real design
argument to establish. Surprising without context: the code reads as ordinary
defensive programming, and nothing in it explains that the interface status lying
about routes is a known OpenWrt behaviour or that re-measuring bbolt's cost is
someone else's job. And the result of a real trade-off: in each case the naive
alternative (trust the interface status, page-scoped subscriptions, SQLite for
convenience, the token in the query string forever) was cheaper and wrong.

## Consequences and invariants

- A rule that only exists in a planning document does not exist. When work ships, its
  normative content moves here or into the owning ADR before the document is deleted.
- Device findings recorded here are evidence, not behaviour. Changing the code
  does not invalidate the finding; superseding it does, explicitly.
- Rule 1 is enforced in code by `hasKernelDefaultRoute` and its three wait windows
  (2 s / 5 s / 8 s); changing any of them without re-checking the failover path is
  a behaviour change.

## Deliberate ceilings

- Rules 4 and 5 describe designs, one built and one planned. Rule 4 is explicitly
  not implemented; do not read this ADR as a description of current auth behaviour.
- Rule 6 documents detection only. Long-press vs short-press and user-configurable
  actions are still open backlog items, not decisions.
