#!/usr/bin/env bash
#
# deploy-local.sh — push a dev build to a local OpenWrt router over SSH.
# Production installs: use scripts/install.sh on the device (release tarball).
#
# Methods (--method):
#   direct   Copy /usr/bin/travo and /www/travo only (fast). Requires /etc/init.d/travo (run setup-local.sh once).
#   release  Stream the same file tree as package-tarball.sh to / (full layout).
#
# Usage:
#   ./scripts/deploy-local.sh [options]
#   make deploy ROUTER_IP=... DEPLOY_METHOD=direct|release
#
# Options:
#   --ip IP              Router address (default: 192.168.1.1)
#   --user USER          SSH user (default: root)
#   --method METHOD      direct | release (default: direct)
#   --legacy-scp         Use scp -O for Dropbear (default: on)
#   --no-legacy-scp      Standard scp
#   --no-build           Skip scripts/build.sh; use existing dist/travo and frontend/dist
#   --binary-only        Upload backend only (direct only; incompatible with release)
#   --no-restart         Do not restart travo after deploy
#   --restart-only       Only restart travo (no file transfer)
#   -h, --help           Usage
# Environment:
#   (none; use flags or Makefile variables ROUTER_IP / DEPLOY_METHOD)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

ROUTER_IP="192.168.1.1"
ROUTER_USER="root"
METHOD="direct"
LEGACY_SCP=true
DO_BUILD=true
DO_RESTART=true
RESTART_ONLY=false
BINARY_ONLY=false
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}→${NC} $*"; }
warn()  { echo -e "${YELLOW}WARNING:${NC} $*"; }
error() { echo -e "${RED}ERROR:${NC} $*" >&2; exit 1; }

usage() {
  cat <<'EOF'
Usage: deploy-local.sh [options]

Options:
  --ip IP              Router IP (default: 192.168.1.1)
  --user USER          SSH user (default: root)
  --method METHOD      direct | release (default: direct)
  --legacy-scp         Use scp -O for Dropbear (default: on)
  --no-legacy-scp
  --no-build           Use existing dist/travo and frontend/dist
  --binary-only        Only upload backend binary (direct method only)
  --no-restart         Skip service restart
  --restart-only       Only restart travo (no transfer)
  -h, --help

Examples:
  ./scripts/deploy-local.sh
  ./scripts/deploy-local.sh --method release --ip 10.0.0.1
  ./scripts/deploy-local.sh --no-build --binary-only

First-time on a clean router: ./scripts/setup-local.sh
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ip)           ROUTER_IP="$2"; shift 2 ;;
    --user)         ROUTER_USER="$2"; shift 2 ;;
    --method)       METHOD="$2"; shift 2 ;;
    --legacy-scp)   LEGACY_SCP=true; shift ;;
    --no-legacy-scp) LEGACY_SCP=false; shift ;;
    --no-build)     DO_BUILD=false; shift ;;
    --binary-only)  BINARY_ONLY=true; shift ;;
    --no-restart)   DO_RESTART=false; shift ;;
    --restart-only) RESTART_ONLY=true; shift ;;
    -h|--help)      usage ;;
    *)              error "Unknown option: $1 (try --help)" ;;
  esac
done

if $BINARY_ONLY && [[ "$METHOD" == "release" ]]; then
  error "Cannot use --binary-only with --method release"
fi

REMOTE="${ROUTER_USER}@${ROUTER_IP}"

scp_cmd() {
  if $LEGACY_SCP; then scp -O $SSH_OPTS "$@"; else scp $SSH_OPTS "$@"; fi
}

ssh_cmd() { ssh $SSH_OPTS "${REMOTE}" "$@"; }

check_connectivity() {
  info "Checking SSH ${REMOTE}..."
  ssh_cmd "echo ok" >/dev/null 2>&1 || error "Cannot SSH to ${REMOTE}"
}

warn_missing_service_script() {
  [[ "$METHOD" == "direct" ]] || return 0
  ssh_cmd "test -x /etc/init.d/travo" 2>/dev/null && return 0
  warn "/etc/init.d/travo missing. Run once: ./scripts/setup-local.sh"
}

do_build() {
  if $BINARY_ONLY; then
    info "Building backend only..."
    (cd "${REPO_ROOT}/backend" && go mod tidy)
    local version
    version=$(cd "${REPO_ROOT}" && git describe --tags --always --dirty 2>/dev/null || echo "dev")
    mkdir -p "${REPO_ROOT}/dist"
    (cd "${REPO_ROOT}/backend" && CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH:-arm64}" go build \
      -ldflags="-s -w -X main.Version=${version}" \
      -o "../dist/travo" ./cmd/server)
  else
    info "Building via scripts/build.sh..."
    bash "${REPO_ROOT}/scripts/build.sh"
  fi
}

deploy_direct() {
  local binary="${REPO_ROOT}/dist/travo"
  [[ -f "$binary" ]] || error "Missing $binary (run build or drop --no-build)"

  info "Stopping travo..."
  ssh_cmd "/etc/init.d/travo stop 2>/dev/null || true"

  info "Uploading binary..."
  scp_cmd "$binary" "${REMOTE}:/usr/bin/travo"
  ssh_cmd "chmod +x /usr/bin/travo"

  if ! $BINARY_ONLY; then
    local frontend_dir="${REPO_ROOT}/frontend/dist"
    [[ -d "$frontend_dir" ]] || error "Missing $frontend_dir"
    info "Uploading frontend assets..."
    ssh_cmd "mkdir -p /www/travo"
    COPYFILE_DISABLE=1 tar -cf - -C "$frontend_dir" . | ssh_cmd "tar -xf - -C /www/travo/"
  else
    info "Skipping frontend (--binary-only)."
  fi
}

deploy_release() {
  local stage_dir
  stage_dir="$(cd "${REPO_ROOT}" && bash scripts/package-tarball.sh --stage-only)"
  [[ -n "$stage_dir" && -d "$stage_dir" ]] || error "staging failed — run build or drop --no-build"

  info "Stopping travo..."
  ssh_cmd "/etc/init.d/travo stop 2>/dev/null || true"

  info "Streaming release tree to / ..."
  COPYFILE_DISABLE=1 tar -cf - -C "$stage_dir" . | ssh_cmd "tar -xf - -C /"

  info "Post-install chmod / uci-defaults..."
  ssh_cmd "chmod +x /usr/bin/travo /etc/init.d/travo"
  ssh_cmd "chmod +x /etc/sysupgrade.d/10-travo-backup.sh /etc/sysupgrade.d/20-travo-restore.sh 2>/dev/null || true"
  ssh_cmd 'if [ -f /etc/uci-defaults/99-travel-gui-ports ]; then sh /etc/uci-defaults/99-travel-gui-ports && rm -f /etc/uci-defaults/99-travel-gui-ports; fi'
  ssh_cmd "/etc/init.d/travo enable"
  ssh_cmd "uci set attendedsysupgrade.client.login_check_for_upgrades='1' 2>/dev/null && uci commit attendedsysupgrade 2>/dev/null || true"
}

restart_service() {
  ssh_cmd "rm -f /etc/travo/ap-health-in-progress /etc/travo/autoreconnect-crash-guard" || true
  info "Restarting travo..."
  ssh_cmd "/etc/init.d/travo restart 2>/dev/null || /etc/init.d/travo start 2>/dev/null || true"
  info "Waiting for process..."
  sleep 8
  local attempts=0
  while [[ $attempts -lt 5 ]]; do
    if ssh_cmd "pgrep -f travo >/dev/null 2>&1"; then
      echo -e "${GREEN}OK${NC} travo running"
      return 0
    fi
    attempts=$((attempts + 1))
    [[ $attempts -lt 5 ]] && { warn "retry ${attempts}/5..."; sleep 3; }
  done
  warn "Not detected. ssh root@${ROUTER_IP} 'logread | grep travo | tail -20'"
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Deploy → ${REMOTE}  |  method=${METHOD}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

check_connectivity
warn_missing_service_script

if $RESTART_ONLY; then
  restart_service
  exit 0
fi

$DO_BUILD && do_build

case "$METHOD" in
  direct)  deploy_direct ;;
  release) deploy_release ;;
  *)       error "method must be direct or release" ;;
esac

$DO_RESTART && restart_service

echo -e "\n${GREEN}OK${NC} Done — http://${ROUTER_IP}/  (LuCI often :8080)"
