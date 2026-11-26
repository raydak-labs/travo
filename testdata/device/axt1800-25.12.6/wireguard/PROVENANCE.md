# wireguard: GL-AXT1800, OpenWrt 25.12.6 r33311-981c978eff

Captured on 2026-11-13 (router `date -u` in each `date.txt`) over Ethernet; clean firstboot + travo-greenfield 0.0.0-hw1, uplink STA `wwan` up. Packages: kmod-wireguard, wireguard-tools, luci-proto-wireguard (preinstalled). `wg0` written with plain `uci` from a commercial provider profile: one peer, `allowed_ips=0.0.0.0/0`, `route_allowed_ips=1`, keepalive 25.

States:
- disabled/: `network.wg0.disabled=1`.
- no-handshake/: `disabled=0`, peer endpoint `192.0.2.1:51820` (TEST-NET-1, unreachable), 30 s after `reload_config`.
- connected/: endpoint set to the real server, then `ifup wg0` (a `reload_config` alone kept the old endpoint), 15 s later; `wget http://ifconfig.me/ip` returned the VPN egress address.

Per state (each file exists in `disabled/`, `no-handshake/` and `connected/`, next to `date.txt`):
- interface-wg0-status.json: ubus call network.interface.wg0 status
- interface-dump.json: ubus call network.interface dump
- device-status-wg0.json: ubus call network.device status '{"name":"wg0"}'
- ip-link-wg0.json: ip -j link show wg0
- wg-show.txt: wg show; echo ---; wg show all dump
- wg-latest-handshakes.txt: wg show wg0 latest-handshakes
- ip-route-main.txt: ip -4 route show table main
- uci-show-network-wg0.txt: uci show network | grep '^network\.wg0'

Redactions (policy: `../PROVENANCE.md`): private key → `REDACTED` (uci `private_key` and column 2 of the `wg show all dump` interface line). Real server endpoint → `203.0.113.8`. Public keys, tunnel addresses (100.64.0.0/10) and provider DNS (198.18.0.0/15) kept.
