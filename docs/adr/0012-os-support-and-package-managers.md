# 0012 OpenWrt support level

Status: accepted

Promised: OpenWrt 25.12.x (apk) on GL-AXT1800 and GL-MT3000. Both apk and opkg adapters are built behind the PackageManager port and container-tested (25.12, 24.10); opkg/older releases are best-effort. The runtime detects the package manager and fails with a clear message rather than guessing. Travo's own install does not use a package manager.

Radios, buttons, LEDs and the package manager are detected at runtime, so nothing is hardcoded to ath11k or a radio name. Other aarch64 and x86_64 OpenWrt builds are best-effort and container-tested only.
