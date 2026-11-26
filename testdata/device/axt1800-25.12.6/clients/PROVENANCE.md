# clients/ (2026-11-18, OpenWrt 25.12.6, #218, #219)

Captured from the GL-AXT1800 (Travo 0.1.0, uplink wwan on radio0, TravelAP and guest AP on radio1, no station associated at capture time). Caller on Ethernet.

Read-only:
- dhcp-leases.txt: `cat /tmp/dhcp.leases`. Hostname column (4th field) replaced with `REDACTED-HOST` (policy in ../PROVENANCE.md).
- ip-neigh.json: `ip -j -4 neigh show`. Shows a `FAILED` entry without `lladdr` (192.168.1.140, a leased phone that left) and uplink-side neighbours on `phy0-sta0`. Verbatim.
- uci-show-network-lan.txt: `uci show network.lan`. On 25.12 `ipaddr` holds CIDR (`192.168.1.1/24`) and there is no `netmask`. Verbatim.
- hostapd-ubus-list.txt: `ubus -v list hostapd.phy1-ap0` (signature of `del_client`). Verbatim.

Staged only, under the router write lock, then `uci revert firewall` and `uci revert dhcp` (nothing committed, `uci changes` empty before and after):
- `uci set` of `dhcp.travo_host_46bae68d7d6b` (host) and `firewall.travo_block_46bae68d7d6b` (rule), then: uci-export-dhcp-host-staged.txt and uci-export-firewall-block-staged.txt are those sections' excerpts of `uci export dhcp` / `uci export firewall`; uci-show-dhcp-host-staged.txt is `uci show dhcp.travo_host_46bae68d7d6b` (a one-element `mac` list prints like an option, device-facts §0). The host name `Watch` is the device's own lease name, kept because it is a generic product name. Every byte of the excerpts is verbatim.
- fw4-print-block.txt: `fw4 print | grep -n -B2 -A2 46:ba` with the rule staged: fw4 renders `src '*' dest '*' src_mac` as `ether saddr ... jump handle_reject` in the global `forward` chain before the zone dispatch (`forward_lan`, `forward_wan`).

Station payloads with a client associated come from `../with-ap-client/`.
