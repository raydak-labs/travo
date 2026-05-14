---
title: "ADR 0001: DNS resolution, VPN, captive portal, and restore semantics"
status: Accepted
date: 2026-05-14
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
- **Wireless** continues to follow the stricter LuCI-style `uci apply` / rollback / confirm flow documented in `docs/architecture.md`. **DNS-related mutations** in this ADR use direct `uci commit` + `dnsmasq` restart patterns already implemented in services; that difference is intentional historical behavior, not an invitation to add new live paths without crash guards where they change connectivity broadly.

### 2. Resolver roles and configuration modes

**2.1 Default (dnsmasq only, no AdGuard path)**

- LAN clients use the router as DNS; **dnsmasq** forwards upstream.
- Upstreams come from **WAN**: with `network.wan.peerdns=1` (default), dnsmasq follows **`/tmp/resolv.conf.d/resolv.conf.auto`** (DHCP/PPP-provided resolvers). With **custom WAN DNS** (`peerdns=0`, `network.wan.dns`), the Network page / API sets static resolvers (OpenWrt-standard pattern).

**2.2 AdGuard Home — “forwarding” mode (supported default)**

- AdGuard listens on a **non-53 port** (stock template uses **5353**).
- dnsmasq is configured with **`server=127.0.0.1#<port>`** and **`noresolv=1`** so LAN DNS hits dnsmasq first, then AdGuard (see `AdGuardService.SetDNS` / auto-configure).
- **DHCP options, local hostnames, and `dhcp` `domain` entries** remain on dnsmasq unless the operator moves them by hand.

**2.3 AdGuard Home — “direct” / primary on port 53 (advanced)**

- Detected when AdGuard’s YAML **`dns.port`** is **53** (`GetDNSMode` returns `adguard-direct`).
- Typically implies dnsmasq is no longer the LAN-facing DNS listener on 53 (operator or packaging may set **`port=0`** on dnsmasq or equivalent). This mode is **powerful but fragile**: DHCP-supplied local names, some split-DNS setups, and naive VPN DNS assumptions may break.
- **Product stance**: forwarding mode is the default safe path; direct mode is **expert / YAML-driven**, with UI copy and plans (`docs/plans/adguard-auto-configure.md`) describing risks.

**2.4 API-visible mode**

- `GET /api/v1/adguard/dns-mode` returns `mode` ∈ `default` | `adguard-forwarding` | `adguard-direct`, human `description`, `adguard_running`, and **`dns_bypassed`** when captive temporary DNS is active (`CaptiveService.IsDNSBypassed`).

### 3. WireGuard VPN and DNS

- **Tunnel DNS** is read from **`network.wg0.dns`** (space- or comma-separated, normalized in `VpnService`).
- **When WireGuard is enabled** and `wg0.dns` is non-empty, Travo applies **VPN DNS forwarding** to dnsmasq: it **snapshots** current `dhcp.@dnsmasq[0].server` list and `noresolv` to **`/etc/travo/vpn-dns-snapshot.json`**, replaces `server` entries with the VPN resolvers, sets `noresolv=1`, commits `dhcp`, and restarts dnsmasq.
- **When WireGuard is disabled**, it **restores** dnsmasq from that snapshot file and **deletes** the snapshot. If no snapshot exists, disable is a no-op for DNS.
- **Interaction with AdGuard forwarding**: the snapshot taken at **VPN enable** may contain `127.0.0.1#5353` (or another AdGuard port). After disable, that forwarding chain is restored, so AdGuard can resume its prior role without a second manual toggle.
- **Interaction with AdGuard upstreams**: AdGuard’s own upstream list (DoH/DoT/plain) determines where **AdGuard** sends queries; those packets follow normal routing (and therefore the tunnel when policy routing sends them via wg). Travo does not automatically rewrite AdGuard YAML when VPN toggles except where **captive portal** logic touches upstreams (below). UI hints (`vpn-adguard-hint` and related copy) document that operators may need to align AdGuard upstreams with VPN DNS in advanced setups.

### 4. Captive portal detection and temporary DNS bypass

- **Detection** uses an HTTP probe (e.g. `connectivitycheck.gstatic.com/generate_204`) with redirect inspection (`CaptiveService`).
- **Bypass** (`BypassDNS`) runs when portal-related DNS is likely blocked, including:
  - dnsmasq **`noresolv=1`** (custom forwarders only),
  - legacy **`network.wan`** static DNS with `peerdns=0`, or
  - **AdGuard using encrypted upstreams** (DoH/DoT), which cannot resolve hijacked “hotel” names the way the upstream expects.
- **Mechanism**: before changing anything, Travo writes a **JSON backup** to **`/etc/travo/captive-dns-in-progress`** (also acts as the “bypass active” marker) containing dnsmasq options, relevant `wan` DNS fields, and AdGuard upstream/bootstrap/fallback slices. It then:
  - relaxes dnsmasq toward **DHCP-provided DNS** (reads **`/tmp/resolv.conf.d/resolv.conf.auto`**),
  - may clear static WAN DNS overrides,
  - points **AdGuard upstream** at plain **hotel DNS** when available so the resolver that actually handles queries can see portal names.
- **Restore** (`RestoreDNS`) reapplies the backup, removes the guard file, and restarts dnsmasq; AdGuard upstreams are restored when the backup captured them.
- **Automatic restore**: when connectivity checks show the internet is reachable, **`MaybeAutoRestoreDNS`** triggers restore; a **5-minute** safety timeout also forces restore if bypass stayed on too long (`captiveDNSRestoreTimeout`).

### 5. Consistent UX and “return to original state”

- **Temporary layers** must always have a **serialized prior state** on disk (`/etc/travo/…`) and a **single restore entrypoint** per feature (captive: `RestoreDNS`; VPN: `disableVpnDNSForwarding`).
- **UI/API** should surface **`dns_bypassed`** and VPN state so users are not surprised by upstream changes.
- **Operator expectations**: finishing captive flows (or explicit restore) before other major DNS toggles reduces edge cases where two features both snapshot different moments. A **unified stack** of snapshots is not implemented today; the ADR records the **current** independent snapshots.

### 6. Built-in mechanisms summary

| Concern | OpenWrt / LuCI mechanism | Travo touchpoint |
| ------- | ------------------------ | ---------------- |
| LAN DNS | dnsmasq UCI `dhcp.@dnsmasq[0]` | `NetworkService`, `AdGuardService`, `VpnService`, `CaptiveService` |
| WAN DNS override | `network.wan.peerdns` / `network.wan.dns` | `NetworkService.SetDNSConfig`, captive bypass/restore |
| Upstream DHCP DNS | `resolv.conf.auto` | Captive bypass reads hotel DNS |
| AdGuard | YAML + `init.d/adguardhome` | `AdGuardService`, captive upstream patch |
| WireGuard DNS | `network.wg0.dns` | `VpnService` forwarding to dnsmasq |

## Consequences

- New features that **temporarily** alter DNS must follow the same **snapshot → mutate → restore** pattern and persist snapshots under `/etc/travo/` with clear naming (see crash-guard conventions in `docs/architecture.md`).
- **Primary-on-53** AdGuard remains an advanced configuration: documentation and UI must keep warning about VPN, local DNS, and portal behavior.
- Changes to UCI **outside** Travo while a bypass or VPN snapshot is active can make on-disk snapshots **stale**; recovery is via LuCI/uci or redeploy as today.

## References

- `backend/internal/services/adguard_service.go` — forwarding, `GetDNSMode`, auto-configure
- `backend/internal/services/vpn_service.go` — `vpn-dns-snapshot.json`, enable/disable forwarding
- `backend/internal/services/captive_service.go` — bypass/restore, guard file, timeouts
- `backend/internal/services/network_service.go` — WAN custom DNS (`SetDNSConfig`)
- `docs/plans/adguard-auto-configure.md` — historical plan including primary vs forwarding
- `docs/plans/2026-03-26-vpn-disable-latency-and-dns-forwarding.md` — VPN DNS restore rationale
- `docs/deployment.md` — packaged AdGuard port and dnsmasq relationship
