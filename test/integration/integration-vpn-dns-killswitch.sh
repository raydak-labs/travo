#!/bin/bash
#
# integration-vpn-dns-killswitch.sh — live router API tests: DNS leak endpoint, VPN kill switch.
# Optional SSH read-only UCI snapshot when --ssh-verify.
#
# Usage:
#   ./test/integration/integration-vpn-dns-killswitch.sh [options]
#
# Options:
#   --ip IP                 Router (default: 192.168.1.1)
#   --login-password PASS   Travo UI / API password (default: admin)
#   --ssh-verify            After HTTP checks, SSH as root and print dnsmasq / wg / killswitch UCI
#   --enable-killswitch     Briefly enable kill switch via API, verify, then disable (use with care)
#   -h, --help              Print usage
#
# Environment:
#   (none)
#
# Exit codes:
#   0  success
#   2  assertion / check failure
#
# Artifacts:
#   tmp/integration-vpn-dns-killswitch-<timestamp>/logs/
#
set -euo pipefail

ROUTER_IP="192.168.1.1"
LOGIN_PASSWORD="admin"
SSH_VERIFY=false
TOGGLE_KILLSWITCH=false

SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Runs authenticated API calls against the travel-gui backend on a live OpenWrt router.

Options:
  --ip IP                 Router IP (default: 192.168.1.1)
  --login-password PASS   App login password (default: admin)
  --ssh-verify            After API tests, SSH and print dnsmasq/wg0/killswitch UCI (read-only)
  --enable-killswitch     PUT kill switch on, verify GET + UCI, then turn off (use with care)
  -h, --help              Show this help

Examples:
  $(basename "$0")
  $(basename "$0") --ssh-verify --login-password '\$MY_PASS'
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ip) ROUTER_IP="$2"; shift 2 ;;
    --login-password) LOGIN_PASSWORD="$2"; shift 2 ;;
    --ssh-verify) SSH_VERIFY=true; shift ;;
    --enable-killswitch) TOGGLE_KILLSWITCH=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

TS="$(date +%Y%m%d-%H%M%S)"
RUN_DIR="tmp/integration-vpn-dns-killswitch-$TS"
mkdir -p "$RUN_DIR/logs"

REMOTE="root@${ROUTER_IP}"

ssh_cmd() {
  ssh $SSH_OPTS "$REMOTE" "$@"
}

echo "Run dir: $RUN_DIR"
echo "Router:  http://${ROUTER_IP}/"

echo "== Auth login =="
LOGIN_JSON="$(
  curl -sS -m 10 -X POST "http://${ROUTER_IP}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"password\":\"${LOGIN_PASSWORD}\"}"
)"
echo "$LOGIN_JSON" > "$RUN_DIR/logs/login.json"
TOKEN="$(python3 -c "import json,sys; print(json.load(open('$RUN_DIR/logs/login.json')).get('token',''))")"
if [[ -z "$TOKEN" ]]; then
  echo "Login failed. See $RUN_DIR/logs/login.json" >&2
  exit 2
fi

echo "== GET /api/v1/vpn/dns-leak-test =="
DNS_JSON="$(
  curl -sS -m 15 "http://${ROUTER_IP}/api/v1/vpn/dns-leak-test" \
    -H "Authorization: Bearer $TOKEN"
)"
echo "$DNS_JSON" > "$RUN_DIR/logs/dns-leak-test.json"
python3 -m json.tool "$RUN_DIR/logs/dns-leak-test.json"

echo "== GET /api/v1/vpn/killswitch =="
KS_JSON="$(
  curl -sS -m 10 "http://${ROUTER_IP}/api/v1/vpn/killswitch" \
    -H "Authorization: Bearer $TOKEN"
)"
echo "$KS_JSON" | python3 -m json.tool | tee "$RUN_DIR/logs/killswitch-get.json"

DNS_OK=false
KS_OK=false

# DNS leak: JSON must be valid; vpn_dns_servers must list separate IPs (comma-separated UCI split in backend).
if python3 - "$RUN_DIR/logs/dns-leak-test.json" <<'PY'
import json, sys
path = sys.argv[1]
with open(path) as f:
    j = json.load(f)
assert "nameservers" in j and "vpn_dns_servers" in j
assert "vpn_active" in j and "potentially_leaking" in j
for s in j.get("vpn_dns_servers") or []:
    assert "," not in s, "vpn_dns_servers should list separate IPs, got %r" % (j["vpn_dns_servers"],)
PY
then
  DNS_OK=true
fi

echo "$KS_JSON" > "$RUN_DIR/logs/killswitch-get-raw.json"
if python3 - "$RUN_DIR/logs/killswitch-get-raw.json" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    j = json.load(f)
assert "enabled" in j and isinstance(j["enabled"], bool)
PY
then
  KS_OK=true
fi

if $SSH_VERIFY; then
  echo "== SSH: UCI snapshot (read-only) =="
  ssh_cmd '
    echo "=== network.wg0 (head) ==="
    uci show network.wg0 2>/dev/null | head -15 || true
    echo "=== dhcp dnsmasq server ==="
    uci -q get dhcp.@dnsmasq[0].server 2>/dev/null || true
    echo "=== firewall vpn_killswitch ==="
    uci show firewall.vpn_killswitch 2>/dev/null || echo "(none)"
  ' | tee "$RUN_DIR/logs/ssh-uci.txt" || true
fi

KILLSWITCH_CYCLE_OK=true
if $TOGGLE_KILLSWITCH; then
  echo "== PUT kill switch enable -> verify -> disable =="
  curl -sS -m 30 -X PUT "http://${ROUTER_IP}/api/v1/vpn/killswitch" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"enabled":true}' | tee "$RUN_DIR/logs/killswitch-on.json"
  sleep 1
  KS_ON="$(curl -sS -m 10 "http://${ROUTER_IP}/api/v1/vpn/killswitch" -H "Authorization: Bearer $TOKEN")"
  echo "$KS_ON" | python3 -m json.tool
  if ! echo "$KS_ON" | python3 -c "import json,sys; j=json.load(sys.stdin); assert j.get('enabled') is True" 2>/dev/null; then
    KILLSWITCH_CYCLE_OK=false
  fi
  if $SSH_VERIFY || $TOGGLE_KILLSWITCH; then
    ssh_cmd 'uci show firewall.vpn_killswitch' | tee "$RUN_DIR/logs/killswitch-uci-on.txt" || true
  fi
  curl -sS -m 30 -X PUT "http://${ROUTER_IP}/api/v1/vpn/killswitch" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"enabled":false}' | tee "$RUN_DIR/logs/killswitch-off.json"
  sleep 1
  KS_OFF="$(curl -sS -m 10 "http://${ROUTER_IP}/api/v1/vpn/killswitch" -H "Authorization: Bearer $TOKEN")"
  echo "$KS_OFF" | python3 -m json.tool
  if ! echo "$KS_OFF" | python3 -c "import json,sys; j=json.load(sys.stdin); assert j.get('enabled') is False" 2>/dev/null; then
    KILLSWITCH_CYCLE_OK=false
  fi
fi

OVERALL="FAIL"
if $DNS_OK && $KS_OK && $KILLSWITCH_CYCLE_OK; then
  OVERALL="PASS"
fi

json_bool() {
  if "$1"; then echo true; else echo false; fi
}

cat > "$RUN_DIR/result.json" <<EOF
{
  "status": "$OVERALL",
  "router_ip": "$ROUTER_IP",
  "checks": {
    "dns_leak_api_ok": $(json_bool "$DNS_OK"),
    "killswitch_get_ok": $(json_bool "$KS_OK"),
    "killswitch_cycle_ok": $(json_bool "$KILLSWITCH_CYCLE_OK")
  }
}
EOF

echo "== Result =="
echo "$OVERALL"
echo "Artifacts: $RUN_DIR"

if [[ "$OVERALL" != "PASS" ]]; then
  exit 2
fi
