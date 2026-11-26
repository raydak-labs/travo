#!/bin/sh
# Serves the built UI (web/dist) via `vite preview` against the mock daemon, runs a command, stops both.
# Usage: scripts/with-mock.sh [--firstrun] -- <command...>. The command sees WEB_PORT and DAEMON_PORT.
# Build first (`pnpm -C web build`); browser checks and screenshots use this.
set -eu
cd "$(dirname "$0")/.."
. scripts/lib/mock.sh

flag=""
if [ "${1:-}" = "--firstrun" ]; then flag="--mock-firstrun"; shift; fi
[ "${1:-}" = "--" ] && shift
[ $# -gt 0 ] || { echo "usage: scripts/with-mock.sh [--firstrun] -- <command...>" >&2; exit 2; }

start_daemon ${flag:+"$flag"}
require_free_port web "$WEB_PORT"
DAEMON_URL="http://127.0.0.1:$DAEMON_PORT" pnpm -C web exec vite preview --port "$WEB_PORT" --strictPort &
preview_pid=$!
pids="$preview_pid $pids"
wait_http "http://localhost:$WEB_PORT/" "$preview_pid" "vite preview"

export WEB_PORT DAEMON_PORT
"$@"
