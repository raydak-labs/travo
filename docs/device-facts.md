# Device facts

Single source of hardware facts. Rules derived from them live in [invariants.md](invariants.md). `[HW]` = observed on hardware, `[TRIED]` = approach that failed, `[BUG]` = past defect kept as a regression check. Fixtures under `testdata/device/axt1800-25.12.6/` back the facts; spike results are indexed in its `SPIKES.md`.

Reference hardware: GL-AXT1800 (IPQ6018, ath11k; radio0 = 5 GHz, usually the uplink STA; radio1 = AP), OpenWrt 25.12.6, netifd 2026.02.26. Also targeted: GL-MT3000. The dnsmasq config is a NAMED section (e.g. `dhcp.cfg01411c`), not `@dnsmasq[0]`. `rg` is absent. BusyBox `sed`/`wget`/`tar` differ from GNU. `jsonfilter` is present. `scp` needs `-O`. `wpad` provides hostapd and wpa_supplicant. procd triggers: a `wireless` change reloads `network`, a `dhcp` change reloads `dnsmasq` and `odhcpd`.

## 0. UCI reads

- [HW] `uci show` prints a one-element LIST exactly like an option (`dhcp.cfg01411c.server='127.0.0.1#5353'`). dnsmasq's init script reads `server` only as a list, so writing it back as `option server` drops it: no `server=` line, `noresolv=1`, every query REFUSED. Read with `uci export` (keeps `list`/`option`) or `ubus call uci get`.
- [HW] Option order: `uci set` on an existing option keeps its place; a deleted then re-added option or list, or a new one, is appended. Restoring a section byte for byte needs the original order (`device.Section.Order` from `uci export`; the DNS layer stack re-appends owned options in base order).
- [HW] `reload_config` hands changes to procd triggers asynchronously: wireless live ~1.4 s, dnsmasq ~2 to 4 s after commit. Probes must wait, not read once.

## 1. Routes and netifd

- [HW] After WireGuard disable, `network.interface.wwan` status can list a DHCP default route while `ip -4 route show table main` has none (`wget` exits 4). Netifd state is not the kernel route; every route check reads the kernel table. `ifup wwan` restores it (and leaves stale `/32` endpoint host routes via the uplink). Captures: `wireguard/`.
- [HW] wg0 in no firewall zone: with `default dev wg0` LAN clients have no internet (fw4 rejects lan to wg0 forwards) while the router is online. A zone holding `wg0` with masq plus a lan to zone forwarding fixes it. Adding only the zone with `reload_config` is enough; wg0 needs no restart.
- [HW] netifd `config route` accepts CIDR in `target`. Switching a connected wg0 from `route_allowed_ips=1` to `0` plus `ifdown/ifup wg0` leaves no default route; `ifup wwan` restores it.
- [HW] fw4 puts user `rule` sections before zone forwardings in `forward_<zone>`: a lan to wan REJECT rule wins over the stock forwarding without editing it. A rule with `src '*'`, `dest '*'` and `list src_mac` renders in the global `forward` chain as `ether saddr <mac> ... jump handle_reject`, before the `jump forward_<zone>` dispatch (testdata `clients/`).
- [HW] On 25.12 `network.lan.ipaddr` is CIDR (`192.168.1.1/24`) with no `netmask`.
- [HW] Changing only `wg0_peer0.endpoint_host` plus `reload_config` keeps the old endpoint live; `ifup wg0` applies it. A WireGuard peer edit needs an interface restart.
- Container, not yet on hardware: netifd ignores an unknown option on a wireguard interface. `wg0` with `travo_dns` comes up without errors and `resolv.conf.auto` is unchanged; changing only that option plus `reload_config` keeps the interface `uptime` and ifindex; a `config route` with the extra option `travo_resolver` is installed as usual (procd `aarch64_generic-25.12.6`, #289). Hardware check: testing.md "VPN DNS layer".
- [HW] Disabled `wg0`: `network.interface.wg0 status` is `Not found`, `wg show` empty. Enabled without handshake: `up:true`, kernel `default dev wg0`, `latest-handshake` 0. Connected: same plus a non-zero handshake epoch. Only a recent handshake proves a working tunnel.
- [HW] `wg show all dump` prints the interface PRIVATE key in column 2 of the interface line; parse `wg show <if> latest-handshakes` or redact.
- The uplink is not always `wan`; internet often arrives via the WiFi STA. Recovery needs several bounded waits; some devices need script-level down/up, not only ubus.

## 2. Wireless liveness (ath11k / netifd)

- [HW] netifd emits `up`, `pending`, `retry_setup_failed`, `autostart` PER RADIO only. A per-interface entry is `{section, config, ifname, vlans, stations}`: no per-interface `up`.
- [HW] Per-interface truth: `network.device status {"name":"<ifname>"}` gives `present`, `up`, `carrier`.
- [HW] Uplink STA with wrong password or nonexistent SSID: radio reads up, STA `carrier:false`. Only carrier proves association. Association plus DHCP takes ~10 s.
- [HW] An invalid STA channel is silently ignored (AP stays up).
- [HW] An AP with zero clients is up. `stations` is always empty for mode=sta; never use `stations` or `assoclist` for liveness.
- [HW] Bring-up: AP link ~10 s after radio start; STA ~4 s; wwan up ~10 s after DHCP.
- [HW] If one radio hosts two expected interfaces (guest + main AP), the radio flag proves only one. Check each interface; a radio flag alone is a false confirm that cancels rollback.
- Ubus round trips ~5 ms; budget ~100 ms per call. An unreadable payload is not "down": report "cannot verify". Transport failure during a probe is unverifiable: retry, never claim "rolled back".
- [TRIED] Trusting per-interface `up` (absent on this firmware). [TRIED] A 4 s probe budget against a ~10 s bring-up.
- [HW] `uci commit wireless` plus `reload_config` (no `wifi`) applied and restored an AP SSID change in ~1 s each; the STA on the other radio stayed associated. After restoring a wrong STA SSID, STA carrier came back in 7.8 s and the default route 0.5 s later.
- [HW] `ubus call iwinfo scan '{"device":"phy0-sta0"}'` on the STA (associated or not) returns one entry per BSSID with `band`, `channel`, `mhz`, `signal` (dBm), `quality`/`quality_max`, `encryption.{enabled,wpa,authentication,ciphers}`; hidden networks have no `ssid` key. No crash with an AP up on the other radio (`wwan-down/`).
- [HW] With no STA interface, `iwinfo scan` on `phy0` or `radio0` scans that radio read-only: rc 0, ~4.7 s, the AP on the other radio stays up. A scan covers only its radio's band: a dual-band list needs one scan per radio (`phy1` or the AP ifname scans 2.4 GHz in ~2 s). Captures: `wifi-scan/`. The 5 GHz scan exceeds `TierRead`, so scan replies use `TierScan`.
- [HW] `hostapd.<ifname> del_client` takes `{addr, reason, deauth, ban_time}` (`clients/hostapd-ubus-list.txt`). A `FAILED` neighbour entry has no `lladdr`. `ubus -v list` shows `hostapd.<if>` with `get_clients`, `del_client`, `reload`.
- [HW] Only enabled APs have a `hostapd.<ifname>` object. `iwinfo assoclist` was empty while `get_clients` listed the station: use `get_clients`.
- [HW] A Join waits up to 30 s for STA carrier and its kernel default route (~10 s each for association and DHCP), bounded by the transaction's shared deadline. On AXT1800 `reload_config` can return before the old STA is replaced, while it still has carrier and the default route; the replacement has a different kernel link index even when the name is reused (`wifi-scan/connected/ip-link-indices.txt`).
- [HW] STA recovery without `wifi`: wpa_supplicant keeps scanning after the upstream goes away and re-associates 2 to 4 s after it returns, with the DHCP address in the same sample. It does not leave `DISCONNECTED` (`wpa_supplicant.<ifname>` `control DISCONNECT`) or a runtime `bssid_filter` by itself. `ifup wwan` (= `ubus call network reload` plus interface down and up) recovers both in 6 to 8 s; it rebuilds the STA vif on radio0 and hostapd logs `Reload all interfaces`; the AP on radio1 keeps carrier. `iw dev <sta> disconnect` is refused (EPERM) while wpa_supplicant owns the STA. Basis of [ADR 0014](adr/0014-capped-uplink-reconnect.md).
- [UNVERIFIED] WPA2/WPA3 mixed-mode join (`sae-mixed`) on the AXT1800.

## 3. ath11k crash triggers

- [HW] AP + STA on the same radio, committed even transiently, crashes the driver. The AP channel follows the STA channel; an upstream drop takes the local AP down. A second AP hot-added to the STA radio never started (`handle_probe_req: send failed`).
- `wifi reload`, `wifi down/up` are crash risks; see [invariants.md](invariants.md) §1 and §10.

## 4. Rollback

- [HW, 25.12.3] rpcd `uci apply` with `rollback:true` snapshots at APPLY time. If the change was already committed, rollback restores the CHANGED config (when the 30 s expired, the broken profile stayed enabled and the working uplink disabled). `uci revert` after commit is a no-op. rpcd `uci apply/confirm` also need an rpcd session id that bare ubus cannot supply (anonymous login works only with no root password). Travo does not use them ([ADR 0005](adr/0005-transactions-and-watchdog.md)).
- [HW, 25.12.6] `reload_config` returns before netifd and hostapd act: after a wireless restore the STA route came back in 4 to 6 s and hostapd's restored config ~2 s after it returned. After restoring the configs, the watchdog waits up to ~20 s for the default route via the pre-change uplink and every previously up radio, cycling the uplink once if it is still without a route. It then reports `restored` whether or not they came back (best effort, `internal/txn/watchdog.sh` `settle_live`).

## 5. Caller classification (lockout)

- [HW] `network.interface dump` gives IPv4 as `{"address":"192.168.1.1","mask":24}`, not a CIDR string. Neither does `wan` (cable out) have an `l3_device`.
- [HW] `br-lan` carries wired and WiFi clients in one /24; a subnet match cannot separate them. Use the neighbour table (`ip -j -4 neigh`: `[{dst, dev, lladdr, state[]}]`) to get the MAC, then match hostapd `get_clients` (`{freq, clients: {<mac>: {assoc, authorized, ...}}}`; empty AP = `"clients": {}`).

## 6. Band switching

- [HW] An STA band switch (change STA radio, apply, confirm) costs a 2 to 5 s connectivity gap. Scanning while the STA is up works.

## 7. Captive portal and DNS

- [HW] A status probe with no default route took 3 s (DNS timeout): skip it when there is no route.
- `noresolv=1` with custom forwarders blocks hotel DNS; DoH/DoT upstreams cannot resolve hotel names; rebind protection drops RFC1918 answers and must be off during bypass.
- The portal's first fetch may need a DHCP renew on wwan (MAC change or stale lease). `RenewLease("wwan")` produced `udhcpc: sending renew` and an unchanged lease with no interface down.
- [TRIED] Restore-on-reconnect inside a GET handler rewrote and restarted dnsmasq on every poll. GET never mutates.

## 8. Scheduled and button WiFi toggle

- Cron is `/etc/crontabs/root`; `/etc/cron.d` does not exist (a write fails with ENOENT and the UI shows a schedule that never fires).
- [HW] 25.12.6 has no zoneinfo. `/etc/TZ` holds the POSIX string (`GMT0`), `/etc/localtime` points to a missing `/tmp/localtime`, so Go's `time.Local` is UTC; crond (musl) applies the TZ. Schedules fire from cron, not a daemon timer ([ADR 0027](adr/0027-wifi-schedule.md)).
- Buttons: detect from devicetree `/sys/firmware/devicetree/base/keys/*/label`; `/etc/rc.button` is generic stock scripts.
- [HW] AXT1800 keys: `reset` (code `0x198` KEY_RESTART, hotplug `BUTTON=reset`, no input type = EV_KEY) and `switch` (code `0x100`, `linux,input-type` 5 = EV_SW, hotplug `BUTTON=BTN_0`, `ACTION=pressed|released`). Labels end in NUL, cells are big-endian u32. `/etc/hotplug.json` runs `/etc/rc.button/%BUTTON%` and, separately, `hotplug-call button` when `/etc/hotplug.d/button` exists (absent on the stock image); `hotplug-call` sources each script in a subshell with `PATH=/usr/sbin:/usr/bin:/sbin:/bin` and waits, so a slow action runs detached. Captures: `buttons/`. Helpers call the daemon over the hook socket ([ADR 0024](adr/0024-daemon-hooks-for-cron-and-buttons.md)).
- [HW] LEDs (`/sys/class/leds`): `blue:run` (trigger `none`, brightness 1 after boot), `white:system`, `mmc0::` (trigger `mmc0`). `/etc/init.d/led` is an rc.common script, not procd: `reload_config` does not apply a `system` led section, `/etc/init.d/led restart` does (no radio effect). `load_led` records each LED's prior trigger and brightness in `/var/run/led.state`; the next start replays it, so deleting the section and restarting restores the LED exactly. Captures: `leds/`.
- [BUG] Auto-reconnect that ran `wifi up` every minute with no uplink flapped the radios. Now `ifup wwan`, capped at 5 ([ADR 0014](adr/0014-capped-uplink-reconnect.md)).
- [BUG] A radio switch via notification broke WiFi. Keep as a regression check.

## 9. mwan3

Design: [ADR 0018](adr/0018-mwan3-failover.md); rules: [invariants.md](invariants.md) §9. Hardware facts (mwan3 2.12.0-r3, iptables-nft 1.8.10, captures in `mwan3/`):
- [HW] Installing mwan3 with no working WAN: the stock catch-all rules route 0/0 into an unhealthy policy and all outbound traffic fails while lease and WiFi look fine. Verify with a real outbound request, not uplink status.
- [HW] `uci apply mwan3` via rpcd is `Permission denied` until `luci-app-mwan3` (its rpcd ACL) is installed. Side-load with `scp -O` and `apk add --allow-untrusted` if the router cannot download it over its own uplink.
- [TRIED, DATA LOSS] Ownership by content deleted the stock `config interface 'wan'` (identical bytes to the generated default). Never delete by content match.
- [HW] Interface sections without `track_ip`: `mwan3 interfaces` says "unknown and tracking is disabled", the policy never gets a member and everything uses the main table. With `track_ip 1.1.1.1 8.8.8.8`, reliability 1, interval 5, down 3: offline 12 s after an nft postrouting drop on `phy0-sta0`, online 12 s after removing it.
- [HW] A native nft rule in `ip mangle` (Tailscale 1.104.1 in nftables mode writes `ct mark` rules there) breaks `iptables -t mangle -S` ("table 'mangle' is incompatible, use 'nft' tool"), so mwan3's restore batches fail with "Chain already exists" and router-local traffic keeps using the main table. The writer on the router was inferred, not observed. Container reproduction: `testdata/container/openwrt-25.12.6/mwan3/`.
- [HW] Empty `ip`/`ip6 mangle` base chains left behind (OUTPUT as `type filter hook output priority mangle`) also break it: iptables-nft wants OUTPUT as `type route`, `iptables -t mangle -S` prints "contains incompatible base-chains" (exit 0) and mwan3's `set_general_iptables` batch fails with `CHAIN_ADD failed (File exists): chain OUTPUT`: no `mwan3_hook`, hotplug "called ... before mwan3 has been set up", and the policy only gets `out <iface> <dev>` rules. Deleting the empty chain fixes it; iptables-nft recreates OUTPUT as `type route`. Captures `mwan3/*stale-base*`.
- [HW] Failover timing, wan (Ethernet) to usbtether (iPhone, 172.20.10.0/28), primary dropped with nft: detection 16 to 17 s (down 3 x interval 5 s), failback 31 to 33 s (up 6 x 5 s). `ip route get 1.1.1.1 mark 0x600` names the usbtether device; an unmarked `ip route get` keeps naming `wan` (only mangle-marked traffic moves).
- [HW] Uplinks in one subnet (wan cable and wwan WiFi to the same home router, 10.0.1.0/24): with wan dropped, wwan's tracking went offline too and the policy had no member for 91 s. Disable leaves only the iptables-nft base chains (PREROUTING filter, OUTPUT route) in `ip|ip6 mangle`.

## 10. Sysupgrade and upgrades

- [HW] The keep-list must include `/etc/rc.d/S99travo-greenfield` (boot link) as well as the init script, binary and UCI enable. A missing link after 25.12.3 to 25.12.6 left the service inactive.
- After an upgrade the old binary may keep running: verify the running exe path; restart, not start.
- Package manager: 25.x = apk, older = opkg; detect at runtime. Package lists live in `/tmp` and vanish on reboot; `apk list --upgradable` reads only those (empty until `apk update`).
- [HW, GET-only] `/sbin/sysupgrade` exists; `ubus -v list system` offers `validate_firmware_image {path}` (fwtool signature, board match and `platform_check_image`, read-only) and `sysupgrade`. User-installed packages = `/etc/apk/world` minus `/rom/etc/apk/world`. The sysupgrade image's own package list is not readable without unpacking squashfs. `/tmp` is a 204 MiB tmpfs with ~149 MB RAM available. Never flash the shared test router.
- [HW] The release index is `<release.firmware_url>.versions.json` (`https://downloads.openwrt.org/`): `stable_version`, `versions_list` with `-rcN` entries. LuCI attended sysupgrade is at `admin/system/attendedsysupgrade` on LuCI's port (:8080 after install). A firmware upload through Travo drops installed packages; attended sysupgrade keeps them.
- [HW, GET-only] `/sbin/firstboot` is a 2-line wrapper for `/sbin/factoryreset` (`jffs2reset` links to it); `setsid` is `/usr/bin/setsid`. Factory reset = `firstboot -y && reboot`.

## 11. AdGuard Home

Design: [ADR 0006](adr/0006-adguard-package-and-topology.md), [ADR 0021](adr/0021-adguard-expert-topology.md).
- [HW] Package `adguardhome` 0.107.79-r1 (25.12.6 feed): runs in ujail as user `adguardhome`; config `/etc/adguardhome/adguardhome.yaml` (UCI `config_file`), work dir `/var/lib/adguardhome`; yaml and `/etc/config/adguardhome` are apk conffiles and in `/lib/upgrade/keep.d/`. `/control/install/get_addresses` returns 404 once configured; `/install.html` still serves the wizard page.
- [HW] The firmware image can ship `adguardhome` with an empty `/etc/adguardhome/`: after `firstboot` AGH ran its first-run wizard on `:::3000` for every LAN client. The init script `chown`s the config dir to the `adguardhome` user on each start. Reconcile therefore seeds the loopback config and restarts AGH whenever the package is installed but the config is missing.
- [HW] The old Travo left `http.address: 0.0.0.0:3000`, `users: []`, `bind_hosts 0.0.0.0` port 5353: a full admin API open to the LAN without auth. A root-path reverse proxy on its own port serves UI and API unchanged; AGH#7985 did not reproduce on 0.107.79.
- [HW] Reinstall over a config written by a newer AdGuard: v0.107.54 crash-loops on `unknown current schema version 33` while dnsmasq keeps forwarding to `127.0.0.1#5353`, so all LAN DNS is dead. Cached answers hide it for minutes; test with a fresh name.
- [HW] AGH stopped: LAN DNS REFUSED; a layer to the uplink resolver restored resolution in 0.13 s.
- [HW] `/var` is `/tmp` (tmpfs): AGH's work dir (query log, `stats.db`, filter lists) is RAM. The package default kept 90 days of query log.
- [HW] The init script (`adguard/initd.txt`) maps UCI `adguardhome.config.memlimit`/`gc`/`maxprocs` (uinteger bytes / percent / count, `0` = unset) to `GOMEMLIMIT`/`GOGC`/`GOMAXPROCS` in the procd environment, read at process start: a change needs an AGH restart ([ADR 0006](adr/0006-adguard-package-and-topology.md#memory-caps)).
- [HW] Memory (418 MB MemTotal): ~160 MB available after boot; the kernel and WiFi (ath11k) take ~75 MB within 20 min, leaving ~80 MB. Travo RSS ~19 to 21 MB. Uncapped AGH with the hagezi pro list: RSS 75 MB growing ~0.8 MB/h, OOM-killed at 116 MB anon-rss after about a day. Capped (6 h soak, wwan uplink, two APs): AGH RSS ~67 MB, system anon pages flat ~74 MB, yet MemAvailable fell from ~75 MB to 20 to 30 MB; 14 h after boot MemAvailable 45480 kB, Slab 38292 kB (SUnreclaim 29468 kB). Kernel or driver memory grows over hours; the cause is open (#285). Captures: `adguard/memory.txt`, `system/proc-meminfo.txt`.
- [HW] dnsmasq on port 5354 listens on every address (127.0.0.1, lan, guest, uplink lease, tailscale 100.x, IPv6), like stock dnsmasq on :53 (`nonwildcard` = `--bind-dynamic`, no `interface`). dnsmasq 2.93 cannot split DNS from DHCP binding (rootfs container, second container as DHCP client): `--listen-address=127.0.0.1` binds DNS to loopback but DHCP gives no lease; adding `--interface=<lan>` brings DHCP back and DNS onto the lan address; `--except-interface=<lan>` drops DHCP too; `--local-service=host` binds loopback only with no lease, and with `--bind-dynamic` (UCI `nonwildcard`, default 1) dnsmasq refuses to start. UCI `localservice` is a boolean; `extraconftext` is the only way to pass `=host`.
- Reported by others, unverified here: AGH on :53 leaves the router unable to resolve for itself; Tailscale MagicDNS overwrites `resolv.conf` (tailscale#15174, luci#8641).

## 12. USB tethering

Design: [ADR 0019](adr/0019-usb-tether-uplink.md).
- Android: RNDIS or CDC-NCM (different kernel drivers, usually `usb0`); probe both. iOS needs `usbmuxd` + `libimobiledevice` and the `ipheth` driver (the iPhone showed up as `eth0` on 25.12.6).
- Identify the uplink by medium (sysfs subsystem and driver), not by IP.

## 13. Clock and sessions

- No RTC: the router boots with a wrong clock. A far-future clock breaks TLS, cron, hwclock; sessions must not depend on wall clock.

## 14. Logs, cron, flash, shell

- [HW] 25.12.6 ships no `/etc/crontabs/root` after firstboot and crond is not running: `/etc/init.d/cron` skips an empty `/etc/crontabs/`, so the first job needs a `cron restart`. The init script passes `-l ${cronloglevel:-7}`.
- [HW] BusyBox crond logs every job start as `cron.err` at the default level; a per-minute job floods errors. Set level 9.
- NAND overlayfs: writes cost wear and stall. `/tmp` is RAM; staged UCI and package lists are lost on reboot.
- [HW] BusyBox on 25.12.6 has no `timeout`, `diff`, `pkill`. Bound commands in Go.
- [HW] BusyBox `wget` has no `-S`; with no default route it fails at once with `Operation not permitted`, exit 4. `wget -O` is required. `ubus call dhcp ipv4leases` does not exist (`Method not found`): read `/tmp/dhcp.leases`.
- [HW] BusyBox `tr` has no character classes: `tr '[:upper:]' '[:lower:]'` turned `OpenWrt` into `wenrt`. Use `tr A-Z a-z`.
- BusyBox `sed` and GNU `sed` escapes disagree: never escape JSON in sed; encode in code. Check signatures with `ubus -v list`.
- `iw event` is a long-running listener: stop via signal.

## 15. Tailscale

Design: [ADR 0017](adr/0017-tailscale.md). Captures: `tailscale/` (AXT1800, 25.12.6, not logged in).
- [HW] Package `tailscale` 1.104.1-r1 pulls `kmod-tun`; installed size 26 MiB, overlay grew 12.3 MiB. The post-install starts tailscaled at once (procd instance with `TS_DEBUG_FIREWALL_MODE=nftables`, port 41641, state `/etc/tailscale/tailscaled.state`). `/etc/config/tailscale` holds daemon flags only; prefs live in the state file.
- [HW] `tailscale up --timeout=N` before login prints the auth URL and exits 1 (`timeout waiting for Tailscale service to enter a Running state`); the backend keeps `AuthURL` in `status --json`. `up --json` prints `{AuthURL, QR, BackendState}`.
- [HW] `tailscale up` without every non-default pref is refused ("requires mentioning all non-default flags"); `tailscale set` is never refused and works while logged out (an exit node by IP is stored as `ExitNodeIP`).
- [HW] `tailscale status` (text) exits 1 when logged out; `status --json` exits 0. `tailscale debug prefs` prints the prefs JSON.
- [HW] After `up`, tailscaled adds `ip rule` 5210 to 5270 (`lookup 52`), the `tailscale0` link (MTU 1280, no address until login) and empty nft tables `ip filter` / `ip nat`.
- [HW] `apk del tailscale` leaves `/etc/tailscale/` (state, `ssh/` host keys once `--ssh` was set), `/var/lib/tailscale/`, the empty nft tables and the loaded `tun` module.
