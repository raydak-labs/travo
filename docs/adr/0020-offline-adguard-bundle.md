# 0020 Offline AdGuard Home bundle

Status: accepted (#184; unit- and container-tested, offline install on hardware in the playbook)

[0006](0006-adguard-package-and-topology.md) installs AdGuard Home from the OpenWrt feed after the first uplink. A router set up without any uplink (or only a captive portal) gets no filtering until then. Each release therefore ships the feed's `adguardhome` package as a separate asset for an offline first install.

## Facts (AXT1800, OpenWrt 25.12.6, apk-tools 3.0.5, 2026-11-18)

- `adguardhome-0.107.80-r1.apk` for `aarch64_cortex-a53` is 11,434,967 bytes (32 MiB installed). Dependencies `ca-bundle` and `libc` are in the base image, so `apk add` resolves it without any feed.
- `apk verify` on a feed package added as a file reports `UNTRUSTED signature`: OpenWrt signs the feed index, not each package. `apk add --allow-untrusted <file>` is required, so the package's integrity must come from somewhere else.
- OpenWrt drops superseded versions from release feeds (0.107.79 was gone within days), so a version pinned in the repository would break the release build.

## Decision

- **Separate asset, not a tarball member.** The release uploads `adguardhome-<version>-aarch64_cortex-a53.apk` next to the tarball. Inside the tarball it would put the archive at about 15.6 MB and the extracted tree near the self-update caps of [0007](0007-distribution-and-self-update.md) (16 MiB archive, 24 MiB extracted), and every self-update would download 11 MB it never uses. The tarball stays at its 0.1 size (4,145,511 bytes for 0.1.0); the 12.8 MB budget is for the binary only and is unaffected.
- **Trust through the signed binary.** The release build (`scripts/fetch-adguard-bundle.sh`) downloads the feed's current `adguardhome` over HTTPS from `downloads.openwrt.org` for the pinned OpenWrt release and stamps its SHA-256 and file name into the daemon (`-ldflags -X adguard.BundleSHA256/BundleName`). The binary is covered by the signed `checksums.txt`, so the pin is as trusted as the release. `travo-greenfield --adguard-bundle-pin` prints it.
- **install.sh stages, the daemon installs.** `install.sh --adguard-bundle FILE` compares the file's hash with the pin of the binary it is about to install and copies it to `/tmp/travo-greenfield-bundle/adguardhome.apk` (dir 0700). RAM only: no 11 MB on flash next to the 32 MiB install, and a reboot simply removes it. `adguard.Service` remains the only code that installs AGH: with the package absent and a bundle whose hash matches the pin, `Install` skips the online gate and the list refresh and runs `PackageManager.InstallFile` (`apk add --allow-untrusted`, `opkg install`). It verifies the hash again right before the install. Seed-first config, flash check, install guard and the one-automatic-attempt marker are unchanged. A missing, unpinned or mismatched bundle falls back to the online path; a mismatched file is never installed.
- **Version drift is accepted.** The bundled version is the feed's at release time. Later feed upgrades are the operator's (`apk upgrade`), as for an online install. A bundle built for a different OpenWrt release may fail `apk add` on dependency versions; the install guard then holds and the user's explicit retry uses the online path once a bundle-less retry is possible (reboot clears the bundle).

## Residual risk

The build trusts `downloads.openwrt.org` over HTTPS at release time, not OpenWrt's index signature (verifying the ADB index in CI needs apk-tools there). A compromised mirror at build time would be pinned and signed. The same mirror is what an online router installs from, but the router checks the index signature; adding that check to the release job is the follow-up if this matters.
