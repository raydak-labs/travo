#!/bin/bash
# Build production binary for OpenWRT (aarch64)
# Usage: ./scripts/build.sh [target_arch] [target_os]
# Default target: linux/arm64 (aarch64 for MT3000/AXT1800)
set -euo pipefail

TARGET_ARCH=${1:-arm64}
TARGET_OS=${2:-linux}
BUILD_DIR="dist"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

echo "Building openwrt-travel-gui v${VERSION} for ${TARGET_OS}/${TARGET_ARCH}..."

# Ensure dist dir exists
mkdir -p "${BUILD_DIR}"

# Step 1: Build frontend
echo "→ Building frontend..."
(cd frontend && pnpm build)

# Step 2: Copy frontend dist into backend for static serving
echo "→ Preparing frontend assets..."
rm -rf backend/static
cp -r frontend/dist backend/static

# Step 3: Cross-compile Go backend
echo "→ Cross-compiling backend for ${TARGET_OS}/${TARGET_ARCH}..."
(
  cd backend
  CGO_ENABLED=0 GOOS=${TARGET_OS} GOARCH=${TARGET_ARCH} go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o "../${BUILD_DIR}/openwrt-travel-gui" \
    ./cmd/server
)

# Cleanup copied static dir
rm -rf backend/static

# Step 4: Show binary info
ls -lh "${BUILD_DIR}/openwrt-travel-gui"
echo "✓ Build complete: ${BUILD_DIR}/openwrt-travel-gui"
