---
title: UCI write serialisation and request-body contracts
description: One-list mutate helpers, globally ordered config locks, and why config PUTs must reject unknown fields.
updated: 2026-09-30
tags: [adr, architecture, uci, api]
---

# ADR 0010 — UCI write serialisation and request-body contracts

Two invariants that are load-bearing, easy to violate from outside
`backend/internal/services`, and whose violation is silent rather than loud.

## 1. The UCI staged-changes file is process-global

The `uci` CLI keeps uncommitted changes in `/tmp/.uci/<config>/changes`, shared
by every writer on the device — including `uci` invocations this backend shells
out to. It is not namespaced per process, per request, or per service.

Consequences:

- Two writers of the same config interleave and lose each other's staged
  sections. The loser's own `uci set` then fails with `uci: Invalid argument`,
  because the section it just created is no longer in the document.
- A write sequence that fails half-way must **revert**, or the abandoned delta is
  committed by the next unrelated writer of that config — turning a change the
  API reported as failed into the running config.
- Keying a mutex on the *service* does not help: `WifiService` and
  `NetworkService` both write `firewall`, and `CaptiveService` and
   `VpnService` both write `network`.

### The rule

Every mutator goes through `mutateUCI` (or `mutateWireless`), which takes the
**config list once** and derives both the lock set and the revert set from it.
"Locks a config but forgot to revert it" is then unrepresentable rather than a
convention repeated at every call site.

```go
return mutateUCI(n.uci, []string{"dhcp"}, func() error { /* … */ })
```

**Name every config the flow can reach, not just the obvious one.** The
non-obvious cases found on real hardware:

| Flow | Configs | The easy-to-miss one |
| ---- | ------- | -------------------- |
| `VpnService` toggle / config / import / split-tunnel / profile activate | `dhcp`, `firewall`, `network` | `enableVpnDNSForwarding` shells out to `uci … dhcp.@dnsmasq[0]` directly, so it is invisible to any audit of `v.uci.Set(…)` |
| `NetworkService.AddDNSEntry` | `dhcp` | shares the delta with the DHCP-reservation writers |
| `NetworkService` port forwards | *(none — plain file)* | the read-modify-write is over `/etc/travo/port-forwards.json`, so the config lock cannot cover it; it needs its own mutex and an atomic write |

### Locking rules

- **Order is global.** `lockUCIConfigs` sorts its inputs, so every path acquires in
  the same order and the wait-for graph is acyclic. Without the sort, two flows
  needing `{network, firewall}` in opposite orders deadlock the moment they meet
  — and since every other endpoint queues on the same per-config locks, that
  takes the whole API down, not just the two flows.
- **Not reentrant.** A flow that holds a config must not ask for it again. So a
  helper reachable from inside a transaction must simply **not lock**: in this
  codebase `setupWireGuardFirewall`, `teardownWireGuardFirewall` and
  `removeVPNOwnedKillSwitch` are called only from inside the VPN transaction and
  take no lock of their own. Do not add one. The AST-walking test
  `TestVPNFlowsDoNotNestConfigLocks` fails the build if a transaction can reach a
  locking helper, and `TestVPNFlowConfigSetCoversShelledOutUCI` fails if the
  transaction's config set stops covering what the flow reaches.
- **Shelled-out writers cannot revert.** `AdGuardService.SetDNS` and
  `USBTetheringService.Configure`/`Unconfigure` invoke `uci` directly, so they have
  no `uci.UCI` to revert through. They use `withConfigLocks`, which takes the same
  ordered locks but does not revert: a failure leaves their staged delta for the
  next writer of that config. Prefer `mutateUCI` wherever a `uci.UCI` is
  available.
- **Held across blocking work** — a commit, an rpcd apply, an init.d reload — so a
  slow reload blocks other mutators of the same config. Do not add a second reload
  inside a mutator. Note the hold time is **not** uniformly short: the VPN
  transaction spans `applyAndVerifyWireGuard` (3 attempts, 12 s each) and
  `restoreDefaultRouteAfterWireGuardDisable` (2 s + 5 s + 8 s route waits), and
  every `execx.Slow` call allows 3 minutes. A hung procd call therefore holds
  `{dhcp, firewall, network}` for minutes, queuing every WiFi, captive, DNS-entry,
  DHCP and firewall write behind it. Reads stay unlocked, so nothing corrupts and
  the API does not die — but the captive-portal DNS flow (`BypassDNS` /
  `RestoreDNS`, which locks `{network, dhcp}`) competes with it.
- **Lock order with the service mutex:** `WifiService.uciWriteMu` first, then the
  config locks. Never the reverse.

### Read paths stay unlocked

Only writers take the lock. `GetSections`, `Get`, and `GetAll` are safe to run
concurrently and must not be moved inside a write transaction for convenience.

## 2. Config endpoints must reject unknown request fields

A handler that binds a whole configuration struct and persists it will, with
Fiber's permissive `c.Bind().Body()`, decode a body that does not match the shape
into the **zero value** and then answer `200` after overwriting the user's
settings with zeros.

`PUT /api/v1/wifi/band-switching` was the live example: `GET` answers
`{"config":{…},"status":{…}}` and `PUT` bound the bare struct, so the natural
client — read the documented GET shape, change a field, PUT it back — silently
zeroed `preferred_band`, both thresholds and both delays. Found on the device.

Therefore:

- Use `api.BindStrictBodyConfig` for any endpoint that persists a whole
  configuration. It rejects unknown fields, wrong JSON kinds, and trailing
  content, so a mistake is a `400` naming the field.
- **Keep `GET` and `PUT` shapes symmetric.** If `GET` returns an envelope, `PUT`
  accepts that envelope (and may also accept the bare form for compatibility).
- Validate ranges and enums explicitly. Silent acceptance is not neutral:
  `storage_percent: 500` persists a threshold nothing can reach, so the alert
  never fires while the UI shows a saved configuration; `preferred_band: "9g"`
  persists a band the switcher can never match.

### The spec is part of this

A spec that names a request field the handler does not accept breaks generated
clients, and the 400 it produces names the *handler's* field, not the mismatch.
`TestOpenAPIRequestFieldNamesMatchModels` compares the spec's declared request
keys against the JSON tags of the struct each handler binds, by reflection, for
every documented config endpoint. It needs no running app.

It has already caught six real drifts: `hostname` vs `name` (DNS entries, DHCP
reservations), `leasetime` vs `lease_time` (DHCP), `proto`/`ipaddr`/`dns` vs
`type`/`ip_address`/`dns_servers` (`PUT /network/wan`), `on_cron`/`off_cron` vs
`on_time`/`off_time` (`PUT /system/leds/schedule`), and two PUTs
(`/network/failover`, `/vpn/wireguard`) that documented no request body at all.

Its table is a **maintained list, not an exhaustive derivation**: it names the
model each handler binds, and a handler that binds something *other* than the
model in its row is not detected. Add the endpoint to the table when you touch it.
Two separate tests guard the two halves: `TestOpenAPIRequestBodiesAreAccepted`
drives the documented example through a live app, and only covers the endpoints
whose handlers can be exercised off-device.

## 3. A missing optional package is 503, not 500

If an operation needs a package that is not installed — `ddns-scripts`,
`tailscale` — the request is well-formed and the server is healthy. Answer
`503` with the package name. `GET` should also report availability so the UI can
explain itself instead of offering a form whose only possible outcome is an
error. `ddns` is not in the service catalog, so there is no install button to
offer; the message has to name the package.

## 4. A handler panic must not kill the process

`recover` middleware is registered first, before CORS and auth. Without it, a nil
dereference in any of the ~250 handlers leaves the router forwarding traffic
while the UI is gone, on a device that may have no supervisor. A nil dependency is
the easy way to trigger this, which is exactly what happens when a contract test
exercises an endpoint whose service is not wired.

## 5. Known residual risks

Stated rather than left implicit. None of these is solved by the config locks.

1. **The rpcd rollback window outlives them.** `StartApply` snapshots whole config
   *files* into the rpcd session dir and arms rpcd's own 30 s timer, which can
   outlive the request that armed it. A change committed to one of those configs
   after the snapshot is reverted when the window expires — and the browser not
   confirming is the common case, not the rare one, because confirming is what
   needs the new SSID to be reachable. The config locks cannot help: `revertUCIConfig`
   reverts a *delta*, while rpcd restores *files*. The rollback set is
   `uciApplyConfigs = {wireless, network, system, firewall, dhcp}`, deliberately
   wider than any one flow's mutation set, because that is the LuCI apply contract.
2. **External writers are outside the model entirely.** The generated
   `/usr/libexec/travo-wireless-toggle.sh` runs from cron and the hotplug script
   and does its own `uci set` / `commit` / `revert wireless`. No in-process lock
   covers a separate process: cron firing the toggle mid-sequence discards the
   API's staged delta, and the API still answers 200.
3. **`system` is in the wireless apply set but no mutator writes it**, so nothing
   ever holds a `system` lock while rpcd can still roll `system` back.
4. **`withConfigLocks` does not revert** (see above), so those two services can
   leave a delta behind on failure.

**Related:** ADR 0003 (crash guards), ADR 0006 (API contract).
