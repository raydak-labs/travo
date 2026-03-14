#!/bin/bash
# Initial one-time setup of a fresh OpenWRT device for local development.
#
# Run this once on a clean device before using deploy-local.sh.
# It is idempotent — safe to run again without breaking anything.
#
# What it does:
#   1. Moves LuCI (uhttpd) to port 8080 so travel-gui can use port 80
#   2. Uploads the init.d service script
#   3. Uploads the UCI config (password, port, etc.)
#   4. Creates /www/openwrt-travel-gui
#   5. Enables the service (does not start — no binary yet)
#
# Usage:
#   ./scripts/setup-local.sh [options]
#
# Examples:
#   ./scripts/setup-local.sh
#   ./scripts/setup-local.sh --ip 10.0.0.1 --password mysecret
#   ./scripts/setup-local.sh --no-luci-move

set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────────────
ROUTER_IP="192.168.1.1"
ROUTER_USER="root"
PASSWORD="admin"
MOVE_LUCI=true
LEGACY_SCP=true
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
  --password PASSWORD  Admin password for travel-gui (default: admin)
  --no-luci-move       Skip moving LuCI to port 8080
  --legacy-scp         Use SCP legacy protocol (-O flag) for Dropbear (default: on)
  --no-legacy-scp      Disable legacy SCP flag
  -h, --help           Show this help
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ip)            ROUTER_IP="$2"; shift 2 ;;
    --user)          ROUTER_USER="$2"; shift 2 ;;
    --password)      PASSWORD="$2"; shift 2 ;;
    --no-luci-move)  MOVE_LUCI=false; shift ;;
    --legacy-scp)    LEGACY_SCP=true; shift ;;
    --no-legacy-scp) LEGACY_SCP=false; shift ;;
    -h|--help)       usage ;;
    *)               error "Unknown option: $1. Use --help for usage." ;;
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

# ── Paths ─────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
INITD_SRC="${REPO_ROOT}/packaging/openwrt/files/etc/init.d/openwrt-travel-gui"
CONFIG_SRC="${REPO_ROOT}/packaging/openwrt/files/etc/config/openwrt-travel-gui"

[[ -f "$INITD_SRC" ]] || error "init.d script not found at $INITD_SRC"
[[ -f "$CONFIG_SRC" ]] || error "UCI config not found at $CONFIG_SRC"

# ── Main ──────────────────────────────────────────────────────────────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Setup openwrt-travel-gui → ${REMOTE}"
echo "  LuCI move: ${MOVE_LUCI}  |  Password: ${PASSWORD}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

info "Checking connectivity to ${REMOTE}..."
# Sync clock early — a wrong clock on the router causes JWT tokens issued by the
# router to appear immediately expired in the browser (clock skew bug).
if ! ssh_cmd "echo ok" >/dev/null 2>&1; then
  error "Cannot reach ${REMOTE} via SSH. Check IP, user, and SSH keys."
fi

# Step 0: Sync clock
CURRENT_TIME=$(date -u +"%Y-%m-%d %H:%M:%S")
info "Syncing router clock to ${CURRENT_TIME} UTC..."
ssh_cmd "date -s '${CURRENT_TIME}' >/dev/null 2>&1 || true; /etc/init.d/sysntpd restart 2>/dev/null || true"

# Step 1: Move LuCI to port 8080
if $MOVE_LUCI; then
  info "Checking LuCI port..."
  CURRENT_PORT=$(ssh_cmd "uci -q get uhttpd.main.listen_http 2>/dev/null || echo '0.0.0.0:80'")
  if echo "$CURRENT_PORT" | grep -q "8080"; then
    info "LuCI already on port 8080 — skipping."
  else
    info "Moving LuCI to port 8080/8443..."
    ssh_cmd "uci set uhttpd.main.listen_http='0.0.0.0:8080' && \
             uci set uhttpd.main.listen_https='0.0.0.0:8443' && \
             uci commit uhttpd && \
             /etc/init.d/uhttpd restart 2>/dev/null || true"
    echo -e "${GREEN}✓${NC} LuCI moved to port 8080."
  fi
fi

# Step 2: Upload init.d service script
info "Uploading init.d service script..."
scp_cmd "$INITD_SRC" "${REMOTE}:/etc/init.d/openwrt-travel-gui"
ssh_cmd "chmod +x /etc/init.d/openwrt-travel-gui"

# Step 3: Upload UCI config (with configured password)
info "Uploading UCI config (password: ${PASSWORD})..."
# Write config to a temp file with the configured password substituted in
TMPCONFIG=$(mktemp)
trap 'rm -f "$TMPCONFIG"' EXIT
sed "s/option password 'admin'/option password '${PASSWORD}'/" "$CONFIG_SRC" > "$TMPCONFIG"
scp_cmd "$TMPCONFIG" "${REMOTE}:/etc/config/openwrt-travel-gui"

# Step 4: Create web asset directory
info "Creating /www/openwrt-travel-gui..."
ssh_cmd "mkdir -p /www/openwrt-travel-gui"

# Step 5: Mark setup as complete (skip the first-run wizard)
info "Marking setup as complete..."
ssh_cmd "mkdir -p /etc/openwrt-travel-gui && touch /etc/openwrt-travel-gui/setup-complete"

# Step 6: Enable service (starts on next boot or manual start)
info "Enabling service..."
ssh_cmd "/etc/init.d/openwrt-travel-gui enable"

echo ""
echo -e "${GREEN}✓${NC} Device setup complete!"
echo "  LuCI:       http://${ROUTER_IP}:8080/"
echo ""
echo "  Next step: deploy the app with:"
echo "    ./scripts/deploy-local.sh --build"
echo ""
