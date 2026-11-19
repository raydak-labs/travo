Container captures, not hardware (format only).
- `apk-query-installed.json`: `apk query --installed --fields name,version --format json "*"` in `openwrt/rootfs:aarch64_generic-25.12.6` (apk-tools 3.0.5), 2026-11-09.
- `opkg-list-installed.txt`: `opkg list-installed` in `openwrt/rootfs:x86-64-24.10.8` (opkg 38eccbb, 2024-10-16), 2026-11-09. Image is amd64 because no arm64 24.10 manifest exists for the host.
- `opkg-list-upgradable.txt`: `opkg update` then `opkg list-upgradable` in `openwrt/rootfs:x86-64-24.10.8`, 2026-11-18 (#193). Verbatim.
- `apk-list-upgradable.txt`: copy of `testdata/device/axt1800-25.12.6/updates/apk-list-upgradable.txt` (hardware, GET-only, #193).
