---
title: "ADR 0001: DNS resolution, VPN, captive portal, and restore semantics"
status: Accepted
date: 2026-05-14
updated: 2026-10-04
tags: [adr, dns, dnsmasq, adguard, vpn, wireguard, captive-portal, openwrt, luci]
---

# ADR 0001: DNS resolution, VPN, captive portal, and restore semantics

## Status

Accepted.

## Context

Travo runs on OpenWrt alongside **LuCI**. Most DNS and DHCP behavior is standard OpenWrt: **`dnsmasq`** (UCI `dhcp` package, typically `dhcp.@dnsmasq[0]`) serves LAN clients; **`network.wan`** controls whether the WAN uses ISP DHCP DNS (`peerdns`) or static DNS; optional **AdGuard Home** adds filtering; **WireGuard** may steer LAN DNS through tunnel resolvers. **Captive portals** on upstream networks often require using the network’s own DNS so portal hostnames resolve to RFC1918 addresses.

We need:

- One mental model for **who answers DNS** in each configuration.
- **Predictable temporary changes** (captive bypass, VPN DNS forwarding) with **restore to the prior UCI-backed state**.
- **VPN + DNS + AdGuard** combinations that avoid silent leaks where possible, without fighting OpenWrt’s native mechanisms.
- **UX consistency**: the API exposes enough state for the UI to explain “what mode we are in” and whether a captive bypass is active.

## Decision

### 1. Prefer OpenWrt / LuCI-native configuration

- **Authoritative stores** for router DNS integration are **UCI** (`dhcp`, `network`, `firewall`, …) and, for AdGuard, its **YAML** under `/opt/AdGuardHome/` or `/etc/adguardhome/` as on the device.
- Travo applies changes through the same knobs LuCI uses (`uci set` / `uci commit`, `init.d` service restarts where the codebase already does so). Operators may still use **LuCI** to inspect or repair configuration; Travo must tolerate UCI edits but cannot infer intent beyond what it reads at runtime.
- **Wireless** continues to follow the stricter LuCI-style `uci apply` / rollback / confirm flow documented in `docs/architecture/overview.md`. **DNS-related mutations** in this ADR use direct `uci commit` + `dnsmasq` restart patterns already implemented in services; that difference is intentional historical behavior, not an invitation to add new live paths without crash guards where they change connectivity broadly.

### 2. Resolver roles and configuration modes

**2.1 Default (dnsmasq only, no AdGuard path)**

- LAN clients use the router as DNS; **dnsmasq** forwards upstream.
- Upstreams come from **WAN**: with `network.wan.peerdns=1` (default), dnsmasq follows **`/tmp/resolv.conf.d/resolv.conf.auto`** (DHCP/PPP-provided resolvers). With **custom WAN DNS** (`peerdns=0`, `network.wan.dns`), the Network page / API sets static resolvers (OpenWrt-standard pattern).

**2.2 AdGuard Home — “forwarding” mode (supported default)**

- AdGuard listens on a **non-53 port** (stock template uses **5353**).
- dnsmasq is configured with **`server=127.0.0.1#<port>`** and **`noresolv=1`** so LAN DNS hits dnsmasq first, then AdGuard (see `AdGuardService.SetDNS` / auto-configure).
- **DHCP options, local hostnames, and `dhcp` `domain` entries** remain on dnsmasq unless the operator moves them by hand.
- Enabling and disabling forwarding does **not** touch UCI directly: both go through the
  shared dnsmasq resolver layer stack of §3.1, because VPN DNS forwarding writes the same
  two options of the same section.
- On first install `AdGuardService.AutoConfigure` seeds `/opt/AdGuardHome/AdGuardHome.yaml`
  (or `/etc/adguardhome/adguardhome.yaml`, if it exists) from the bundled template
  **`/etc/travo/adguardhome.yaml`**, falling back to the embedded `defaultAdGuardConfig`.
  That template path is **read only** — AdGuard's own YAML is the file Travo edits (§2.5
  covers its credentials).

**2.3 AdGuard Home — “direct” / primary on port 53 (advanced)**

- Detected when AdGuard’s YAML **`dns.port`** is **53** (`GetDNSMode` returns `adguard-direct`).
- Typically implies dnsmasq is no longer the LAN-facing DNS listener on 53 (operator or packaging may set **`port=0`** on dnsmasq or equivalent). This mode is **powerful but fragile**: DHCP-supplied local names, some split-DNS setups, and naive VPN DNS assumptions may break.
- **Product stance**: forwarding mode is the default safe path; direct mode is **expert / YAML-driven**, with UI copy and plans (`docs/plans/adguard-auto-configure.md`) describing risks.

**2.4 API-visible mode**

- `GET /api/v1/adguard/dns-mode` returns `mode` ∈ `default` | `adguard-forwarding` | `adguard-direct`, human `description`, `adguard_running`, and **`dns_bypassed`** when captive temporary DNS is active (`CaptiveService.IsDNSBypassed`).

**2.5 AdGuard Home ships no account, and the first-run wizard is open (accepted risk)**

- The packaged `packaging/adguard/AdGuardHome.yaml` and the embedded fallback
  `defaultAdGuardConfig` in `adguard_service.go` both carry **`users: []`**. An earlier
  build committed a bcrypt hash of a published `admin`/`password` pair with the plaintext
  in the README: one credential, identical on every device in the field, published to
  anyone who read the repository. That is a total compromise of every deployed unit
  and is not an acceptable alternative.
- With an empty `users` list **and** `http.address` bound to `0.0.0.0:3000` (and the
  embedded template's `bind_host: 0.0.0.0`), AdGuard Home serves its **first-run setup
  wizard** to any LAN client that reaches port 3000. The first client to complete it
  becomes the admin and owns the filter configuration.
- **This is not better or worse than the published credential — it is a different
  compromise.** The published credential compromised every device to the whole world;
  the wizard race compromises the filter to the first LAN client, which is a much
  smaller blast radius (LAN-only, and only until someone opens the UI). Recording it
  here rather than in a commit message is the point: the tradeoff is real, it was
  chosen deliberately, and a future reader must not "fix" it by reintroducing a
  default account.
- An operator can still create an admin deliberately from Travo (Settings → AdGuard
  Password): `AdGuardService.SetPassword` hashes the password and writes it into
  `users`, appending the entry when none exists.
- **Caveat, and it is the sharpest edge of this section:**
  `SetPassword` *appends*, so it does not claim the wizard. If a LAN client completed
  the first-run wizard before the operator got there, the operator's action adds a
  **second** admin rather than replacing the first, and whoever arrived first keeps
  full control of the filter — upstream resolvers, rewrites, per-client settings —
  with no indication in the Travo UI. Until the loopback bind exists, the only
  reliable recovery is to stop AdGuard, restore a config with a known `users` list,
  and start it again; the UI cannot show which accounts exist, because
  `GET /api/v1/adguard/config` redacts the password hashes and there is no
  account-listing endpoint.
- **The correct fix, not done here:** bind the AdGuard web UI to loopback
  (`127.0.0.1:3000`) and proxy it through Travo's authenticated API, so creating the
  admin account requires a Travo session. Until that exists the wizard race stays open
  and this section is the record of it.

### 3. WireGuard VPN and DNS

- **Tunnel DNS** is read from **`network.wg0.dns`** (space- or comma-separated, normalized
  by `splitWireGuardDNSOption` in `VpnService`).
- **VPN profiles** are persisted at **`/etc/travo/wireguard_profiles.json`**; the
  split-tunnel allowed-IP set lives at **`/etc/travo/split-tunnel.json`** (its firewall
  side is ADR 0004).
- **When WireGuard is enabled** and `wg0.dns` is non-empty, `enableVpnDNSForwarding`
  pushes the VPN resolvers onto the **shared dnsmasq resolver layer stack** of §3.1 as
  the layer `vpn`. It keeps no snapshot of its own. The tunnel is verified up before
  this runs, so a DNS bookkeeping failure is reported but never fails the toggle:
  aborting a working VPN over resolver bookkeeping would be the worse outcome.
- **When WireGuard is disabled**, `disableVpnDNSForwarding` pops the `vpn` layer, which
  restores the layer below it — or the pre-any-layer state when it was the last one.
  With no record at all it is a no-op for DNS.
- **Interaction with AdGuard forwarding**: while both layers are stacked, dnsmasq
  forwards to the **most recently enabled** layer with `noresolv=1`; the entry below
  (often `127.0.0.1#5353`) is recorded but not consulted. Popping the top layer points
  dnsmasq at the one below, so AdGuard can resume its prior role without a second
  manual toggle.
- **Interaction with AdGuard upstreams**: AdGuard’s own upstream list (DoH/DoT/plain)
  determines where **AdGuard** sends queries; those packets follow normal routing (and
  therefore the tunnel when policy routing sends them via wg). Travo does not
  automatically rewrite AdGuard YAML when VPN toggles except where **captive portal**
  logic touches upstreams (below). UI hints (`vpn-adguard-hint` and related copy)
  document that operators may need to align AdGuard upstreams with VPN DNS in advanced
  setups.

### 3.1 The shared dnsmasq resolver layer stack

AdGuard forwarding and VPN DNS forwarding write the **same two options of the same
section**, `dhcp.@dnsmasq[0].server` and `.noresolv`. Separate per-feature snapshot
files could not keep them from colliding: the files stayed separate but the
*effects* overwrote each other, so the last restore won with a value that was
valid in a state the operator had since left. There is therefore **one** record.

- **Location and shape**: **`/etc/trafo/dnsmasq-layers.json`** (`dnsmasqLayerStackPath`),
  holding `noresolv` and `servers` — dnsmasq's resolver state **before any layer was
  enabled** — plus `layers`, the stack bottom-first, each `{name, servers, noresolv?}`. The
  three layer names are `vpn`, `adguard` and `captive`. A missing file is not an error: it is
  how “no layer was ever enabled” is represented.
- **The record is written atomically** (temp file + `fsync` + rename). It is the only restore
  target for dnsmasq's resolvers, and a half-written JSON document is not a recoverable state:
  `load` fails, so every layer enable aborts and every restore is refused until someone deletes
  a file whose contents they cannot read.
- **First layer to enable** reads the live `server`/`noresolv` as the base state (or
  the pre-stack snapshot of §3.2, when one exists), records it together with its own
  resolvers, and **writes the record before dnsmasq is touched**. A record that cannot
  be written aborts the change rather than following it: without a persisted restore
  target there is nothing to put back. A record that exists but holds **no layer**
  is treated as “nothing owns dnsmasq”, so its base is re-read live: that is the shape
  the heal of §3.3 leaves behind, and re-using its recorded base would put a
  pre-heal resolver back over whatever the operator configured in LuCI in between.
- **Last layer to disable** applies the base state to dnsmasq and then **removes the
  record**.
- **A layer disabling while others remain** removes only its own entry and leaves
  dnsmasq pointed at whatever is now on top. If the layer is not stacked at all while
  something else is, the call reports “not applied” and touches nothing — that other
  layer owns the state, and writing over it would break the one that works.
- **Enable is idempotent**: re-enabling a stacked layer refreshes its resolvers in
  place and never re-reads dnsmasq as the base, so a second enable can never record a
  layer's own entry as the pre-layer state.
- **A stacked layer sets `noresolv=1`** by default (forwarding must not fall back to
  `resolv.conf` while a layer owns the resolvers); an empty base `noresolv` is written
  as `0`. A layer may record its own `noresolv` instead: the **captive** layer uses `0`,
  because the bypass exists to hand resolution to the upstream network's own resolver.
- **AdGuard disable with no record at all** falls back to `ClearNoResolv`: it clears
  `noresolv` only and leaves the server list alone, because that list may be entirely
  the operator's own split-DNS entries.
- **The write sequence** is `uci delete` / `uci add_list` / `uci set`, then `uci commit
  dhcp`, then `/etc/init.d/dnsmasq restart`. These shell out instead of going through
  the UCI interface, so they are invisible to an audit of “which configs does this
  write”; they hold the **`dhcp` config lock** (ADR 0010) and `dhcp` is in
  `vpnFlowConfigs` for exactly that reason.
- **Ownership**: the record is implemented in `vpn_service.go` because `VpnService`
  also owns the read-path self-heal of §3.3; `AdGuardService` drives the same code
  through `AdGuardChecker`.
- **Directory note**: the record is state, not a crash guard, yet it lives in
  `/etc/trafo`, where ADR 0003 otherwise keeps only guards. This is a recorded
  deviation, not an oversight: `deploy-local.sh` removes guards **by name**, never by
  directory glob, so a redeploy does not delete the record. A future recovery step that
  clears `/etc/trafo` wholesale must not treat this file as a guard.

### 3.2 Pre-stack snapshots and the device upgraded mid-VPN

Two older files are still **read**, once, as the migration source for the base state:

- **`/etc/travo/vpn-dns-snapshot.json`** (`legacyVpnDnsSnapshotPath`) — the VPN's own
  snapshot from before the stack existed, at the pre-unification path.
- **`/etc/trafo/adguard-dns-snapshot.json`** (`adguardDnsSnapshotPath`) — AdGuard's own
  snapshot from the same era.

A device upgraded while a layer was active therefore still has a restorable base
state. **Both** files are **removed** as soon as the stack next saves a record or applies
the legacy restore (plus whichever path the saving feature is configured with): two files
holding a restore target for the same two UCI options is precisely the collision the stack
removes. Removing only the saving feature's own file left the other feature's snapshot
behind for the whole life of the record.

### 3.3 VPN DNS self-heal (a second restore entrypoint)

`GetVpnStatus` is polled by the dashboard and the VPN page, which makes it the only
health signal guaranteed to be hit after a reboot. `maybeSelfHealVpnDNS` therefore pops
the `vpn` layer when the tunnel that justified it can no longer carry DNS, instead of
leaving LAN DNS pointing at resolvers inside a tunnel that is not there. This is a
status read that heals rather than only reports, and that is deliberate.

- **When it fires**: the only terminal reading is **`disabled`** — the tunnel is off in
  UCI — and it heals immediately. `connected`, `up_no_handshake` (peers, tunnel up, no
  handshake yet) and `configured` (no peers yet) mean the tunnel is healthy or on its
  way up and are never healed; burning the restore on those is what a reboot into an
  unreachable upstream used to do.
- **The debounce**: `enabled_not_up` covers two different worlds — a `wg show wg0` that
  failed on a busy router, and a `wg0` interface that is gone for good while UCI still
  says enabled — so it is not trusted on its own. It heals only after
  `vpnDNSHealStreakRequired` (3) consecutive readings **spanning at least
  `vpnDNSHealGracePeriod` (2 minutes)**. Any other reading resets the streak, so one
  healthy poll in between is enough to prove the failure was transient. Both are
  variables so tests compress the window instead of waiting it out.
- **Known gap**: if the `network.wg0` UCI section is removed outright while the `vpn`
  layer is stacked, `GetVpnStatus` no longer reports a WireGuard status at all and the
  heal never runs, so dnsmasq keeps forwarding to tunnel resolvers until the profile is
  restored and toggled. Closing that needs a heal path that does not depend on `wg0`
  existing.
- **A broken probe is not a broken tunnel.** `wg show wg0 dump` failing is ambiguous:
  a busy router, a missing `wg` binary, or output that no longer parses all look like a
  dead interface, and the debounce above only helps with the transient one. `wgRuntimeState`
  therefore falls back to the link state when the tool is unusable, and an **UP `wg0`**
  reports `up_no_handshake` — not healable. Without it, a persistently broken probe
  eventually satisfies the debounce and pops the DNS layer of a tunnel that is carrying
  it perfectly well.
- **It deliberately keeps the record** (`RemoveLayer(vpn, keepRecord: true)`): popping
  the last layer still restores the base state, but the file survives so the heal is
  idempotent and a later **explicit** disable — or a re-enable followed by a disable —
  still has the true pre-any-layer state to restore. The explicit disable passes
  `keepRecord: false` because it has applied the base and the record is then done.
- It no-ops when no `vpn` layer is stacked, and it takes the **`dhcp` config lock**
  (ADR 0010) because the stack shells out to `uci` for `dhcp`.

### 4. Captive portal detection and temporary DNS bypass

- **Detection** uses an HTTP probe (e.g. `connectivitycheck.gstatic.com/generate_204`) with redirect inspection (`CaptiveService`).
- **Bypass** (`BypassDNS`) runs when portal-related DNS is likely blocked, including:
  - dnsmasq **`noresolv=1`** (custom forwarders only),
  - legacy **`network.wan`** static DNS with `peerdns=0`, or
  - **AdGuard using encrypted upstreams** (DoH/DoT), which cannot resolve hijacked “hotel” names the way the upstream expects.
- **Mechanism**: before changing anything, Travo writes a **JSON backup** to **`/etc/trafo/captive-dns-in-progress`** (also acts as the “bypass active” marker) containing relevant `wan` DNS fields, the dnsmasq `noresolv` flag, the dnsmasq `rebind_protection` flag, and AdGuard upstream/bootstrap/fallback slices. The dnsmasq **`server` list is recorded only by a bypass that actually removes it** — i.e. never by the current code; it appears in a guard file only when one was written before the layer stack (§3.2). A **`dnsmasq_layer`** flag records that this bypass put the resolvers under the stack's control, which is what lets a restore tell "the record is lost" from "this bypass never touched the list". It then:
  - **pushes the hotel DNS onto the shared dnsmasq layer stack as the `captive` layer** with `noresolv=0`, whenever dnsmasq is actually blocking (`noresolv=1`). That is the only owner of `server`/`noresolv` (§3.1): a bypass that snapshotted those two options outside the stack cannot see the layer below it, so its restore replaces the VPN or AdGuard resolvers with whatever the bypass found, and the next enable then records the bypass's own value as the “pre-any-layer” base. On a VPN that means LAN DNS forwarding into a tunnel that is not there, with `noresolv=1` and no fallback left.
  - clears static WAN DNS overrides when `peerdns=0`,
  - turns off dnsmasq `rebind_protection` while the bypass is active (the hotel resolver answers with private addresses),
  - points **AdGuard upstream** at plain **hotel DNS** when available so the resolver that actually handles queries can see portal names.
- **Restore** (`RestoreDNS`) pops the `captive` layer, which puts back whatever is stacked beneath it, restores `rebind_protection` and the `wan`/AdGuard fields from the guard file, and removes the guard file.
- **Restore writes only what the guard file actually holds.** The no-layer fallback (a guard file written **before** the stack existed, §3.2) is applied **per field, and only when that field is present**: a `peerdns=0` or AdGuard-encrypted bypass never removed dnsmasq's `server` list, so its guard file records none, and a restore that deleted the list anyway destroyed a split-DNS entry the operator configured that nothing could put back. The same rule covers the staged `uci delete`: a delete that is staged is always followed by `uci commit dhcp` in the same restore, never left for the next unrelated writer of `dhcp` to flush. A guard file that carries no dnsmasq resolver state and no layer record leaves dnsmasq untouched. Applying recorded values **over a live layer** would break the layer that owns dnsmasq, so that case is skipped too.
- **An unreadable layer record is not an empty one.** `hasAnyLayer` fails closed and reports the error: a torn `/etc/trafo/dnsmasq-layers.json` means dnsmasq may be owned by something the guard knows nothing about, so `RestoreDNS` fails loudly, keeps the guard file for retry, and writes nothing.
- **A lost layer record is not an empty one either, and `RestoreDNS` REFUSES.** The record is **state**, not a crash guard (§3.1): a partial `/etc/trafo` loss or a cleanup from an older package layout can take it while the guard survives. The guard holds `noresolv` but **never the resolver list** (§4), so a restore that finds no record cannot learn what the list used to be — while the hotel resolver the bypass pushed is still sitting in it. Restoring `noresolv` alone there left dnsmasq forwarding **all** DNS to the portal's resolver and then reported success, restarted dnsmasq, logged "DNS restored" and deleted the guard, so nothing would ever retry.
  - **Invariant: `RestoreDNS` never reports success while the bypass can still be in force.** If the pre-bypass resolver list cannot be determined, it writes **nothing** to dnsmasq, returns an error naming the missing `dnsmasq-layers.json`, and **keeps the guard** (ADR 0003 §2) so a later attempt can retry. Deleting the guard on a failure is the specific harm — the guard is the only remaining record that anything was ever bypassed.
  - The bypass records **`dnsmasq_layer`** in the guard file, written out again right after the layer is pushed (the guard itself is persisted before any mutation, so a flag only known afterwards is recorded by rewriting it). It distinguishes "this bypass put dnsmasq's resolvers under the stack's control" from "this bypass never touched them", so the refusal cannot fire on the `peerdns=0` / AdGuard-encrypted paths — those bypasses never took a layer, have nothing to put back, and must still clear cleanly instead of wedging `dns_bypassed` on forever. A guard file written **before** the flag existed carries no flag, so a recorded `noresolv=1` with no recorded list is treated the same way: conservatively, as a bypass whose resolvers are unaccounted for.
  - The opposite case needs no refusal: when **another layer is stacked**, that layer already re-applied its own resolvers on top, so dnsmasq is not pointing at the hotel resolver and the bypass is genuinely gone even though the `captive` entry is not in the record.
  - No path through Travo's own code removes the `captive` entry without popping the layer first, so this is a **fail-open-on-inconsistent-state hole**, not a self-inflicted one; the hole is closed by refusing rather than by trusting the state to stay consistent.
- **Automatic restore**: a **5-minute** safety timeout forces restore if bypass stayed on too long (`captiveDNSRestoreTimeout`, on the bounded startup reconcile).
- **Restore on reconnect is not on the read path.** `GET /api/v1/captive/status` used to call `MaybeAutoRestoreDNS`, so every poll of the captive status committed `dhcp` and `network`, rewrote dnsmasq's resolver options and restarted dnsmasq. A GET that mutates DNS is indistinguishable from an outage when it lands mid-resolve. Recovery now has two mutating entrypoints: `POST /api/v1/captive/dns-restore` and the 5-minute startup restore. If restore-on-reconnect is wanted again it belongs on a scheduler that owns its own crash guard, not on a GET. (`MaybeAutoRestoreDNS` still exists for those callers; the handler no longer calls it.)

### 4.1 Captive auto-accept: the wwan bounce is conditional

When the first page fetch of a detected portal fails, the auto-accept flow may renew
the **`wwan` DHCP lease** (needed after a MAC change, when the gateway blocks a stale
MAC/IP pair). This is a **live network mutation inside an HTTP request**, so it is
bounded and gated:

- It happens **only on step 0** and **only when the first fetch failed**, never later in
  the multi-step portal walk.
- It runs **only when `wwan` is the active uplink** — netifd must report `wwan` up with
  an IPv4 default route (`wwanIsActiveUplink`). On an ethernet-only or USB-tether
  setup, `wwan` carries nothing, so bouncing it would only disturb an unrelated
  interface; the bounce is then skipped, not failed.
- It is **crash-guarded** (`/etc/trafo/captive-wwan-bounce-in-progress`, ADR 0003 §2):
  the guard is written before `ifdown` and removed **only** after a fresh lease came
  back. A failure at any step returns the error and **keeps** the guard, because the
  interface may be half down.
- The waits are bounded constants (2 s after `ifdown`, 8 s for the lease, with a poll
  interval) so the handler goroutine cannot be pinned indefinitely. Every step's error
  is returned instead of discarded, and a failure is reported in the result message
  rather than swallowed.

**Stance:** the bounce is a last resort scoped to the interface that is actually
carrying the portal connection. It is not a general "reset the network" tool, and
new callers must not widen the condition.

### 5. Consistent UX and “return to original state”

- **Temporary layers** must always have a **serialized prior state** on disk and a
  **restore entrypoint per feature that is named in this ADR**. There are three:
  - captive: `RestoreDNS`, which pops the `captive` layer and is backed by the guard/backup
    `/etc/trafo/captive-dns-in-progress` (ADR 0003 §2) for the `wan`, rebind and AdGuard fields.
    It is the **only** restore entrypoint for dnsmasq's resolvers under the bypass, which makes
    the invariant of §4 load-bearing: with the layer record gone it cannot restore the
    pre-bypass resolver list, so it **refuses and keeps the guard** rather than reporting a
    restore that never happened.
  - VPN, explicit disable: `disableVpnDNSForwarding`, which pops the `vpn` layer and,
    as the last layer, removes `/etc/trafo/dnsmasq-layers.json`;
  - VPN, read path: `maybeSelfHealVpnDNS` (§3.3), which pops the same layer on a
    terminal tunnel state and deliberately **keeps** the record.
- The two VPN entrypoints are not redundant: one owns the record's lifecycle, the other
  is the heal that runs where a health check is guaranteed to happen, and they pass
  different `keepRecord` values on purpose. Captive keeps its own guard file for the state
  that is **not** a dnsmasq resolver option — `wan` DNS, `rebind_protection`, AdGuard
  upstreams — but its dnsmasq resolvers go through the shared stack like everyone else's.
- **UI/API** should surface **`dns_bypassed`** and VPN state so users are not surprised
  by upstream changes.
- **Operator expectations**: finishing captive flows (or explicit restore) before other
  major DNS toggles reduces edge cases. All three dnsmasq resolver features — VPN, AdGuard
  and the captive bypass — are unified on the one stack of §3.1; this ADR records that
  implementation rather than the older per-feature snapshots it replaced.
- **Drift gate**: `TestDnsResolverStatePathsAreNamedInAdr0001` and
  `TestEtcTrafoStatePathsAreNamedInAnAdr` in
  `backend/internal/services/dns_docs_test.go` derive the persistent paths from the
  services package and fail when this ADR (or any ADR, for `/etc/trafo`) stops naming
  them. Adding a DNS state path without documenting it here is therefore a red test.

### 6. Built-in mechanisms summary

| Concern | OpenWrt / LuCI mechanism | Travo touchpoint |
| ------- | ------------------------ | ---------------- |
| LAN DNS | dnsmasq UCI `dhcp.@dnsmasq[0]` | `NetworkService`, `AdGuardService`, `VpnService`, `CaptiveService` |
| WAN DNS override | `network.wan.peerdns` / `network.wan.dns` | `NetworkService.SetDNSConfig`, captive bypass/restore |
| Upstream DHCP DNS | `resolv.conf.auto` | Captive bypass reads hotel DNS |
| AdGuard | YAML + `init.d/adguardhome` | `AdGuardService`, captive upstream patch |
| WireGuard DNS | `network.wg0.dns` | `VpnService` forwarding to dnsmasq, through the shared layer record |
| Shared dnsmasq resolver state | one record, `/etc/trafo/dnsmasq-layers.json` | `vpn_service.go` (stack, self-heal), `adguard_service.go` (forwarding), `captive_service.go` (bypass layer) |
| VPN profiles / split tunnel | `/etc/travo/wireguard_profiles.json`, `/etc/travo/split-tunnel.json` | `VpnService` |

## Consequences

- New features that **temporarily** alter dnsmasq's resolver options must **stack onto
  the shared record of §3.1** rather than taking a second snapshot of the same two UCI
  options, and must pass the `dhcp` config lock (ADR 0010) because the stack shells out
  to `uci`. Other temporary state keeps the ordinary `/etc/travo/` convention.
- **Primary-on-53** AdGuard remains an advanced configuration: documentation and UI must
  keep warning about VPN, local DNS, and portal behavior.
- Changes to UCI **outside** Travo while a layer is stacked can make the recorded base
  state **stale**; recovery is via LuCI/uci or redeploy as today.
- With `users: []` and a `0.0.0.0:3000` web UI, AdGuard's first-run wizard is reachable
  from the LAN until an admin exists (§2.5). This is a known, recorded exposure with a
  known fix that is not implemented.

## References

- `backend/internal/services/adguard_service.go` — forwarding via the layer stack,
  `GetDNSMode`, auto-configure, `SetPassword`, bundled template
  `/etc/travo/adguardhome.yaml`
- `backend/internal/services/vpn_service.go` — `/etc/trafo/dnsmasq-layers.json`
  (`dnsmasqLayerStackPath`), the legacy `/etc/travo/vpn-dns-snapshot.json`,
  `enableVpnDNSForwarding` / `disableVpnDNSForwarding`, `maybeSelfHealVpnDNS`
- `backend/internal/services/captive_service.go` — bypass as the `captive` layer, guard
  file, restore, timeouts
- `backend/internal/services/captive_autoaccept.go` — portal walk, conditional wwan
  bounce
- `backend/internal/services/network_service.go` — WAN custom DNS (`SetDNSConfig`)
- `backend/internal/services/dns_docs_test.go` — the drift gate that keeps the paths
  above and this ADR in agreement
- `docs/adr/0003-crash-guards-and-live-state.md` — guard directory contract (§2)
- `docs/adr/0010-uci-write-serialisation-and-request-contracts.md` — the `dhcp` lock
- `packaging/adguard/AdGuardHome.yaml` — the shipped AdGuard config, incl. `users: []`
- `docs/examples/adguard.yml` — the documented example of the same file
- `docs/plans/adguard-auto-configure.md` — historical plan: primary vs forwarding
- `docs/plans/2026-03-26-vpn-disable-latency-and-dns-forwarding.md` — VPN DNS restore
- `docs/guides/deployment.md` — packaged AdGuard port and dnsmasq relationship
