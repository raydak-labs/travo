#!/bin/bash
#
# integration-device.sh — real-device smoke: baseline, WiFi disconnect/reconnect, online checks.
# Run from repo root; requires router API + SSH.
#
# Usage:
#   ./test/integration/integration-device.sh [options]
#
# Options:
#   --ip IP                 Router (default: 192.168.1.1)
#   --user USER             SSH user (default: root)
#   --login-password PASS   Travo UI / API password (default: admin)
#   --wifi-ssid SSID        Upstream/client test SSID (default: Cappuxinno)
#   --wifi-pass-file PATH   File with WPA secret for that SSID (default: test/integration/.wifi_pass)
#   -h, --help              Print usage
#
# Environment:
#   (none; use flags)
#
# Artifacts:
#   tmp/integration-<timestamp>/  logs and result.json
#
set -euo pipefail

ROUTER_IP="192.168.1.1"
ROUTER_USER="root"
LOGIN_PASSWORD="admin"
WIFI_SSID="Cappuxinno"
WIFI_PASS_FILE="test/integration/.wifi_pass"

SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --ip IP                 Router IP (default: 192.168.1.1)
  --user USER             SSH user (default: root)
  --login-password PASS   App login password (default: admin)
  --wifi-ssid SSID        Target upstream SSID (default: Cappuxinno)
  --wifi-pass-file PATH   File containing SSID password (default: test/integration/.wifi_pass)
  -h, --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ip) ROUTER_IP="$2"; shift 2 ;;
    --user) ROUTER_USER="$2"; shift 2 ;;
    --login-password) LOGIN_PASSWORD="$2"; shift 2 ;;
    --wifi-ssid) WIFI_SSID="$2"; shift 2 ;;
    --wifi-pass-file) WIFI_PASS_FILE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ ! -f "$WIFI_PASS_FILE" ]]; then
  echo "WiFi password file not found: $WIFI_PASS_FILE" >&2
  exit 1
fi

WIFI_PASS="$(tr -d '\n' < "$WIFI_PASS_FILE")"
if [[ -z "$WIFI_PASS" ]]; then
  echo "WiFi password is empty in $WIFI_PASS_FILE" >&2
  exit 1
fi

TS="$(date +%Y%m%d-%H%M%S)"
RUN_DIR="tmp/integration-$TS"
mkdir -p "$RUN_DIR"/{before,after,logs}

REMOTE="${ROUTER_USER}@${ROUTER_IP}"

ssh_cmd() {
  ssh $SSH_OPTS "$REMOTE" "$@"
}

http_code_or_000() {
  local url="$1"
  curl -sS -m 10 -o /dev/null -w "%{http_code}\n" "$url" || echo "000"
}

extract_json_field() {
  local key="$1"
  sed -n "s/.*\"$key\":\"\\([^\"]*\\)\".*/\\1/p"
}

echo "Run dir: $RUN_DIR"
echo "Router: $REMOTE"

echo "== Baseline =="
ssh_cmd '
  echo "=== date ==="; date
  echo "=== release ==="; (cat /etc/openwrt_release 2>/dev/null || cat /etc/os-release 2>/dev/null)
  echo "=== uname ==="; uname -a
  echo "=== df -h ==="; df -h
  echo "=== travel-gui status ==="; /etc/init.d/travo status 2>/dev/null || true
  echo "=== uhttpd ports ==="; uci get uhttpd.main.listen_http; uci get uhttpd.main.listen_https
' > "$RUN_DIR/before/system.txt"

ssh_cmd '
  uci show network
  uci show wireless
  uci show firewall
  uci show dhcp
  uci show travo 2>/dev/null || true
' > "$RUN_DIR/before/uci-show.txt"

curl -sS -m 10 -D "$RUN_DIR/before/health.headers" -o "$RUN_DIR/before/health.body" \
  "http://${ROUTER_IP}/api/health" || true
http_code_or_000 "http://${ROUTER_IP}/" > "$RUN_DIR/before/ui.http_code"
http_code_or_000 "http://${ROUTER_IP}:8080/" > "$RUN_DIR/before/luci.http_code"

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

echo "== WiFi disconnect =="
DISC_JSON="$(
  curl -sS -m 20 -X POST "http://${ROUTER_IP}/api/v1/wifi/disconnect" \
    -H "Authorization: Bearer $TOKEN"
)"
echo "$DISC_JSON" > "$RUN_DIR/logs/wifi-disconnect.json"
DISC_APPLY_TOKEN="$(printf '%s' "$DISC_JSON" | extract_json_field token)"
if [[ -n "$DISC_APPLY_TOKEN" ]]; then
  curl -sS -m 20 -X POST "http://${ROUTER_IP}/api/v1/wifi/apply/confirm" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "{\"token\":\"$DISC_APPLY_TOKEN\"}" \
    > "$RUN_DIR/logs/wifi-disconnect-confirm.json"
fi

sleep 2
ssh_cmd '
  echo "=== wireless sta sections ==="
  uci show wireless | grep -E "mode=.sta" || true
  echo "=== wwan status ==="
  ubus call network.interface.wwan status 2>/dev/null || true
' > "$RUN_DIR/logs/sta-after-disconnect.txt"

ssh_cmd '
  wget -qO- --timeout=8 http://connectivitycheck.gstatic.com/generate_204 >/dev/null
  echo "exit_code=$?"
' > "$RUN_DIR/logs/offline-check.txt" || true

echo "== WiFi connect to $WIFI_SSID =="
CONN_JSON="$(
  curl -sS -m 30 -X POST "http://${ROUTER_IP}/api/v1/wifi/connect" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "{\"ssid\":\"${WIFI_SSID}\",\"password\":\"${WIFI_PASS}\",\"encryption\":\"psk2\"}"
)"
echo "$CONN_JSON" > "$RUN_DIR/logs/wifi-connect-response.json"
CONN_APPLY_TOKEN="$(printf '%s' "$CONN_JSON" | extract_json_field token)"
if [[ -n "$CONN_APPLY_TOKEN" ]]; then
  curl -sS -m 20 -X POST "http://${ROUTER_IP}/api/v1/wifi/apply/confirm" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "{\"token\":\"$CONN_APPLY_TOKEN\"}" \
    > "$RUN_DIR/logs/wifi-connect-confirm.json"
fi

sleep 10

ssh_cmd '
  echo "=== wwan ubus ==="
  ubus call network.interface.wwan status 2>/dev/null || true
  echo "=== routes ==="
  ip route
' > "$RUN_DIR/logs/wwan-status-after.txt"

ssh_cmd '
  wget -qO- --timeout=8 http://connectivitycheck.gstatic.com/generate_204 >/dev/null
  echo "exit_code=$?"
' > "$RUN_DIR/logs/online-check.txt" || true

curl -sS -m 10 -H "Authorization: Bearer $TOKEN" \
  "http://${ROUTER_IP}/api/v1/network/status" > "$RUN_DIR/logs/network-status-after.json"
curl -sS -m 10 -H "Authorization: Bearer $TOKEN" \
  "http://${ROUTER_IP}/api/v1/wifi/connection" > "$RUN_DIR/logs/wifi-connection-after.json"

echo "== After snapshot =="
ssh_cmd '
  echo "=== date ==="; date
  echo "=== status ==="; /etc/init.d/travo status 2>/dev/null || true
  echo "=== uhttpd ports ==="; uci get uhttpd.main.listen_http; uci get uhttpd.main.listen_https
' > "$RUN_DIR/after/system.txt"

ssh_cmd '
  uci show network
  uci show wireless
  uci show firewall
  uci show dhcp
  uci show travo 2>/dev/null || true
' > "$RUN_DIR/after/uci-show.txt"

diff -u "$RUN_DIR/before/uci-show.txt" "$RUN_DIR/after/uci-show.txt" > "$RUN_DIR/uci.diff" || true
ssh_cmd 'logread -e travo -l 500' > "$RUN_DIR/after/logread-travo.txt"

HEALTH_OK=false
UI_OK=false
LUCI_OK=false
ONLINE_OK=false
WIFI_CONNECTED=false

if rg -q '"status":"ok"' "$RUN_DIR/before/health.body"; then HEALTH_OK=true; fi
if [[ "$(tr -d '\n' < "$RUN_DIR/before/ui.http_code")" == "200" ]]; then UI_OK=true; fi
if [[ "$(tr -d '\n' < "$RUN_DIR/before/luci.http_code")" == "200" ]]; then LUCI_OK=true; fi
if rg -q 'exit_code=0' "$RUN_DIR/logs/online-check.txt"; then ONLINE_OK=true; fi
if rg -q '"connected":true' "$RUN_DIR/logs/wifi-connection-after.json"; then WIFI_CONNECTED=true; fi

OVERALL="FAIL"
if $HEALTH_OK && $UI_OK && $LUCI_OK && $ONLINE_OK && $WIFI_CONNECTED; then
  OVERALL="PASS"
fi

cat > "$RUN_DIR/result.json" <<EOF
{
  "status": "$OVERALL",
  "router_ip": "$ROUTER_IP",
  "ssid": "$WIFI_SSID",
  "checks": {
    "health_ok": $HEALTH_OK,
    "ui_http_200": $UI_OK,
    "luci_http_200": $LUCI_OK,
    "online_check_exit_0": $ONLINE_OK,
    "wifi_connected_true": $WIFI_CONNECTED
  }
}
EOF

echo "== Result =="
echo "$OVERALL"
echo "Artifacts: $RUN_DIR"
echo "Summary:   $RUN_DIR/result.json"

if [[ "$OVERALL" != "PASS" ]]; then
  exit 2
fi

