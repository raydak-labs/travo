# ipv6/ (2026-11-19, OpenWrt 25.12.6, #245)

Captured read-only on the GL-AXT1800 (Travo 0.1.0 after firstboot, uplink wwan). Verbatim; nothing needed redaction (the ULA prefix is random and local).

- uci-show-ipv6.txt: `uci show network | grep -E "wan6|ip6|ula|wwan|lan\.|ipv6|delegate"; uci show dhcp | grep -E "ra|dhcpv6|ndp|odhcpd|wan6|lan\."`. The image ships IPv6 on: `wan6` (DHCPv6 on the cable port), `ip6assign 60` on lan, odhcpd `ra`/`dhcpv6 server` on dhcp.lan. The WiFi uplink `wwan` has no DHCPv6 client (`ubus list network.interface.*` lists no `wwan6` or `wwan_6`, and `ip -6 route` shows only the ULA and link-local routes).

The unit tests use the full exports `wan/uci-export-network.txt` and `dhcp/uci-export-dhcp.txt`, which carry the same options.
