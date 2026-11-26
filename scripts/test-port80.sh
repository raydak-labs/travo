#!/bin/sh
# Proves the installer port swap in a booted OpenWrt container. Never run against the router.
set -eu
cd "$(dirname "$0")/.."

IMAGE=openwrt/rootfs:aarch64_generic-25.12.6@sha256:22f13387da208c0fb4150c41f3ec12e34cc5994a0717a49e50aa9f9ae6cb5e7b
PLATFORM=linux/aarch64_generic
NAME=${CONTAINER_NAME:-travo-port80-$$}
VERSION=${VERSION:-0.0.0-dev}
export VERSION

# shellcheck source=scripts/lib/openwrt-container.sh
. scripts/lib/openwrt-container.sh

tmp=$(mktemp -d)
trap 'cleanup; rm -rf "$tmp"' EXIT
mise run package
mkdir "$tmp/pkg"
tar -xzf "dist/travo-greenfield-$VERSION-aarch64.tar.gz" -C "$tmp/pkg"
docker run -d --name "$NAME" --platform "$PLATFORM" "$IMAGE" /sbin/init >/dev/null
wait_boot
dx 'mkdir -p /tmp/tg'
docker cp "$tmp/pkg/." "$NAME:/tmp/tg/"

listen_lines() { dx '. /tmp/tg/port80.sh; uhttpd_listen_lines'; }
install() { dx 'sh /tmp/tg/install.sh --insecure'; }
uninstall() { dx 'sh /tmp/tg/install.sh --uninstall'; }
purge() { dx 'sh /tmp/tg/install.sh --uninstall --purge'; }

expect_refusal() {
  out=$(dx "$1 2>&1; echo rc=\$?")
  echo "$out" | grep -qx 'rc=1' || fail "expected exit 1: $out"
  echo "$out" | grep -Fq "$2" || fail "expected '$2': $out"
}

expect_luci() {
  # uhttpd can take a moment to listen again after install or uninstall restarts it.
  attempts=0
  until dx "wget -T 2 -qO- http://127.0.0.1:$1/" 2>/dev/null | grep -q cgi-bin/luci; do
    attempts=$((attempts + 1))
    [ "$attempts" -lt 15 ] || fail "LuCI landing page missing on $1"
    sleep 1
  done
}

expect_unchanged_uhttpd() {
  dx 'uci export uhttpd' | cmp - "$tmp/before.export" || fail "uhttpd changed"
}

# A direct uhttpd instance is the only test-owned process on the given address.
start_holder() {
  dx "/usr/sbin/uhttpd -f -p '$1' -h /tmp </dev/null >/dev/null 2>&1 & echo \$! >/tmp/test-holder.pid"
  dx ". /tmp/tg/port80.sh; wait_listen '$1' 10"
}

stop_holder() {
  # shellcheck disable=SC2016 # substitutions run in the container
  dx 'kill "$(cat /tmp/test-holder.pid)"; rm /tmp/test-holder.pid'
}

expect_luci 80
listen_lines >"$tmp/before.listen"

step "1: install on :80 and move LuCI"
install
wait_health "after swap" 80
dx 'wget -qO- http://[::1]:80/api/health' | grep -qF "\"version\":\"$VERSION\"" || fail "IPv6 health failed"
expect_luci 8080
listeners >"$tmp/swapped"
for address in 0.0.0.0:8080 :::8080 0.0.0.0:8443 :::8443; do
  grep -qxF "$address" "$tmp/swapped" || fail "listener $address missing"
done
dx '[ ! -e /etc/travo-greenfield-guards/port-swap ] && [ ! -e /etc/travo-greenfield-guards/port-swap.uhttpd ]'
dx 'sysupgrade -l 2>/dev/null' | grep -qxF /etc/travo-greenfield/uhttpd.orig || fail "uhttpd.orig would not survive sysupgrade"

step "2: reinstall without replacing the original snapshot"
dx 'cp /etc/travo-greenfield/uhttpd.orig /tmp/original.before'
install
wait_health "after reinstall" 80
dx 'cmp /tmp/original.before /etc/travo-greenfield/uhttpd.orig'

step "3: uninstall handles staged writes and failed restoration before restoring original listeners"
dx 'uci set uhttpd.main.home=/tmp'
expect_refusal 'sh /tmp/tg/install.sh --uninstall' 'uncommitted changes'
wait_health "after refused uninstall" 80
# shellcheck disable=SC2016 # substitution runs in the container
dx '[ "$(uci -q get uhttpd.main.home)" = /tmp ] && uci revert uhttpd'
dx 'cat /etc/config/uhttpd' >"$tmp/moved.config"
# Fail the first add_list after the restore has staged its deletes. Rollback
# uses the actual uci to revert the pending changes and copies the snapshot.
# shellcheck disable=SC2016 # quoted heredoc runs in the container
dx 'mkdir /tmp/failing-uci
cat >/tmp/failing-uci/uci <<\EOF_UCI
#!/bin/sh
[ "$1" != add_list ] || exit 1
exec /sbin/uci "$@"
EOF_UCI
chmod +x /tmp/failing-uci/uci'
# shellcheck disable=SC2016 # PATH is the container's
expect_refusal 'PATH=/tmp/failing-uci:$PATH sh /tmp/tg/install.sh --uninstall' 'left LuCI on the moved ports'
dx 'cat /etc/config/uhttpd' | cmp - "$tmp/moved.config" || fail "uninstall rollback differs from its snapshot"
dx '[ -L /etc/rc.d/S99travo-greenfield ] && [ ! -e /etc/travo-greenfield-guards/port-swap ]'
wait_health "after uninstall rollback" 8090
expect_luci 8080
uninstall
listen_lines | cmp - "$tmp/before.listen" || fail "original listen options or order changed"
listeners >"$tmp/restored"
grep -qxF '0.0.0.0:80' "$tmp/restored"
grep -qxF ':::80' "$tmp/restored"
purge

step "4: refuse when LuCI target port is occupied"
dx 'uci export uhttpd' >"$tmp/before.export"
start_holder 0.0.0.0:8080
expect_refusal 'sh /tmp/tg/install.sh --insecure' 'port 8080 is in use'
expect_unchanged_uhttpd
dx '[ ! -e /etc/travo-greenfield-guards/port-swap ]'
stop_holder
purge

step "5: roll back when greenfield cannot take :80"
dx 'cat /etc/config/uhttpd' >"$tmp/before.config"
expect_refusal 'TRAVO_TEST_HOLD_80=1 sh /tmp/tg/install.sh --insecure' 'uhttpd restored'
dx 'cat /etc/config/uhttpd' | cmp - "$tmp/before.config" || fail "rollback differs from the snapshot"
wait_health "after rollback" 8090
expect_luci 80
listeners >"$tmp/rolled-back"
grep -qxF '0.0.0.0:80' "$tmp/rolled-back"
grep -qxF ':::80' "$tmp/rolled-back"
dx '[ ! -e /etc/travo-greenfield-guards/port-swap ]'
purge

step "6: refuse when uhttpd has staged changes"
dx 'uci set uhttpd.main.home=/tmp'
expect_refusal 'sh /tmp/tg/install.sh --insecure' 'uncommitted changes'
# shellcheck disable=SC2016 # substitution runs in the container
dx '[ "$(uci -q get uhttpd.main.home)" = /tmp ] && uci revert uhttpd'
purge

step "7: recover an interrupted swap on the next install"
expect_refusal 'TRAVO_TEST_STOP_AFTER_SWAP=1 sh /tmp/tg/install.sh --insecure' 'WARNING: --insecure'
dx '[ -f /etc/travo-greenfield-guards/port-swap ]'
out=$(install)
echo "$out" | grep -qF 'restored uhttpd from an interrupted install'
wait_health "after interrupted swap retry" 80
purge

step "8: keep LuCI in place with --port"
dx 'uci export uhttpd' >"$tmp/before.export"
dx 'sh /tmp/tg/install.sh --insecure --port 8091'
wait_health "alternate port" 8091
expect_unchanged_uhttpd
purge

step "9: refuse an unlisted helper and a conflicting port 80 listener"
dx 'cp /tmp/tg/checksums.txt /tmp/checksums.before; sed -i "/port80\.sh$/d" /tmp/tg/checksums.txt'
expect_refusal 'sh /tmp/tg/install.sh --insecure' 'port80.sh checksum missing or invalid'
dx 'mv /tmp/checksums.before /tmp/tg/checksums.txt'
dx '[ ! -e /usr/bin/travo-greenfield ] &&
    [ ! -e /etc/init.d/travo-greenfield ] &&
    [ ! -e /etc/rc.d/S99travo-greenfield ] &&
    [ ! -e /etc/config/travo-greenfield ]'

# Move LuCI aside so the standalone process is the only port 80 listener.
dx 'cp /etc/config/uhttpd /tmp/uhttpd.before
uci delete uhttpd.main.listen_http
uci add_list uhttpd.main.listen_http=0.0.0.0:8080
uci add_list uhttpd.main.listen_http="[::]:8080"
uci commit uhttpd
/etc/init.d/uhttpd restart'
expect_luci 8080
dx 'uci export uhttpd' >"$tmp/before.export"
dx 'printf "%s\n" foreign-listener >/tmp/port80-probe'
start_holder 0.0.0.0:80
expect_refusal 'sh /tmp/tg/install.sh --insecure' 'port 80 is in use by something other than uhttpd'
expect_unchanged_uhttpd
dx 'wget -T 2 -qO- http://127.0.0.1:80/port80-probe' | grep -qxF foreign-listener || fail "foreign listener changed"
expect_luci 8080
purge
stop_holder
dx 'cp /tmp/uhttpd.before /etc/config/uhttpd; /etc/init.d/uhttpd restart'
expect_luci 80

step "10: reinstall after a plain uninstall preserves boot support"
install
uninstall
expect_luci 80
# shellcheck disable=SC2016 # substitution runs in the container
dx '[ "$(uci -q get travo-greenfield.main.listen)" = 0.0.0.0:8090 ]'
install
wait_health "after reinstall with retained settings" 80
expect_luci 8080
dx '[ -L /etc/rc.d/S99travo-greenfield ]'
purge
step "port 80 tests passed"
