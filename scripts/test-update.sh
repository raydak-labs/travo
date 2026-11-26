#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
IMAGE=openwrt/rootfs:aarch64_generic-25.12.6@sha256:22f13387da208c0fb4150c41f3ec12e34cc5994a0717a49e50aa9f9ae6cb5e7b
PLATFORM=linux/aarch64_generic
NAME=${CONTAINER_NAME:-travo-updatetest-$$}
VERSION=1.2.344
export VERSION
# shellcheck source=scripts/lib/openwrt-container.sh
. scripts/lib/openwrt-container.sh

tmp=$(mktemp -d)
trap 'cleanup; rm -rf "$tmp"' EXIT
mise run package
mkdir "$tmp/old" "$tmp/new"
tar -xzf "dist/travo-greenfield-$VERSION-aarch64.tar.gz" -C "$tmp/old"
VERSION=1.2.345 mise run package
tar -xzf dist/travo-greenfield-1.2.345-aarch64.tar.gz -C "$tmp/new"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -tags integration -c -o "$tmp/apply.test" ./internal/updateapply

docker run -d --name "$NAME" --platform "$PLATFORM" "$IMAGE" /sbin/init >/dev/null
wait_boot
docker cp "$tmp/old" "$NAME:/tmp/old"
docker cp "$tmp/new" "$NAME:/tmp/new"
docker cp "$tmp/apply.test" "$NAME:/tmp/apply.test"
dx 'sh /tmp/old/install.sh --insecure --port 8097'
wait_health "old version" 8097

step "installer refuses contention and interrupted update"
# shellcheck disable=SC2016 # Expand variables inside the container.
dx 'set -eu
lock=/var/lock/travo-greenfield-update.lock
guard=/etc/travo-greenfield-guards/update-in-progress
journal=/etc/travo-greenfield/update-operation/journal.json
mkdir -p /etc/travo-greenfield/update-operation /etc/travo-greenfield-guards
printf "rollback state\n" >"$journal"
capture() { sha256sum /usr/bin/travo-greenfield /www/travo-greenfield/index.html /etc/travo-greenfield/manifest.txt "$journal"; }
before=$(capture)
(
 exec 8>>"$lock"
 flock -n -x 8
 if sh /tmp/old/install.sh --insecure >/tmp/refused 2>&1; then exit 1; fi
 grep -q "already in progress" /tmp/refused
)
printf "stale guard\n" >"$guard"
for args in --insecure "--uninstall --purge"; do
 if sh /tmp/old/install.sh $args >/tmp/refused 2>&1; then exit 1; fi
 grep -q "recovery required" /tmp/refused
done
test "$(capture)" = "$before"
test "$(cat "$guard")" = "stale guard"
rm "$guard" "$journal"'

step "apply, rollback and recover the staged package"
docker exec -e TRAVO_APPLY_FIXTURE=/tmp/new -e TRAVO_APPLY_PREVIOUS="$VERSION" \
  "$NAME" /tmp/apply.test -test.run '^TestRealApply$' -test.v -test.timeout 3m
VERSION=1.2.345 wait_health "restored version" 8097
