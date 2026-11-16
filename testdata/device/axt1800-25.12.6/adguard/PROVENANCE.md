# PROVENANCE: AdGuard Home capture (GL-AXT1800)

Captured read-only from GL-AXT1800 (OpenWrt 25.12.6) on Mon Nov  9 16:14:41 UTC 2026, AdGuard Home installed from the OpenWrt package by the old Travo 0.10.2.

Host: ssh root@192.168.1.1. Only read commands and HTTP GET requests (curl/wget, no -X/POST) were issued. No install/remove/restart/uci set/commit.
Stdout saved verbatim unless noted under Redactions. Failed commands have stderr in <file>.err (none remain).

## Commands

apk-info.txt: apk info -a adguardhome   (remote rc=1 observed on re-run; stdout captured intact)
apk-contents.txt: apk info -L adguardhome
apk-version.txt: apk list --installed adguardhome
version.txt: /usr/bin/AdGuardHome --version
initd.txt: cat /etc/init.d/adguardhome
uci-adguardhome.txt: uci show adguardhome
capabilities.json: cat /etc/capabilities/adguardhome.json
yaml-ls.txt: ls -la /etc/adguardhome/ /var/lib/adguardhome/
adguardhome.yaml: cat /etc/adguardhome/adguardhome.yaml | sed -E 's/^([[:space:]]*-?[[:space:]]*password:).*/\1 REDACTED/'  (redaction applied locally after capture)
keep-list.txt: cat /lib/upgrade/keep.d/* ; echo ---; sysupgrade -l 2>/dev/null | grep -iE "adguard|travo"
conffiles.txt: cat /lib/apk/packages/adguardhome.conffiles; echo ---; ls /lib/apk/packages/ | grep -i adguard
apk-info-rc.txt: apk info -a adguardhome >/dev/null; echo "rc=$?"
listen.txt: netstat -lntup | grep -E "AdGuard|:3000|:5353"
dnsmasq-servers.txt: uci show dhcp | grep -E "server|noresolv|port|rebind"
http-status.txt: curl -sS http://127.0.0.1:3000/control/status; echo
http-status-code.txt: curl -s -o /dev/null -w 'HTTP %{http_code}\n' http://127.0.0.1:3000/control/status
http-install-get-addresses.txt: curl -sS -i http://127.0.0.1:3000/control/install/get_addresses | head -40
http-root-headers.txt: curl -sS -D - -o /dev/null http://127.0.0.1:3000/ | head -15
dns-probe.txt: nslookup example.com 127.0.0.1 2>&1 | head; echo ---; nslookup -port=5353 example.com 127.0.0.1 2>&1 | head
adguard-log.txt: logread -e AdGuardHome | tail -40

## Redactions

- adguardhome.yaml: every line matching `password:` replaced with `password: REDACTED`. Count in the captured file: 0. The file has `users: []`, so no password lines or user names exist to redact.
- No other values redacted.

## Notes

- Failed first-attempt captures (BusyBox wget has no -S flag; the first run used wget for HTTP headers) were overwritten with curl-based captures. No .err files remain.
- conffiles.txt was first captured with a broad sed range that over-printed; overwritten with the narrower command above.

## Addendum 2026-11-16 (issue #132)

Captured read-only on the same router (OpenWrt 25.12.6 r33311-981c978eff) on Mon Nov 16 20:00 UTC 2026, AGH 0.107.79-r1 still installed and running. No redactions (the output holds no secrets).

- service-list-adguardhome.json: `ubus call service list '{"name":"adguardhome"}'`
- service-list-absent.json: `ubus call service list '{"name":"nonexistent-svc"}'` (procd answers an empty object for an unknown service)
- df-overlay.txt: `df -k /overlay`
