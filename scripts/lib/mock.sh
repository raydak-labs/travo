# Shared by scripts/dev.sh and scripts/with-mock.sh (POSIX sh, sourced). Sets DAEMON_PORT, WEB_PORT.
DAEMON_PORT=${DAEMON_PORT:-18081}
WEB_PORT=${WEB_PORT:-18092}

# Fail early with a hint instead of letting the daemon or Vite die on bind.
require_free_port() { # name port
  if lsof -nP -iTCP:"$2" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "$1 port $2 is already in use: see 'lsof -i :$2'" >&2
    exit 1
  fi
}

# `go run` execs the compiled daemon as a child: killing only the go PID orphans it on the port.
kill_tree() {
  for c in $(pgrep -P "$1" 2>/dev/null || true); do kill_tree "$c"; done
  kill "$1" 2>/dev/null || true
}

# Polls url once a second while pid lives; 15 s covers a cold `go run` compile.
wait_http() { # url pid what
  i=0
  until curl -fsS -o /dev/null "$1"; do
    i=$((i + 1))
    if [ "$i" -ge 15 ] || ! kill -0 "$2" 2>/dev/null; then
      echo "$3 did not answer on $1" >&2
      exit 1
    fi
    sleep 1
  done
}

pids=""
stop_all() { for pid in $pids; do kill_tree "$pid"; wait "$pid" 2>/dev/null || true; done; }
trap stop_all EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# start_daemon [--mock-firstrun]: mock daemon on 127.0.0.1:$DAEMON_PORT, returns when healthy.
start_daemon() {
  require_free_port daemon "$DAEMON_PORT"
  go run ./cmd/travo-greenfield --mock "$@" --listen "127.0.0.1:$DAEMON_PORT" &
  daemon_pid=$!
  pids="$daemon_pid $pids"
  wait_http "http://127.0.0.1:$DAEMON_PORT/api/health" "$daemon_pid" daemon
}
