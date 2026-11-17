#!/bin/sh
# Shared OpenWrt integration-test helpers.

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
step() { echo "== $*"; }
dx() { docker exec "$NAME" sh -c "$1"; }

listeners() { dx "netstat -ltn | awk 'NR>2 {print \$4}' | sort"; }

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }

# diagnose <label> <started epoch>: why a wait gave up, so a CI failure explains itself.
diagnose() {
  echo "$1: gave up after $(($(date +%s) - $2))s" >&2
  dx 'echo "-- ps"; ps w; echo "-- logread"; logread | tail -n 40; echo "-- travo-greenfield"; /etc/init.d/travo-greenfield status' >&2 2>&1 || true
}

# wait_boot: until procd has run every /etc/rc.d script. In the container S98sysntpd blocks
# ~30s on a ubus call netifd never answers, so S99 (the daemon) starts ~35s after boot; an
# earlier readiness signal (dnsmasq running) left wait_health racing that delay.
wait_boot() {
  t0=$(date +%s)
  until dx 'logread | grep -q -- "- init complete -"' 2>/dev/null; do
    [ $(($(date +%s) - t0)) -lt 90 ] || { diagnose "boot" "$t0"; fail "container did not finish booting"; }
    sleep 1
  done
  echo "ok: booted in $(($(date +%s) - t0))s"
}

wait_health() { # label port
  t0=$(date +%s)
  until dx "wget -T 2 -qO- http://127.0.0.1:$2/api/health" 2>/dev/null | grep -qF "\"version\":\"$VERSION\""; do
    [ $(($(date +%s) - t0)) -lt 30 ] || { diagnose "$1" "$t0"; fail "$1: /api/health did not report version $VERSION"; }
    sleep 1
  done
  echo "ok: $1: health reports $VERSION"
}
