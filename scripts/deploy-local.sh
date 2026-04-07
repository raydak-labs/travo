#!/bin/bash
# Deploy travo to a local OpenWRT device via SSH/SCP.
#
# Supports both .ipk-based install (opkg/apk) and direct file copy.
# Handles legacy SCP protocol for older OpenWrt/Dropbear SSH servers.
#
# Direct mode only updates:
#   - /usr/bin/travo
#   - files under /www/travo/ (extracted from frontend/dist; overwrites
#     same paths, does not delete extra files left in that directory)
# It does NOT remove or replace:
#   - /etc/init.d/travo
#   - /etc/config/travo
#   - /etc/uci-defaults, /etc/sysupgrade.d, default AdGuard template under /etc/travo/
# If the service script is missing, port 80 will stay down until you run
#   ./scripts/setup-local.sh [--no-luci-move]
#
# Release mode (--method release) stages the same tree as scripts/package-tarball.sh
# and streams it to / on the router (no .tar.gz file on disk). Matches tarball extract +
# postinst-style uci-defaults apply. Overwrites packaged paths under /etc and /www.
#
# Usage:
#   ./scripts/deploy-local.sh [options]
#
# Examples:
#   # Default: build frontend + backend, then upload (direct copy, 192.168.1.1)
#   ./scripts/deploy-local.sh
#
#   # Skip build; upload existing dist/ + frontend/dist only
#   ./scripts/deploy-local.sh --no-build
#
#   # Deploy .ipk via apk (OpenWrt 25.x+); still builds first by default
#   ./scripts/deploy-local.sh --method apk
#
#   # Deploy to a different IP with legacy SCP
#   ./scripts/deploy-local.sh --ip 10.0.0.1 --legacy-scp
#
#   # Just restart the service (no build, no file transfer)
#   ./scripts/deploy-local.sh --restart-only
#
#   # Build + backend-only upload (skip frontend assets)
#   ./scripts/deploy-local.sh --binary-only
#
#   # Same filesystem layout as a release tarball (binary, LuCI ports uci-default,
#   # sysupgrade hooks, adguard template, etc.) — no tarball or .ipk step
#   ./scripts/deploy-local.sh --method release

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ── Defaults ──────────────────────────────────────────────────────────────────
ROUTER_IP="192.168.1.1"
ROUTER_USER="root"
METHOD="direct"        # direct | release | opkg | apk
LEGACY_SCP=true        # use -O flag for older dropbear
DO_BUILD=true
DO_RESTART=true
RESTART_ONLY=false
BINARY_ONLY=false
IPK_PATH=""
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5"

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
  --method METHOD      Deploy method: direct, release, opkg, apk (default: direct)
  --ipk PATH           Path to .ipk file (auto-detected from dist/ if omitted)
  --legacy-scp         Use SCP legacy protocol (-O flag) for Dropbear (default: on)
  --no-legacy-scp      Disable legacy SCP flag
  --no-build           Skip build; use existing dist/ and frontend/dist
  --binary-only        Only upload the backend binary (skip frontend assets)
  --no-restart         Skip service restart after deploy
  --restart-only       Only restart the service (no file transfer)
  -h, --help           Show this help

Deploy methods:
  direct   Copy binary + frontend assets only (fastest; skips tarball-only files)
  release  Same tree as package-tarball.sh → extract to / (no local .tar.gz; tests release layout)
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
    --no-build)     DO_BUILD=false; shift ;;
    --binary-only)  BINARY_ONLY=true; shift ;;
    --no-restart)   DO_RESTART=false; shift ;;
    --restart-only) RESTART_ONLY=true; shift ;;
    -h|--help)      usage ;;
    *)              error "Unknown option: $1. Use --help for usage." ;;
  esac
done

if $BINARY_ONLY && [[ "$METHOD" == "release" ]]; then
  error "Cannot use --binary-only with --method release (release deploys the full staged tree)."
fi

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

# Warn when direct deploy cannot start the app (init.d never uploaded by this script).
warn_missing_service_script() {
  if [[ "$METHOD" != "direct" ]]; then
    return 0
  fi
  if ssh_cmd "test -x /etc/init.d/travo" 2>/dev/null; then
    return 0
  fi
  warn "/etc/init.d/travo is missing or not executable on the router."
  warn "deploy-local.sh does not install it. Run once: ./scripts/setup-local.sh [--no-luci-move]"
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
        -o "../dist/travo" \
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
  local binary="dist/travo"

  [[ -f "$binary" ]] || error "Binary not found at $binary. Run 'make build-prod' or remove --no-build."

  info "Stopping service..."
  ssh_cmd "/etc/init.d/travo stop 2>/dev/null || true"

  info "Uploading binary..."
  scp_cmd "$binary" "${REMOTE}:/usr/bin/travo"
  ssh_cmd "chmod +x /usr/bin/travo"

  if ! $BINARY_ONLY; then
    local frontend_dir="frontend/dist"
    [[ -d "$frontend_dir" ]] || error "Frontend assets not found at $frontend_dir. Run 'pnpm build' in frontend/ or remove --no-build."
    info "Uploading frontend assets..."
    ssh_cmd "mkdir -p /www/travo"
    COPYFILE_DISABLE=1 tar -cf - -C "$frontend_dir" . | ssh_cmd "tar -xf - -C /www/travo/"
  else
    info "Skipping frontend (--binary-only)."
  fi

  info "Files deployed via direct copy."
}

# ── Deploy: release layout (same as tarball extract, no .tar.gz artifact) ────
deploy_release() {
  local stage_dir
  stage_dir="$(cd "${REPO_ROOT}" && bash scripts/stage-release-layout.sh)"
  [[ -n "$stage_dir" && -d "$stage_dir" ]] || error "Release staging failed — run scripts/build.sh or remove --no-build."

  info "Stopping service..."
  ssh_cmd "/etc/init.d/travo stop 2>/dev/null || true"

  info "Uploading release filesystem tree (tar stream to /)..."
  COPYFILE_DISABLE=1 tar -cf - -C "$stage_dir" . | ssh_cmd "tar -xf - -C /"

  info "Applying post-install steps (chmod, uci-defaults, enable)..."
  ssh_cmd "chmod +x /usr/bin/travo /etc/init.d/travo"
  ssh_cmd "chmod +x /etc/sysupgrade.d/10-travo-backup.sh /etc/sysupgrade.d/20-travo-restore.sh 2>/dev/null || true"
  ssh_cmd 'if [ -f /etc/uci-defaults/99-travel-gui-ports ]; then sh /etc/uci-defaults/99-travel-gui-ports && rm -f /etc/uci-defaults/99-travel-gui-ports; fi'
  ssh_cmd "/etc/init.d/travo enable"
  ssh_cmd "uci set attendedsysupgrade.client.login_check_for_upgrades='1' 2>/dev/null && uci commit attendedsysupgrade 2>/dev/null || true"

  info "Release layout deployed (matches package-tarball contents → /)."
}

# ── Deploy: package manager (opkg/apk) ───────────────────────────────────────
deploy_package() {
  local pkg_cmd="$1"
  local install_flag="$2"

  # Find .ipk
  if [[ -z "$IPK_PATH" ]]; then
    IPK_PATH=$(ls -1t dist/*.ipk 2>/dev/null | head -1)
  fi
  [[ -n "$IPK_PATH" && -f "$IPK_PATH" ]] || error "No .ipk found in dist/. Run 'make package' or remove --no-build."

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
  # Clear the AP health check crash guard so the freshly deployed binary gets
  # a clean attempt on startup. A manual redeploy is explicit permission to retry.
  ssh_cmd "rm -f /etc/travo/ap-health-in-progress /etc/travo/autoreconnect-crash-guard" || true

  info "Restarting travo service..."
  ssh_cmd "/etc/init.d/travo restart 2>/dev/null || /etc/init.d/travo start 2>/dev/null || true"

  # Give procd time to (re)start the process. On slower routers or after a
  # package install that triggers a reboot, the service may take longer.
  info "Waiting for service to start..."
  sleep 8

  # Retry up to 5 times (3s apart) in case procd is still initializing.
  local attempts=0
  while [[ $attempts -lt 5 ]]; do
    if ssh_cmd "pgrep -f travo >/dev/null 2>&1"; then
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
  warn "  ssh root@${ROUTER_IP} 'logread | grep travo | tail -20'"
}

# ── Main ──────────────────────────────────────────────────────────────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Deploy travo → ${REMOTE}"
echo "  Method: ${METHOD}  |  Legacy SCP: ${LEGACY_SCP}  |  Binary only: ${BINARY_ONLY}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

check_connectivity
warn_missing_service_script

if $RESTART_ONLY; then
  restart_service
  echo -e "\n${GREEN}✓${NC} Service restarted at http://${ROUTER_IP}/"
  exit 0
fi

if $DO_BUILD; then
  do_build
fi

case "$METHOD" in
  direct)  deploy_direct ;;
  release) deploy_release ;;
  opkg)    deploy_package "opkg" "install" ;;
  apk)     deploy_package "apk" "add --allow-untrusted" ;;
  *)       error "Unknown method: ${METHOD}. Use direct, release, opkg, or apk." ;;
esac

if $DO_RESTART; then
  restart_service
fi

echo -e "\n${GREEN}✓${NC} Deployment complete!"
echo "  App:   http://${ROUTER_IP}/"
echo "  LuCI:  http://${ROUTER_IP}:8080/"
