# 0017 Tailscale through the OpenWrt package

Status: accepted (install, login and MagicDNS layer: #179; exit node, routes, SSH, firewall: #180)

Tailscale is the OpenWrt `tailscale` package (1.104.1-r1 on 25.12.6, pulls `kmod-tun`), driven through the `tailscale` CLI. Travo ships no Tailscale code: the binaries are 26 MiB installed and would blow the 12.8 MB budget (invariants §15). Evidence: `testdata/device/axt1800-25.12.6/tailscale/`.

## Install

- Explicit only, never automatic: Tailscale is opt-in. The install is guarded (`tailscale-install-in-progress`), needs the real Internet probe and 46 MiB free overlay (26 MiB installed size + 20 MB headroom); the measured overlay growth was 12.3 MiB (ubifs compression), so the check is conservative.
- The package's post-install starts tailscaled on its own. Travo still runs `enable` and `restart`, waits until `tailscale status --json` answers, and pins `--accept-dns=false`. Any failure keeps the guard; the user's next install request clears it by name.
- `apk del` leaves `/etc/tailscale/` (node key, state, SSH host keys) and `/var/lib/tailscale/`. Uninstall is not offered yet; when it is, it removes those explicitly (device-facts §15).
- Install and login are refused (409 `tailscale_failover_conflict`) while failover is enabled: Tailscale's nftables rules in `ip mangle` break mwan3 ([ADR 0018](0018-mwan3-failover.md) "Failover and Tailscale"; running both together is #279).

## Preferences are owned by Travo

- tailscaled stores its prefs in its state file, not UCI, so the transaction module cannot snapshot them. Travo treats them as its own: every write passes the full set (`--accept-dns=false`, exit node, LAN access, advertised routes, SSH; `tailscale.Flags`).
- Login is `tailscale up --reset <full set> --timeout=10s`. `--reset` because `up` otherwise refuses any call that does not repeat every non-default pref (`up-revert.txt`). Before login the CLI times out in `NeedsLogin` by design; the backend keeps the auth URL, which the status then returns (`status-authurl.json`). The UI shows that URL; the operator opens it on any device.
- Login, logout and pref changes run under `tailscale-change-in-progress`. A failure keeps the guard; the user's next action clears it by name (its retry path), and the DNS reconciler skips while it is held.

## MagicDNS as a DNS layer

- tailscaled never touches `resolv.conf` (`--accept-dns=false`, invariants §11). Its resolver `100.100.100.100` still serves the tailnet names, so the `tailscale` layer on the shared stack (ADR 0010) adds `server=/<MagicDNSSuffix>/100.100.100.100`. The layer is additive: it never sets `noresolv` and never replaces AdGuard, VPN or captive resolvers; LAN clients resolve tailnet names through dnsmasq whatever else is on top.
- A 15 s reconciler owns the layer: push while `Running` with MagicDNS enabled and a DNS-name suffix; pop when the package or daemon is absent or the backend is `NeedsLogin`, `Stopped` or `NoState`; transient states (`Starting`, `NeedsMachineAuth`) and unreadable status change nothing. A held stack guard or Tailscale guard stops it until the explicit retry.
- dnsmasq's rebind protection filters RFC 1918, link-local and ULA answers, so MagicDNS AAAA answers (`fd7a:115c:a1e0::/48`) are dropped while A answers (100.64.0.0/10) pass. Accepted: IPv4 is the supported path (invariants, deliberate ceilings). Verified only once a real tailnet is logged in (docs/testing.md, Tailscale).

## Exit node, LAN advertisement, SSH (#180)

- `PUT /api/v1/tailscale/settings` takes the complete set (`exit_node` as a peer's stable ID, `advertise_lan`, `ssh`) and applies it with one `tailscale set`. The exit node must be a peer with `ExitNodeOption`; Travo passes its Tailscale IPv4 address. LAN access stays on while an exit node is used. `advertise_lan` advertises the masked IPv4 prefix of `network.lan` (25.12 writes CIDR; older `ipaddr` + `netmask` is read too); nothing else is advertised.
- A newly selected exit node is live state that moves the default route, so it must prove Internet (the shared real probe) within 20 s. Otherwise the previous prefs are written back, the guard is released and the answer is 409 `tailscale_exit_node_offline`. If writing them back fails, the guard stays.
- Not a UCI transaction: the prefs live in tailscaled's state file, which the watchdog cannot snapshot. The probe-and-restore above replaces the confirm window. The admin UI stays reachable throughout (br-lan is local and `--exit-node-allow-lan-access` is on).

## Firewall zone and the WireGuard kill switch

- `travo_ts` (device `tailscale0`, input and forward REJECT, output ACCEPT, masq, mtu_fix), forwarding `travo_ts_lan` (lan -> travo_ts) and, only with `advertise_lan`, `travo_ts_in` (travo_ts -> lan). Staged by `POST /api/v1/tailscale/firewall` in a `firewall` transaction the user confirms; sections owned by name; a foreign section on an owned name, a foreign zone named `travo_ts` or a missing `lan` zone is 409 `tailscale_firewall_conflict` before any write. `GET` reports `firewall_in_sync`; the UI offers the transaction whenever it is false.
- Input REJECT: the admin UI and LuCI are not reachable from the tailnet (invariants §3). Tailscale SSH is answered inside tailscaled before packets reach the router's input chain, so it needs no input rule (verified in the playbook).
- No inbound rule on the uplink for UDP 41641: direct connections are outbound-initiated and come back as established; without them tailscaled falls back to DERP relays.
- Guest stays out of the tailnet: no guest -> travo_ts forwarding.
- Kill switch interplay: `travo_ts` holds no uplink network, so `vpnkill.Desired` does not see it as an uplink and adds no rule for it (test `TestKillSwitchDoesNotTreatTailscaleAsUplink`). With the kill switch on, LAN -> tailnet traffic is still allowed. That traffic leaves the router encrypted as tailscaled's own output, like WireGuard's handshakes, so it is not the clear-text uplink leak the kill switch exists to stop. LAN clients can therefore use a Tailscale exit node while the kill switch holds.
- Exit node and WireGuard both claim the default route. tailscaled's `ip rule` 5270 (`lookup 52`) sits before `main`, so an exit node would silently override a wg0 full tunnel. Selecting an exit node while wg0 is enabled (any state but `disabled`/`configured`) is refused with 409 `tailscale_exit_node_conflict`, and an unreadable tunnel state fails closed. Known gap: enabling WireGuard while an exit node is selected is not refused yet. Clear the exit node first.
