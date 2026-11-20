# 0007 Release tarball, install script, signed self-update

Status: accepted

Not a native package: a self-updating binary and apk/opkg would both own the same files and desync the package database. A tarball holds binary, UI, init script, `uci-defaults` and a `/lib/upgrade/keep.d/` entry (incl. `S99travo-greenfield` boot link). The installer targets a fresh OpenWrt installation and verifies the service answers before moving LuCI to :8080/:8443 (reversible on uninstall). Only `install.sh` moves the uhttpd ports. `internal/txn` has no uhttpd path. The install and uninstall port changes use a guard and a uhttpd snapshot. Restoration preserves each section’s listen values, their order, and whether they were an option or list. Settings live in UCI so backup/sysupgrade cover them.

Update: GitHub release assets `<name>-<arch>.tar.gz`, `checksums.txt`, minisign signature (public key embedded). Download to /tmp, verify, keep `.old`, atomic swap, procd restart, post-restart health check, automatic rollback. No background polling: check on demand and at most daily when the UI opens. Release source configurable; disabled while the repo is private. Version via release-please, stamped with build time. A native `.apk` is deferred with conditions in [0015](0015-no-native-apk-yet.md).

## Signing key

- One minisign key pair signs `checksums.txt` of every release, in the legacy format (`minisign -S -l`) so the router's `usign` verifies it (OpenWrt ships `usign`, not minisign). `usign` reads only the first two lines of the `.minisig` (untrusted comment and signature); the release ships them as `checksums.txt.sig`. The trusted comment is not checked by `usign` and must not be relied on.
- The secret key lives only in the GitHub secrets `MINISIGN_SECRET_KEY` and `MINISIGN_PASSWORD`, plus the operator's offline backup. The public key is `packaging/minisign.pub`, shipped in the tarball as `minisign.pub`. Without that file builds are unsigned and `release.yml` fails before uploading anything.
- Trust model: a key inside the tarball proves integrity only. `install.sh` verifies against `/etc/travo-greenfield/minisign.pub` when it exists (kept by sysupgrade through the `/etc/travo-greenfield/` keep.d entry). On the first install (no key on the router) it verifies against the tarball's key: trust on first use, the operator fetched both over HTTPS from the GitHub release. A tarball with a different key is refused unless `--rotate-key` is given. `--insecure` installs without a signature check and never writes the key. Self-update embeds the key in the binary instead.
- Rotation: generate a new key pair, commit the new `packaging/minisign.pub`, update the secrets; the release notes tell users to pass `--rotate-key` once.



## Release staging

Verification allows one active attempt and one verified stage per daemon. A stage lasts 30 minutes and lives under a private, exclusively locked directory in /tmp. Downloads and extraction share a 120-second budget. Download caps are 16 MiB for the archive, 64 KiB for the checksum manifest and 1 KiB for its signature. Extraction is limited to 24 MiB, 13 MiB per member and 256 members. The verifier checks the detached signature with the embedded public key, then checks the exact archive members and release version against the signed manifest. Verification leaves the running installation untouched.

## Activation and recovery

The update worker owns the daemon binary, packaged UI tree and their installed manifest entries. The installer owns the init script, button hotplug script, defaults, keep list, configuration and trust key. An update refuses a custom static directory.

The worker copies verified staged inputs into private durable state before releasing the stage lease. It holds the same operation lock as install and uninstall, writes the guard before activation, and retains a complete rollback snapshot. Recovery verifies the running executable identity and configured health endpoint. Install and uninstall also honor the durable guard after an interrupted worker releases the process lock.

Successful activation retains one previous generation under `/etc/travo-greenfield/update-operation/.old`. The updater accounts for simultaneous copies and reserves 20 MiB of free space on each affected filesystem. For a failed recovery use `/usr/bin/travo-greenfield --recover-update` at the local console. If the installed executable cannot start, use `/etc/travo-greenfield/update-operation/recovery --recover-update`.
