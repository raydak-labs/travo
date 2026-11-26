#!/bin/sh
# Runs the mock daemon, which serves the built UI (web/dist) itself, then a command, then stops it.
# Usage: scripts/with-daemon-ui.sh -- <command...>. The command sees DAEMON_PORT.
# Build first (`pnpm -C web build`). Unlike with-mock.sh there is no vite preview: the daemon's
# own headers (CSP, caching) are what the browser gets.
set -eu
cd "$(dirname "$0")/.."
. scripts/lib/mock.sh

[ "${1:-}" = "--" ] && shift
[ $# -gt 0 ] || { echo "usage: scripts/with-daemon-ui.sh -- <command...>" >&2; exit 2; }

# shellcheck disable=SC2119 # start_daemon takes optional flags; none here
start_daemon
export DAEMON_PORT
"$@"
