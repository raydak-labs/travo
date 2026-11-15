# wifi-scan: GL-AXT1800, OpenWrt 25.12.6 r33311-981c978eff

Captured on 2026-11-13 (router `date -u` in each `date.txt`) over Ethernet, after `firstboot` to clean OpenWrt plus travo-greenfield 0.0.0-hw1. Test setup written with plain `uci` (not greenfield): STA section `wwan` on radio0 (network `wwan`, dhcp), AP `default_radio1` on radio1 (lan). Only reads below. Redaction policy: `../PROVENANCE.md`.

## connected/

STA `phy0-sta0` associated, `wwan` up with kernel default route; AP `phy1-ap0` up.

- wireless-status.json: ubus call network.wireless status
- iwinfo-devices.json: ubus call iwinfo devices
- iwinfo-info-phy0-sta0.json / iwinfo-info-phy1-ap0.json: ubus call iwinfo info '{"device":"<ifname>"}'
- device-status-phy0-sta0.json / device-status-phy1-ap0.json: ubus call network.device status '{"name":"<ifname>"}'
- iwinfo-scan-phy0-sta0.json: ubus call iwinfo scan '{"device":"phy0-sta0"}'
- uci-show-wireless.txt: uci show wireless
- iw-dev.txt: iw dev
- ip-link-indices.txt: `ip -o link show` captured over Ethernet on 2026-11-15 for #115 on the same firmware with the STA and AP up. Each row was reduced to its index and displayed interface name; values and row order are unchanged.

## no-sta/

Same config with `wireless.wwan.disabled=1` (commit + `reload_config`): no STA interface exists; only AP `phy1-ap0` on radio1; radio0 has no interface.

- wireless-status.json, iwinfo-devices.json, uci-show-wireless.txt: as above
- iwinfo-scan-phy0.json: ubus call iwinfo scan '{"device":"phy0"}' (radio0 has no netdev: 5 GHz results only)
- iwinfo-scan-radio0.json: ubus call iwinfo scan '{"device":"radio0"}' (same 5 GHz results)
- iwinfo-scan-phy1.json: ubus call iwinfo scan '{"device":"phy1"}' (phy of the live AP: 2.4 GHz results only)
- iwinfo-scan-phy1-ap0.json: ubus call iwinfo scan '{"device":"phy1-ap0"}'
- iw-dev-before.txt / iw-dev-after.txt: iw dev before and after the four scans (identical)
- uci-changes.txt: uci changes after the scans (empty)

Observed during the no-sta scans (not stored as files): every call returned rc 0; phy0/radio0 scans took ~4.7 s, phy1/phy1-ap0 ~2 s; after each scan `network.device status phy1-ap0` read `up:true, carrier:true` and logread showed no hostapd/AP events. The scan of an interface-less radio (`phy0`, `radio0`) left no interface behind. A scan only covers the band of the radio it runs on.

## Redactions

- `"key"` values in wireless-status.json and `wireless.*.key=` lines: `REDACTED`.
- SSIDs: home uplink → `HomeUplink`, test AP → `TravelAP`, other visible networks → `Neighbor1`…`Neighbor6` (stable across files). BSSIDs kept.
