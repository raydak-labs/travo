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

## WireGuard toggle and default-route rollback (#87, #113)

Tested over Ethernet on OpenWrt 25.12.6, using the configured single-peer
test profile through SSH stdin. A fresh handshake let enable reach Pending
and confirm after 1.18 s. Disable removed wg0 and restored the underlying
kernel default before confirmation after 12.18 s.

An in-memory unreachable endpoint first exposed the rollback defect. The
watchdog returned RolledBack after 69.24 s with the network file restored
and wg0 absent, but the kernel had no default. Manual uplink restart
restored that route. After #113's watchdog fix, the same failed-enable
scenario reached RolledBack after 80.50 s with the original underlying
default present and wg0 absent. The network configuration and crontabs
matched the backup byte for byte, and the guard cleared.

## WiFi Join and fresh STA proof (#71, #115)

On the AXT1800 over Ethernet SSH, changing the key for the already-connected
WPA2 uplink could arm confirmation on the old STA's carrier and default route.
The next sample showed no carrier or default. A valid change likewise armed
on the old kernel link, before the replacement gained carrier and a route.

With the link-index check, the wrong key was rejected after 39.86 s without
reaching Pending. The original kernel default returned following rollback,
and configuration and crontabs matched the pre-test backup byte for byte.
For a valid key and a changed BSSID pin, the replacement had carrier at
17.21 s and the independent sample saw the kernel default at 17.62 s. The
transaction confirmed and the route persisted. The AP stayed up on its own
radio in both cases. Times are from invoking Join and include the site survey.
This test used WPA2; mixed WPA2/WPA3 was not exercised.


## WiFi Join browser acceptance (#71, 2026-11-15)

GL-AXT1800, OpenWrt 25.12.6, Ethernet. Tested the SPA and daemon built
from `2341539`, including #115, on a temporary LAN listener at port 8082.
Health reported `dev`; the running executable was verified at the temporary
test path. The installed daemon on port 80 stayed running. A fresh sysupgrade
backup was copied off the router before testing.

The browser signed in with the router's root password and used WiFi → Scan →
the operator-owned WPA2 uplink → Join. Credentials and network identities
are omitted here. Fetch observations recorded only HTTP status, transaction
state and elapsed time; they did not replace responses.

- Wrong key for the connected SSID: the page showed Connecting while POST
  was in flight, then Rolled back only after the 202 response reported
  `rolled_back`, after 39.74 s. No Pending response occurred. Independent
  samples observed STA carrier false and no STA default before recovery.
- Valid key, no confirmation: POST reached Pending after 15.65 s. The UI
  showed Confirm, Rollback and the countdown, then polled every two seconds.
  After the normal confirmation window expired, a GET returned
  `rolled_back` and the page showed Rolled back.
- Valid key, explicit confirmation: POST reached Pending after 21.54 s.
  Before confirmation, an independent SSH read proved STA carrier and the
  kernel default via that STA. Clicking Confirm returned `confirmed` and
  the page displayed Connection confirmed. The kernel default persisted.

A guarded API join first pinned the existing uplink BSSID so the browser's
un-pinned Join made an effective change. The timeout restored that pin;
final browser confirmation removed it. The setup confirmation connection
reset after the request; a subsequent read found the pin persisted and no
guard. This setup is separate from the successful browser confirmation.

The AP was up in the independent readable samples, including while the
wrong key had no carrier. `iw dev` showed AP and STA on separate radios.
A combined sample could not parse the transiently absent STA during reload;
it is not evidence of an AP outage or uninterrupted AP availability.
After recovery, all `/etc/config` files and crontabs in the pre-test backup
matched byte for byte, the kernel default used the original STA, and no
transaction guard remained. The temporary listener was stopped after testing.
Mixed WPA2/WPA3 was not tested. The AP lockout-refusal case is not applicable
to Join because it does not edit APs. `mise run check` passed with unchanged
limits (WiFi chunk 5,210 bytes gzip).

## VPN kill switch firewall (#89), 2026-11-17

AXT1800 25.12.6 over Ethernet, uplink STA `wwan`, operator WireGuard profile already in
`network.wg0` (one peer, `allowed_ips=0.0.0.0/0`, `route_allowed_ips=1`). Lock held,
`uci export firewall`/`network` saved before and restored byte for byte after (cmp), wg0
back to `disabled=1`. The LAN client was a laptop on `br-lan` bound to its router-facing NIC
(`curl --interface`); egress addresses compared by hash only.

- Stock firewall (zones `lan`, `wan` with `wan wan6 wwan`, forwarding lan -> wan), wg0
  enabled, handshake within 1 s, kernel `default dev wg0`: the LAN client had NO internet.
  wg0 is in no zone, so fw4 rejects lan -> wg0 forwards. The router itself was online.
- Added zone `travo_vpn` (network wg0, masq, mtu_fix) and forwarding `travo_vpn_lan`
  exactly as `vpnkill.VPNZone()` writes them: `forward_lan` gained
  `jump accept_to_travo_vpn`; the LAN client egressed with the VPN address.
- Added rule `travo_killswitch_wan` (src lan, dest wan, proto all, REJECT): fw4 placed
  `jump reject_to_wan` FIRST in `forward_lan`, before `accept_to_wan`. Tunnel up: LAN
  egress still via VPN. `ifdown wg0` removed every default route (device-facts §1); after
  `ifup wwan` the kernel default used the STA and the router was online, but the LAN
  client stayed blocked and the rule counter increased. The admin UI answered 200.
- Deleted the rule only: LAN egress returned via the uplink address.

## WireGuard split tunnel UCI shape (#90), 2026-11-17

Same session and restore as the kill switch spike above. Starting from a connected full
tunnel, I set every `wireguard_wg0` peer to `route_allowed_ips=0` and added
`network.travo_split_<n>` route sections (`interface=wg0`, `target=<a.b.c.d>/32`) for the
echo service and both profile resolvers, as `vpntunnel.SetSplit` stages them, then ran
`reload_config`, `ifdown wg0` and `ifup wg0`. The handshake came back within 1 s, and
`ip -4 route show dev wg0` listed the three /32 routes (netifd accepts CIDR in `target`).
No default route remained, so leaving full mode dropped the uplink default (device-facts §1).
After `ifup wwan` the default used the STA. The LAN client reached the echo service with the
VPN address, and other sites answered (HTTP 204) via the uplink. With
`travo_killswitch_wan` added, the echo service still answered through the tunnel and other
sites were blocked.

## VPN zone on tunnel enable (#87 follow-up), 2026-11-17

AXT1800 25.12.6, lock held, `uci export firewall`/`network` restored byte for byte after (cmp),
wg0 back to `disabled=1`, router and LAN egress via the uplink afterwards. Kill switch OFF
throughout (no `travo_killswitch_*`). The LAN client was bound to its `br-lan` NIC, and egress
addresses were compared by hash only.
- wg0 enabled, handshake within 1 s, `default dev wg0`, no `travo_vpn`: LAN client BLOCKED.
- Added `travo_vpn` + `travo_vpn_lan`, the sections tunnel enable now stages: LAN egress used the
  VPN address.
- Deleted both sections: LAN blocked again. Re-adding only the zone (`reload_config`, no wg0
  restart) restored LAN egress through the VPN. The wg0 link stayed present and the handshake
  stayed fresh, so the zone-only path does not need a tunnel restart.
## Captive bypass and wwan renew (#130, #131, 2026-11-17)

GL-AXT1800, OpenWrt 25.12.6, uplink wwan on phy0-sta0 (no AdGuard base: dnsmasq had no `server`, no `noresolv`, `rebind_protection 1`). `internal/captive/hardware_test.go` under the shared router lock: Enable pushed `list server` = the uplink DHCP resolver, `noresolv 0`, `rebind_protection 0`; resolution worked during the bypass (after the 2 to 4 s procd reload) and after restore; restore returned the base exactly. `RenewLease("wwan")` produced `udhcpc: sending renew` and an unchanged lease, no interface down; the default route via phy0-sta0 was proven after the 2 s settle (2.02 s total). `uci export` was identical before and after; no guard left. Not covered: a real portal (needs the simulator in testing.md).
