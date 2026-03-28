#!/bin/bash
# Build production binary for OpenWRT
# Usage: ./scripts/build.sh [target_arch] [target_os]
# Default target: linux/arm64 (aarch64 for MT3000/AXT1800)
# Override with GOARCH/GOOS env vars or positional arguments.
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
