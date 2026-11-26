# ddns/ (2026-11-19, OpenWrt 25.12.6, #245)

Captured on the GL-AXT1800 (Travo 0.1.0 after firstboot) under the agent lock: `apk add ddns-scripts`, read-only captures, then `apk del ddns-scripts ddns-scripts-services`. Afterwards `uci export`, `/etc/apk/world` and `ls /etc/config` equalled the snapshots taken before, `/etc/config/ddns`, `/usr/lib/ddns` and `/usr/share/ddns` were gone. Verbatim; the package's sample credentials are placeholders, nothing needed redaction.

- apk-add.txt: `apk add ddns-scripts` (pulls `ddns-scripts-services`).
- apk-info-size.txt: `apk info -s ddns-scripts ddns-scripts-services` (71 KiB + 17 KiB installed).
- uci-export-ddns.txt: `uci export ddns` right after install: `global` and two sample `service` sections (`myddns_ipv4`, `myddns_ipv6`) without `enabled`, so ddns-scripts does not run them.
- initd-ddns.txt: `cat /etc/init.d/ddns`: one procd instance per enabled `service` section, `procd_add_reload_trigger ddns`.
- hotplug-95-ddns.txt: `cat /etc/hotplug.d/iface/95-ddns`: a section starts on `ifup` of its `interface` (default `wan`) and stops on its `ifdown`.
- services-default-ls.txt: `ls /usr/share/ddns/default/; ls /usr/share/ddns/default | wc -l` (64 providers).
- service-duckdns.org.json: `cat /usr/share/ddns/default/duckdns.org.json`.

Read from the package's scripts (not stored): `IPFILE="$ddns_rundir/$SECTION_ID.ip"` with `ddns_rundir` default `/var/run/ddns`; `ip_source web` without `ip_url` uses `http://checkip.dyndns.com`; `cacert` may be a file or a directory; `ls /etc/ssl/certs` lists `ca-certificates.crt` (ca-bundle). Not captured: a running section (`/var/run/ddns/travo.ip`), which needs a real provider account; see the hardware playbook.
