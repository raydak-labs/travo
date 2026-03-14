#!/bin/bash
# Deploy openwrt-travel-gui to a local OpenWRT device via SSH/SCP.
#
# Supports both .ipk-based install (opkg/apk) and direct file copy.
# Handles legacy SCP protocol for older OpenWrt/Dropbear SSH servers.
#
# Usage:
#   ./scripts/deploy-local.sh [options]
#
# Examples:
#   # Quick deploy with defaults (direct copy, 192.168.1.1)
#   ./scripts/deploy-local.sh
#
#   # Deploy .ipk via apk (OpenWrt 25.x+)
#   ./scripts/deploy-local.sh --method apk
#
#   # Deploy to a different IP with legacy SCP
#   ./scripts/deploy-local.sh --ip 10.0.0.1 --legacy-scp
#
#   # Build + deploy in one go
#   ./scripts/deploy-local.sh --build
#
#   # Just restart the service (no file transfer)
#   ./scripts/deploy-local.sh --restart-only
#
#   # Backend-only: just build & upload the binary (skip frontend)
#   ./scripts/deploy-local.sh --build --binary-only

set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────────────
ROUTER_IP="192.168.1.1"
ROUTER_USER="root"
METHOD="direct"        # direct | opkg | apk
LEGACY_SCP=true        # use -O flag for older dropbear
DO_BUILD=false
DO_RESTART=true
RESTART_ONLY=false
BINARY_ONLY=false
IPK_PATH=""
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=5"

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}→${NC} $*"; }
warn()  { echo -e "${YELLOW}⚠${NC} $*"; }
error() { echo -e "${RED}✗${NC} $*" >&2; exit 1; }

# ── Parse arguments ──────────────────────────────────────────────────────────
usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --ip IP              Router IP address (default: 192.168.1.1)
  --user USER          SSH user (default: root)
  --method METHOD      Deploy method: direct, opkg, apk (default: direct)
  --ipk PATH           Path to .ipk file (auto-detected from dist/ if omitted)
  --legacy-scp         Use SCP legacy protocol (-O flag) for Dropbear (default: on)
  --no-legacy-scp      Disable legacy SCP flag
  --build              Run build before deploying
  --binary-only        Only upload the backend binary (skip frontend assets)
  --no-restart         Skip service restart after deploy
  --restart-only       Only restart the service (no file transfer)
  -h, --help           Show this help

Deploy methods:
  direct   Copy binary + frontend assets directly (fastest, no package manager)
  opkg     Build .ipk and install via opkg (OpenWrt <25)
  apk      Build .ipk and install via apk (OpenWrt 25.x+)
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ip)           ROUTER_IP="$2"; shift 2 ;;
    --user)         ROUTER_USER="$2"; shift 2 ;;
    --method)       METHOD="$2"; shift 2 ;;
    --ipk)          IPK_PATH="$2"; shift 2 ;;
    --legacy-scp)   LEGACY_SCP=true; shift ;;
    --no-legacy-scp) LEGACY_SCP=false; shift ;;
    --build)        DO_BUILD=true; shift ;;
    --binary-only)  BINARY_ONLY=true; shift ;;
    --no-restart)   DO_RESTART=false; shift ;;
    --restart-only) RESTART_ONLY=true; shift ;;
    -h|--help)      usage ;;
    *)              error "Unknown option: $1. Use --help for usage." ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────
REMOTE="${ROUTER_USER}@${ROUTER_IP}"

scp_cmd() {
  if $LEGACY_SCP; then
    scp -O $SSH_OPTS "$@"
  else
    scp $SSH_OPTS "$@"
  fi
}

ssh_cmd() {
  ssh $SSH_OPTS "${REMOTE}" "$@"
}

check_connectivity() {
  info "Checking connectivity to ${REMOTE}..."
  if ! ssh_cmd "echo ok" >/dev/null 2>&1; then
    error "Cannot reach ${REMOTE} via SSH. Check IP, user, and SSH keys."
  fi
}

# ── Build ─────────────────────────────────────────────────────────────────────
do_build() {
  if $BINARY_ONLY; then
    info "Building backend binary only..."
    (cd backend && go mod tidy)
    local version
    version=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
    mkdir -p dist
    (
      cd backend
      CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH:-arm64} go build \
        -ldflags="-s -w -X main.Version=${version}" \
        -o "../dist/openwrt-travel-gui" \
        ./cmd/server
    )
  else
    info "Building production binary..."
    bash scripts/build.sh
    if [[ "$METHOD" == "opkg" || "$METHOD" == "apk" ]]; then
      info "Packaging .ipk..."
      bash scripts/package-ipk.sh
    fi
  fi
}

# ── Deploy: direct copy ──────────────────────────────────────────────────────
deploy_direct() {
  local binary="dist/openwrt-travel-gui"

  [[ -f "$binary" ]] || error "Binary not found at $binary. Run with --build or 'make build-prod' first."

  info "Stopping service..."
  ssh_cmd "/etc/init.d/openwrt-travel-gui stop 2>/dev/null || true"

  info "Uploading binary..."
  scp_cmd "$binary" "${REMOTE}:/usr/bin/openwrt-travel-gui"
  ssh_cmd "chmod +x /usr/bin/openwrt-travel-gui"

  if ! $BINARY_ONLY; then
    local frontend_dir="frontend/dist"
    [[ -d "$frontend_dir" ]] || error "Frontend assets not found at $frontend_dir. Run with --build first."
    info "Uploading frontend assets..."
    ssh_cmd "mkdir -p /www/openwrt-travel-gui"
    COPYFILE_DISABLE=1 tar -cf - -C "$frontend_dir" . | ssh_cmd "tar -xf - -C /www/openwrt-travel-gui/"
  else
    info "Skipping frontend (--binary-only)."
  fi

  info "Files deployed via direct copy."
}

# ── Deploy: package manager (opkg/apk) ───────────────────────────────────────
deploy_package() {
  local pkg_cmd="$1"
  local install_flag="$2"

  # Find .ipk
  if [[ -z "$IPK_PATH" ]]; then
    IPK_PATH=$(ls -1t dist/*.ipk 2>/dev/null | head -1)
  fi
  [[ -n "$IPK_PATH" && -f "$IPK_PATH" ]] || error "No .ipk found in dist/. Run with --build or 'make package' first."

  local ipk_file
  ipk_file=$(basename "$IPK_PATH")

  info "Uploading ${ipk_file}..."
  scp_cmd "$IPK_PATH" "${REMOTE}:/tmp/${ipk_file}"

  info "Installing via ${pkg_cmd}..."
  ssh_cmd "${pkg_cmd} ${install_flag} /tmp/${ipk_file} && rm -f /tmp/${ipk_file}"

  info "Package installed via ${pkg_cmd}."
}

# ── Restart ───────────────────────────────────────────────────────────────────
restart_service() {
  info "Restarting openwrt-travel-gui service..."
  ssh_cmd "/etc/init.d/openwrt-travel-gui restart 2>/dev/null || /etc/init.d/openwrt-travel-gui start 2>/dev/null || true"

  # Give procd time to (re)start the process. On slower routers or after a
  # package install that triggers a reboot, the service may take longer.
  info "Waiting for service to start..."
  sleep 8

  # Retry up to 5 times (3s apart) in case procd is still initializing.
  local attempts=0
  while [[ $attempts -lt 5 ]]; do
    if ssh_cmd "pgrep -f openwrt-travel-gui >/dev/null 2>&1"; then
      echo -e "${GREEN}✓${NC} Service is running."
      return 0
    fi
    attempts=$((attempts + 1))
    if [[ $attempts -lt 5 ]]; then
      warn "Service not yet detected, retrying in 3s... (${attempts}/5)"
      sleep 3
    fi
  done
  warn "Service not detected after $((8 + 4 * 3))s. Check logs with:"
  warn "  ssh root@${ROUTER_IP} 'logread | grep openwrt-travel-gui | tail -20'"
}

# ── Main ──────────────────────────────────────────────────────────────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Deploy openwrt-travel-gui → ${REMOTE}"
echo "  Method: ${METHOD}  |  Legacy SCP: ${LEGACY_SCP}  |  Binary only: ${BINARY_ONLY}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

check_connectivity

if $RESTART_ONLY; then
  restart_service
  echo -e "\n${GREEN}✓${NC} Service restarted at http://${ROUTER_IP}/"
  exit 0
fi

if $DO_BUILD; then
  do_build
fi

case "$METHOD" in
  direct) deploy_direct ;;
  opkg)   deploy_package "opkg" "install" ;;
  apk)    deploy_package "apk" "add --allow-untrusted" ;;
  *)      error "Unknown method: ${METHOD}. Use direct, opkg, or apk." ;;
esac

if $DO_RESTART; then
  restart_service
fi

echo -e "\n${GREEN}✓${NC} Deployment complete!"
echo "  App:   http://${ROUTER_IP}/"
echo "  LuCI:  http://${ROUTER_IP}:8080/"
