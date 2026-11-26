# wwan-down: GL-AXT1800, OpenWrt 25.12.6 r33311-981c978eff

Captured read-only on Wed Nov 11 09:22:06 UTC 2026 (router `date -u`) over Ethernet. State: STA `wifinet3` (radio0) configured but not associated (wwan down, no kernel default route), AP `default_radio1` up on radio1, `wg0` configured with `disabled=1` (not up), old Travo 0.10.2 running. Use these for "uplink down" read models, scan results, frequency lists, and the dashboard/system data sources.

## Commands

- iwinfo-scan-phy0-sta0.json: ubus call iwinfo scan "{\"device\":\"phy0-sta0\"}"
- iwinfo-freqlist-phy0-sta0.json: ubus call iwinfo freqlist "{\"device\":\"phy0-sta0\"}"
- iwinfo-freqlist-phy1-ap0.json: ubus call iwinfo freqlist "{\"device\":\"phy1-ap0\"}"
- iwinfo-info-phy0-sta0-disconnected.json: ubus call iwinfo info "{\"device\":\"phy0-sta0\"}"
- phy-addresses.txt: for p in /sys/class/ieee80211/phy*; do echo "$p: $(cat $p/addresses | tr "\n" " ")"; done
- ip-link-phy0-sta0.json: ip -j link show phy0-sta0
- interface-wwan-status-down.json: ubus call network.interface.wwan status
- device-status-phy0-sta0-disconnected.json: ubus call network.device status "{\"name\":\"phy0-sta0\"}"
- ip-route-default-none.txt: ip -4 route show default
- ubus-v-network-wireless.txt: ubus -v list network.wireless
- ubus-v-iwinfo.txt: ubus -v list iwinfo
- ubus-v-system.txt: ubus -v list system
- proc-loadavg.txt: cat /proc/loadavg
- proc-meminfo.txt: head -8 /proc/meminfo
- nproc.txt: grep -c ^processor /proc/cpuinfo
- thermal.txt: for z in /sys/class/thermal/thermal_zone*; do echo "$z $(cat $z/type) $(cat $z/temp)"; done
- df.txt: df -k /overlay /
- net-counters.txt: for i in 1 2; do for n in /sys/class/net/*; do echo "$(date +%s) $(basename $n) $(cat $n/statistics/rx_bytes) $(cat $n/statistics/tx_bytes)"; done; sleep 2; done
- probe-no-route.txt: wget -S -O /dev/null -T 3 http://connectivitycheck.gstatic.com/generate_204
- wg-show.txt: wg show 2>&1; wg show all dump 2>&1
- wireguard-packages.txt: apk list --installed 2>/dev/null | grep -i -E "wireguard|wg"
- uci-export-network.txt: uci export network
- uci-export-firewall.txt: uci export firewall
- uci-export-wireless.txt: uci export wireless
- sysupgrade-help.txt: sysupgrade -h
- probe-no-route.txt: wget -q -O /dev/null -T 3 http://connectivitycheck.gstatic.com/generate_204 (no default route; wget has no -S)

Empty files are genuine empty output (`ip-route-default-none.txt`: no default route; `wg-show.txt`: no WireGuard interface up). `probe-no-route.txt` holds the output and exit code of the probe with no route; BusyBox `wget` has no `-S`.

## Redactions

Parent directory policy (../PROVENANCE.md). Applied here: WiFi keys and the WireGuard private key → `REDACTED`; WireGuard endpoint → `203.0.113.8`; own SSIDs → the parent's placeholders by section name; neighbour SSIDs in the scan → `Neighbour2`…`Neighbour5` (hidden networks stay without `ssid`); every BSSID in the scan → `02:00:00:00:00:NN` (locally administered, one per BSSID, consistent across files). Every other byte is verbatim.
