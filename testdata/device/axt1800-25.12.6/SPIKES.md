# Spike results index: GL-AXT1800, OpenWrt 25.12.6

One line per spike or issue run on hardware: what was tested, result, where the fact now lives. Facts are in [docs/device-facts.md](../../../docs/device-facts.md) (DF), rules in [docs/invariants.md](../../../docs/invariants.md), run history in git. Layout used: radio0 = uplink STA `phy0-sta0` (wwan), radio1 = AP `phy1-ap0` (lan), caller on Ethernet. Release gate: the hardware playbook in [docs/testing.md](../../../docs/testing.md).

Flagged: on the test device the old Travo left the AdGuard admin API open on `0.0.0.0:3000` with `users: []`, so GET and POST `/control/*` answered 200 to any LAN client without auth. [ADR 0006](../../../docs/adr/0006-adguard-package-and-topology.md) binds AGH to loopback behind a Travo-gated proxy; do not reintroduce.

## Initial spikes S1 to S5 (ADR 0006 cites these)

| Id | Tested | Result | Fact lives in |
|---|---|---|---|
| S1 | AdGuard package `adguardhome` 0.107.79-r1 on the device | Runs in ujail, conffiles kept by sysupgrade, config and work dir paths recorded (`adguard/`) | DF §11 |
| S2 | Plain Go `httputil.ReverseProxy` in front of AGH with `users: []` | UI, JS and `/control/*` GET and POST work; AGH#7985 did not reproduce | DF §11, ADR 0006 |
| S3 | Caller payloads: `network.interface dump`, hostapd `get_clients`, `ip -j -4 neigh` | Pass; fixtures in this directory and `with-ap-client/` | DF §5 |
| S4 | Watchdog on hardware (`spikes/s4-*.log`): AP rename, restore, daemon death, reboot in window | Pass; restore plus STA re-association ~25 s end to end, so the UI keeps "restoring" that long | DF §2, §4 |
| S5 | DNS layer over AdGuard base (`spikes/s5-*.log`) | One real bug: `uci show` prints a one-element list like an option, restored as `option server` and dnsmasq dropped it. Fixed by reading `uci export`. Fail-open restored DNS in 0.13 s | DF §0, §11 |

## Issue results

| Issue | Tested | Result | Fact lives in |
|---|---|---|---|
| #108 | WireGuard rollback: explicit, probe failure, confirm timeout, killed watchdog, disabled baseline | Pass, 3.0 to 7.4 s; configs byte-identical afterwards | DF §1, invariants §1 |
| #87, #113 | WireGuard toggle and default-route rollback | Pass after #113: an unreachable endpoint rolled back in 80.5 s with the underlying default restored | DF §1 |
| #71, #115 | WiFi Join with fresh STA proof, API and browser | Pass: wrong key rejected after ~40 s without reaching Pending; valid key had carrier at ~17 s. WPA2 only | DF §2 |
| #89 | VPN kill switch firewall | Pass: wg0 needs a zone plus lan forwarding; REJECT rule sits before `accept_to_wan` | DF §1 |
| #90 | WireGuard split tunnel UCI shape | Pass: netifd accepts CIDR `target`; leaving full mode drops the uplink default | DF §1 |
| #87 follow-up | VPN zone on tunnel enable | Pass: zone-only re-add restores LAN egress without a wg0 restart | DF §1 |
| #130, #131 | Captive DNS bypass and wwan lease renew (`internal/captive/hardware_test.go`) | Pass; real portal still needs the simulator (testing.md) | DF §7 |
| #73 | STA recovery without `wifi`: `ifup wwan` vs self-recovery | `ifup wwan` heals sticky `DISCONNECT` in 6 to 8 s, AP unaffected | DF §2, ADR 0014 |
| #132, #133, #134 | AdGuard loopback migration, default DNS and fail-open, gated proxy | Pass; backoff after a failed migration is #164 | DF §11, ADR 0006 |
| #157, #158, #159 | DNS restore order, restore settling before the result, reconnect tick | Fixed and verified | DF §0, §4, ADR 0014 |
| #168 | 2.4 GHz clients could not join any AP after firstboot | Open: joins started working after a regulatory change on radio1 re-initialised the radio | needs-hardware, open |
| #74, #75, #99 | AP settings, guest AP, join AP reconcile, with a phone | Pass; the 5 GHz AP next to the STA is refused as `wifi_radio_in_use` | DF §3 |
| #73 | Reconnect cap with a phone hotspot | Pass: attempts 1 to 5 every 2 min, then `exhausted`, no `wifi`, APs stayed up | DF §2, ADR 0014 |
| #76 | Uplink MAC policy | Pass: factory MAC is universally administered; random and custom LAA MACs confirmed and rolled back | SPIKES.md (this row) |
| #89, #156 | VPN on the guest zone | Pass in config, nft and with a guest client; PR #156 is merged | invariants §11 |
| #81 | Signed update | Operator, needs a signed build; see testing.md | ADR 0007 |
| #65 | Reboot through the API | Pass: new `boot_id`, no stale guard | testing.md |
| #169 | AdGuard preinstalled without config after `firstboot` | Fixed: seeded loopback config in 2 s | DF §11 |
| #277 | Failover apply probe and live failover, wan to usbtether | Pass after fix: stale mangle base chains deleted on Enable; detection 16 to 17 s, failback 31 to 33 s; same-subnet uplinks refused (`failover_same_subnet`) | DF §9, ADR 0018 |
| #285 | Quiet 24 h memory soak with slabinfo and vmstat | Open | DF §11 |

Open `needs-hardware` issues: #199, #168, #131 (renew on a real portal), #97, #25 (portal detection and assist).
