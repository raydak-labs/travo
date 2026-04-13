#!/bin/bash
#
# package-tarball.sh — stage rootfs layout and create travo_<version>_<arch>.tar.gz for OpenWrt.
# Tarball extracts to /. Works with apk, opkg, or no package manager.
#
# Usage:
#   bash scripts/package-tarball.sh
#   bash scripts/package-tarball.sh aarch64_cortex-a53
#   VERSION=1.0.0 ARCH=x86_64 bash scripts/package-tarball.sh
#   bash scripts/package-tarball.sh --stage-only
#
# Arguments:
#   --stage-only     Only build the stage directory; print its absolute path to stdout; no .tar.gz.
#   [ARCH]           Positional OpenWrt-style arch label for the tarball filename (after flags).
#                    Default: aarch64_cortex-a53
#
# Environment:
#   VERSION     Release version string for the filename (default: git describe, v prefix stripped)
#   ARCH        Same as positional ARCH (env wins if set before parse)
#   BUILD_DIR   Where dist/travo and output tarball live (default: dist)
#   STAGE_DIR   Staged rootfs tree (default: ${BUILD_DIR}/tarball-stage)
#
# Prerequisites:
#   dist/travo from scripts/build.sh, frontend/dist populated.
#
# Outputs:
#   Without --stage-only: ${BUILD_DIR}/travo_${VERSION}_${ARCH}.tar.gz
#   With --stage-only: stdout = absolute path of STAGE_DIR
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

STAGE_ONLY=false
POS_ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --stage-only) STAGE_ONLY=true; shift ;;
    *) POS_ARGS+=("$1"); shift ;;
  esac
done
set -- "${POS_ARGS[@]}"

VERSION="${VERSION:-$(git -C "${REPO_ROOT}" describe --tags --always --dirty 2>/dev/null || echo "0.0.0")}"
VERSION="${VERSION#v}"
ARCH="${ARCH:-${1:-aarch64_cortex-a53}}"
PKG_NAME="travo"
BUILD_DIR="${BUILD_DIR:-dist}"
STAGE_DIR="${STAGE_DIR:-${BUILD_DIR}/tarball-stage}"

stage_release_layout() {
  cd "${REPO_ROOT}"

  if [ ! -f "${BUILD_DIR}/travo" ]; then
    echo "Error: binary not found at ${BUILD_DIR}/travo — run scripts/build.sh first" >&2
    exit 1
  fi
  if [ ! -d "frontend/dist" ]; then
    echo "Error: frontend/dist not found — run pnpm build in frontend/ or scripts/build.sh" >&2
    exit 1
  fi

  rm -rf "${STAGE_DIR}"
  mkdir -p \
    "${STAGE_DIR}/usr/bin" \
    "${STAGE_DIR}/www/travo" \
    "${STAGE_DIR}/etc/init.d" \
    "${STAGE_DIR}/etc/config" \
    "${STAGE_DIR}/etc/uci-defaults" \
    "${STAGE_DIR}/etc/sysupgrade.d" \
    "${STAGE_DIR}/etc/travo"

  cp "${BUILD_DIR}/travo"                                           "${STAGE_DIR}/usr/bin/travo"
  chmod +x "${STAGE_DIR}/usr/bin/travo"
  cp -r frontend/dist/*                                             "${STAGE_DIR}/www/travo/"
  cp packaging/openwrt/files/etc/init.d/travo                      "${STAGE_DIR}/etc/init.d/travo"
  chmod +x "${STAGE_DIR}/etc/init.d/travo"
  cp packaging/openwrt/files/etc/config/travo                      "${STAGE_DIR}/etc/config/travo"
  cp packaging/openwrt/files/etc/uci-defaults/99-travel-gui-ports  "${STAGE_DIR}/etc/uci-defaults/99-travel-gui-ports"
  cp packaging/openwrt/files/etc/sysupgrade.d/10-travo-backup.sh   "${STAGE_DIR}/etc/sysupgrade.d/"
  cp packaging/openwrt/files/etc/sysupgrade.d/20-travo-restore.sh  "${STAGE_DIR}/etc/sysupgrade.d/"
  chmod +x "${STAGE_DIR}/etc/sysupgrade.d/"*
  cp packaging/adguard/AdGuardHome.yaml                             "${STAGE_DIR}/etc/travo/adguardhome.yaml"
}

if $STAGE_ONLY; then
  stage_release_layout
  echo "$(cd "${STAGE_DIR}" && pwd)"
  exit 0
fi

echo "==> Packaging ${PKG_NAME} v${VERSION} for ${ARCH}"

stage_release_layout

OUTPUT="${BUILD_DIR}/${PKG_NAME}_${VERSION}_${ARCH}.tar.gz"
tar -czf "${OUTPUT}" -C "${STAGE_DIR}" .

echo "==> Created: ${OUTPUT} ($(du -sh "${OUTPUT}" | cut -f1))"
