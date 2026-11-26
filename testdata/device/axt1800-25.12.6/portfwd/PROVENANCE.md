# portfwd/ (2026-11-19, OpenWrt 25.12.6, #238)

Captured read-only from the GL-AXT1800 (Travo 0.1.0, uplink wwan on radio0, TGTestAP and TGGuest on radio1). Caller on Ethernet. The live firewall has no redirect sections (`uci show firewall | grep -c redirect` is 0); its export is `../network/uci-export-firewall.txt`. Nothing needed redaction.

- rom-firewall-redirect-examples.txt: `sed -n 152,167p /rom/etc/config/firewall`, the stock config's two commented `config redirect` examples (wan → lan port 80, wan 22001 → lan port 22 without `dest_ip`). Tests uncomment them and re-quote the tab-aligned values the way `uci export` prints them; keys and values are unchanged. Verbatim.
- fw4-parse-redirect.uc.txt: `sed -n 2612,2662p /usr/share/ucode/fw4.uc`, fw4's `parse_redirect` option table: `enabled` defaults to 1, `proto` to `tcpudp` (parsed as a list and flattened, so `option proto 'tcp udp'` and `list proto` both work), `target` to `dnat`, `src`/`dest` are zone names. Verbatim.

Not captured: a staged `travo_fwd_<n>` section and its `fw4 print` rendering (router writes were not available to this session); the hardware playbook step "Port forwards (#238)" in docs/testing.md captures them.
