Captured read-only from GL-AXT1800 (OpenWrt 25.12.6) on Mon Nov  9 16:15:42 UTC 2026. Same layout as parent dir, plus one WiFi client associated to phy1-ap0 (AP "TravelAP"). Caller (capture host) still on Ethernet.

hostapd-phy1-ap0-get_clients.json: ubus call hostapd.phy1-ap0 get_clients
ip-neigh.json: ip -j -4 neigh show
ip-neigh.txt: ip -4 neigh show
iwinfo-assoclist-phy1-ap0.json: ubus call iwinfo assoclist '{"device":"phy1-ap0"}'
wireless-status.json: ubus call network.wireless status
dhcp-leases.txt: cat /tmp/dhcp.leases
Redacted: dhcp-leases.txt hostname column (4th field) replaced with REDACTED-HOST.
