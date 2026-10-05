---
title: "ADR 0013: Operational invariants and device findings"
status: Accepted
date: 2026-10-05
updated: 2026-10-05
tags: [adr, operations, invariants, vpn, wifi, hardware, findings]
---

# ADR 0013: Operational invariants and device findings

Rules that cut across subsystems, plus the device findings that are expensive to
rediscover and cheap to lose. The subsystem ADRs own *what* each area does; this
one owns the cross-cutting invariants and the evidence behind them.

It exists because those facts used to live only in executed plans under
`docs/plans/`, which are deleted once their work ships. Deleting the plan must
never delete the rule.

## Status

Accepted.

## Context

A documentation audit of `docs/plans/` found eight decisions or findings that no
ADR and no section of the architecture overview recorded, all of them written down
exactly once, in a plan scheduled for deletion. Each is recorded here instead.

## Decision

### 1. VPN disable restores the default route or fails

Disabling WireGuard leaves the device without a default route more often than the
API shape suggests. The shipped recovery is in `VpnService.restoreDefaultRouteAfterWireGuardDisable`
(`backend/internal/services/vpn_service.go`):

1. If the kernel already has a default route, touch nothing.
2. Otherwise renew every uplink netifd reports as up — discovered from a ubus dump,
   falling back to `[wwan, wan]` — for both the interface and its `6` variant, then
   reload and wait up to 2 s.
3. Still nothing: force a down/up cycle on those uplinks, reload, wait up to 5 s.
4. Still nothing: **return an error**. Success is never reported without a kernel
   default route, and the crash guard stays (ADR 0003).

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

### 8. One retired question, recorded so it stops resurfacing

A plan in March 2026 asked for a measurement explaining VPN disable latency and
what dominates it. No measurement was ever taken and none of the follow-ups depend
on the answer: disable is now a single bounded apply/confirm cycle with a bounded
uplink recovery (rule 1) and returns success only once a default route exists. The
question is retired, not answered.

## Why

Hard to reverse: each of these cost a real device incident or a real design
argument to establish. Surprising without context: the code reads as ordinary
defensive programming, and nothing in it explains that the interface status lying
about routes is a known OpenWrt behaviour or that re-measuring bbolt's cost is
someone else's job. And the result of a real trade-off: in each case the naive
alternative (trust the interface status, page-scoped subscriptions, SQLite for
convenience, the token in the query string forever) was cheaper and wrong.

## Consequences and invariants

- A rule that only exists in a plan under `docs/plans/` does not exist. When a plan
  ships, its normative content moves here or into the owning ADR before the plan
  is deleted.
- Device findings recorded here are evidence, not behaviour. Changing the code
  does not invalidate the finding; superseding it does, explicitly.
- Rule 1 is enforced in code by `hasKernelDefaultRoute` and its two wait windows;
  changing either without re-checking the failover path is a behaviour change.

## Deliberate ceilings

- Rules 4 and 5 describe designs, one built and one planned. Rule 4 is explicitly
  not implemented; do not read this ADR as a description of current auth behaviour.
- Rule 6 documents detection only. Long-press vs short-press and user-configurable
  actions are still open backlog items, not decisions.
