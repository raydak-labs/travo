# sqm/ (2026-11-27, container capture from OpenWrt 25.12.6 feed, #302)

**Container fixture** (see `docs/testing.md`: proves format, defaults, and quoting only, not live kernel qdisc behaviour; hardware verification is in the release playbook).

Captured on 2026-11-27 from `openwrt/rootfs:aarch64_generic-25.12.6@sha256:22f13387da208c0fb4150c41f3ec12e34cc5994a0717a49e50aa9f9ae6cb5e7b` by fetching and extracting `sqm-scripts-1.7.2-r1.apk` (the same package version installed on the GL-AXT1800 25.12.6 image per `catalog/sqm-builtin.txt`):

```sh
apk update
apk fetch -o /tmp/sqm-ext sqm-scripts
apk extract --allow-untrusted --destination /tmp/sqm-ext/root /tmp/sqm-ext/sqm-scripts-1.7.2-r1.apk
```

- `stock-config-sqm.txt`: `/etc/config/sqm` shipped by `sqm-scripts-1.7.2-r1`.
- `initd-sqm.txt`: `/etc/init.d/sqm` shipped by `sqm-scripts-1.7.2-r1` (`USE_PROCD=1`, `procd_add_reload_trigger "sqm"`).
- `uci-export-sqm.txt`: `uci -c /tmp/sqm-ext/root/etc/config export sqm` run inside the container against that stock config.

The package ships 5 scripts under `/usr/lib/sqm/*.qos`: `layer_cake.qos`, `piece_of_cake.qos`, `simple.qos`, `simplest.qos`, `simplest_tbf.qos`.

Verbatim; nothing needed redaction. Verified on 2026-11-27 against the live GL-AXT1800 (OpenWrt 25.12.6, `sqm-scripts-1.7.2-r1`): `uci export sqm` and `/etc/init.d/sqm` match identically (only a trailing blank line in live `uci export`).
