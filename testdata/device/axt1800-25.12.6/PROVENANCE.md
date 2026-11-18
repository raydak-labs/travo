Captured read-only from GL-AXT1800 (OpenWrt 25.12.6 r33311-981c978eff, qualcommax/ipq60xx) on Mon Nov  9 16:12:15 UTC 2026 via `date -u` on the router. Layout at capture: radio0 = uplink STA phy0-sta0 (SSID HomeUplink, wwan up), radio1 = AP phy1-ap0 (lan), radio0 AP disabled. Caller on Ethernet (LAN).

interface-dump.json: ubus call network.interface dump
interface-wwan-status.json: ubus call network.interface.wwan status
interface-lan-status.json: ubus call network.interface.lan status
interface-wan-status.json: ubus call network.interface.wan status
device-status-all.json: ubus call network.device status
device-status-phy1-ap0.json: ubus call network.device status '{"name":"phy1-ap0"}'
device-status-phy0-sta0.json: ubus call network.device status '{"name":"phy0-sta0"}'
device-status-br-lan.json: ubus call network.device status '{"name":"br-lan"}'
wireless-status.json: ubus call network.wireless status
hostapd-phy1-ap0-get_clients.json: ubus call hostapd.phy1-ap0 get_clients
hostapd-list.txt: ubus -v list 'hostapd.*'
wpa-list.txt: ubus -v list 'wpa_supplicant.*'
iwinfo-devices.json: ubus call iwinfo devices
iwinfo-info-phy1-ap0.json: ubus call iwinfo info '{"device":"phy1-ap0"}'
iwinfo-info-phy0-sta0.json: ubus call iwinfo info '{"device":"phy0-sta0"}'
iwinfo-assoclist-phy1-ap0.json: ubus call iwinfo assoclist '{"device":"phy1-ap0"}'
ip-neigh.txt: ip -4 neigh show
ip-neigh.json: ip -j -4 neigh show
ip-route-main.txt: ip -4 route show table main
ip-route-default.txt: ip -4 route show default
ip-addr.json: ip -j -4 addr show
uci-show-wireless.txt: uci show wireless
uci-show-network.txt: uci show network
uci-show-dhcp.txt: uci show dhcp
uci-show-firewall.txt: uci show firewall
uci-changes.txt: uci changes
system-board.json: ubus call system board
system-info.json: ubus call system info
service-list-dnsmasq.json: ubus call service list '{"name":"dnsmasq"}'
dhcp-leases.txt: cat /tmp/dhcp.leases
resolv-auto.txt: cat /tmp/resolv.conf.d/resolv.conf.auto
buttons.txt: for f in /sys/firmware/devicetree/base/keys/*/label; do echo "$f: $(cat $f)"; done
ubus-list.txt: ubus list
rpcd-acl-ls.txt: ls /usr/share/rpcd/acl.d/
reload-config-which.txt: which reload_config wifi setsid ip jsonfilter

REDACTIONS (values replaced with 'REDACTED', lines kept):
- uci-show-wireless.txt: all wireless.*.key= lines (5 values, WiFi passphrases)
- uci-show-network.txt: network.wg0.private_key= (WireGuard private key)
Not redacted: network.wg0_peer0.public_key (public key, not secret). "psk"/"psk2" matches in JSON/encryption fields are type names, not secrets.
- ubus-uci-get-dhcp.json: ubus call uci get '{"config":"dhcp"}' (after restoring /etc/config/dhcp to the backup: `list server '127.0.0.1#5353'`)
- uci-show-dhcp-list-single.txt: uci show dhcp (same state). Shows `dhcp.cfg01411c.server='127.0.0.1#5353'`: a one-element LIST prints exactly like an option. Only `ubus call uci get` (JSON array) or `uci export` (`list` keyword) tell them apart.

## Redaction policy (applies to this directory and subdirectories)
Secrets, replaced with `REDACTED`: WiFi passphrases (`uci` `key`, and `"key"` inside `network.wireless status` JSON: wireless-status.json in both dirs), WireGuard private key.
Location-identifying values, replaced with stable placeholders in every file (raw captures and spike logs alike): SSIDs (home uplink → `HomeUplink`, travel AP → `TravelAP`, other saved networks → `HomeNet`, `HotelA`, `HotelB`), DNS search domain → `home.lan`, WireGuard endpoint → `203.0.113.8` (RFC 5737). DHCP lease hostnames → `REDACTED-HOST`.
Kept as captured: MAC addresses and private RFC1918 addresses. They are needed for classification tests and do not locate the user. Public keys are kept.
Every other byte is verbatim. Placeholders were substituted after capture with exact string replacement.

## guest/ (2026-11-17, 0.1.0 release playbook, #75)
Captured while a `POST /api/v1/wifi/guest` transaction (radio1, SSID `TGPlaybookGuest`, a throwaway test key) was Pending, before it was rolled back:
- uci-network-guest.txt: `uci show network | grep -E "^network\.(guest|travo_guest|br_guest|guest_dev)[.=]"`
- uci-firewall-guest.txt: `uci show firewall | grep -iE "guest"`
- uci-dhcp-guest.txt: `uci show dhcp | grep -E "^dhcp\.guest[.=]"`
- uci-wireless-guest.txt: `uci show wireless | grep -iE "guest" | grep -v "\.key="` (the key line is omitted, not redacted)
- interface-guest-status.json: `ubus call network.interface.guest status`
No placeholders were needed: the SSID is a test name and the only address is the guest subnet `192.168.2.1/24`. Every byte is verbatim.

## sysfs-net/ (2026-11-18, #175)
- links.tsv: `cd /sys/class/net && for n in *; do printf "%s\t%s\t%s\t%s\t%s\n" "$n" "$(readlink $n/device)" "$(readlink $n/device/driver)" "$(readlink $n/device/subsystem)" "$(grep ^DEVTYPE= $n/uevent | cut -d= -f2)"; done`. Columns: name, device link, driver link, subsystem link, uevent `DEVTYPE`. Taken with wg0 disabled (absent) and no USB device attached. Verbatim; nothing needed redaction.

## usbtether/ (2026-11-18, 25.12.6 package feed and UCI, #175)
- apk-packages.txt: after `apk update`, `apk search -v kmod-usb-net-rndis kmod-usb-net-cdc-ncm kmod-usb-net-cdc-ether kmod-usb-net-ipheth usbmuxd`, then `apk info -R` for each tether package and `apk info -s` for them and their dependencies. None was installed. Verbatim.
- uci-export-firewall.txt, uci-export-network.txt: `uci export firewall`, `uci export network` (router state after the 0.1.0 install: wwan uplink, guest AP, VPN profile stored, tunnel off). The wan zone is the anonymous `@zone[1]` with `list network`. Redacted: `wg0` `private_key` → `REDACTED`, peer `endpoint_host` → `203.0.113.8`.
- usbmuxd-init.txt, usbmuxd-hotplug.txt: `/etc/init.d/usbmuxd` and `/etc/hotplug.d/usb/40-usbmuxd` from `apk fetch usbmuxd` + `apk extract` into `/tmp` (package not installed). Verbatim.
No phone was attached: the tether link payloads (sysfs, `network.device status`, `network.interface.usbtether status`) are a hardware playbook capture step (docs/testing.md).

## uci-export-system.txt (2026-11-18, 0.2.0 system settings, #190)
- uci-export-system.txt: `uci export system` on the firstboot 25.12.6 image (hostname, zonename/timezone POSIX pair, sysntpd `timeserver ntp` with its server list). No values needed redaction; every byte is verbatim.

## mwan3/ (2026-11-18, 25.12.6 package feed, #208)
Packages fetched and extracted into `/tmp` on the router without installing them: `apk fetch mwan3 luci-app-mwan3`, `apk extract --allow-untrusted --destination <dir> <pkg>.apk`.
- stock-config-mwan3.txt: `/etc/config/mwan3` of `mwan3-2.12.0-r3` (the conffile a fresh install writes).
- mwan3-conffiles-init.txt: `lib/apk/packages/mwan3.conffiles`, a `---` line, then `/etc/init.d/mwan3`.
- luci-app-mwan3-acl.json: `/usr/share/rpcd/acl.d/luci-app-mwan3.json` of `luci-app-mwan3-26.317.52279~1c75ea4`.
Verbatim; nothing needed redaction.

## logs/ (2026-11-18, OpenWrt 25.12.6, #192)
- logs/logread.txt: `logread` (GET-only) after the 0.1.0 install, firstboot plus a day of uptime; then, in file order, at most 3 lines per (facility.level, program) were kept so every line shape appears once without the repetitive bulk (AdGuard probe errors, odhcpd RA warnings). Lines carrying a Tailscale login URL were dropped. Placeholders: the uplink SSID → `HomeUplink`, its BSSID → `REDACTED-BSSID`, the Tailscale `LogID` → `REDACTED`, a DHCPACK client hostname → `REDACTED-HOST`. Every kept line is otherwise verbatim, including the empty program in `user.notice : ...`, programs without a pid (`kernel:`, `netifd:`, `hostapd:`) and the kernel uptime brackets.

## system/ (2026-11-18, OpenWrt 25.12.6, #196)
- system/proc-mounts.txt: `cat /proc/mounts` (GET-only) on the running router: ubifs `/overlay` (`/dev/ubi0_2`, rw) under the overlayfs root. Verbatim; nothing needed redaction. Alert tests derive the degraded shapes (tmpfs overlay, read-only overlay) by replacing only that line's file system type or `rw` flag.

## clients/ (2026-11-18, #218, #219)
Leases, neighbours, `network.lan` and hostapd method list, plus staged-only (reverted) block rule and host section captures. See `clients/PROVENANCE.md`.

## network/ (2026-11-18, #237)
Interface dump with the guest interface and uptime, and `uci export firewall` with anonymous and named zones. See `network/PROVENANCE.md`.
