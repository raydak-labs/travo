#!/bin/sh
# Attaches web/screenshots-diff/*.png to PR N with `gh pr edit --attach` (gh uploads them as
# GitHub attachments and rewrites the image references). The PR body keeps everything above its
# "## Screenshots" heading; that section is replaced, so re-running after new commits is safe.
# Usage: scripts/screenshots-publish.sh N
set -eu
cd "$(dirname "$0")/.."
pr=${1:?usage: scripts/screenshots-publish.sh PR-NUMBER}
case $pr in *[!0-9]*) echo "PR number must be digits" >&2; exit 2 ;; esac
src=web/screenshots-diff
ls "$src"/*.png >/dev/null 2>&1 || { echo "no images in $src: run mise run screenshots:compare first" >&2; exit 1; }
count=$(find "$src" -maxdepth 1 -name "*.png" | wc -l | tr -d " ")
[ "$count" -le 50 ] || { echo "$count images: gh attaches at most 50; narrow the MATCH filter" >&2; exit 1; }

# The body file lives next to the images: gh resolves ./name.png relative to it.
body=$src/.pr-body.md
gh pr view "$pr" --json body --jq .body | awk '/^## Screenshots[[:space:]]*$/ { exit } { print }' >"$body"
{
  echo "## Screenshots"
  echo
  sed -n '3p' "$src/index.md"
  echo
  for f in "$src"/*.png; do
    n=$(basename "$f" .png)
    echo "**$n**"
    echo
    echo "![$n](./$n.png)"
    echo
  done
} >>"$body"

set --
for f in "$src"/*.png; do set -- "$@" --attach "$f"; done
gh pr edit "$pr" --body-file "$body" "$@"
rm -f "$body"
echo "attached $count images to #$pr"
