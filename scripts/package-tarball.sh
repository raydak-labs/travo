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
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Packaging ${PKG_NAME} v${VERSION} for ${ARCH}"

STAGE_DIR="$(bash "${SCRIPT_DIR}/stage-release-layout.sh")"

OUTPUT="${BUILD_DIR}/${PKG_NAME}_${VERSION}_${ARCH}.tar.gz"
tar -czf "${OUTPUT}" -C "${STAGE_DIR}" .

echo "==> Created: ${OUTPUT} ($(du -sh "${OUTPUT}" | cut -f1))"
