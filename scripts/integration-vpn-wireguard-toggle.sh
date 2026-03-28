#!/bin/bash
# Real-device integration test: WireGuard enable -> disable must keep internet working.

set -euo pipefail

ROUTER_IP="192.168.1.1"
LOGIN_PASSWORD="admin"

SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --ip IP                 Router IP (default: 192.168.1.1)
  --login-password PASS   App login password (default: admin)
  -h, --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ip) ROUTER_IP="$2"; shift 2 ;;
    --login-password) LOGIN_PASSWORD="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

TS="$(date +%Y%m%d-%H%M%S)"
RUN_DIR="tmp/integration-vpn-wireguard-toggle-$TS"
mkdir -p "$RUN_DIR"/{0-before,1-enabled,2-disabled,logs}

REMOTE="root@${ROUTER_IP}"

ssh_cmd() {
  ssh $SSH_OPTS "$REMOTE" "$@"
}

extract_json_field() {
  local key="$1"
  sed -n "s/.*\"$key\":\"\\([^\"]*\\)\".*/\\1/p"
}

router_connectivity_check() {
  # Use set +e to capture exit status in output even when it fails.
  ssh_cmd 'set -u
    set +e
    wget -qO- --timeout=8 http://connectivitycheck.gstatic.com/generate_204 >/dev/null
    code=$?
    echo "wget_exit_code=$code"
    exit 0
  '
}

snap() {
  local label="$1"
  ssh_cmd 'set -u
    echo "=== date ==="; date
    echo "=== uci wg0 disabled ==="; uci -q get network.wg0.disabled || true
    echo "=== ip -4 route main ==="; ip -4 route show table main || true
    echo "=== ip -4 route default ==="; ip -4 route show default || true
    echo "=== ip rule ==="; ip rule show || true
    echo "=== ifstatus wwan ==="; ifstatus wwan 2>/dev/null || true
    echo "=== ifstatus wan ==="; ifstatus wan 2>/dev/null || true
    echo "=== ifstatus wg0 ==="; ifstatus wg0 2>/dev/null || true
    echo "=== wg show ==="; wg show 2>/dev/null || true
  ' > "$RUN_DIR/$label/snapshot.txt" || true
}

echo "Run dir: $RUN_DIR"
echo "Router:  $REMOTE"

echo "== Auth login =="
LOGIN_JSON="$(
  curl -sS -m 10 -X POST "http://${ROUTER_IP}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"password\":\"${LOGIN_PASSWORD}\"}"
)"
echo "$LOGIN_JSON" > "$RUN_DIR/logs/login.json"
TOKEN="$(printf '%s' "$LOGIN_JSON" | extract_json_field token)"
if [[ -z "$TOKEN" ]]; then
  echo "Login failed. See $RUN_DIR/logs/login.json" >&2
  exit 1
fi

echo "== Force WireGuard off =="
curl -sS -m 30 -X POST "http://${ROUTER_IP}/api/v1/vpn/wireguard/toggle" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"enabled":false}' \
  > "$RUN_DIR/logs/force-off.json" || true
sleep 2

echo "== Baseline snapshot =="
snap 0-before
router_connectivity_check > "$RUN_DIR/0-before/connectivity.txt"

echo "== Enable WireGuard =="
curl -sS -m 45 -X POST "http://${ROUTER_IP}/api/v1/vpn/wireguard/toggle" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"enabled":true}' \
  > "$RUN_DIR/logs/enable.json"
sleep 4
snap 1-enabled
router_connectivity_check > "$RUN_DIR/1-enabled/connectivity.txt"

echo "== Disable WireGuard =="
curl -sS -m 45 -X POST "http://${ROUTER_IP}/api/v1/vpn/wireguard/toggle" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"enabled":false}' \
  > "$RUN_DIR/logs/disable.json"
sleep 4
snap 2-disabled
router_connectivity_check > "$RUN_DIR/2-disabled/connectivity.txt"

BEFORE_OK=false
ENABLED_OK=false
DISABLED_OK=false

if grep -q 'wget_exit_code=0' "$RUN_DIR/0-before/connectivity.txt"; then BEFORE_OK=true; fi
if grep -q 'wget_exit_code=0' "$RUN_DIR/1-enabled/connectivity.txt"; then ENABLED_OK=true; fi
if grep -q 'wget_exit_code=0' "$RUN_DIR/2-disabled/connectivity.txt"; then DISABLED_OK=true; fi

OVERALL="FAIL"
if $BEFORE_OK && $ENABLED_OK && $DISABLED_OK; then
  OVERALL="PASS"
fi

cat > "$RUN_DIR/result.json" <<EOF
{
  "status": "$OVERALL",
  "router_ip": "$ROUTER_IP",
  "checks": {
    "baseline_internet_ok": $BEFORE_OK,
    "wireguard_enabled_internet_ok": $ENABLED_OK,
    "wireguard_disabled_internet_ok": $DISABLED_OK
  }
}
EOF

echo "== Result =="
echo "$OVERALL"
echo "Artifacts: $RUN_DIR"

if [[ "$OVERALL" != "PASS" ]]; then
  exit 2
fi

