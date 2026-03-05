#!/bin/sh
# uninstall.sh — Remove OpenWRT Travel GUI and restore defaults
#
# Usage: sh uninstall.sh [--yes]
#
# This is a convenience wrapper around install.sh --uninstall.
# POSIX sh compatible — works with busybox ash on OpenWRT.

set -eu

SCRIPT_DIR="$(dirname "$0")"
SCRIPT_DIR="$(cd "$SCRIPT_DIR" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/install.sh"

# If install.sh is available locally, use it directly
if [ -f "$INSTALL_SCRIPT" ]; then
    exec sh "$INSTALL_SCRIPT" --uninstall "$@"
fi

# Otherwise, download and run from GitHub
GITHUB_REPO="${GITHUB_REPO:-openwrt-travel-gui/openwrt-travel-gui}"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/main"

_tmp_script="/tmp/travel-gui-install.sh"

if command -v wget >/dev/null 2>&1; then
    wget -q -O "$_tmp_script" "${GITHUB_RAW_BASE}/scripts/install.sh"
elif command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$_tmp_script" "${GITHUB_RAW_BASE}/scripts/install.sh"
else
    printf '[error] Neither wget nor curl found\n' >&2
    exit 1
fi

exec sh "$_tmp_script" --uninstall "$@"
