# wan/ (2026-11-19, OpenWrt 25.12.6, #240)

Captured on the GL-AXT1800 (Travo 0.1.0 after firstboot, uplink wwan on radio0, guest network on, no Ethernet cable on the wan port). Read-only (GET-only probes).

- uci-export-network.txt: `uci export network`. `wan` is `proto 'dhcp'` on device `wan`, `wan6` shares the device, lan holds `list ipaddr '192.168.1.1/24'`, guest `option ipaddr '192.168.2.1/24'`. Redacted per the directory policy: `wg0` `private_key` → `REDACTED`, peer `endpoint_host` → `203.0.113.8`.
- device-status-wan.json: `ubus call network.device status '{"name":"wan"}'`: link up, `"carrier": false` (no cable).
- ip-route-default.txt: `ip -4 route show default`: the kernel default route via the STA uplink, not wan.
- netifd-proto-ppp.sh.txt: `sed -n 66,82p; sed -n 118,124p; sed -n 148,170p /lib/netifd/proto/ppp.sh` (package `ppp-mod-pppoe` is installed): the `pppoe` proto reads `username`/`password` from the interface and names its kernel link `pppname`, default `<proto>-<config>` = `pppoe-wan`.

Not captured yet: a static or PPPoE wan on the device and the kernel route over a cabled wan. The hardware playbook (docs/testing.md, "WAN settings") captures them in the joint Ethernet session.
