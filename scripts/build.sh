#!/bin/bash
#
# build.sh — production frontend build + cross-compiled Go binary for OpenWrt.
#
# Usage:
#   ./scripts/build.sh [GOARCH] [GOOS]
#   GOARCH=amd64 GOOS=linux ./scripts/build.sh
#
# Positional arguments (optional; overridden by GOARCH / GOOS when set):
#   $1  Target Go architecture (default: arm64)
#   $2  Target Go OS (default: linux)
#
# Environment:
#   GOARCH   Target architecture (e.g. arm64, amd64)
#   GOOS     Target OS (typically linux)
#
# Outputs:
#   dist/travo           Stripped Go binary (main.Version from git describe)
#   frontend/dist/       Vite production assets (embedded path expectations)
#
set -euo pipefail

TARGET_ARCH="${GOARCH:-${1:-arm64}}"
TARGET_OS="${GOOS:-${2:-linux}}"
BUILD_DIR="dist"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

echo "Building travo v${VERSION} for ${TARGET_OS}/${TARGET_ARCH}..."

# Ensure dist dir exists
mkdir -p "${BUILD_DIR}"

# Step 1: Build frontend
echo "→ Building frontend..."
(cd frontend && pnpm build)

# Step 2: Tidy Go modules
echo "→ Tidying Go modules..."
(cd backend && go mod tidy)

# Step 3: Cross-compile Go backend
echo "→ Cross-compiling backend for ${TARGET_OS}/${TARGET_ARCH}..."
(
  cd backend
  CGO_ENABLED=0 GOOS=${TARGET_OS} GOARCH=${TARGET_ARCH} go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o "../${BUILD_DIR}/travo" \
    ./cmd/server
)

# Step 4: Show binary info
ls -lh "${BUILD_DIR}/travo"
echo "✓ Build complete: ${BUILD_DIR}/travo"
echo "  Frontend assets preserved at frontend/dist/"
