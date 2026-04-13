#!/bin/sh
#
# uninstall.sh — remove Travo (delegates to install.sh --uninstall).
#
# Usage:
#   sh scripts/uninstall.sh
#   sh scripts/uninstall.sh --yes
#
# Arguments:
#   Passed through to install.sh after --uninstall (e.g. --yes / -y to skip prompts).
#
# Environment:
#   GITHUB_REPO   owner/repo when fetching install.sh from GitHub (default: raydak-labs/travo)
#
# Behavior:
#   If scripts/install.sh exists next to this file, runs it locally; otherwise downloads install.sh from GitHub.
#
# POSIX sh (busybox ash on OpenWrt).
#
set -eu

SCRIPT_DIR="$(dirname "$0")"
SCRIPT_DIR="$(cd "$SCRIPT_DIR" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/install.sh"

# If install.sh is available locally, use it directly
if [ -f "$INSTALL_SCRIPT" ]; then
    exec sh "$INSTALL_SCRIPT" --uninstall "$@"
fi

# Otherwise, download and run from GitHub
GITHUB_REPO="${GITHUB_REPO:-raydak-labs/travo}"
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
