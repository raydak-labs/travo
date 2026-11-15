# Spike results: GL-AXT1800, OpenWrt 25.12.6, 2026-11-09

Device: GL.iNet GL-AXT1800, OpenWrt 25.12.6 r33311-981c978eff (qualcommax/ipq60xx, ath11k), old Travo 0.10.2 installed.
Backup before any change: `~/travo-router-backups/20261109-171110-axt1800/` (sysupgrade backup, full overlay, RESTORE.md).
During live spikes, old Travo was stopped and its per-minute `wifi-reconnect.sh` cron paused; both restored afterwards. `/etc/config/{dhcp,wireless,network,firewall}` and the crontab were verified identical to the backup at the end.
Layout: radio0 = uplink STA `phy0-sta0` (wwan), radio1 = AP `phy1-ap0` (lan). Caller on Ethernet.

## S3 caller payloads: done
Captures in this directory (`PROVENANCE.md`, `with-ap-client/`). Parsers in `internal/device/facts`, end-to-end classification test in `internal/lockout/device_test.go`.
- `network.interface dump`: IPv4 as `{address, mask}`; `wan` (cable out) has no `l3_device`.
- `hostapd.<if> get_clients`: `{freq, clients: {<mac>: {assoc, authorized, ...}}}`; empty AP = `"clients": {}`.
- `ip -j -4 neigh`: `[{dst, dev, lladdr, state[]}]`. The phone on the AP and the Ethernet laptop are both on `br-lan`; only the hostapd MAC match separates them.
- Only one `hostapd.*` object exists (`hostapd.phy1-ap0`): disabled APs have none. `iwinfo assoclist` was empty while `get_clients` listed the station.

## S4 watchdog on hardware: done (logs `spikes/s4-*.log`)
| Check | Result |
|---|---|
| AP SSID change via `uci commit` + `reload_config` (no `wifi`) | live in 1.4 s |
| Not confirmed: watchdog restore | 16.2 s after begin (15 s window); original SSID live 1.0 s later |
| STA during AP-only change | stayed associated, default route kept |
| Confirm | change kept past the window |
| Wrong uplink SSID | `network.device status`: `present:true, up:true, carrier:false`; kernel default route gone |
| Daemon death, wrong uplink | watchdog restored at 16.2 s; STA carrier back 7.8 s later; default route 0.5 s after that |
| Reboot during window | boot hook ran `Recover` at uptime 13 s: `rolled_back`; original SSID live right after boot |

Window: 15 s minimum stays. Restore plus uplink re-association took about 25 s end to end, so the UI must keep showing "restoring" that long.

## S5 DNS on the device: done, one real bug found (`spikes/s5-*.log`)
- Base on this device: old Travo's AdGuard forwarding, `list server '127.0.0.1#5353'`, `noresolv=1`.
- **Bug**: `uci show` prints a one-element list exactly like an option. The stack restored it as `option server`, and dnsmasq's init script ignores `option server` (`config_list_foreach`). Result: no `server=` line, `noresolv` set, so every query was REFUSED. The config looked identical in `uci show`. Fixed: the CLI adapter reads with `uci export`, which keeps `list` vs `option`. A device-fixture regression test was added.
- After the fix: the captive layer over AdGuard resolves through the hotel/uplink resolver, and the pop restores base byte for byte with DNS working.
- `reload_config` reloads dnsmasq asynchronously (procd trigger): the new `server=` was live about 2 to 4 s after commit.
- AdGuard stopped: all LAN DNS REFUSED (confirms device-facts §11). Pushing a layer that points at the uplink resolver brought DNS back within 0.13 s (fail-open path works).

## S1 AdGuard package: done (`adguard/`)
- Package `adguardhome` 0.107.79-r1 (0.107.80-r1 available). Runs in `ujail` as user `adguardhome`.
- Config `/etc/adguardhome/adguardhome.yaml` (UCI `adguardhome.config.config_file`), work dir `/var/lib/adguardhome`. Both the yaml and `/etc/config/adguardhome` are apk conffiles and in `/lib/upgrade/keep.d/` (sysupgrade keeps them).
- As old Travo left it: `http.address: 0.0.0.0:3000`, `users: []`, DNS `bind_hosts: 0.0.0.0` port 5353. **The admin API is open to the whole LAN without auth** (GET and POST `/control/*` return 200). This is the risk from the baseline and still live on this device.
- `/control/install/get_addresses` returns 404 once configured, and `/install.html` still serves the wizard page.

## S2 AdGuard behind a proxy: done
A plain Go `httputil.ReverseProxy` on its own port (root path, no rewriting) served the UI, JS bundle and `/control/*` (GET and POST) against `users: []`. Issue AGH#7985 (`/control/profile` 401 without users) did not show up on 0.107.79. Conclusion: ADR 0006 works. Bind AGH to `127.0.0.1:3000`, and a Travo-gated port proxies it.

## New device facts
- BusyBox on the device has no `timeout`, `diff` or `pkill`.
- `wpad` provides hostapd and wpa_supplicant; `ubus -v list` shows `hostapd.<if>` with `get_clients`, `del_client`, `reload`.
- procd triggers: a `wireless` change reloads `network`, and a `dhcp` change reloads `dnsmasq` and `odhcpd`.

## WireGuard rollback acceptance (#108, 2026-11-15)

GL-AXT1800, OpenWrt 25.12.6, connected by Ethernet. A fresh sysupgrade
backup passed archive verification. The test used the production transaction
and watchdog code from PR #109, with a temporary hardware adaptation of the
container harness. It retained the existing key material, disabled peer
route installation and used two literal documentation addresses.

| Restore trigger | Result | Scenario elapsed |
| --- | --- | ---: |
| Explicit rollback | RolledBack, original live endpoint observed | 3.04 s |
| Probe failure after the new endpoint was live | RolledBack, original live endpoint observed | 3.22 s |
| Confirmation timeout | RolledBack, original live endpoint observed | 5.21 s |
| Startup recovery of a killed, unclaimed watchdog | RolledBack, original live endpoint observed | 3.43 s |
| Explicit rollback to a disabled baseline | RolledBack, kernel wg0 link absent | 7.44 s |

The first fixture setup attempt encountered netifd exit 4 before the logical
object appeared. The guard restored the original configuration. The passing
run waited for the netifd reload before checking the fixture endpoint.
After cleanup, configuration and crontabs matched the backup byte for byte.
The kernel default remained on the WiFi uplink, wg0 was absent and no
transaction guard remained.
