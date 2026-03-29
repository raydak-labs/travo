#!/bin/bash
# Create an install tarball for OpenWRT — works with any package manager (apk, opkg, or none).
# The tarball extracts directly to / and contains all runtime files.
#
# Usage: VERSION=1.0.0 ARCH=aarch64_cortex-a53 bash scripts/package-tarball.sh
set -euo pipefail

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "0.0.0")}"
VERSION="${VERSION#v}"
ARCH="${ARCH:-${1:-aarch64_cortex-a53}}"
PKG_NAME="travo"
BUILD_DIR="dist"
STAGE_DIR="${BUILD_DIR}/tarball-stage"

echo "==> Packaging ${PKG_NAME} v${VERSION} for ${ARCH}"

if [ ! -f "${BUILD_DIR}/travo" ]; then
    echo "Error: binary not found at ${BUILD_DIR}/travo — run scripts/build.sh first" >&2
    exit 1
fi
if [ ! -d "frontend/dist" ]; then
    echo "Error: frontend/dist not found — run pnpm build in frontend/ first" >&2
    exit 1
fi

rm -rf "${STAGE_DIR}"
mkdir -p \
    "${STAGE_DIR}/usr/bin" \
    "${STAGE_DIR}/www/travo" \
    "${STAGE_DIR}/etc/init.d" \
    "${STAGE_DIR}/etc/config" \
    "${STAGE_DIR}/etc/uci-defaults" \
    "${STAGE_DIR}/etc/travo"

cp "${BUILD_DIR}/travo"                                           "${STAGE_DIR}/usr/bin/travo"
chmod +x "${STAGE_DIR}/usr/bin/travo"
cp -r frontend/dist/*                                             "${STAGE_DIR}/www/travo/"
cp packaging/openwrt/files/etc/init.d/travo                      "${STAGE_DIR}/etc/init.d/travo"
chmod +x "${STAGE_DIR}/etc/init.d/travo"
cp packaging/openwrt/files/etc/config/travo                      "${STAGE_DIR}/etc/config/travo"
cp packaging/openwrt/files/etc/uci-defaults/99-travel-gui-ports  "${STAGE_DIR}/etc/uci-defaults/99-travel-gui-ports"
cp packaging/adguard/AdGuardHome.yaml                             "${STAGE_DIR}/etc/travo/adguardhome.yaml"

OUTPUT="${BUILD_DIR}/${PKG_NAME}_${VERSION}_${ARCH}.tar.gz"
tar -czf "${OUTPUT}" -C "${STAGE_DIR}" .

echo "==> Created: ${OUTPUT} ($(du -sh "${OUTPUT}" | cut -f1))"
