#!/bin/sh
# Publishes web/screenshots-diff/*.png for PR N to the orphan branch `pr-assets` (folder pr-N/)
# and prints the Markdown to paste into the PR body. The images never enter main's history.
# Usage: scripts/screenshots-publish.sh N
set -eu
cd "$(dirname "$0")/.."
pr=${1:?usage: scripts/screenshots-publish.sh PR-NUMBER}
case $pr in *[!0-9]*) echo "PR number must be digits" >&2; exit 2 ;; esac
src=web/screenshots-diff
ls "$src"/*.png >/dev/null 2>&1 || { echo "no images in $src: run mise run screenshots:compare first" >&2; exit 1; }
repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
wt=$(mktemp -d "${TMPDIR:-/tmp}/tg-pr-assets.XXXXXX")
cleanup() { git worktree remove --force "$wt" >/dev/null 2>&1 || true; rm -rf "$wt"; }
trap cleanup EXIT INT TERM

if git fetch -q origin pr-assets 2>/dev/null; then
  git worktree add -q --detach "$wt" FETCH_HEAD
else
  git worktree add -q --detach "$wt" HEAD
  (cd "$wt" && git checkout -q --orphan pr-assets-new && git rm -rq --cached . && find . -mindepth 1 -maxdepth 1 ! -name .git -exec rm -rf {} +)
fi
rm -rf "${wt:?}/pr-$pr"
mkdir -p "$wt/pr-$pr"
cp "$src"/*.png "$src/index.md" "$wt/pr-$pr/"
(cd "$wt" && git add "pr-$pr" && git commit -qm "screenshots for #$pr" && git push -q origin HEAD:refs/heads/pr-assets)

echo "## Screenshots"
echo
sed -n '3p' "$src/index.md"
echo
for f in "$src"/*.png; do
  n=$(basename "$f")
  echo "**${n%.png}**"
  echo
  echo "![${n%.png}](https://github.com/$repo/blob/pr-assets/pr-$pr/$n?raw=true)"
  echo
done
