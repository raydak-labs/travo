# 0017 Tailscale through the OpenWrt package

Status: accepted (install, login and MagicDNS layer: #179; exit node, routes, SSH, firewall: #180)

Tailscale is the OpenWrt `tailscale` package (1.104.1-r1 on 25.12.6, pulls `kmod-tun`), driven through the `tailscale` CLI. Travo ships no Tailscale code: the binaries are 26 MiB installed and would blow the 12.8 MB budget (invariants §15). Evidence: `testdata/device/axt1800-25.12.6/tailscale/`.

## Install

- Explicit only, never automatic: Tailscale is opt-in. The install is guarded (`tailscale-install-in-progress`), needs the real Internet probe and 46 MiB free overlay (26 MiB installed size + 20 MB headroom); the measured overlay growth was 12.3 MiB (ubifs compression), so the check is conservative.
- The package's post-install starts tailscaled on its own. Travo still runs `enable` and `restart`, waits until `tailscale status --json` answers, and pins `--accept-dns=false`. Any failure keeps the guard; the user's next install request clears it by name.
- `apk del` leaves `/etc/tailscale/` (node key, state, SSH host keys) and `/var/lib/tailscale/`. Uninstall is not offered yet; when it is, it removes those explicitly (device-facts §15).

## Preferences are owned by Travo

- tailscaled stores its prefs in its state file, not UCI, so the transaction module cannot snapshot them. Travo treats them as its own: every write passes the full set (`--accept-dns=false`, exit node, LAN access, advertised routes, SSH; `tailscale.Flags`).
- Login is `tailscale up --reset <full set> --timeout=10s`. `--reset` because `up` otherwise refuses any call that does not repeat every non-default pref (`up-revert.txt`). Before login the CLI times out in `NeedsLogin` by design; the backend keeps the auth URL, which the status then returns (`status-authurl.json`). The UI shows that URL; the operator opens it on any device.
- Login, logout and pref changes run under `tailscale-change-in-progress`. A failure keeps the guard; the user's next action clears it by name (its retry path), and the DNS reconciler skips while it is held.

## MagicDNS as a DNS layer

- tailscaled never touches `resolv.conf` (`--accept-dns=false`, invariants §11). Its resolver `100.100.100.100` still serves the tailnet names, so the `tailscale` layer on the shared stack (ADR 0010) adds `server=/<MagicDNSSuffix>/100.100.100.100`. The layer is additive: it never sets `noresolv` and never replaces AdGuard, VPN or captive resolvers; LAN clients resolve tailnet names through dnsmasq whatever else is on top.
- A 15 s reconciler owns the layer: push while `Running` with MagicDNS enabled and a DNS-name suffix; pop when the package or daemon is absent or the backend is `NeedsLogin`, `Stopped` or `NoState`; transient states (`Starting`, `NeedsMachineAuth`) and unreadable status change nothing. A held stack guard or Tailscale guard stops it until the explicit retry.
- dnsmasq's rebind protection filters RFC 1918, link-local and ULA answers, so MagicDNS AAAA answers (`fd7a:115c:a1e0::/48`) are dropped while A answers (100.64.0.0/10) pass. Accepted: IPv4 is the supported path (invariants, deliberate ceilings). Verified only once a real tailnet is logged in (docs/testing.md, Tailscale).
