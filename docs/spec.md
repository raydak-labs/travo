# Spec: travo-greenfield 0.1.0

Travel-router web UI and API that runs ON the router, next to stock LuCI. Clean-room rewrite of Travo; reuses its lessons ([invariants.md](invariants.md), [device-facts.md](device-facts.md)), not its code. Terms: [GLOSSARY.md](../GLOSSARY.md). Decisions: [adr/](adr/).

## Users

- **Traveler (primary)**: non-technical. Daily jobs: join hotel WiFi, handle the captive portal, answer "am I online?", turn VPN on/off.
- **Power user (secondary)**: machine-readable API, SSH keys, DNS layering, later failover.

## Targets

- Verified: GL.iNet GL-AXT1800 (Slate AX, IPQ6018/ath11k) and GL-MT3000 (Beryl AX), aarch64, OpenWrt 25.12.x (apk).
- Built on capability detection (radios, buttons, package manager) so nothing is hardcoded to ath11k or a radio name. Other aarch64/x86_64 OpenWrt and opkg releases are best-effort, container-tested only.

## 0.1.0 scope

1. **Install and run**: release tarball + install script on fresh OpenWrt; travo-greenfield on `:80` (configurable), LuCI moved to `:8080`/`:8443` reversibly. Mock mode (`--mock`).
2. **Auth and first-run**: root-password login, cookie session; first-run "set password" reachable from `br-lan` only; password change; SSH keys optional later.
3. **Dashboard**: internet reachability, carrier and the active IPv4 uplink device (GET /api/v1/dashboard/connectivity). When this status is requested and a default route is selected, the daemon may send an HTTP GET to `http://connectivitycheck.gstatic.com/generate_204`. Dashboard also shows CPU load, memory, uptime, per-interface RX/TX throughput by kernel name (GET /api/v1/dashboard/throughput), quick actions (WiFi connect, VPN toggle). Version visible. The authenticated SPA seeds these values through the existing GETs and refreshes them through one shared SSE subscription (GET /api/v1/dashboard/events); throughput is sampled every 2 seconds and connectivity/resources every 5 seconds. If live updates disconnect, last known values stay visible and marked stale while the client reconnects and polls the GETs every 5 seconds.
4. **Uplink WiFi**: radio, STA and AP summary (GET /api/v1/wifi), on-demand read-only scan (GET /api/v1/wifi/scan, one row per SSID, dual-band badge), join (`POST /api/v1/wifi/join` with guarded confirmation, ≤3 taps from dashboard), ordered saved-upstream list, auto-reconnect with failure cap, MAC clone/randomize.
5. **AP and guest basics**: SSID/key, enable/disable per radio, guest AP with own subnet/zone.
6. **Safe apply**: snapshot → commit → watchdog → probe → explicit confirm; lockout guard.
7. **Captive portal assist**: detection and status card, "Open hotel login", DNS-bypass toggle with 5-min auto-restore, conditional DHCP renew.
8. **WireGuard**: profile import/list/select/delete through authenticated `/api/v1/vpn/profiles` routes, on/off (≤3 taps), kill switch, split tunnel, 5-state status (`GET /api/v1/vpn/status`), DNS verify.
9. **AdGuard Home (default on)**: installed after the first uplink; dnsmasq `:53` → AGH `127.0.0.1:5353`; on/off and "default DNS" toggle in Travo; full UI via the Travo-gated port `:3001`. See [adr/0006](adr/0006-adguard-package-and-topology.md).
10. **System**: info, reboot, authenticated config tarball download (`GET /api/v1/system/backup`), and signed self-update from GitHub releases. System checks the configured release source on demand and at most once per 24 hours on page entry. The running version is shown even when checks are disabled. System can verify and temporarily stage a signed release, apply it after confirmation, and report the outcome after restart. A failed activation restores the previous installation.
11. **UI**: Svelte 5 SPA, dark mode without flash, English via Paraglide, shell ≤ 40 KB gzip, route chunk ≤ 8 KB, total ≤ 100 KB.

## Out of 0.1 (do not break when adding)

Planned work lives in GitHub issues (milestone `0.1.0`, project board "travo-greenfield"); later features are issues without a milestone. Every later feature must go through the `Device` port, the transaction module and the DNS layer stack; a contract test enforces it.

## Frontend requirements

- Responsive, mobile-first; route-level code split; vector assets; no CDN or external fonts (device is often offline).
- Only the generated API client is called; no hand-typed endpoints.
- One card surface; one status vocabulary (ok / warn / danger / info / neutral / stale), no raw colour classes; dark mode mandatory.
- One group-heading component. One load/error wrapper: skeleton / inline error+retry / content. A failed request NEVER renders as "absent".
- Multi-status pages have a summary band of flat tiles that each degrade separately (WiFi is the reference).
- `stale` = last known value shown dimmed and marked; never blank a known value.
- One page scroll container. Live charts: no animation on push; state the retained window honestly. Rates in bits, ≤2 integer digits, unit per direction.
- Sidebar-only nav: Dashboard · WiFi · Network · VPN · Services · System. Header = actions only. First visit lands on Dashboard with groups collapsed; a child route auto-expands its parent; group label = default page, chevron = toggle; stored nav state is versioned.
- Optional service pages leave the nav when the package is absent (a failed check ≠ absent).
- Errors name the cause. "Rolled back" is shown ONLY when true.

## Build and delivery

- Go daemon, Huma-generated OpenAPI 3.1, hey-api TS client; CI fails on a generated-file diff. See [adr/0003](adr/0003-api-and-client-generation.md).
- Release artifact per arch: `travo-greenfield-<arch>.tar.gz` (binary, UI, init script, `uci-defaults`, keep-list entry), `checksums.txt`, minisign signature. See [adr/0007](adr/0007-distribution-and-self-update.md).
- Footprint: ~12.8 MB stripped binary budget. Measure before adding dependencies.
- Versioning: Conventional Commits + release-please; version and build time stamped into the binary.

## Acceptance for 0.1.0

Hardware playbook ([testing.md](testing.md)) passes on the GL-AXT1800, including lockout refusal, watchdog restore after daemon kill, AdGuard fail-open with AGH stopped, sysupgrade survival, and captive-portal bypass restore.
