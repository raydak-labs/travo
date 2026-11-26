# 0015 No native .apk yet

Status: accepted (decides #185; revisits the "a real `.apk` can come later" note in [0007](0007-distribution-and-self-update.md))

Travo stays a signed release tarball plus `install.sh` with signed self-update. It does not ship as a native OpenWrt `.apk` in 0.2.

## Why not now

- **Two owners of the same files.** apk records every file it installs (`/lib/apk/packages/<name>.list`, checksums in its database). Self-update swaps `/usr/bin/travo-greenfield` and `/www/travo-greenfield/` in place and keeps `.old` for rollback (ADR 0007). After one self-update apk's database is wrong: `apk audit` reports modified files, `apk upgrade` from a feed would overwrite a newer binary with an older one, and `apk del` would leave the update state behind. Making the two coexist means either self-update goes through apk (next point) or apk-installed copies disable self-update; neither exists yet.
- **apk cannot install our package with a trust chain.** On the AXT1800 (OpenWrt 25.12.6, apk-tools 3.0.5) even OpenWrt's own feed package is `UNTRUSTED signature` when added as a file; only the signed feed index (`/etc/apk/keys/openwrt-25.12.pem`) makes packages trusted. A Travo `.apk` would need either `--allow-untrusted` (no better than the tarball, and worse: no signature check at all) or a Travo key in `/etc/apk/keys` plus a Travo feed index. Adding a key there trusts it for every package the router installs; that is a much larger grant than the minisign key that verifies only Travo releases.
- **Build cost.** A correct package needs the OpenWrt SDK per target (or a hand-built ADB package with apk-tools `mkpkg`), conffile and `uci-defaults` handling that duplicates `install.sh`, and an opkg `.ipk` variant for the best-effort releases (ADR 0012). The tarball already covers fresh installs, the LuCI port move and its rollback, and sysupgrade keep-lists.
- **No user need yet.** The install path is one command; updates are in-app. The things a package would add (dependency resolution, listing in LuCI's package manager) do not apply to a static binary.

## Revisit when

All of these hold:

1. Self-update is redesigned so the package manager is the only writer: the daemon downloads a signed Travo `.apk` and runs `apk add` itself (guarded, invariants §13), keeping the health check and automatic rollback of ADR 0007 by re-adding the previous `.apk`.
2. A Travo feed exists (signed `packages.adb` per target, hosted with the GitHub release), and the operator accepts installing its key under `/etc/apk/keys` at first install, with removal on uninstall.
3. The OpenWrt SDK build runs in CI for every promised target and the container tests (25.12) cover install, upgrade, sysupgrade and removal of the package.

Until then a feed or third-party `.apk` of Travo is unsupported; `install.sh` stays the only installer.
