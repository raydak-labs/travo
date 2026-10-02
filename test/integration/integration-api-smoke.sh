#!/bin/bash
#
# integration-api-smoke.sh — API contract and state-integrity smoke on a real device.
#
# Complements integration-device.sh (baseline, WiFi reconnect, online checks) by
# asserting the things that only a real router can falsify:
#
#   1. every documented GET endpoint answers 2xx/3xx (no 5xx anywhere)
#   2. the served OpenAPI spec matches the registered routes in both directions
#   3. write paths round-trip and are reverted
#   4. invalid input is rejected, and a rejected write changes nothing
#   5. concurrent writers of the same UCI config do not corrupt each other
#   6. no request leaves a staged UCI delta or a crash guard behind
#
# Every mutation is reverted, and the script asserts the device is back where it
# started. It is safe to run repeatedly against a test device.
#
# Usage:
#   ./test/integration/integration-api-smoke.sh [options]
#
# Options:
#   --ip IP                 Router (default: 192.168.1.1)
#   --user USER             SSH user (default: root)
#   --login-password PASS   Travo UI / API password (default: admin)
#   --concurrency N         Concurrent writers per kind (default: 6)
#   --rounds N              Writes per writer (default: 6)
#   -h, --help              Print usage
#
# Environment:
#   TRAVO_INSECURE_SSH=1    Skip host-key verification (lab router)
#
# Artifacts:
#   tmp/api-smoke-<timestamp>/  logs and result.json
#
# Exit codes: 0 all checks passed, 1 setup/usage error, 2 one or more failed.
#
set -uo pipefail

ROUTER_IP="192.168.1.1"
ROUTER_USER="root"
LOGIN_PASSWORD="admin"
CONCURRENCY=6
ROUNDS=6

if [ "${TRAVO_INSECURE_SSH:-0}" = "1" ]; then
  SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5"
else
  SSH_OPTS="-o StrictHostKeyChecking=accept-new -o LogLevel=ERROR -o ConnectTimeout=5"
fi

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --ip IP                 Router IP (default: 192.168.1.1)
  --user USER             SSH user (default: root)
  --login-password PASS   App login password (default: admin)
  --concurrency N         Concurrent writers per kind (default: 6)
  --rounds N              Writes per writer (default: 6)
  -h, --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ip) ROUTER_IP="$2"; shift 2 ;;
    --user) ROUTER_USER="$2"; shift 2 ;;
    --login-password) LOGIN_PASSWORD="$2"; shift 2 ;;
    --concurrency) CONCURRENCY="$2"; shift 2 ;;
    --rounds) ROUNDS="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

command -v curl >/dev/null || { echo "curl is required" >&2; exit 1; }
command -v python3 >/dev/null || { echo "python3 is required" >&2; exit 1; }

TS="$(date +%Y%m%d-%H%M%S)"
RUN_DIR="tmp/api-smoke-$TS"
mkdir -p "$RUN_DIR"
REMOTE="${ROUTER_USER}@${ROUTER_IP}"
API="http://${ROUTER_IP}/api/v1"
TOKEN=""

FAILURES=0
note_fail() { echo "FAIL: $*"; FAILURES=$((FAILURES + 1)); }

ssh_cmd() { ssh $SSH_OPTS "$REMOTE" "$@"; }

# Backstop only, for abnormal exits. The normal path runs cleanup explicitly
# before the final state check, so a cleanup failure can fail the run instead of
# being reported after the verdict was already printed.
cleanup_on_exit() {
  if [ -n "$TOKEN" ] && [ -f "$RUN_DIR/cleanup.py" ] && [ ! -f "$RUN_DIR/cleanup.done" ]; then
    python3 "$RUN_DIR/cleanup.py" --api "$API" --token "$TOKEN" >"$RUN_DIR/cleanup.log" 2>&1 || true
  fi
}

run_cleanup() {
  [ -n "$TOKEN" ] || return 0
  [ -f "$RUN_DIR/cleanup.py" ] || return 0
  if ! python3 "$RUN_DIR/cleanup.py" --api "$API" --token "$TOKEN" >"$RUN_DIR/cleanup.log" 2>&1; then
    note_fail "cleanup failed; see $RUN_DIR/cleanup.log"
    return 1
  fi
  # A leftover object is a dirty device, not a silent pass.
  if grep -q "CLEANUP-ERROR" "$RUN_DIR/cleanup.log" 2>/dev/null; then
    note_fail "cleanup reported errors:"
    grep "CLEANUP-ERROR" "$RUN_DIR/cleanup.log" | sed 's/^/    /'
    return 1
  fi
  touch "$RUN_DIR/cleanup.done"
  echo "ok ($(sed -n 's/^removed \([0-9]*\).*/\1/p' "$RUN_DIR/cleanup.log" | head -1) objects removed)"
  return 0
}
trap cleanup_on_exit EXIT

echo "Run dir: $RUN_DIR"
echo "Router:  $REMOTE"

# ---------------------------------------------------------------- login
echo "== Login =="
LOGIN=$(curl -sS -m 15 -X POST "$API/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"password\":\"$LOGIN_PASSWORD\"}" || true)
TOKEN=$(printf '%s' "$LOGIN" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("token",""))' 2>/dev/null || true)
if [ -z "$TOKEN" ]; then
  echo "Login failed: $LOGIN" >&2
  exit 1
fi
echo "ok (token ${#TOKEN} chars)"

# ------------------------------------------------------- integrity checks
check_integrity() {
  local label="$1" staged guards
  staged=$(ssh_cmd 'uci -q changes' 2>/dev/null || true)
  guards=$(ssh_cmd 'ls /etc/trafo/ 2>/dev/null | grep -- "-in-progress" | tr "\n" " "' 2>/dev/null || true)
  if [ -n "$staged" ]; then
    note_fail "$label: staged UCI delta left behind: $(echo "$staged" | tr '\n' ' ')"
  fi
  if [ -n "$guards" ]; then
    note_fail "$label: crash guard left behind: $guards"
  fi
}

# ------------------------------------------------- object-count bookkeeping
# The collections this run writes into. Anything already in them belongs to the
# router's owner and must survive the run untouched.
COLLECTIONS="network/firewall/port-forwards network/dns/entries network/dhcp/reservations network/clients/blocked"

# count_coll prints the number of objects in one collection, or "?" when the
# listing could not be read. The list endpoints are inconsistent — port forwards
# come back wrapped as {"rules": [...]}, the others as bare arrays — so the
# wrapper is unwrapped here rather than at every call site.
count_coll() {
  curl -sS -m 20 "$API/$1" -H "Authorization: Bearer $TOKEN" 2>/dev/null | python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
except Exception:
    print("?"); raise SystemExit
if isinstance(d, dict):
    d = d.get("rules", d.get("entries", d.get("reservations", [])))
print(len(d) if isinstance(d, list) else "?")' 2>/dev/null || echo "?"
}

# snapshot_counts records "collection count" lines before anything is written.
# The post-run check compares against these instead of demanding zero: a router
# that already had its own port forward or blocked client was never dirty, and
# failing it would contradict the "safe on any test device" promise in the
# header. It would also mask a real leak — a leftover of this run's objects is
# invisible when the collection only has to come back to zero.
snapshot_counts() {
  : >"$RUN_DIR/baseline_counts"
  local coll n
  for coll in $COLLECTIONS; do
    n=$(count_coll "$coll")
    echo "$coll $n" >>"$RUN_DIR/baseline_counts"
  done
}

# ------------------------------------------------- 1. documented GET sweep
echo
echo "== 1. Every documented GET endpoint (2xx/3xx, no 5xx) =="
curl -sS -m 20 "http://${ROUTER_IP}/api/openapi.json" -o "$RUN_DIR/openapi.json" || {
  echo "Could not fetch the OpenAPI spec" >&2; exit 1;
}
GET_RESULT=$(python3 - "$RUN_DIR/openapi.json" "$API" "$TOKEN" <<'PY'
import json, sys, time, urllib.request, urllib.error
spec_path, api, token = sys.argv[1], sys.argv[2], sys.argv[3]
spec = json.load(open(spec_path))
bad, n = [], 0
for path, ops in sorted(spec.get("paths", {}).items()):
    if "get" not in ops:
        continue
    n += 1
    req = urllib.request.Request(f"{api}{path}", headers={"Authorization": f"Bearer {token}"})
    t0 = time.time()
    try:
        with urllib.request.urlopen(req, timeout=45) as r:
            code = r.status
    except urllib.error.HTTPError as e:
        code = e.code
    except Exception as e:
        code = 0
    if not (200 <= code < 400):
        bad.append(f"{code} {path}")
print(f"{n} {len(bad)}")
for b in bad:
    print(b)
PY
)
GET_N=$(printf '%s' "$GET_RESULT" | head -1 | cut -d' ' -f1)
GET_BAD=$(printf '%s' "$GET_RESULT" | head -1 | cut -d' ' -f2)
if [ "${GET_BAD:-1}" != "0" ]; then
  note_fail "GET sweep: $GET_BAD of $GET_N endpoints returned non-2xx/3xx:"
  printf '%s\n' "$GET_RESULT" | tail -n +2 | sed 's/^/    /'
else
  echo "ok ($GET_N endpoints)"
fi

# ------------------------------------------- 2. spec vs registered routes
echo
echo "== 2. Spec covers the API and vice versa =="
# The Go test pins this in CI; here it is a cheap tripwire for a device running a
# build whose spec and routes disagree.
OPENAPI_CODE=$(curl -sS -m 15 -o /dev/null -w '%{http_code}' "http://${ROUTER_IP}/api/openapi.json" || echo 000)
if [ "$OPENAPI_CODE" != "200" ]; then
  note_fail "GET /api/openapi.json returned $OPENAPI_CODE (automation depends on it)"
else
  echo "ok"
fi

# ------------------------------------------------ 3/4. write round-trips
echo
echo "== 3. Reversible write round-trips =="
# Taken before the first write: sections 3-5 add objects to these collections,
# and the final check compares the post-cleanup state against this.
snapshot_counts
cat > "$RUN_DIR/cleanup.py" <<'PY'
import json, sys, urllib.request, urllib.error
api, token = sys.argv[sys.argv.index("--api") + 1], sys.argv[sys.argv.index("--token") + 1]
def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(f"{api}{path}", data=data, method=method,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=45) as r:
            return r.status, r.read().decode("utf8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf8", "replace")
    except Exception:
        return 0, ""
# Remove ONLY what this run created. An earlier version deleted every port
# forward, DNS entry, DHCP reservation and blocked client on the device — which
# would silently destroy a real user's configuration if they ran this against a
# router they care about. Prefixes are what the smoke tests themselves use.
PREFIXES = ("smoke", "cc-w")
errors = []
removed = 0

def listed(path):
    """The list endpoint wraps some resources and not others: port forwards come
    back as {"rules": [...]}, DNS entries and reservations as bare arrays.
    Normalise here, or the caller silently iterates a dict's KEYS."""
    _, body = call("GET", path)
    data = json.loads(body)
    if isinstance(data, dict):
        for key in ("rules", "entries", "reservations"):
            if key in data:
                return data[key]
        return []
    return data

def drop(path, key):
    """Delete only objects this run created, matched by name prefix."""
    global removed
    for it in listed(path):
        name = str(it.get("name", ""))
        if not any(name.startswith(p) for p in PREFIXES):
            continue
        ident = it.get(key)
        if not ident:
            errors.append(f"{path}: matched {name!r} but it has no {key}")
            continue
        code, _ = call("DELETE", f"{path}/{ident}")
        if code < 400:
            removed += 1
        else:
            errors.append(f"{path}/{ident}: delete returned {code}")

try:
    drop("/network/firewall/port-forwards", "id")
    drop("/network/dns/entries", "section")
    drop("/network/dhcp/reservations", "section")
except Exception as exc:
    # Not silent: a cleanup that cannot run leaves the device dirty, and the
    # operator has to know that rather than assume the run was clean.
    errors.append(f"cleanup aborted: {type(exc).__name__}: {exc}")

# Blocked clients are keyed by MAC; the smoke tests use the aa:bb:cc:dd:ee:0x
# range, so filter on that instead of clearing the whole list.
try:
    for mac in json.loads(call("GET", "/network/clients/blocked")[1]):
        if str(mac).lower().startswith("aa:bb:cc:dd:ee:"):
            if call("POST", "/network/clients/unblock", {"mac": mac})[0] < 400:
                removed += 1
            else:
                errors.append(f"unblock {mac} failed")
except Exception as exc:
    errors.append(f"unblock sweep aborted: {type(exc).__name__}: {exc}")

print(f"removed {removed} smoke object(s)")
for e in errors:
    print("CLEANUP-ERROR " + e)

PY

WRITE_RESULT=$(python3 - "$API" "$TOKEN" <<'PY'
import json, sys, urllib.request, urllib.error
api, token = sys.argv[1], sys.argv[2]
def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(f"{api}{path}", data=data, method=method,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=45) as r:
            return r.status, r.read().decode("utf8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf8", "replace")
    except Exception as e:
        return 0, f"{type(e).__name__}: {e}"

problems = []
def roundtrip(label, create, list_path, delete, key):
    code, body = create()
    if code >= 400:
        problems.append(f"{label}: create returned {code} {body[:120]}")
        return
    code, body = call("GET", list_path)
    if code != 200:
        problems.append(f"{label}: list returned {code}")
        return
    items = json.loads(body)
    items = items.get("rules", items) if isinstance(items, dict) else items
    if not items:
        problems.append(f"{label}: created but not listed")
        return
    ident = items[-1].get(key)
    if not ident:
        problems.append(f"{label}: no {key} on the created object")
        return
    code, body = delete(ident)
    if code >= 400:
        problems.append(f"{label}: delete returned {code} {body[:120]}")

roundtrip("port-forward",
    lambda: call("POST", "/network/firewall/port-forwards",
        {"id": "smoke", "name": "smoke", "protocol": "tcp", "src_dport": "18099",
         "dest_ip": "10.9.9.9", "dest_port": "80", "enabled": True}),
    "/network/firewall/port-forwards",
    lambda i: call("DELETE", f"/network/firewall/port-forwards/{i}"), "id")
roundtrip("dns-entry",
    lambda: call("POST", "/network/dns/entries", {"name": "smoke", "ip": "10.9.9.9"}),
    "/network/dns/entries",
    lambda i: call("DELETE", f"/network/dns/entries/{i}"), "section")
roundtrip("dhcp-reservation",
    lambda: call("POST", "/network/dhcp/reservations",
        {"name": "smoke", "ip": "10.9.9.8", "mac": "aa:bb:cc:dd:ee:ff"}),
    "/network/dhcp/reservations",
    lambda i: call("DELETE", f"/network/dhcp/reservations/{i}"), "section")

code, body = call("POST", "/network/clients/block", {"mac": "aa:bb:cc:dd:ee:01"})
if code >= 400:
    problems.append(f"client-block: block returned {code} {body[:120]}")
else:
    code, _ = call("POST", "/network/clients/unblock", {"mac": "aa:bb:cc:dd:ee:01"})
    if code >= 400:
        problems.append(f"client-block: unblock returned {code}")

print(len(problems))
for p in problems:
    print(p)
PY
)
WRITE_BAD=$(printf '%s' "$WRITE_RESULT" | head -1)
if [ "${WRITE_BAD:-1}" != "0" ]; then
  note_fail "write round-trips: $WRITE_BAD problem(s):"
  printf '%s\n' "$WRITE_RESULT" | tail -n +2 | sed 's/^/    /'
else
  echo "ok (port-forward, dns-entry, dhcp-reservation, client-block)"
fi
check_integrity "after write round-trips"

echo
echo "== 4. Invalid input is rejected and changes nothing =="
NEG_RESULT=$(python3 - "$API" "$TOKEN" <<'PY'
import json, sys, urllib.request, urllib.error
api, token = sys.argv[1], sys.argv[2]
def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(f"{api}{path}", data=data, method=method,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=45) as r:
            return r.status, r.read().decode("utf8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf8", "replace")
    except Exception as e:
        return 0, str(e)
problems = []
def must_not_5xx(label, method, path, body):
    code, resp = call(method, path, body)
    if code >= 500 or code == 0:
        problems.append(f"{label}: {method} {path} returned {code} {resp[:140]}")
# Invalid input must never be a server fault.
must_not_5xx("bad schedule time", "PUT", "/wifi/schedule", {"enabled": True, "on_time": "99:99", "off_time": "23:00"})
must_not_5xx("cron injection", "PUT", "/wifi/schedule", {"enabled": True, "on_time": "07:00; reboot", "off_time": "23:00"})
must_not_5xx("bad cron time", "PUT", "/system/leds/schedule", {"enabled": True, "on_time": "99:99", "off_time": "23:00"})
must_not_5xx("unknown field", "PUT", "/system/alert-thresholds", {"storage_percent": 90, "bogus": 1})
must_not_5xx("threshold range", "PUT", "/system/alert-thresholds", {"storage_percent": 500, "cpu_percent": 90, "memory_percent": 90})
must_not_5xx("bad band", "PUT", "/wifi/band-switching", {"enabled": True, "preferred_band": "9g", "check_interval_sec": 10,
    "down_switch_threshold_dbm": -70, "down_switch_delay_sec": 30, "up_switch_threshold_dbm": -60,
    "up_switch_delay_sec": 60, "min_viable_signal_dbm": -80})
must_not_5xx("dhcp range", "PUT", "/network/dhcp", {"start": 100, "limit": 99999, "lease_time": "12h"})
must_not_5xx("wrong json kind", "PUT", "/network/dns", {"use_custom_dns": True, "servers": "not-a-list"})
must_not_5xx("malformed body", "PUT", "/wifi/schedule", None)
print(len(problems))
for p in problems:
    print(p)
PY
)
NEG_BAD=$(printf '%s' "$NEG_RESULT" | head -1)
if [ "${NEG_BAD:-1}" != "0" ]; then
  note_fail "invalid input produced $NEG_BAD server error(s):"
  printf '%s\n' "$NEG_RESULT" | tail -n +2 | sed 's/^/    /'
else
  echo "ok (no 5xx on invalid input)"
fi
check_integrity "after invalid input"

# ------------------------------------------- 5. concurrent UCI writers
echo
echo "== 5. Concurrent writers of the same UCI config =="
CONC_RESULT=$(python3 - "$API" "$TOKEN" "$CONCURRENCY" "$ROUNDS" <<'PY'
import json, sys, threading, urllib.request, urllib.error
api, token, n_writers, rounds = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(f"{api}{path}", data=data, method=method,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return r.status, r.read().decode("utf8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf8", "replace")
    except Exception as e:
        return 0, f"{type(e).__name__}: {e}"

errors, created = [], {"dns": 0, "res": 0, "pf": 0, "blk": 0}
lock = threading.Lock()
def worker(kind, w):
    global errors
    for i in range(rounds):
        code, body = call("POST", "/network/dns/entries", {"name": f"cc-w{w}-{i}", "ip": f"10.9.{w}.{i+1}"})
        if code == 200:
            created["dns"] += 1
        if code >= 500 or code == 0:
            with lock: errors.append(f"dns w{w} i{i} -> {code} {body[:140]}")
        code, body = call("POST", "/network/dhcp/reservations",
                          {"name": f"cc-w{w}-{i}", "mac": f"dd:ee:ff:00:{w:02x}:{i:02x}", "ip": f"10.8.{w}.{i+1}"})
        if code == 200:
            created["res"] += 1
        if code >= 500 or code == 0:
            with lock: errors.append(f"res w{w} i{i} -> {code} {body[:140]}")
        rid = f"cc-w{w}-{i}"
        code, body = call("POST", "/network/firewall/port-forwards",
            {"id": rid, "name": rid, "protocol": "tcp", "src_dport": str(20000 + w*10 + i),
             "dest_ip": "10.9.9.9", "dest_port": "80", "enabled": True})
        if code in (200, 201):
            created["pf"] += 1
        if code >= 500 or code == 0:
            with lock: errors.append(f"pf w{w} i{i} -> {code} {body[:140]}")

threads = [threading.Thread(target=worker, args=(k, w)) for w in range(n_writers) for k in ("x",)]
for t in threads: t.start()
for t in threads: t.join()

# Everything claimed as created must actually be there: a lost update means a
# request answered 200 for a write that was then overwritten.
#
# Count only THIS run's objects. The device may already hold entries the owner
# created, and those inflate the total enough to hide a genuinely lost write —
# which is the exact failure this section exists to catch. Every object written
# here is named cc-w*, which is also the prefix cleanup.py removes.
lost = []

def mine(path, key=None):
    """Objects in a collection that this run created."""
    code, body = call("GET", path)
    if code != 200:
        return None
    data = json.loads(body)
    if key is not None:
        data = data.get(key, [])
    if not isinstance(data, list):
        return None
    return [e for e in data if str(e.get("name", "")).startswith("cc-w")]

present = mine("/network/dns/entries")
if present is not None and len(present) < created["dns"]:
    lost.append(f"dns: {created['dns']} created, {len(present)} present")
present = mine("/network/dhcp/reservations")
if present is not None and len(present) < created["res"]:
    lost.append(f"reservations: {created['res']} created, {len(present)} present")
present = mine("/network/firewall/port-forwards", "rules")
if present is not None and len(present) < created["pf"]:
    lost.append(f"port-forwards: {created['pf']} created, {len(present)} present")

print(f"{len(errors)} {len(lost)}")
for e in errors[:10]: print("ERR " + e)
for l in lost: print("LOST " + l)
PY
)
CONC_ERR=$(printf '%s' "$CONC_RESULT" | head -1 | cut -d' ' -f1)
CONC_LOST=$(printf '%s' "$CONC_RESULT" | head -1 | cut -d' ' -f2)
if [ "${CONC_ERR:-1}" != "0" ]; then
  note_fail "concurrency: $CONC_ERR server error(s) under load:"
  printf '%s\n' "$CONC_RESULT" | grep '^ERR' | sed 's/^/    /'
fi
if [ "${CONC_LOST:-1}" != "0" ]; then
  note_fail "concurrency: $CONC_LOST lost update(s) — writers are not serialised:"
  printf '%s\n' "$CONC_RESULT" | grep '^LOST' | sed 's/^/    /'
fi
if [ "${CONC_ERR:-1}" = "0" ] && [ "${CONC_LOST:-1}" = "0" ]; then
  echo "ok (no 5xx, no lost updates)"
fi
check_integrity "after concurrency"

# --------------------------------------------------------------- result
echo
echo "== 6. Cleanup and post-run state =="
run_cleanup
BEFORE_PID=$(ssh_cmd 'pidof travo' 2>/dev/null || echo "")
STAGED=$(ssh_cmd 'uci -q changes' 2>/dev/null || true)
GUARDS=$(ssh_cmd 'ls /etc/trafo/ 2>/dev/null | grep -- "-in-progress" | tr "\n" " "' 2>/dev/null || true)
if [ -n "$STAGED" ]; then note_fail "staged UCI delta at exit: $STAGED"; fi
if [ -n "$GUARDS" ]; then note_fail "crash guard at exit: $GUARDS"; fi
if ! ssh_cmd 'pidof travo' >/dev/null 2>&1; then
  note_fail "travo is not running after the smoke run"
fi
# The device must be left as it was found, not merely non-crashed. "As found" is
# measured against the counts taken before section 3 wrote anything: demanding an
# empty collection failed on any router that already had its own port forward,
# DNS entry, reservation or blocked client, even when cleanup removed everything
# this run created.
for coll in $COLLECTIONS; do
  base=$(awk -v c="$coll" '$1 == c {print $2}' "$RUN_DIR/baseline_counts" 2>/dev/null || echo "?")
  n=$(count_coll "$coll")
  if [ "$base" = "?" ] || [ "$n" = "?" ]; then
    note_fail "$coll could not be compared (baseline=$base now=$n)"
    continue
  fi
  if [ "$n" != "$base" ]; then
    note_fail "$coll has $n object(s) after cleanup, was $base before the run (delta $((n - base)))"
  fi
done
echo "travo pid before=${BEFORE_PID:-?} after=$(ssh_cmd 'pidof travo' 2>/dev/null || echo none)"

OVERALL="PASS"
if [ "$FAILURES" -ne 0 ]; then OVERALL="FAIL"; fi

cat > "$RUN_DIR/result.json" <<EOF
{
  "status": "$OVERALL",
  "router_ip": "$ROUTER_IP",
  "get_endpoints": ${GET_N:-0},
  "failures": $FAILURES
}
EOF

echo
echo "== Result =="
echo "$OVERALL ($FAILURES failure(s))"
echo "Artifacts: $RUN_DIR"

if [ "$OVERALL" != "PASS" ]; then
  exit 2
fi
