#!/bin/sh
# Runs the web UI against the mock daemon: daemon in the background, Vite dev in the foreground.
# Usage: scripts/dev.sh [--firstrun]. Ports: DAEMON_PORT (18081), WEB_PORT (18092).
set -eu
cd "$(dirname "$0")/.."
. scripts/lib/mock.sh

case "${1:-}" in
  "") start_daemon ;;
  --firstrun) start_daemon --mock-firstrun ;;
  *) echo "usage: scripts/dev.sh [--firstrun]" >&2; exit 2 ;;
esac

require_free_port web "$WEB_PORT"
DAEMON_URL="http://127.0.0.1:$DAEMON_PORT" pnpm -C web exec vite --port "$WEB_PORT" --strictPort
