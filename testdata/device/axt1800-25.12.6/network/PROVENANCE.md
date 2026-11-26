# network/ (2026-11-18, OpenWrt 25.12.6, #237)

Captured read-only from the GL-AXT1800 at `Wed Nov 18 23:03:07 UTC 2026` (`date -u` on the router; Travo 0.1.0, uplink wwan on radio0, TGTestAP and TGGuest on radio1, guest zone and the `travo_vpn` zone present, tunnel off). Caller on Ethernet. Nothing was redacted: every address is private (RFC 1918 or the router's ULA), no SSIDs, keys or endpoints appear.

- interface-dump.json: `ubus call network.interface dump`. Down interfaces (wan, wan6) carry `proto` but no `uptime`, `l3_device` or address arrays; lan's only IPv6 is the `local-address` of its `ipv6-prefix-assignment` (empty `ipv6-address`). `wg0` is absent: the tunnel interface does not exist while it is off. Verbatim.
- uci-export-firewall.txt: `uci export firewall`. Zones are both anonymous (`lan`, `wan`) and named sections (`guest`, `travo_vpn`); `network` is a list in some zones and an option in `guest`. Verbatim.
