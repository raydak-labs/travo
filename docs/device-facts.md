# Device facts

Hard-won findings. Do not re-learn. Origin: the previous Travo implementation. Re-capture and re-date them in this repo as fixtures (see [testing.md](testing.md)). `[HW]` = observed on hardware, `[TRIED]` = approach that failed, `[BUG]` = past defect to keep as a regression check.

Reference hardware: GL-AXT1800 (IPQ6018, ath11k, radio0 = 5 GHz usually uplink STA, radio1 = AP), OpenWrt 25.12.3 (also 25.12.6), netifd 2026.02.26. Also targeted: GL-MT3000. dnsmasq config is a NAMED section (e.g. `dhcp.cfg01411c`), not `@dnsmasq[0]`. `rg` is absent on device. BusyBox `sed`/`wget`/`tar` differ from GNU. `jsonfilter` is present. `scp` needs `-O`.

Greenfield captures and spike results (2026-11-09, AXT1800 25.12.6): `testdata/device/axt1800-25.12.6/SPIKES.md`.

## 0. UCI reads

- [HW] `uci show` prints a one-element LIST exactly like an option (`dhcp.cfg01411c.server='127.0.0.1#5353'`). dnsmasq's init script reads `server` only as a list (`config_list_foreach`), so writing it back as `option server` silently drops it: no `server=` line, `noresolv=1`, every query REFUSED, config "looks" unchanged. Read with `uci export` (keeps `list`/`option`) or `ubus call uci get` (JSON array).
- [HW] Option order: `uci set` on an existing option keeps its place in the section; a deleted then re-added option or list, or a new one, is appended at the end (checked on AXT1800 25.12.6, 2026-11-17). Restoring a section byte-for-byte therefore needs the original order: `device.Section.Order` carries it from `uci export`, and the DNS layer stack re-appends owned options in base order (#157).
- [HW] `reload_config` hands changes to procd triggers asynchronously: wireless live ~1.4 s, dnsmasq ~2–4 s after commit. Probes must wait, not read once.

## 1. Routes and netifd

- [HW] After WireGuard disable: `network.interface.wwan` status shows a DHCP default route, `ip -4 route show table main` has none, `wget` exits 4. Netifd state ≠ kernel route. Every "route/internet ok" check reads the KERNEL table.
- [HW greenfield 2026-11-13] Reproduced: `wg0` `disabled=1` + `reload_config` removed `default dev wg0` and left NO default route although `wwan` status listed one (`wget` exit 4). `ifup wwan` restored the kernel default route (also left stale `/32` endpoint host routes via the uplink). Captures: `testdata/device/axt1800-25.12.6/wireguard/`.
- [HW greenfield 2026-11-17] wg0 in no firewall zone: with `default dev wg0` LAN clients have NO internet (fw4 rejects lan -> wg0 forwards), while the router itself is online. A zone holding `wg0` with masq plus a lan -> zone forwarding fixes it (SPIKES.md, #89).
- [HW greenfield 2026-11-17] netifd `config route` accepts CIDR in `target` (`target='a.b.c.d/32'`, `interface='wg0'`). Switching a connected wg0 from `route_allowed_ips=1` to `0` plus `ifdown/ifup wg0` left NO default route; `ifup wwan` restored it (SPIKES.md, #90).
- [HW greenfield 2026-11-17] fw4 puts user `rule` sections before zone forwardings in `forward_<zone>`: a lan -> wan REJECT rule wins over the stock lan -> wan forwarding without editing it.
- [HW greenfield] Changing only `wg0_peer0.endpoint_host` + `reload_config` kept the old endpoint live (`wg show wg0 endpoints`); `ifup wg0` applied it. A WireGuard peer edit needs an interface restart.
- [HW greenfield] Disabled `wg0`: `network.interface.wg0 status` → `Not found`, `wg show` empty. Enabled, no handshake: interface `up:true`, kernel `default dev wg0`, `wg show all dump` peer `latest-handshake` 0. Connected: same plus non-zero handshake epoch. `up:true` does not prove a working tunnel; only a recent handshake does.
- [HW] `wg show all dump` prints the interface PRIVATE key in column 2 of the interface line; parse `wg show <if> latest-handshakes` or redact.
- Disable recovery must not assume `wan` is the uplink; internet often arrives via WiFi STA. Recovery needs several bounded waits; some devices need script-level down/up, not only ubus.

## 2. Wireless liveness (ath11k / netifd)

- [HW] netifd emits `up`, `pending`, `retry_setup_failed`, `autostart` PER RADIO only. A per-interface entry is `{section, config, ifname, vlans, stations}`: no per-interface `up`, no config path.
- [HW] Per-interface truth: `network.device status {"name":"<ifname>"}` → `present`, `up`, `carrier`.
- [HW] Uplink STA with wrong password or nonexistent SSID: radio reads up, STA `carrier:false`. Only carrier proves association; must be REFUSED. Association + DHCP ≈ 10 s.
- [HW] Invalid STA channel is silently ignored (AP stays up); cannot simulate failure that way.
- [HW] AP with zero clients is genuinely up. `stations` is always empty for mode=sta; never use stations/assoclist for liveness.
- [HW] Bring-up: AP link ~10 s after radio start; STA ~4 s; wwan up ~10 s after DHCP.
- [HW] If one radio hosts two expected interfaces (guest + main AP), the radio flag proves only one. Check EACH interface; a radio flag alone is a false confirm that cancels rollback.
- Ubus round trips ~5 ms; budget ~100 ms per call. Unreadable payload (missing keys, unknown shape) ≠ down: report "cannot verify". Transport failure during a probe = unverifiable, retry; never claim "rolled back".
- [TRIED] Trusting per-interface `up` (absent on this firmware). [TRIED] A 4 s probe budget against a ~10 s bring-up. Derive budgets from measured latency.
- [HW greenfield] `uci commit wireless` + `reload_config` (no `wifi`) applied and restored an AP SSID change in ~1 s each; the STA on the other radio stayed associated. Wrong STA SSID: `carrier:false` and no kernel default route; after restore STA carrier in 7.8 s, default route 0.5 s later.
- [HW] `ubus call iwinfo scan '{"device":"phy0-sta0"}'` on the STA (associated or not) returns one entry per BSSID with `band` (2/5), `channel`, `mhz`, `signal` (dBm), `quality`/`quality_max`, `encryption.{enabled,wpa,authentication,ciphers}`; hidden networks have no `ssid` key. No crash with an AP up on the other radio (testdata `wwan-down/`).
- [HW greenfield 2026-11-13] With no STA interface, `ubus call iwinfo scan '{"device":"phy0"}'` (or `radio0`) scans that radio read-only: rc 0, ~4.7 s, AP on the other radio stayed `up`/`carrier`, `iw dev` and `uci changes` unchanged. A scan covers only its radio's band; a dual-band list needs one scan per radio (`phy1`/the AP ifname scans 2.4 GHz in ~2 s without dropping the AP). Captures: `testdata/device/axt1800-25.12.6/wifi-scan/`.
- [HW greenfield] `iwinfo scan` can exceed `TierRead` (about 4.7 s on 5 GHz), so scan replies use `TierScan`.
- [HW] Only enabled APs have a `hostapd.<ifname>` ubus object. `iwinfo assoclist` was empty while `hostapd.*` `get_clients` listed the station: use `get_clients`.
- Join allows up to 30 s for STA carrier and its kernel default route, based on roughly 10 s each for association and DHCP. The transaction’s shared live apply deadline can end the probe sooner. On AXT1800, reload can return before replacing the old STA, while it still has carrier and the default route (#115). The replacement has a different kernel link index even when the interface name is reused.
- [HW greenfield 2026-11-17] STA recovery without `wifi`: wpa_supplicant keeps scanning after the upstream goes away and re-associates on its own 2 to 4 s after it returns, with the DHCP address in the same 2 s sample. It does not leave `DISCONNECTED` (`wpa_supplicant.<ifname>` `control DISCONNECT`) or a runtime `bssid_filter` by itself. `ifup wwan` (= `ubus call network reload` plus interface down and up) recovered both in 6 to 8 s. It rebuilt the STA vif on radio0 (`wifi-scripts: Starting`), and hostapd logged `Reload all interfaces` / `Reloaded settings for phy phy1`. The AP on radio1 kept carrier in every 2 s sample, with no ath11k error and UCI unchanged. `iw dev <sta> disconnect` is refused (EPERM) while wpa_supplicant owns the STA. Basis of [ADR 0014](adr/0014-capped-uplink-reconnect.md).
- [UNVERIFIED] WPA2/WPA3 mixed-mode join stages `sae-mixed`. Its behavior on the AXT1800 remains to be verified.

## 3. ath11k crash triggers

- [HW] AP + STA on the same radio, committed even transiently → driver crash. The AP channel follows the STA channel; an upstream drop takes the local AP down.
- `wifi reload` is a crash risk on ath11k/IPQ6018. `wifi down/up` from timers or hotplug has no rollback → forbidden except bounded documented recovery.
- Required order: stage everything, reconcile layout, single commit, single apply.

## 4. Rollback

- [HW, AXT1800 25.12.3] rpcd `uci apply` with `rollback:true` snapshots at APPLY time. If the change was already committed, rollback restores the CHANGED config (30 s expired → broken profile still enabled, working uplink disabled). `uci revert` after commit is a no-op.
- Requirement: snapshot BEFORE the commit; restore explicitly; reload via `uci reload_config` + `network reload`. Never `wifi`.
- [HW, AXT1800 25.12.6] `reload_config` returns before netifd and hostapd act: after a wireless restore the STA route came back 4–6 s and hostapd's restored config ~2 s after it returned (#158). The watchdog therefore reports `restored` only after the default route via the pre-change uplink and every radio that was up before (`up`, not `pending`) are back, waiting up to ~20 s and cycling the uplink once if it stays without a route. Best effort: a missing route or radio does not turn a config restore into `failed`.
- rpcd `uci apply/confirm` REQUIRE an rpcd session id that bare ubus CLI cannot supply; login needs the real root password (anonymous login works only with NO root password). Travo does not use them (ADR-0005).

## 5. Caller classification (lockout)

- [HW] `network.interface dump` gives IPv4 as `{"address":"192.168.1.1","mask":24}`, NOT a CIDR string. A parser expecting a CIDR key made every client "unknown" while tests stayed green (the fixture invented the key).
- [HW] `br-lan` carries wired AND WiFi clients in one /24; subnet match cannot separate them. Use the neighbour table (ARP) → MAC → AP station dumps.
- Incident: an operator switched to Client mode from a phone on the AP; the AP vanished; recovery needed a wired console. Nothing asserted the rule.

## 6. Band switching

- [HW] STA band switch = change STA radio + apply/confirm = 2–5 s connectivity gap. Guard it. Scanning while STA is up works.

## 7. Captive portal and DNS

- [HW] A status probe with no default route took 3 s (DNS timeout). Skip the probe when there is no route.
- `noresolv=1` with custom forwarders blocks hotel DNS. Encrypted upstreams (DoH/DoT) cannot resolve hotel names. Rebind protection drops RFC1918 answers → must be off during bypass.
- The portal's first fetch may need a DHCP renew on wwan (MAC change / stale lease): conditional and guarded.
- [TRIED] Restore-on-reconnect inside a GET handler: every poll rewrote dnsmasq and restarted it. GET must never mutate.

## 8. Scheduled and button WiFi toggle (post-0.1)

- Cron is `/etc/crontabs/root`; `/etc/cron.d` does NOT exist (write → ENOENT; the UI then shows a schedule that never fires).
- Buttons: detect from devicetree `/sys/firmware/devicetree/base/keys/*/label`; `/etc/rc.button` is generic stock scripts, not a source of truth. Hotplug button scripts run as root via procd.
- The old toggle helper needed a stored rpcd login file; missing → every scheduled/button toggle silently no-opped. It also did uci set/commit/apply/confirm OUTSIDE in-process locks and could clobber the API's staged delta. In travo-greenfield helpers call the daemon instead (hook socket, ADR 0024).
- [BUG] Auto-reconnect cron ran `wifi up` every minute with no uplink → radios flapping. Cap and guard. greenfield uses `ifup wwan`, capped at 5 (§2, ADR 0014).
- [BUG] Radio switch via notification broke WiFi. Keep as a regression check.
- [BUG] After an OpenWrt upgrade Travo did not start (see §10).

## 9. mwan3 (post-0.1)

- [HW] Installing mwan3 with no working WAN → stock catch-all rules route 0/0 into an unhealthy policy → every policy `unreachable`, all outbound "Network unreachable" while lease and WiFi look fine. Recovery: remove the catch-all rules. Verify with a real outbound request, not uplink status.
- [HW] `uci apply mwan3` via rpcd → `Permission denied` until `luci-app-mwan3` is installed (its rpcd ACL). The package alone is not enough. A stuck failover guard can mean "missing ACL".
- The router may be unable to download `luci-app-mwan3` over its own uplink → side-load (`scp -O`, `apk add --allow-untrusted`).
- [TRIED, DATA LOSS] Ownership-by-content deleted stock `config interface 'wan'` (identical bytes to the generated default). Never delete by content match.
- Failback: evaluating "keep current active" first made failback unreachable. Priority walk first.

## 10. Sysupgrade and upgrades

- [HW] The keep-list must include `/etc/rc.d/S99travo` (boot link) as well as the init script, binary and UCI enable. A missing link after 25.12.3 → 25.12.6 left the service inactive, no API.
- After an upgrade the old binary may keep running: verify the running exe path; restart, not start.
- Package manager: 25.x = apk, older = opkg; detect at runtime. Package lists live in `/tmp` and vanish on reboot.
- Travo's own firmware upload drops installed packages; LuCI attended sysupgrade keeps them. Travo reports updates, LuCI flashes.
- [HW, GET-only] On 25.12.6 `/sbin/firstboot` is a 2-line wrapper for `/sbin/factoryreset` (`jffs2reset` links to it); `setsid` is `/usr/bin/setsid`. Factory reset = `firstboot -y && reboot`; never run it on the shared router.

## 11. AdGuard Home

- [HW, AXT1800 25.12.6 image, 2026-11-17] The firmware image can ship `adguardhome` preinstalled with an empty `/etc/adguardhome/`: after `firstboot` AGH ran its first-run wizard on `:::3000` for every LAN client. The init script `chown`s the config dir to the `adguardhome` user on each start. Reconcile therefore seeds the loopback config and restarts AGH whenever the package is installed but the config is missing.
- [HW] Reinstall over a config written by a newer AdGuard: v0.107.54 crash-loops on `unknown current schema version 33`. dnsmasq kept forwarding to `127.0.0.1#5353` → ALL LAN DNS dead, uplink fine, Travo reported healthy. Cached answers hide it for minutes; test with a fresh name.
- [HW] The init script read `/opt/AdGuardHome/AdGuardHome.yaml`; editing the path named in UCI `config_file` did nothing (relevant to the manual-binary install; the package route in ADR-0006 must be re-verified).
- Open decision: refuse a downgrade on reinstall instead of silent breakage.
- Old design exposed the first-run wizard LAN-wide and setting a password ADDED an admin instead of replacing one. Never reproduce: loopback bind, no accounts, Travo-gated proxy.
- [HW greenfield] Package `adguardhome` 0.107.79-r1 (25.12.6 feed): config `/etc/adguardhome/adguardhome.yaml`, work dir `/var/lib/adguardhome`, runs in ujail; yaml and `/etc/config/adguardhome` are apk conffiles and in `/lib/upgrade/keep.d/`.
- [HW greenfield] As old Travo left it: `0.0.0.0:3000`, `users: []` → full admin API (GET and POST) open to the LAN without auth. A root-path reverse proxy on its own port serves UI and API unchanged; AGH#7985 did not reproduce on 0.107.79.
- [HW greenfield] AGH stopped → LAN DNS REFUSED; a layer to the uplink resolver restored resolution in 0.13 s.
- Reported by others, unverified here: AGH on :53 leaves the router unable to resolve for itself; Tailscale MagicDNS overwrites `resolv.conf` (tailscale#15174, luci#8641).

## 12. USB tethering (0.2)

- Android: RNDIS on `usb0` or CDC-NCM on `usb0` (different kernel drivers); probe both. iOS needs `usbmuxd` + `libimobiledevice`, not plain RNDIS/NCM.
- Identify the uplink by MEDIUM, not by IP. It joins the `wan` zone → a direct firewall commit exception.

## 13. Clock and sessions

- No RTC: the router boots with a wrong clock. Sessions must not depend on wall clock. A far-future clock breaks TLS, cron, hwclock.

## 14. Logs, cron, flash, shell

- [HW] 25.12.6 ships no `/etc/crontabs/root` after firstboot and crond is not running: `/etc/init.d/cron` skips an empty `/etc/crontabs/`, so the first job needs a `cron restart`. The init script passes `-l ${cronloglevel:-7}`.
- [HW] BusyBox crond logs every job start as `cron.err` at the default level; a per-minute job floods errors. Set level 9.
- NAND overlayfs: writes = wear + stall. `/tmp` is RAM; staged UCI and package lists are lost on reboot.
- [HW] BusyBox on 25.12.6 has no `timeout`, `diff`, `pkill`. Bound commands in Go, not in shell.
- [HW] BusyBox `wget` has no `-S`; with no default route it fails at once with `Operation not permitted`, exit 4. `ubus call dhcp ipv4leases` does not exist on 25.12.6 (`Method not found`): read `/tmp/dhcp.leases`.
- [HW] BusyBox `tr` has no character classes: `tr '[:upper:]' '[:lower:]'` turned `OpenWrt` into `wenrt`. Use explicit ranges (`tr A-Z a-z`).
- BusyBox `sed` vs GNU `sed` escapes disagree: never escape JSON in sed; encode in code and write bytes verbatim. `wget -O` is required. Check signatures with `ubus -v list`; do not assume.
- `iw event` is a long-running listener: stop via signal, not a timeout tier. The uplink medium (ethernet/WiFi/USB) drives caller classification and failover candidates.

## 15. Tailscale (0.2)

Captures: `testdata/device/axt1800-25.12.6/tailscale/` (AXT1800, 25.12.6, 2026-11-17, not logged in).
- [HW] Package `tailscale` 1.104.1-r1 pulls `kmod-tun`; installed size 26 MiB, overlay grew 12.3 MiB. The post-install starts tailscaled at once (procd instance with `TS_DEBUG_FIREWALL_MODE=nftables`, port 41641, state `/etc/tailscale/tailscaled.state`). Settings in `/etc/config/tailscale` are daemon flags only; prefs live in the state file.
- [HW] `tailscale up --timeout=N` before login prints the auth URL, exits 1 with `timeout waiting for Tailscale service to enter a Running state`, and the backend keeps `AuthURL` in `tailscale status --json`. `up --json` prints `{AuthURL, QR, BackendState}`.
- [HW] `tailscale up` without every non-default pref is refused ("requires mentioning all non-default flags"); `tailscale set` is never refused and works while logged out (an exit node by IP is stored as `ExitNodeIP` until resolved).
- [HW] `tailscale status` (text) exits 1 when logged out; `status --json` exits 0. `tailscale debug prefs` prints the prefs JSON.
- [HW] After `up`, tailscaled adds `ip rule` 5210-5270 (`lookup 52`), the `tailscale0` link (MTU 1280, no address until login) and empty nft tables `ip filter` / `ip nat`.
- [HW] `apk del tailscale` leaves `/etc/tailscale/` (state, `ssh/` host keys once `--ssh` was set), `/var/lib/tailscale/`, the empty nft tables and the loaded `tun` module.
