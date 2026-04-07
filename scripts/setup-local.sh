#!/bin/bash
# Initial one-time setup of a fresh OpenWRT device for local development.
#
# Run this once on a clean device before using deploy-local.sh.
# It is idempotent — safe to run again without breaking anything.
#
# What it does:
#   1. Moves LuCI (uhttpd) to port 8080 so travel-gui can use port 80
#   2. Uploads the init.d service script
#   3. Uploads the UCI config (port, etc.) and sets root password on the device
#   4. Creates /www/travo
#   5. Ensures initial wireless AP state (radios + default_radio0/1, default SSID/key)
#   6. Enables the service (does not start — no binary yet)
#
# Usage:
#   ./scripts/setup-local.sh [options]
#
# Examples:
#   ./scripts/setup-local.sh
#   ./scripts/setup-local.sh --ip 10.0.0.1 --password mysecret
#   ./scripts/setup-local.sh --wifi-ssid MyRoute --wifi-password 'same-for-all-aps'
#   ./scripts/setup-local.sh --no-luci-move

set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────────────
ROUTER_IP="192.168.1.1"
ROUTER_USER="root"
PASSWORD="admin"
MOVE_LUCI=true
LEGACY_SCP=true
INTERACTIVE_MODE="auto"
INSTALL_SSH_KEY="auto"
SSH_KEY_PATH="${HOME}/.ssh/id_ed25519.pub"
SSH_KEY_VALUE=""
# Optional WiFi AP naming (if unset, setup-wireless-ap.sh keeps OpenWrt-Travel / OpenWrt-Travel-5G + default key)
WIFI_SSID_BASE=""
WIFI_AP_KEY=""
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
  --password PASSWORD  Root password for LuCI and Travo login (default: admin)
  --wifi-ssid NAME     Base SSID for APs: 2.4 GHz = NAME, 5 GHz = NAME-5G (default: OpenWrt-Travel / OpenWrt-Travel-5G)
  --wifi-password KEY  WPA2 key for all APs (default: travelrouter from setup-wireless-ap.sh)
  --ssh-key PATH       Public SSH key path to append to /etc/dropbear/authorized_keys (default: ~/.ssh/id_ed25519.pub)
  --ssh-key-value KEY  Public SSH key text to append directly to authorized_keys
  --no-ssh-key         Skip installing a public SSH key on the router
  --no-luci-move       Skip moving LuCI to port 8080
  --interactive        Prompt for setup values before applying changes
  --no-interactive     Disable prompts and use flags/defaults only
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
    --wifi-ssid)     WIFI_SSID_BASE="$2"; shift 2 ;;
    --wifi-password) WIFI_AP_KEY="$2"; shift 2 ;;
    --ssh-key)       SSH_KEY_PATH="$2"; INSTALL_SSH_KEY=true; shift 2 ;;
    --ssh-key-value) SSH_KEY_VALUE="$2"; INSTALL_SSH_KEY=true; shift 2 ;;
    --no-ssh-key)    INSTALL_SSH_KEY=false; shift ;;
    --no-luci-move)  MOVE_LUCI=false; shift ;;
    --interactive)   INTERACTIVE_MODE=true; shift ;;
    --no-interactive) INTERACTIVE_MODE=false; shift ;;
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

is_interactive_session() {
  [[ -t 0 && -t 1 ]]
}

prompt_with_default() {
  local prompt="$1"
  local default_value="$2"
  local result
  read -r -p "$prompt [$default_value]: " result
  printf '%s' "${result:-$default_value}"
}

prompt_optional() {
  local prompt="$1"
  local default_value="$2"
  local result
  read -r -p "$prompt [$default_value]: " result
  printf '%s' "${result:-$default_value}"
}

prompt_yes_no() {
  local prompt="$1"
  local default_answer="$2"
  local reply
  local suffix="[y/N]"
  if [[ "$default_answer" == "y" ]]; then
    suffix="[Y/n]"
  fi

  while true; do
    read -r -p "$prompt $suffix: " reply
    reply="${reply:-$default_answer}"
    case "$reply" in
      y|Y|yes|YES) return 0 ;;
      n|N|no|NO) return 1 ;;
    esac
  done
}

prompt_secret() {
  local prompt="$1"
  local current_value="$2"
  local first=""
  local second=""

  if [[ -n "$current_value" ]]; then
    if prompt_yes_no "$prompt (keep current value?)" "y"; then
      printf '%s' "$current_value"
      return 0
    fi
  fi

  while true; do
    read -r -s -p "$prompt: " first
    echo
    read -r -s -p "Confirm password: " second
    echo

    if [[ "$first" != "$second" ]]; then
      warn "Passwords did not match. Try again."
      continue
    fi

    if [[ -z "$first" ]]; then
      warn "Password cannot be empty."
      continue
    fi

    printf '%s' "$first"
    return 0
  done
}

configure_interactive_defaults() {
  local interactive=false
  case "$INTERACTIVE_MODE" in
    true) interactive=true ;;
    false) interactive=false ;;
    auto)
      if is_interactive_session; then
        interactive=true
      fi
      ;;
  esac

  if [[ "$INSTALL_SSH_KEY" == "auto" ]]; then
    if [[ -n "$SSH_KEY_VALUE" || -f "$SSH_KEY_PATH" ]]; then
      INSTALL_SSH_KEY=true
    else
      INSTALL_SSH_KEY=false
    fi
  fi

  if [[ "$interactive" != "true" ]]; then
    return 0
  fi

  echo "Interactive setup"
  echo "Press Enter to keep the current value shown in brackets."
  echo ""

  ROUTER_IP=$(prompt_with_default "Router IP" "$ROUTER_IP")
  ROUTER_USER=$(prompt_with_default "SSH user" "$ROUTER_USER")
  PASSWORD=$(prompt_secret "Root password for LuCI / Travo" "$PASSWORD")
  WIFI_SSID_BASE=$(prompt_optional "WiFi SSID base (blank uses setup-wireless defaults)" "$WIFI_SSID_BASE")
  WIFI_AP_KEY=$(prompt_optional "WiFi password (blank uses setup-wireless default)" "$WIFI_AP_KEY")

  if prompt_yes_no "Move LuCI to ports 8080/8443?" "$([[ "$MOVE_LUCI" == "true" ]] && echo y || echo n)"; then
    MOVE_LUCI=true
  else
    MOVE_LUCI=false
  fi

  if prompt_yes_no "Use legacy SCP mode for Dropbear?" "$([[ "$LEGACY_SCP" == "true" ]] && echo y || echo n)"; then
    LEGACY_SCP=true
  else
    LEGACY_SCP=false
  fi

  if [[ -n "$SSH_KEY_VALUE" ]]; then
    if prompt_yes_no "Install the provided SSH key value on the router?" "$([[ "$INSTALL_SSH_KEY" == "true" ]] && echo y || echo n)"; then
      INSTALL_SSH_KEY=true
    else
      INSTALL_SSH_KEY=false
    fi
  elif [[ -f "$SSH_KEY_PATH" ]]; then
    SSH_KEY_PATH=$(prompt_with_default "SSH public key to install" "$SSH_KEY_PATH")
    if prompt_yes_no "Install this SSH key on the router?" "$([[ "$INSTALL_SSH_KEY" == "true" ]] && echo y || echo n)"; then
      INSTALL_SSH_KEY=true
    else
      INSTALL_SSH_KEY=false
    fi
  else
    warn "Default SSH public key not found at $SSH_KEY_PATH"
    if prompt_yes_no "Provide a different SSH public key path?" "n"; then
      SSH_KEY_PATH=$(prompt_with_default "SSH public key path" "$SSH_KEY_PATH")
      if [[ -f "$SSH_KEY_PATH" ]] && prompt_yes_no "Install this SSH key on the router?" "y"; then
        INSTALL_SSH_KEY=true
      else
        INSTALL_SSH_KEY=false
      fi
    else
      INSTALL_SSH_KEY=false
    fi
  fi
}

get_ssh_key_content() {
  if [[ -n "$SSH_KEY_VALUE" ]]; then
    printf '%s' "$SSH_KEY_VALUE"
    return 0
  fi

  [[ -f "$SSH_KEY_PATH" ]] || error "SSH public key not found at $SSH_KEY_PATH"
  [[ -s "$SSH_KEY_PATH" ]] || error "SSH public key file is empty: $SSH_KEY_PATH"
  <"$SSH_KEY_PATH" tr -d '\n'
}

install_ssh_key() {
  local ssh_key_content
  local ssh_key_source

  if [[ "$INSTALL_SSH_KEY" != "true" ]]; then
    return 0
  fi

  ssh_key_content="$(get_ssh_key_content)"
  [[ -n "$ssh_key_content" ]] || error "SSH public key is empty"

  if [[ -n "$SSH_KEY_VALUE" ]]; then
    ssh_key_source="provided value"
  else
    ssh_key_source="$SSH_KEY_PATH"
  fi

  info "Installing SSH public key from $ssh_key_source..."
  ssh_cmd "mkdir -p /etc/dropbear && touch /etc/dropbear/authorized_keys && chmod 600 /etc/dropbear/authorized_keys"
  if ssh_cmd "grep -qxF $(printf '%q' "$ssh_key_content") /etc/dropbear/authorized_keys" >/dev/null 2>&1; then
    info "SSH public key already present — skipping."
    return 0
  fi

  if ! printf '%s\n' "$ssh_key_content" | ssh $SSH_OPTS "${REMOTE}" "tee -a /etc/dropbear/authorized_keys >/dev/null"; then
    error "Failed to install SSH public key on ${REMOTE}"
  fi
}

# ── Paths ─────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
INITD_SRC="${REPO_ROOT}/packaging/openwrt/files/etc/init.d/travo"
CONFIG_SRC="${REPO_ROOT}/packaging/openwrt/files/etc/config/travo"

[[ -f "$INITD_SRC" ]] || error "init.d script not found at $INITD_SRC"
[[ -f "$CONFIG_SRC" ]] || error "UCI config not found at $CONFIG_SRC"

configure_interactive_defaults
REMOTE="${ROUTER_USER}@${ROUTER_IP}"

# ── Main ──────────────────────────────────────────────────────────────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Setup travo → ${REMOTE}"
echo "  LuCI move: ${MOVE_LUCI}  |  Travel GUI password: (hidden)"
if [[ -n "$WIFI_SSID_BASE" ]]; then
  echo "  WiFi AP SSID base: ${WIFI_SSID_BASE} (5G: ${WIFI_SSID_BASE}-5G)"
else
  echo "  WiFi AP SSID: default (OpenWrt-Travel / OpenWrt-Travel-5G)"
fi
if [[ -n "$WIFI_AP_KEY" ]]; then
  echo "  WiFi AP key: (custom)"
else
  echo "  WiFi AP key: default (travelrouter)"
fi
if [[ "$INSTALL_SSH_KEY" == "true" ]]; then
  if [[ -n "$SSH_KEY_VALUE" ]]; then
    echo "  SSH public key: provided directly"
  else
    echo "  SSH public key: ${SSH_KEY_PATH}"
  fi
else
  echo "  SSH public key: skipped"
fi
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

# Step 0b: Install SSH key for future access
install_ssh_key

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
scp_cmd "$INITD_SRC" "${REMOTE}:/etc/init.d/travo"
ssh_cmd "chmod +x /etc/init.d/travo"

# Step 3: Upload UCI config
info "Uploading UCI config..."
scp_cmd "$CONFIG_SRC" "${REMOTE}:/etc/config/travo"

# Step 3b: Root password (same credential as LuCI / Travo API)
info "Setting root password on device..."
if ! printf '%s\n%s\n' "$PASSWORD" "$PASSWORD" | ssh $SSH_OPTS "${REMOTE}" 'passwd root' 2>/dev/null; then
  warn "Could not set root password via SSH — run: ssh ${REMOTE} 'passwd root'"
fi

# Step 4: Create web asset directory
info "Creating /www/travo and /etc/travo..."
ssh_cmd "mkdir -p /www/travo /etc/travo && chmod 700 /etc/travo"

# Step 5: Initial wireless AP state (radios + default_radio0 / default_radio1, no STA changes)
info "Ensuring initial wireless AP state..."
WIRELESS_SCRIPT="${SCRIPT_DIR}/setup-wireless-ap.sh"
if [[ -f "$WIRELESS_SCRIPT" ]]; then
  # Try to get session via LuCI HTTP RPC (works when ubus session login is not available on device)
  UCI_SID=""
  LUCI_AUTH=$(curl -s -X POST "http://${ROUTER_IP}:8080/cgi-bin/luci/rpc/auth" \
    -H "Content-Type: application/json" \
    -d '{"id":1,"method":"login","params":["root",""]}' \
    --connect-timeout 3 2>/dev/null) || true
  if echo "$LUCI_AUTH" | grep -q '"result":"[^"]*"'; then
    UCI_SID=$(echo "$LUCI_AUTH" | sed -n 's/.*"result":"\([^"]*\)".*/\1/p' | head -1)
  fi
  scp_cmd "$WIRELESS_SCRIPT" "${REMOTE}:/tmp/setup-wireless-ap.sh"
  WIRELESS_ENV=""
  if [[ -n "$WIFI_SSID_BASE" ]]; then
    WIRELESS_ENV+="SETUP_WIFI_SSID=$(printf '%q' "$WIFI_SSID_BASE") "
  fi
  if [[ -n "$WIFI_AP_KEY" ]]; then
    WIRELESS_ENV+="SETUP_WIFI_KEY=$(printf '%q' "$WIFI_AP_KEY") "
  fi
  WIRELESS_OUT=$(ssh_cmd "${WIRELESS_ENV}UCI_APPLY_SID=${UCI_SID:-} sh /tmp/setup-wireless-ap.sh 2>&1; rm -f /tmp/setup-wireless-ap.sh" || true)
  echo "$WIRELESS_OUT" | grep -E '^\[setup-wireless\]' || true
  if echo "$WIRELESS_OUT" | grep -q 'UCI_APPLY_SESSION='; then
    UCI_SID=$(echo "$WIRELESS_OUT" | grep 'UCI_APPLY_SESSION=' | tail -1 | sed 's/^.*UCI_APPLY_SESSION=//')
  fi
  if [[ -n "$UCI_SID" ]]; then
    info "Sending uci confirm (rollback window ~25s)..."
    _confirmed=false
    for _i in $(seq 1 13); do
      sleep 2
      if ssh_cmd "ubus call uci confirm '{\"ubus_rpc_session\":\"$UCI_SID\"}'" 2>/dev/null; then
        _confirmed=true
        break
      fi
    done
    if [[ "$_confirmed" != "true" ]]; then
      warn "Could not confirm within window; device may roll back wireless config in a few seconds."
    fi
  else
    echo -e "${YELLOW}  To bring WiFi up: use LuCI → Network → Wireless → Save & Apply, or reboot the device.${NC}"
  fi
  info "Verifying AP entries on device..."
  if ! ssh_cmd "uci get wireless.default_radio0.ssid >/dev/null 2>&1 && uci get wireless.default_radio1.ssid >/dev/null 2>&1"; then
    error "Wireless AP verification failed: default_radio0 or default_radio1 missing or empty on device. Check wireless config (e.g. ssh ${REMOTE} 'uci show wireless')."
  fi
else
  warn "setup-wireless-ap.sh not found — skipping wireless AP setup."
fi

info "Attended Sysupgrade: set login_check_for_upgrades=1 (best effort)..."
ssh_cmd "uci set attendedsysupgrade.client.login_check_for_upgrades='1' 2>/dev/null && uci commit attendedsysupgrade 2>/dev/null || true"

# Step 6: Mark setup as complete (skip the first-run wizard)
info "Marking setup as complete..."
ssh_cmd "mkdir -p /etc/travo && touch /etc/travo/setup-complete"

# Step 7: Enable service (starts on next boot or manual start)
info "Enabling service..."
ssh_cmd "/etc/init.d/travo enable"

echo ""
echo -e "${GREEN}✓${NC} Device setup complete!"
echo "  LuCI:       http://${ROUTER_IP}:8080/"
echo ""
echo "  Next step: deploy the app with:"
echo "    ./scripts/deploy-local.sh              # fast: binary + frontend only"
echo "    ./scripts/deploy-local.sh --method release  # same files as a release tarball → /"
echo ""
