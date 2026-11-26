#!/bin/sh
# Before/after screenshots for a PR (docs/testing.md "Screenshots"). Builds the screenshots of
# BASE (default origin/main) in a temporary worktree, then of this tree, and writes the changed
# ones as side-by-side images with the changes boxed to web/screenshots-diff/ (+ index.md).
# Usage: scripts/screenshots-compare.sh [BASE] [MATCH]   MATCH filters names, default -1280-light
# Needs the mise tools on PATH (run it as `mise run screenshots:compare`).
set -eu
cd "$(dirname "$0")/.."
base=${1:-origin/main}
match=${2:--1280-light}
wt=$(mktemp -d "${TMPDIR:-/tmp}/tg-shots-base.XXXXXX")
cleanup() { git worktree remove --force "$wt" >/dev/null 2>&1 || true; rm -rf "$wt"; }
trap cleanup EXIT INT TERM

git worktree add --detach "$wt" "$base" >/dev/null
echo "screenshots of $base"
(cd "$wt" && pnpm -C web install --frozen-lockfile --prefer-offline >/dev/null &&
  pnpm -C web build >/dev/null && scripts/with-mock.sh -- node web/scripts/screenshots.mjs >/dev/null)
echo "screenshots of this tree"
pnpm -C web build >/dev/null
scripts/with-mock.sh -- node web/scripts/screenshots.mjs >/dev/null
node web/scripts/screenshot-diff.mjs --base "$wt/web/screenshots" --head web/screenshots \
  --out web/screenshots-diff --match "$match"
echo "wrote web/screenshots-diff/ (see index.md)"
