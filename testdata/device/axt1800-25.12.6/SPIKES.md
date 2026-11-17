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
## STA recovery without `wifi` (#73, 2026-11-17)

Router shared under the agent lock; `uci export` before and after was byte-identical.
Layout: STA `wwan` on radio0 (5 GHz), AP on radio1 (2.4 GHz). Sampled every 2 s:
STA and AP `network.device status` carrier, `wwan` IPv4 present, and `wpa_state`
from `ubus call wpa_supplicant.phy0-sta0 control '{"command":"STATUS"}'`.
SSIDs, BSSIDs and addresses are not recorded.

- `iw dev phy0-sta0 disconnect`: `Operation not permitted (-1)`. wpa_supplicant owns the STA.
- Upstream hidden with `control "SET bssid_filter 02:00:00:00:00:01"` and then
  `DISCONNECT`/`RECONNECT`: `SCANNING`, no STA carrier and no wwan IPv4 for 40 s.
  After the filter was cleared, with no other action, the STA was `COMPLETED` with
  carrier and IPv4 at the next sample (2 s). (`BSSID_IGNORE` is not a valid
  stand-in: wpa_supplicant cleared it and re-associated within 4 s.)
- `ifup wwan` while the filter was still set: the STA was `COMPLETED` with
  IPv4 after 6 s. netifd rebuilt the STA, which reset the runtime filter.
- Sticky `control DISCONNECT`: `DISCONNECTED` for 10 s with no self-recovery.
  `ifup wwan` led to `SCANNING` at 4 s and `COMPLETED` with IPv4 at 8 s.
- AP `phy1-ap0` carrier was true in every sample. logread for each `ifup` showed
  `hostapd: Reload all interfaces`, `Reloaded settings for phy phy1`,
  `radio0 ... wifi-scripts: Starting`, and `Interface 'wwan' is disabled/enabled`.
  It also showed a harmless `rmdir[ctrl_interface=...]: Permission denied` from
  wpa_supplicant. dmesg had no new ath11k lines.
- `/sbin/ifup` on this firmware is `ubus call network reload` followed by
  `network.interface down` and `up` for the named interface.

Outcome: ADR 0014 adopts `ifup wwan`, capped at 5, after two disconnected
60 s ticks. Plain `ubus call network.interface.wwan down/up` without the global
reload was not measured; the lock was held by another agent.

## 0.1.0 release playbook, 2026-11-17

GL-AXT1800, OpenWrt 25.12.6 r33311-981c978eff. Build: `mise run package` of main at `dc403f6`
(`VERSION=0.1.0-rc.dc403f6`), installed with `install.sh --insecure` (no release key used).
Exclusive router lock held for the run; fresh `sysupgrade -b` taken first and kept outside the
repo. Caller on Ethernet (br-lan) unless noted; API reached directly on `192.168.1.1` or through an
SSH local forward (which the daemon correctly classifies as an unknown caller, see step 2).
Layout throughout: uplink STA `phy0-sta0` (wwan) on radio0, AP `phy1-ap0` on radio1. SSIDs,
BSSIDs, MACs, keys, addresses and endpoints are not recorded.

| Step | Result | Notes |
| --- | --- | --- |
| 1 install / LuCI port | pass | Clean baseline via `--uninstall --purge` of the previous build. That left a stray `/www/travo-greenfield` from an out-of-band asset deploy (not in its manifest), removed by hand. Clean: LuCI login on :80 OK, uhttpd `list listen_http '0.0.0.0:80'`, `'[::]:80'`, `list listen_https '0.0.0.0:443'`, `'[::]:443'`. Install without `--port`: 4 s, LuCI moved to 8080/8443, daemon on `:::80`, greenfield login OK on `http://192.168.1.1/`, LuCI login OK on :8080. `sysupgrade -l` lists config, init, rc links, binary, `manifest.txt`, `uhttpd.orig` and the `/www` assets. After the step-8 reboot both UIs answered. `--uninstall`: the four listener values back in the original order and `list` form, LuCI login OK on :80, then reinstalled. Cosmetic: the first install prints `Command failed: ubus call service delete ... (Not found)`. |
| 2 wireless change not confirmed | pass | AP rename reached Pending after 2.3 s, and `iwinfo` already showed the new SSID. Unconfirmed: the watchdog restored after the 60 s window and `uci changes` was empty. Rename + confirm: still live 70 s later. Renamed back + confirm. `rolled_back` is reported about 2 s before hostapd reloads (#158). |
| 3 lockout from a phone | operator | No phone on the AP. From Ethernet: disabling the only enabled AP over the SSH forward (unknown caller) gave 409 `wifi_lockout_risk` with no write, the fail-closed direction. From the wired LAN it proceeded. |
| 4 wrong uplink password | pass with #158 | Wrong key for the connected SSID: `rolled_back` after 39.8 s, never Pending, wireless config byte-identical. At the response the STA had no carrier or default route; the route returned 4 to 6 s later (#158). |
| 5 DNS captive over AdGuard / fail-open | pass with #157 | Bypass over the AdGuard base and restore: same values, but `list server` moves after `option noresolv`, so not byte-identical (#157). AdGuard stopped: no `server`/`noresolv` after 40 s (repeat: 33 s), LAN resolution via the uplink. |
| 6 config backup | pass | `GET /api/v1/system/backup` 200 `application/gzip`, `gzip -t` OK, 107 entries incl. `etc/config/*`. Archive deleted after the check. |
| 7 dashboard connectivity | pass | Connected: `phy0-sta0`, carrier true, online, matching the kernel default and `wwan` status. `ifdown wwan`: Offline, interface and carrier null, answered in 7 ms. `ifup wwan`: route back after about 11 s. |
| 8 reboot (#65) | pass (API) | (a) Pending AP change: 409 `transaction_pending`, no reboot, guard only `txn-in-progress`. (b) Confirmed reboot: 202. New health `boot_id` after 41 s. Uptime 0 min, kernel boot_id changed, no guards, no `reboot.json`, no `stale guard` log. The UI "Restarting…" and "Router restarted" text was not watched (operator). |
| 8 WireGuard status (#86) | pass | `disabled`, then `configured` after select. `no_handshake` with null age during the unreachable-endpoint enable (24/25 samples). `connected` with handshake age 21 s. |
| WiFi Join (#71) | pass with #158 | Join with a 5 GHz BSSID pin: carrier 16.6 s, kernel default 17.7 s, Pending 17.7 s; confirmed, route persisted. Unconfirmed: rolled back after the window and the route returned about 6 s after `rolled_back`. The AP stayed up on radio1 throughout. WPA2/WPA3 not exercised. |
| Saved uplinks (#72) | pass | Two saves, wireless unchanged and no `uci changes`; file `-rw-------`. The order survived reorder + daemon restart. Selecting the connected profile: 409 `wifi_no_change`. After a BSSID pin, selecting it: Pending 16 s, confirmed, `connected_id` matches the live SSID. Forgetting it kept carrier and the default. The second profile was a neighbour network (not ours), so it was never joined. |
| WireGuard toggle (#87) | pass | Enable: Pending 1.4 s, handshake under 1 s, `default dev wg0`, LAN egress = VPN hash; confirmed. Disable: Pending 14.3 s, wg0 absent and default via `phy0-sta0` before confirm, LAN egress = uplink hash. Unreachable documentation endpoint from a disabled baseline: `rolled_back` after 80.6 s, no wg0, default via the STA, configs identical. Dashboard VPN card (#63) not checked (UI, operator). |
| VPN zone on enable | pass | Zone deleted by hand: LAN blocked; enable again re-added only the zone (same wg0 ifindex, handshake 6 s old), LAN egress via the VPN. First enable without the zone and unconfirmed: `restoring` at 62 s, `rolled_back` at 78 s, `firewall` and `network` byte-identical, no wg0. |
| VPN kill switch (#89) | pass | Rule `travo_killswitch_wan` (REJECT lan→wan) first in `forward_lan`, before `jump accept_to_wan`. User `@forwarding[0]` lan→wan unchanged. `ifdown wg0`: LAN curl failed (exit 7), router :80 200. `ifup wg0`: VPN egress. Tunnel off: LAN still blocked. Kill switch off: uplink egress, `travo_vpn` kept. Unconfirmed enable: firewall byte-identical after rollback; the live nft rule cleared about 2 s after `rolled_back` (#158). Enabling with no tunnel: 409 `vpn_not_connected`. |
| VPN split tunnel (#90) | pass | One /32 for the echo service: 3 `travo_split_*` sections (echo + 2 resolvers), `route_allowed_ips='0'`, 3 routes on wg0, default via the STA. Echo service = VPN hash, other site = uplink hash. Kill switch on: echo still answers, other sites blocked. Empty list: `default dev wg0`, no `travo_split`, `network` byte-identical to before. Unconfirmed: `restoring` 61 s, `rolled_back` 63 s, byte-identical. |
| VPN DNS layer (#88) | pass | The layer is present at confirm; `top: vpn`, verify `ok`. Both resolvers route via wg0 (`ip route get`; no tcpdump on the device). AdGuard on top: verify `overridden`, `top: adguard`, server = `127.0.0.1#5353`. AdGuard off: VPN resolvers return. Disable: record gone and dhcp equal to base within 30 s. |
| WireGuard rollback (#108) | partial | Failed probe with the tunnel enabled: the documentation endpoint was live in `wg show wg0 endpoints` for 64 s, then the old endpoint returned with a fresh handshake; `rolled_back` after 67.4 s and `network` byte-identical. The explicit-rollback, confirm-timeout and killed-watchdog paths need a second reachable endpoint (operator). The container script was not run here. |
| Access point settings (#74) | pass (Ethernet part) | Rename: Pending 2.3 to 3 s, only after `iwinfo` showed the SSID. Disable from the wired LAN: Pending 2.3 s with `phy1-ap0` gone from `network.wireless status`; rollback brought it back. Enabling the 5 GHz AP while the STA is on radio0: 409 `wifi_radio_in_use`, `uci changes` empty. Phone lockout: operator. `TestHWAPRenameProvesNewSSIDThenRollsBack` PASS (2.3 s). |
| Guest AP (#75) | pass (Ethernet part) | Pending 3.9 s with `phy1-ap0` and `phy1-ap1` up/present, guest SSID in `iwinfo`, `guest` up. Fixtures in `guest/`. Unconfirmed: rolled back at 45 s, all four configs byte-identical, no `br-guest`. Confirmed + same settings again: 409 `wifi_no_change`. Phone checks (192.168.2.x, internet, no LAN access): operator. Guest removed afterwards. |
| Uplink MAC policy (#76) | pass | Factory MAC is universally administered (LAA bit 0). Random: Pending after 8.4 s with an LAA MAC, carrier and route; rollback restored the factory MAC and route within 5 s. Custom LAA: Pending 7 s, confirmed. Default: Pending 8 s, confirmed; wireless byte-identical. `TestHWUplinkMACRandomThenRollsBack` PASS (9.3 s). |
| Join AP reconcile (#99) | pass (Ethernet part) | AP on radio0 (5 GHz), radio1 AP and STA disabled; joined the uplink pinned to its 5 GHz BSSID. Pending 19.6 s with the AP on `phy1-ap0` (moved as `default_radio0.device='radio1'`) and the STA with carrier and default; confirmed. Unconfirmed repeat: `rolled_back` at 61 s, AP back on radio0 about 2 s later, wireless byte-identical to the baseline. The dialog text is not checked in the UI. From a phone: operator. |
| Uplink reconnect cap (#73) | partial, bug fixed (#159) | Sticky `DISCONNECT` is healed by attempt 1 (`ifup wwan` rebuilds the STA), so the 5-attempt cap cannot be reached without powering off the upstream (operator). Attempt 1 came 24 s after the outage, because the previous tick fell inside a profile-select outage (wwan down 14:36:30 to 14:36:35): fixed in #159. The counter returned to 0 within 90 s of the address coming back. No `wifi` command or ath11k error, AP carrier kept, `uci export` unchanged. A 45 s outage healed by `RECONNECT` gave no attempt. |
| AdGuard install / migration (#132) | pass (a, c) | (c) `apk del`, yaml and marker removed, daemon restarted: 0.107.80-r1 installed. 0 samples with a wildcard :3000 (1 s polling); loopback :3000 after 65 s. The yaml equals `internal/adguard/seed.yaml`; hagezi pro enabled (242264 rules); no install guard. (a) Old-Travo yaml (`0.0.0.0:3000`, `bind_hosts 0.0.0.0`, AGH on `:::3000` and `:::5353`): migrated within 1 s of daemon start; only `127.0.0.1:3000` and `:5353` listen, LAN :3000 refused, the diff changes only those two lines, no `.old` file or guard. (b) port-taken restore: not run. |
| AdGuard default DNS (#133) | pass | (a) Adopt: record within 1 s, dhcp byte-identical. (b) Fail-open 40 s / 33 s, `fail_open_since` set. (c) Hold-down: the layer returned about 10.5 min after AGH restarted; re-enable via `POST /adguard/dns` took 0.4 s. (d) Service off: layer removed, 0/40 lookups failed during the call, never re-added. (e) `ifdown wwan` for 60 s: no fail-open. Note: a transient VPN outage (no handshake while wg0 held the default) correctly failed AGH open too. |
| AdGuard gated proxy (#134) | pass (curl) | No session: 401. Session: dashboard 200, `/control/status` 200, protection POST with the same origin 200. `Origin: http://evil.example`: 403. AGH stopped: 502 `adguard_unavailable`. Uplink side: wan zone input REJECT (config only, no probe from the uplink network). |
| Captive DNS bypass (#130) | pass with #157 (no portal) | Bypass without a portal: `server` = uplink resolver, `rebind_protection 0`, `noresolv 0`, countdown 294. Restore now: values back (order: #157). Daemon restart during the bypass: countdown re-armed at 297 s; auto-restore landed within 10 s of the deadline (2 s poll). No `dns-layer-in-progress` guard. Portal name resolution: operator. |
| Captive portal (#129), DHCP renew (#131) | operator | Needs the portal simulator on the WAN port. `TestHWBypassUsesUplinkDNSAndRestoresBase` PASS (8.7 s), `TestHWRenewWWANProvesRoute` PASS (2.5 s), `TestHWCaptiveOverAdGuardRestoresBase` PASS, `TestHWAdGuardDownFailOpen` PASS. |
| Signed update (#81) | operator | Needs a newer signed build and a signed broken release; no signing keys were used here. |
| VPN on the guest zone (#89, PR #156) | pending #156 | #156 was not merged during the run. |

End state: main `dc403f6` installed, uplink STA on radio0 and AP on radio1, AdGuard default DNS on,
wg0 `disabled=1` (`travo_vpn` zone kept, as disable keeps it), one saved uplink and the restored VPN
profile, no guards. Against the pre-run backup, `wireless`, `uhttpd` and `travo-greenfield`
are byte-identical. `network` differs only in the option order of the wg0 peer, `firewall` by
`travo_vpn`/`travo_vpn_lan`, and `dhcp` by the AdGuard `server`/`noresolv`.
The exclusive lock `/tmp/agent-lock` lives on tmpfs: the step-8 reboot removed it, and it was not
re-taken for the rest of the run. No other writer showed up in `logread`.
