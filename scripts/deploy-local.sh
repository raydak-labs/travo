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

# Force mise-managed toolchain versions, regardless of caller's PATH order.
command -v mise >/dev/null 2>&1 && eval "$(mise env -s bash)"

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

# Host-key verification is on by default. Set TRAVO_INSECURE_SSH=1 only for a
# throwaway lab router whose key changes on every flash; silently skipping
# verification would let a MITM capture the root SSH session this script uses.
if [[ "${TRAVO_INSECURE_SSH:-0}" == "1" ]]; then
  SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5"
  warn "TRAVO_INSECURE_SSH=1: host-key verification disabled for this deploy"
else
  SSH_OPTS="-o StrictHostKeyChecking=accept-new -o LogLevel=ERROR -o ConnectTimeout=5"
fi

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
    # Swap the directory rather than extracting over it. Vite emits
    # content-hashed filenames, so extracting on top of the previous tree leaves
    # every superseded chunk behind forever -- forty deploys while iterating on
    # the UI is enough to fill a travel router's overlay, after which uci
    # commits start failing and the device needs a reflash. The staged directory
    # plus rename is busybox-safe and leaves no window where /www/travo is empty.
    ssh_cmd "rm -rf /www/travo.new && mkdir -p /www/travo.new"
    COPYFILE_DISABLE=1 tar -cf - -C "$frontend_dir" . | ssh_cmd "tar -xf - -C /www/travo.new"
    ssh_cmd "mv /www/travo /www/travo.old 2>/dev/null || true; \
             mv /www/travo.new /www/travo; \
             rm -rf /www/travo.old"
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
  ssh_cmd "mkdir -p /lib/upgrade/keep.d"
  ssh_cmd 'if [ -f /etc/uci-defaults/99-travel-gui-ports ]; then sh /etc/uci-defaults/99-travel-gui-ports && rm -f /etc/uci-defaults/99-travel-gui-ports; fi'
  ssh_cmd "/etc/init.d/travo enable"
  ssh_cmd "uci set attendedsysupgrade.client.login_check_for_upgrades='1' 2>/dev/null && uci commit attendedsysupgrade 2>/dev/null || true"
}

restart_service() {
  # Clear the crash guards the Go code actually writes (the authoritative list
  # lives in docs/adr/0003 section 2). The old list removed ap-health-in-progress,
  # which no Go code writes, and missed the failover / band-switch / captive
  # guards -- a stuck guard permanently disables those features, so the
  # documented recovery path (architecture.md section 4, step 4) did not exist
  # for them.
  info "Clearing crash guards..."
  # autoreconnect-failcount is intentionally NOT cleared: it is the bounded
  # retry counter that stops a broken saved network being replayed every
  # minute. A successful reconnect clears it on the device.
  # firmware-upgrade-in-progress and factory-reset-in-progress are also NOT
  # cleared: ADR 0003 section 2 keeps them so an interrupted sysupgrade or
  # firstboot stays discoverable after the reboot. deploy-local.sh restarts the
  # service but must not erase the record of a device mid-recovery.
  #
  # One ssh call, not one per guard per directory: 12 guards x 2 directories is
  # 24 round-trips, and a device that accepts TCP but then hangs costs
  # ConnectTimeout seconds on each of them before the restart even starts.
  # The paths are expanded here rather than with a remote brace expansion,
  # which BusyBox ash does not support.
  #
  # /etc/travo is the legacy location: guards were split across both directories
  # before they were unified on /etc/trafo, so a device upgraded from an older
  # build can still carry a guard there. Clear both.
  local guard_paths=()
  for guard in failover-in-progress band-switch-in-progress captive-dns-in-progress \
    captive-wwan-bounce-in-progress vpn-in-progress usbtether-in-progress \
    wifi-toggle-in-progress mac-in-progress pkg-install-in-progress restore-in-progress \
    system-config-in-progress autoreconnect-crash-guard; do
    for dir in /etc/trafo /etc/travo; do
      guard_paths+=("${dir}/${guard}")
    done
  done
  ssh_cmd "rm -f ${guard_paths[*]}" >/dev/null 2>&1 || true
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
  # Report the failure instead of letting the caller print "OK Done" over a
  # service that is not running.
  #
  # error() exits, so the log lines have to be printed BEFORE it is called:
  # collecting them and then exiting discarded the one thing a failed deploy
  # needs to diagnose itself.
  local logs
  logs=$(ssh_cmd "logread 2>/dev/null | grep -i travo | tail -20" 2>/dev/null || true)
  if [[ -n "$logs" ]]; then
    echo "$logs" >&2
    error "travo is not running after restart - deploy NOT verified (log lines above)"
  else
    error "travo is not running after restart - deploy NOT verified"
  fi
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Deploy → ${REMOTE}  |  method=${METHOD}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

check_connectivity
warn_missing_service_script

if $RESTART_ONLY; then
  restart_service || exit 1
  exit 0
fi

$DO_BUILD && do_build

case "$METHOD" in
  direct)  deploy_direct ;;
  release) deploy_release ;;
  *)       error "method must be direct or release" ;;
esac

if $DO_RESTART; then
  restart_service || exit 1
fi

echo -e "\n${GREEN}OK${NC} Done — http://${ROUTER_IP}/  (LuCI often :8080)"
