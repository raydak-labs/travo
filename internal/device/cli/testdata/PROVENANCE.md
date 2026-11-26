Captured from `openwrt/rootfs:aarch64_generic-25.12.6` (OpenWrt 25.12.6 r33311-981c978eff) with the `uci` CLI on 2026-11-09.
Container, not hardware: proves format and quoting, not wireless/netifd behaviour.
`uci-show-dhcp.txt` has two list entries on `server` and a value containing an apostrophe, added with `uci add_list` / `uci set` before `uci show`.
`uci-show-dhcp-scalar-server.txt`: same image, after `uci set dhcp.@dnsmasq[0].server="9.9.9.9"; uci commit dhcp` (scalar `option server`, as some operators configure it).
`uci-export-dhcp.txt`: same image, `uci export dhcp` after `uci add_list ...server=1.1.1.1`, `add_list ...server=/lan/127.0.0.1#53`, `set ...note="it's quoted"`, `set ...dq='a "b" $x'`, `set dhcp.extra=host; set dhcp.extra.name=x` (staged, not committed: export includes staged changes).
`uci-export-dhcp-scalar-server.txt`: same image, `uci export dhcp` after `uci set dhcp.@dnsmasq[0].server="9.9.9.9"; uci commit dhcp`.
Reads use `uci export`; `uci show` fixtures stay only to document why (a one-element list shows like an option).
