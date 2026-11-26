#!/bin/sh
# Downloads the adguardhome package of the pinned OpenWrt release feed into dist/ for the
# offline bundle (docs/adr/0020) and prints its path. The feed's current version is used:
# OpenWrt removes superseded packages from release feeds, so a pinned version would vanish.
# Trust: HTTPS from downloads.openwrt.org at build time; the pin stamped into the signed
# binary then covers the file end to end.
set -eu
cd "$(dirname "$0")/.."

OPENWRT_RELEASE=${OPENWRT_RELEASE:-25.12.6}
TARGET_ARCH=${TARGET_ARCH:-aarch64_cortex-a53}
feed="https://downloads.openwrt.org/releases/$OPENWRT_RELEASE/packages/$TARGET_ARCH/packages"

version=$(curl -fsSL --proto '=https' "$feed/index.json" |
  sed -n 's/^ *"adguardhome": *"\([0-9A-Za-z.~_+-]*\)",\{0,1\}$/\1/p')
[ -n "$version" ] || { echo "fetch-adguard-bundle: adguardhome not in $feed/index.json" >&2; exit 1; }

mkdir -p dist
out="dist/adguardhome-$version-$TARGET_ARCH.apk"
curl -fsSL --proto '=https' -o "$out.tmp" "$feed/adguardhome-$version.apk"
# An apk v3 package starts with the ADB magic (ADBd: deflate); anything else is an error page or garbage.
[ "$(head -c 3 "$out.tmp")" = ADB ] || { rm -f "$out.tmp"; echo "fetch-adguard-bundle: not an apk package" >&2; exit 1; }
mv "$out.tmp" "$out"
echo "$out"
