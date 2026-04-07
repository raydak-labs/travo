#!/usr/bin/env bash
# Stage the same root filesystem layout as scripts/package-tarball.sh (release tarball).
# Used by package-tarball.sh and deploy-local.sh --method release.
#
# Prerequisites: dist/travo and frontend/dist (run scripts/build.sh).
#
# Env:
#   BUILD_DIR   default: dist
#   STAGE_DIR   default: ${BUILD_DIR}/tarball-stage
#
# Prints the absolute STAGE_DIR path on success.
set -euo pipefail

BUILD_DIR="${BUILD_DIR:-dist}"
STAGE_DIR="${STAGE_DIR:-${BUILD_DIR}/tarball-stage}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

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

# shellcheck disable=SC2046
echo "$(cd "${STAGE_DIR}" && pwd)"
