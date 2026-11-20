#!/bin/sh
# Installs the release tarball into a booted OpenWrt container (procd as PID 1), then checks
# health, listeners, reboot survival and uninstall, plus the signature cases of docs/adr/0007.
# Signs with throwaway minisign keys only (never the release key). Needs Docker and minisign.
# Run: mise run test:package
set -eu
cd "$(dirname "$0")/.."

IMAGE=openwrt/rootfs:aarch64_generic-25.12.6@sha256:22f13387da208c0fb4150c41f3ec12e34cc5994a0717a49e50aa9f9ae6cb5e7b
PLATFORM=linux/aarch64_generic
NAME=${CONTAINER_NAME:-travo-pkgtest-$$}
VERSION=${VERSION:-0.0.0-dev}
export VERSION

# shellcheck source=scripts/lib/openwrt-container.sh
. scripts/lib/openwrt-container.sh

# sign_to <secret key> <out>: legacy-format signature of dist/checksums.txt as the router's
# usign reads it (the first two lines of the minisign file).
sign_to() {
  minisign -S -l -s "$1" -m dist/checksums.txt -x "$tmp/full.minisig" >/dev/null
  head -n 2 "$tmp/full.minisig" >"$2"
}

# build_pkg <public key> <dir>: package with that key shipped, extracted into <dir>.
build_pkg() {
  PUBKEY=$1 mise run package
  mkdir "$2"
  tar -xzf "$tarball" -C "$2"
}

step "build the tarball"
tmp=$(mktemp -d)
trap 'cleanup; rm -rf "$tmp"' EXIT
tarball=dist/travo-greenfield-$VERSION-aarch64.tar.gz
minisign -G -W -p "$tmp/k1.pub" -s "$tmp/k1.key" >/dev/null
minisign -G -W -p "$tmp/k2.pub" -s "$tmp/k2.key" >/dev/null
# A stand-in offline AdGuard bundle: only its hash matters to install.sh (docs/adr/0020).
printf 'ADBd stand-in bundle\n' >"$tmp/adguardhome-0.0-r0-test.apk"
ADGUARD_BUNDLE=$tmp/adguardhome-0.0-r0-test.apk build_pkg "$tmp/k1.pub" "$tmp/pkg"
[ -f "$tarball" ] || fail "$tarball missing"
cmp dist/checksums.txt "$tmp/pkg/checksums.txt" || fail "dist/checksums.txt differs from the one in the tarball"
cmp "$tmp/k1.pub" "$tmp/pkg/minisign.pub" || fail "tarball minisign.pub is not the key given to PUBKEY"
grep -q ' \./minisign.pub$' dist/checksums.txt || fail "minisign.pub not listed in checksums.txt"
# before copying the .sig into pkg: variants that share pkg's files
cp -R "$tmp/pkg" "$tmp/pkg-nosig"
sign_to "$tmp/k1.key" "$tmp/pkg/checksums.txt.sig"
cp -R "$tmp/pkg" "$tmp/pkg-tampered"
echo "0000000000000000000000000000000000000000000000000000000000000000  ./tampered" >>"$tmp/pkg-tampered/checksums.txt"
cp -R "$tmp/pkg" "$tmp/pkg-wrongsig"
sign_to "$tmp/k2.key" "$tmp/pkg-wrongsig/checksums.txt.sig"
# validly signed, plus a file under root/ that checksums.txt does not list
cp -R "$tmp/pkg" "$tmp/pkg-extra"
mkdir -p "$tmp/pkg-extra/root/etc/rc.d"
printf '#!/bin/sh\n' >"$tmp/pkg-extra/root/etc/rc.d/S00evil"
# validly signed, but the listed binary replaced by a symlink to an identical copy outside root/
cp -R "$tmp/pkg" "$tmp/pkg-link"
mv "$tmp/pkg-link/root/usr/bin/travo-greenfield" "$tmp/pkg-link/binary-copy"
ln -s /tmp/tg-link/binary-copy "$tmp/pkg-link/root/usr/bin/travo-greenfield"

step "tarball entries equal root/ plus the manifest"
(cd "$tmp/pkg" && find root -type f | sed 's|^root||' | LC_ALL=C sort) >"$tmp/on-disk.txt"
tar -tzf "$tarball" | grep -v '/$' | grep '^root/' | sed 's|^root||' | LC_ALL=C sort >"$tmp/listed.txt"
cmp "$tmp/on-disk.txt" "$tmp/listed.txt" || fail "tar listing differs from extracted files"
cmp "$tmp/listed.txt" "$tmp/pkg/manifest.txt" || fail "manifest.txt differs from the files under root/"
tar -tzf "$tarball" | LC_ALL=C sort >"$tmp/all.txt"
for f in install.sh port80.sh VERSION manifest.txt checksums.txt minisign.pub; do
  grep -qx "$f" "$tmp/all.txt" || fail "$f missing from the tarball"
done
echo "ok: $(wc -l <"$tmp/pkg/manifest.txt" | tr -d ' ') manifest paths"

step "boot the container"
docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" --platform "$PLATFORM" "$IMAGE" /sbin/init >/dev/null
wait_boot
for d in tg:pkg tg-nosig:pkg-nosig tg-tampered:pkg-tampered tg-wrongsig:pkg-wrongsig tg-extra:pkg-extra tg-link:pkg-link; do
  dx "mkdir -p /tmp/${d%%:*}"
  docker cp "$tmp/${d##*:}/." "$NAME:/tmp/${d%%:*}/"
done

step "verify the release package through the updater adapter"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -tags integration -c -o "$tmp/stage.test" ./internal/updatestage
dx 'mkdir /tmp/stage-fixture'
docker cp "$tmp/stage.test" "$NAME:/tmp/stage.test"
docker cp "$tarball" "$NAME:/tmp/stage-fixture/release.tar.gz"
for f in checksums.txt checksums.txt.sig minisign.pub; do
  docker cp "$tmp/pkg/$f" "$NAME:/tmp/stage-fixture/$f"
done
docker cp "$tmp/pkg-wrongsig/checksums.txt.sig" "$NAME:/tmp/stage-fixture/bad.sig"
docker exec -e TRAVO_STAGE_PACKAGE_FIXTURE=/tmp/stage-fixture -e TRAVO_STAGE_PACKAGE_VERSION="$VERSION" \
  "$NAME" /tmp/stage.test -test.run '^TestRealReleasePackage$' -test.v

# Self-update runs the INSTALLED release's stager on the new tarball (#287, docs/adr/0032).
step "the previous release's stager accepts this package"
prev=$(git ls-remote --tags origin 'v*' | sed -n 's|.*refs/tags/\(v[0-9][0-9.]*\)$|\1|p' | sort -V | tail -n 1)
[ -n "$prev" ] || fail "no previous release tag on origin"
git rev-parse -q --verify "refs/tags/$prev" >/dev/null || git fetch -q --depth=1 origin tag "$prev"
mkdir "$tmp/prev"
git archive "$prev" | tar -x -C "$tmp/prev"
(cd "$tmp/prev" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -tags integration -c -o "$tmp/stage-prev.test" ./internal/updatestage)
docker cp "$tmp/stage-prev.test" "$NAME:/tmp/stage-prev.test"
docker exec -e TRAVO_STAGE_PACKAGE_FIXTURE=/tmp/stage-fixture -e TRAVO_STAGE_PACKAGE_VERSION="$VERSION" \
  "$NAME" /tmp/stage-prev.test -test.run '^TestRealReleasePackage$' -test.v ||
  fail "$prev cannot stage this package: self-update from $prev would break"
dx 'rm /tmp/stage-prev.test'
echo "ok: $prev stages it"
dx 'rm -r /tmp/stage-fixture /tmp/stage.test'

step "refusals before the real install"
dx 'cp /etc/openwrt_release /tmp/openwrt_release.real'
out=$(dx "sed -i \"s/^DISTRIB_RELEASE=.*/DISTRIB_RELEASE='24.10.8'/\" /etc/openwrt_release; sh /tmp/tg/install.sh 2>&1; echo rc=\$?")
dx 'cp /tmp/openwrt_release.real /etc/openwrt_release'
echo "$out" | grep -qx 'rc=1' || fail "old release not refused with exit 1: $out"
echo "$out" | grep -q '24.10.8' || fail "refusal does not name the release: $out"
echo "ok: 24.10.8 refused: $(echo "$out" | head -n 1)"
out=$(docker exec -u 65534 "$NAME" sh -c 'sh /tmp/tg/install.sh 2>&1; echo rc=$?')
echo "$out" | grep -qx 'rc=1' || fail "non-root install not refused with exit 1: $out"
echo "$out" | grep -q 'root' || fail "non-root refusal has no reason: $out"
echo "ok: non-root refused: $(echo "$out" | head -n 1)"
dx '[ ! -e /usr/bin/travo-greenfield ]' || fail "a refused install left files behind"

# Signature cases 1 and 2 (3 is the install below). Each runs install.sh and prints rc=<exit code>.
step "signature: no .sig refuses"
out=$(dx 'sh /tmp/tg-nosig/install.sh 2>&1; echo rc=$?')
echo "$out" | grep -qx 'rc=1' || fail "install without .sig not refused with exit 1: $out"
echo "$out" | grep -q -e '--insecure' || fail "refusal does not mention --insecure: $out"
dx '[ ! -e /usr/bin/travo-greenfield ]' || fail "a refused install left files behind"
echo "ok: refused: $(echo "$out" | head -n 1)"

step "signature: --insecure installs without writing the key"
dx 'sh /tmp/tg-nosig/install.sh --insecure'
dx '[ -x /usr/bin/travo-greenfield ]' || fail "--insecure install did not install"
dx '[ ! -e /etc/travo-greenfield/minisign.pub ]' || fail "--insecure install wrote the trusted key"
dx 'sh /tmp/tg-nosig/install.sh --uninstall --purge'
dx '[ ! -e /usr/bin/travo-greenfield ] && [ ! -e /etc/travo-greenfield ]' || fail "uninstall/purge left files"
echo "ok: --insecure installed, no key written"

step "install"
listeners >"$tmp/listeners-before.txt"
dx 'sh /tmp/tg/install.sh'
wait_health "after install" 80
dx 'cat /etc/travo-greenfield/minisign.pub' | cmp - "$tmp/k1.pub" || fail "signed install did not store the tarball key"
echo "ok: signed install stored the key"
listeners >"$tmp/listeners-after.txt"
while IFS= read -r l; do
  case "$l" in *:80|*:443) continue ;; esac
  grep -qxF "$l" "$tmp/listeners-after.txt" || fail "listener $l disappeared after install"
done <"$tmp/listeners-before.txt"
grep -q ':80$' "$tmp/listeners-after.txt" || fail "nothing listens on :80"
grep -q ':8080$' "$tmp/listeners-after.txt" || fail "nothing listens on :8080"
echo "ok: LuCI moved and the other pre-install listeners are still present"
[ "$(dx 'uci -q get travo-greenfield.main.listen_tls')" = :443 ] || fail "install did not enable HTTPS on :443"
attempts=0
until dx 'wget --no-check-certificate -T 2 -qO- https://127.0.0.1/api/health' 2>/dev/null | grep -q '"status"'; do
  attempts=$((attempts + 1))
  [ "$attempts" -lt 15 ] || fail "no HTTPS health answer on :443"
  sleep 1
done
dx '[ -s /etc/travo-greenfield/tls/cert.pem ] && ls -l /etc/travo-greenfield/tls/key.pem | grep -q "^-rw-------"' ||
  fail "self-signed certificate not stored with a private key file"
echo "ok: HTTPS answers on :443 with the generated certificate"
dx '[ -L /etc/rc.d/S99travo-greenfield ] && [ -L /etc/rc.d/K10travo-greenfield ]' || fail "boot links missing"
dx '[ ! -e /etc/uci-defaults/90-travo-greenfield ]' || fail "uci-defaults script not consumed"
# BusyBox tr has no [:upper:] class: it once turned OpenWrt into wenrt.lan
# shellcheck disable=SC2018,SC2019 # same ASCII-only mapping as the script under test
want=$(dx 'uci -q get system.@system[0].hostname' | tr 'A-Z' 'a-z').lan
got=$(dx 'uci -q get travo-greenfield.main.host_allow')
[ "$got" = "$want" ] || fail "host_allow is '$got', want '$want'"
dx 'sysupgrade -l 2>/dev/null' | grep -q travo-greenfield || fail "sysupgrade -l lists no travo-greenfield path"

step "offline AdGuard bundle: a foreign file is refused, the pinned one is staged in RAM"
docker cp "$tmp/adguardhome-0.0-r0-test.apk" "$NAME:/tmp/agh-good.apk"
dx 'printf "other\n" >/tmp/agh-bad.apk'
out=$(dx 'sh /tmp/tg/install.sh --adguard-bundle /tmp/agh-bad.apk 2>&1; echo rc=$?')
echo "$out" | grep -qx 'rc=1' || fail "foreign bundle not refused: $out"
echo "$out" | grep -q 'does not match this release' || fail "refusal does not name the mismatch: $out"
dx '[ ! -e /tmp/travo-greenfield-bundle/adguardhome.apk ]' || fail "a refused bundle was staged"
dx 'sh /tmp/tg/install.sh --adguard-bundle /tmp/agh-good.apk'
dx 'cmp /tmp/agh-good.apk /tmp/travo-greenfield-bundle/adguardhome.apk' || fail "pinned bundle not staged"
dx 'ls -ld /tmp/travo-greenfield-bundle' | grep -q '^drwx------' || fail "bundle dir is not root-only"
echo "ok: bundle pin enforced by install.sh"

step "restart the container (boot link)"
docker restart "$NAME" >/dev/null
wait_boot
wait_health "after reboot" 80

# Signature case 4: the installed key (k1) is trusted; a tarball with another key needs --rotate-key.
step "signature: key rotation"
build_pkg "$tmp/k2.pub" "$tmp/pkg2"
sign_to "$tmp/k2.key" "$tmp/pkg2/checksums.txt.sig"
dx 'mkdir -p /tmp/tg2'
docker cp "$tmp/pkg2/." "$NAME:/tmp/tg2/"
out=$(dx 'sh /tmp/tg2/install.sh 2>&1; echo rc=$?')
echo "$out" | grep -qx 'rc=1' || fail "different key not refused with exit 1: $out"
echo "$out" | grep -q 'differs from the installed key' || fail "refusal does not say the key differs: $out"
dx 'cat /etc/travo-greenfield/minisign.pub' | cmp - "$tmp/k1.pub" || fail "refused rotation changed the installed key"
dx 'sh /tmp/tg2/install.sh --rotate-key'
dx 'cat /etc/travo-greenfield/minisign.pub' | cmp - "$tmp/k2.pub" || fail "--rotate-key did not store the new key"
wait_health "after key rotation" 80
echo "ok: different key refused, --rotate-key stored it"

step "uninstall keeps config and ignores foreign manifest paths"
# A tampered manifest must not reach files the package does not own.
dx 'printf "%s\n" /etc/config/network /etc /www/travo-greenfield/../../etc/config/firewall >> /etc/travo-greenfield/manifest.txt'
dx 'sh /tmp/tg/install.sh --uninstall'
dx '[ -f /etc/config/network ] && [ -f /etc/config/firewall ] && [ -d /etc ]' || fail "uninstall followed a foreign manifest path"
while IFS= read -r p; do
  # the manifest copy lives under /etc/travo-greenfield/ and is not in the manifest
  dx "[ ! -e '$p' ]" || fail "$p still exists after uninstall"
done <"$tmp/pkg/manifest.txt"
dx '[ ! -e /etc/rc.d/S99travo-greenfield ] && [ ! -e /etc/rc.d/K10travo-greenfield ]' || fail "boot links remain"
dx '[ -e /etc/config/travo-greenfield ]' || fail "uninstall removed /etc/config/travo-greenfield"
dx '[ ! -e /www/travo-greenfield ]' || fail "/www/travo-greenfield remains"
echo "ok: uninstalled, config kept"

step "uninstall --purge"
dx 'sh /tmp/tg/install.sh --uninstall --purge'
dx '[ ! -e /etc/config/travo-greenfield ] && [ ! -e /etc/travo-greenfield ]' || fail "purge left config or state"
echo "ok: purge removed config and state"

# Signature cases 5 and 6 run on the clean container left by the purge: the tarball key is used.
step "signature: .sig from another key refuses"
out=$(dx 'sh /tmp/tg-wrongsig/install.sh 2>&1; echo rc=$?')
echo "$out" | grep -qx 'rc=1' || fail "wrong-key .sig not refused with exit 1: $out"
echo "$out" | grep -q 'signature check failed' || fail "refusal is not 'signature check failed': $out"
dx '[ ! -e /usr/bin/travo-greenfield ]' || fail "a refused install left files behind"
echo "ok: refused: $(echo "$out" | head -n 1)"

step "signature: tampered checksums.txt refuses"
out=$(dx 'sh /tmp/tg-tampered/install.sh 2>&1; echo rc=$?')
echo "$out" | grep -qx 'rc=1' || fail "tampered checksums.txt not refused with exit 1: $out"
echo "$out" | grep -q 'signature check failed' || fail "refusal is not 'signature check failed': $out"
dx '[ ! -e /usr/bin/travo-greenfield ] && [ ! -e /etc/travo-greenfield ]' || fail "a refused install left files behind"
echo "ok: refused: $(echo "$out" | head -n 1)"

step "signature: unlisted file under root/ refuses"
out=$(dx 'sh /tmp/tg-extra/install.sh 2>&1; echo rc=$?')
echo "$out" | grep -qx 'rc=1' || fail "unlisted file not refused with exit 1: $out"
echo "$out" | grep -q 'not all listed' || fail "refusal does not name the unlisted files: $out"
dx '[ ! -e /etc/rc.d/S00evil ] && [ ! -e /usr/bin/travo-greenfield ]' || fail "a refused install left files behind"
echo "ok: refused: $(echo "$out" | head -n 1)"

step "signature: symlink under root/ refuses"
out=$(dx 'sh /tmp/tg-link/install.sh 2>&1; echo rc=$?')
echo "$out" | grep -qx 'rc=1' || fail "symlink not refused with exit 1: $out"
echo "$out" | grep -q 'other than regular files' || fail "refusal does not name the symlink: $out"
dx '[ ! -e /usr/bin/travo-greenfield ]' || fail "a refused install left files behind"
echo "ok: refused: $(echo "$out" | head -n 1)"

step "package test passed"
