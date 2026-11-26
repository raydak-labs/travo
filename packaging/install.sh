#!/bin/sh
# Installs or removes travo on OpenWrt. Run as root from the extracted tarball:
#   tar -xzf travo-<version>-aarch64.tar.gz -C /tmp/tg && sh /tmp/tg/install.sh
# Flags: (none) install | --uninstall | --purge (with --uninstall: also config and state)
#        | --port N (alternate port, keeps LuCI unchanged)
#        | --force (skip the OS and architecture checks only)
#        | --insecure (install without a signature check) | --rotate-key (accept a new signing key)
#        | --adguard-bundle FILE (offline AdGuard Home package from the same release, docs/adr/0020)
#        | --lan CIDR (router address on a first install, default 192.168.50.1/24) | --keep-lan
#        | --wifi (ask for the WiFi password) | --wifi-name NAME (default Travo).
# First install on a factory router (docs/adr/0034): the LAN moves off OpenWrt's 192.168.1.1/24,
# which hotel and home networks often use too, and with --wifi or TRAVO_WIFI_PASSWORD the WiFi
# comes up as one WPA2/WPA3 network on every band. Both only replace OpenWrt's factory values.
# Before installing it verifies $here/checksums.txt.sig over checksums.txt with usign (docs/adr/0007).
# Test hooks: TRAVO_TEST_STOP_AFTER_SWAP=1 exits after moving LuCI.
# TRAVO_TEST_HOLD_80=1 starts a listener that blocks travo from taking port 80.
# Touches only the paths in manifest.txt, the ones named in uninstall() and stale files in the
# UI directory it owns (invariants §15).
set -eu

NAME=travo
STATE_DIR=/etc/travo
MIN_FREE_KIB=20480
PORT_SWAP_GUARD=/etc/travo-guards/port-swap
UI_DIR=/www/travo

die() {
  echo "install.sh: $*" >&2
  exit 1
}

usage() {
  echo "usage: install.sh [--force] [--insecure] [--rotate-key] [--port N] [--adguard-bundle FILE] [--lan CIDR | --keep-lan] [--wifi] [--wifi-name NAME] | --uninstall [--purge]" >&2
  exit 2
}

# Major.minor of DISTRIB_RELEASE must be >= 25.12; SNAPSHOT builds pass.
check_os() {
  release=$(sed -n "s/^DISTRIB_RELEASE=['\"]\{0,1\}\([^'\"]*\)['\"]\{0,1\}\$/\1/p" /etc/openwrt_release 2>/dev/null) ||
    release=""
  [ -n "$release" ] || die "cannot read DISTRIB_RELEASE from /etc/openwrt_release (not OpenWrt? use --force)"
  case "$release" in *SNAPSHOT*) return 0 ;; esac
  major=${release%%.*}
  rest=${release#*.}
  minor=${rest%%.*}
  minor=${minor%%[!0-9]*}
  case "$major$minor" in
    "" | *[!0-9]*) die "unrecognised OpenWrt release '$release' (use --force)" ;;
  esac
  if [ "$major" -lt 25 ] || { [ "$major" -eq 25 ] && [ "${minor:-0}" -lt 12 ]; }; then
    die "OpenWrt $release is not supported, need 25.12 or newer (use --force)"
  fi
}

check_arch() {
  arch=$(uname -m)
  [ "$arch" = aarch64 ] || die "architecture $arch is not supported, need aarch64 (use --force)"
}

check_space() {
  target=/overlay
  [ -d "$target" ] || target=/
  free=$(df -k "$target" | tail -n 1 | awk '{print $4}')
  case "$free" in "" | *[!0-9]*) die "cannot read free space of $target" ;; esac
  [ "$free" -ge "$MIN_FREE_KIB" ] || die "only $free KiB free on $target, need $MIN_FREE_KIB"
}

# Polls /api/health once a second (BusyBox has no timeout or curl) until it reports $1.
wait_healthy() {
  expected_health='"version":"'
  [ -z "$1" ] || expected_health=$expected_health$1'"'
  listen=$(uci -q get "$NAME.main.listen") || die "no $NAME.main.listen in UCI"
  port=${listen##*:}
  i=0
  while [ "$i" -lt 30 ]; do
    if wget -q -T 2 -O - "http://127.0.0.1:$port/api/health" 2>/dev/null | grep -qF "$expected_health"; then
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  return 1
}

# Verifies checksums.txt against checksums.txt.sig with a key the router already trusts: the
# installed one, or on the first install the one in the tarball (trust on first use).
# Sets new_key=1 when the verified key is the tarball's and must be saved after the install.
verify_signature() {
  new_key=0
  key=$here/minisign.pub
  installed=$STATE_DIR/minisign.pub
  if [ -f "$installed" ]; then
    key=$installed
    if [ -f "$here/minisign.pub" ] && ! cmp -s "$installed" "$here/minisign.pub"; then
      [ "$rotate" = 1 ] || die "tarball key differs from the installed key; re-run with --rotate-key if this rotation is intended"
      key=$here/minisign.pub
      new_key=1
    fi
  elif [ -f "$key" ]; then
    new_key=1
  fi
  if [ -f "$key" ] && [ -f "$here/checksums.txt.sig" ]; then
    command -v usign >/dev/null 2>&1 || die "usign not found"
    usign -V -q -p "$key" -m "$here/checksums.txt" -x "$here/checksums.txt.sig" || die "signature check failed"
    return 0
  fi
  new_key=0
  [ "$insecure" = 1 ] || die "unsigned: no checksums.txt.sig or minisign.pub; re-run with --insecure to install without a signature check"
  if [ -f "$installed" ]; then
    echo "install.sh: WARNING: --insecure although a signing key is installed: this tarball is NOT verified against it" >&2
  else
    echo "install.sh: WARNING: --insecure: installing without a signature check" >&2
  fi
}

load_port80() {
  [ -f "$here/port80.sh" ] && [ ! -L "$here/port80.sh" ] ||
    die "port80.sh must be a regular file from the release tarball"
  # Verify the exact entry; checking the whole manifest alone does not require
  # an entry for the helper. Uninstall can still use a tarball from before key rotation.
  helper_checksum=$(sed -n '/^[0-9a-f]\{64\}  \.\/port80\.sh$/p' "$here/checksums.txt") ||
    die "port80.sh checksum missing or invalid"
  [ -n "$helper_checksum" ] && [ "$(printf '%s\n' "$helper_checksum" | wc -l)" -eq 1 ] ||
    die "port80.sh checksum missing or invalid"
  (cd "$here" && printf '%s\n' "$helper_checksum" | sha256sum -c >/dev/null) ||
    die "port80.sh checksum mismatch"
  # shellcheck source=packaging/port80.sh
  . "$here/port80.sh"
}

# Stages the offline AdGuard Home package for the daemon's first install (docs/adr/0020).
# Its hash must equal the pin stamped into this release's (verified) binary. RAM only.
stage_adguard_bundle() {
  [ -n "$adguard_bundle" ] || return 0
  [ -f "$adguard_bundle" ] && [ ! -L "$adguard_bundle" ] || die "$adguard_bundle is not a regular file"
  pin=$("$here/root/usr/bin/$NAME" --adguard-bundle-pin) || die "this release has no AdGuard Home bundle pin"
  want=${pin%% *}
  got=$(sha256sum "$adguard_bundle") || die "cannot hash $adguard_bundle"
  got=${got%% *}
  [ "$got" = "$want" ] || die "$adguard_bundle does not match this release (want ${pin#*  })"
  dir=/tmp/$NAME-bundle
  if ! mkdir -p "$dir" || ! chmod 700 "$dir" || ! cp "$adguard_bundle" "$dir/adguardhome.apk.tmp" ||
     ! mv "$dir/adguardhome.apk.tmp" "$dir/adguardhome.apk"; then
    die "cannot stage the AdGuard Home bundle in $dir"
  fi
  echo "AdGuard Home bundle staged; it installs on first start without an uplink"
}

# The UI directory belongs to Travo alone. Asset names carry a content hash, so every release
# ships new names; drop the files this release does not list. Without this a reinstall leaves the
# old assets behind and the self-updater refuses (installed manifest does not match, ADR 0007).
prune_ui() {
  [ -d "$UI_DIR" ] || return 0
  # Direct children only: the UI is a flat directory (the updater refuses subdirectories).
  find "$UI_DIR" -mindepth 1 -maxdepth 1 -type f | while IFS= read -r f; do
    grep -qxF "$f" manifest.txt || rm -f "$f"
  done
}

install() {
  [ "$force" = 1 ] || { check_os; check_arch; }
  check_space
  cd "$here"
  [ -f checksums.txt ] && [ -d root ] || die "$here is not an extracted release tarball"
  verify_signature
  sha256sum -c checksums.txt >/dev/null || die "checksum mismatch in $here (run sha256sum -c checksums.txt)"
  # Only regular files and directories: a symlink passes sha256sum (it follows it) but cp -a
  # would install the link itself.
  if [ -n "$(find root ! -type f ! -type d | head -n 1)" ]; then
    die "$here/root contains something other than regular files and directories"
  fi
  # sha256sum -c checks only listed files; cp copies all of root/. Refuse anything unlisted.
  find root ! -type d | sed 's|^|./|' | LC_ALL=C sort >"$here/.on-disk"
  sed -n 's|^[0-9a-f]\{64\}  \(\./root/.*\)$|\1|p' checksums.txt | LC_ALL=C sort >"$here/.listed"
  if ! cmp -s "$here/.on-disk" "$here/.listed"; then
    rm -f "$here/.on-disk" "$here/.listed"
    die "files under $here/root are not all listed in checksums.txt"
  fi
  rm -f "$here/.on-disk" "$here/.listed"
  version=$(cat VERSION)
  stage_adguard_bundle

  load_port80
  mkdir -p /etc/travo-guards
  recover_port_swap
  first_install=0
  [ -e /etc/init.d/$NAME ] || first_install=1

  cp -a root/. / || die "copying files to / failed"
  prune_ui
  # OpenWrt runs uci-defaults only at boot; run it now. It deletes itself on success.
  defaults=/etc/uci-defaults/90-$NAME
  if [ -e "$defaults" ]; then
    sh "$defaults" 9>&- || die "$defaults failed"
    rm -f "$defaults"
  fi
  if [ "$first_install" = 1 ] && [ ! -e /etc/rc.d/S99$NAME ] &&
     [ "$(uci -q get "$NAME.main.enabled" || true)" != 0 ]; then
    /etc/init.d/$NAME enable || die "could not enable $NAME"
  fi
  mkdir -p "$STATE_DIR"
  cp manifest.txt "$STATE_DIR/manifest.txt" || die "cannot save the manifest to $STATE_DIR"
  # only a verified install may establish or rotate the trusted key
  if [ "$new_key" = 1 ]; then
    cp "$here/minisign.pub" "$STATE_DIR/minisign.pub" || die "cannot save the signing key to $STATE_DIR"
  fi

  # restart, not start: an older binary may be running
  /etc/init.d/$NAME restart 9>&- || die "/etc/init.d/$NAME restart failed"
  if ! wait_healthy "$version"; then
    echo "install.sh: $NAME $version did not answer on /api/health within 30 s; files stay installed. Last log lines:" >&2
    logread -e "$NAME" | tail -20 >&2
    exit 1
  fi
  # A healthy reinstall is the explicit retry path after a firmware flash (invariants §1);
  # the keep-list carried its guard across the flash. Cleared by name, never by glob.
  rm -f /etc/travo-guards/firmware-upgrade-in-progress
  install_ports "$version"
  first_run
}

FACTORY_LAN=192.168.1.1/24
DEFAULT_LAN=192.168.50.1/24
DEFAULT_SSID=Travo

# Quotes a value for `uci batch`. The value travels through printf (a builtin), never argv.
uci_quote() {
  printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"
}

# ipv4/prefix with every octet 0-255 and a prefix of 8-30.
valid_cidr() {
  printf '%s' "$1" | grep -Eq '^([0-9]{1,3}\.){3}[0-9]{1,3}/([89]|[12][0-9]|30)$' || return 1
  for octet in $(printf '%s' "${1%/*}" | tr . ' '); do [ "$octet" -le 255 ] || return 1; done
}

# The router's own WiFi is untouched since firstboot: every AP bridged into lan off and open.
wifi_factory() {
  found=0
  for iface in $(uci -q show wireless | sed -n "s/^wireless\.\([^.=]*\)=wifi-iface$/\1/p"); do
    [ "$(uci -q get "wireless.$iface.mode")" = ap ] || continue
    uci -q get "wireless.$iface.network" | grep -qw lan || continue
    found=1
    [ "$(uci -q get "wireless.$iface.disabled")" = 1 ] || return 1
    case "$(uci -q get "wireless.$iface.encryption")" in ""|none) ;; *) return 1 ;; esac
  done
  [ "$found" = 1 ]
}

# Stages the factory APs as one network on every band (the same plan as POST /api/v1/wifi/setup).
stage_wifi() {
  for iface in $(uci -q show wireless | sed -n "s/^wireless\.\([^.=]*\)=wifi-iface$/\1/p"); do
    [ "$(uci -q get "wireless.$iface.mode")" = ap ] || continue
    uci -q get "wireless.$iface.network" | grep -qw lan || continue
    radio=$(uci -q get "wireless.$iface.device")
    # An AP and a STA on one radio crash ath11k (device-facts §3): that AP stays off.
    radio_has_sta "$radio" && { echo "install.sh: $radio carries an uplink; its access point stays off" >&2; continue; }
    printf 'set wireless.%s.ssid=%s\nset wireless.%s.encryption=sae-mixed\nset wireless.%s.key=%s\nset wireless.%s.disabled=0\nset wireless.%s.disabled=0\n' \
      "$iface" "$(uci_quote "$wifi_name")" "$iface" "$iface" "$(uci_quote "$wifi_key")" "$iface" "$radio"
  done | uci batch || die "could not stage the WiFi settings"
}

radio_has_sta() {
  for sta in $(uci -q show wireless | sed -n "s/^wireless\.\([^.=]*\)=wifi-iface$/\1/p"); do
    [ "$(uci -q get "wireless.$sta.mode")" = sta ] && [ "$(uci -q get "wireless.$sta.device")" = "$1" ] &&
      [ "$(uci -q get "wireless.$sta.disabled")" != 1 ] && return 0
  done
  return 1
}

# a.b.c.d -> integer, for range comparisons. Shell arithmetic: BusyBox awk caps %d at 2^31-1.
ip_int() {
  # shellcheck disable=SC2046 # the split on dots is the point
  set -- $(printf '%s' "$1" | tr . ' ')
  echo $(((($1 * 256 + $2) * 256 + $3) * 256 + $4))
}

# Whether two CIDRs share any address: their networks under the shorter prefix are equal.
cidr_overlap() {
  a=$(ip_int "${1%/*}") b=$(ip_int "${2%/*}") p=${1#*/} q=${2#*/}
  [ "$q" -lt "$p" ] && p=$q
  shift_by=$((32 - p))
  [ $((a >> shift_by)) -eq $((b >> shift_by)) ]
}

# An address outside br-lan (an uplink) that overlaps the target LAN.
uplink_overlaps() {
  for cidr in $(ip -4 -o addr show | awk '$2 != "br-lan" && $2 != "lo" { print $4 }'); do
    cidr_overlap "$cidr" "$1" && return 0
  done
  return 1
}

read_wifi_key() {
  [ -n "${TRAVO_WIFI_PASSWORD:-}" ] && { wifi_key=$TRAVO_WIFI_PASSWORD; return; }
  [ -t 0 ] || die "--wifi needs a terminal to ask for the password, or TRAVO_WIFI_PASSWORD"
  stty -echo
  printf 'WiFi password for %s (8 to 63 characters): ' "$wifi_name" >&2
  IFS= read -r wifi_key || wifi_key=
  printf '\nRepeat it: ' >&2
  IFS= read -r again || again=
  stty echo
  printf '\n' >&2
  [ "$wifi_key" = "$again" ] || die "the two passwords differ"
}

# After a healthy install: the factory LAN and WiFi get Travo's defaults. UCI is committed first;
# the network reload runs detached after this script ends, since it drops an SSH session on the
# LAN. Travo restarts afterwards so its HTTPS certificate names the new address.
first_run() {
  [ "$first_install" = 1 ] || return 0
  new_lan=
  if [ "$keep_lan" = 0 ] && [ "$(uci -q get network.lan.ipaddr)" = "$FACTORY_LAN" ]; then
    if uplink_overlaps "$lan_target"; then
      echo "install.sh: an uplink already uses addresses in $lan_target; the LAN stays $FACTORY_LAN (pass --lan)" >&2
    else
      new_lan=$lan_target
      printf 'del_list network.lan.ipaddr=%s\nadd_list network.lan.ipaddr=%s\n' "$FACTORY_LAN" "$new_lan" |
        uci batch || die "could not stage the LAN address"
    fi
  fi
  wifi_on=0
  if [ -n "$wifi_key" ]; then
    if wifi_factory; then
      stage_wifi
      wifi_on=1
    else
      echo "install.sh: the WiFi is not as OpenWrt ships it (or has no access point); it stays as it is" >&2
    fi
  fi
  [ -n "$new_lan" ] || [ "$wifi_on" = 1 ] || return 0
  # Commit only what changed: a router without radios has no wireless config to commit.
  [ -z "$new_lan" ] || uci commit network || die "could not save the LAN address"
  [ "$wifi_on" = 0 ] || uci commit wireless || die "could not save the WiFi settings"
  [ -z "$new_lan" ] || echo "The router's address becomes ${new_lan%/*}: open http://${new_lan%/*}/ once your device has a new address (unplug and replug the cable, or rejoin the WiFi)."
  [ "$wifi_on" = 0 ] || echo "WiFi: $wifi_name on every band (WPA2/WPA3)."
  echo "Applying the network settings now; this SSH session may drop."
  # 9>&-: the detached job must not keep the installer's lock.
  (sleep 2; reload_config; sleep 15; /etc/init.d/$NAME restart) </dev/null >/dev/null 2>&1 9>&- &
}

uninstall() {
  manifest=$STATE_DIR/manifest.txt
  [ -f "$manifest" ] || die "$manifest is missing: nothing to uninstall (or it was purged)"
  load_port80
  recover_port_swap
  uninstall_port80
  # A subsequent --purge has no init script and only removes retained state.
  if [ -x /etc/init.d/$NAME ]; then
    /etc/init.d/$NAME disable 9>&- || true
  fi
  while IFS= read -r path; do
    # Only paths this package can own, and only files: a tampered manifest cannot reach others.
    case "$path" in
      *..*)
        echo "install.sh: skipping $path: contains .." >&2
        continue
        ;;
      "/usr/bin/$NAME" | "/etc/init.d/$NAME" | "/etc/uci-defaults/90-$NAME" | "/lib/upgrade/keep.d/$NAME" | "/etc/hotplug.d/button/50-$NAME" | "/www/$NAME/"?*) ;;
      *)
        echo "install.sh: skipping $path: not a $NAME path" >&2
        continue
        ;;
    esac
    if [ -f "$path" ] || [ -L "$path" ]; then
      rm -f "$path"
    elif [ -e "$path" ]; then
      echo "install.sh: skipping $path: not a file" >&2
    fi
    # empty directories below the web root go too, innermost first
    dir=$(dirname "$path")
    while :; do
      case "$dir" in /www/$NAME/*) ;; *) break ;; esac
      rmdir "$dir" 2>/dev/null || break
      dir=$(dirname "$dir")
    done
  done <"$manifest"
  rm -rf "$STATE_DIR/update-operation"
  rm -f /etc/rc.d/S99$NAME /etc/rc.d/K10$NAME
  rmdir /www/$NAME 2>/dev/null || true
  if [ "$purge" = 1 ]; then
    rm -f /etc/config/$NAME
    rm -rf "$STATE_DIR" /etc/travo-guards
  fi
  echo "$NAME removed"
}

force=0 purge=0 insecure=0 rotate=0 new_key=0 mode=install keep_lan=0 ask_wifi=0
alternate_port=
adguard_bundle=
lan_target=$DEFAULT_LAN
wifi_name=$DEFAULT_SSID
wifi_key=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --uninstall) mode=uninstall ;;
    --purge) purge=1 ;;
    --force) force=1 ;;
    --insecure) insecure=1 ;;
    --rotate-key) rotate=1 ;;
    --port)
      shift
      [ "$#" -gt 0 ] || usage
      case "$1" in ""|*[!0-9]*) usage ;; esac
      alternate_port=$(printf '%s' "$1" | sed 's/^0*//')
      [ -n "$alternate_port" ] && [ "${#alternate_port}" -le 5 ] &&
        [ "$alternate_port" -le 65535 ] && [ "$alternate_port" -ne 80 ] || usage
      ;;
    --adguard-bundle)
      shift
      [ "$#" -gt 0 ] && [ -n "$1" ] || usage
      adguard_bundle=$1
      ;;
    --lan)
      shift
      { [ "$#" -gt 0 ] && valid_cidr "$1"; } || usage
      lan_target=$1
      ;;
    --keep-lan) keep_lan=1 ;;
    --wifi) ask_wifi=1 ;;
    --wifi-name)
      shift
      [ "$#" -gt 0 ] && [ -n "$1" ] && [ "${#1}" -le 32 ] || usage
      wifi_name=$1
      ;;
    *) usage ;;
  esac
  shift
done
[ "$purge" = 0 ] || [ "$mode" = uninstall ] || usage
[ "$mode" = install ] || { [ "$insecure" = 0 ] && [ "$rotate" = 0 ] && [ -z "$alternate_port" ] && [ -z "$adguard_bundle" ] && [ "$ask_wifi" = 0 ]; } || usage

[ "$(id -u)" = 0 ] || die "must run as root"
mkdir -p /var/lock
exec 9>>/var/lock/travo-update.lock
flock -n -x 9 || die "update or installation already in progress"
update_guard=/etc/travo-guards/update-in-progress
if [ -e "$update_guard" ] || [ -L "$update_guard" ]; then
  die "update recovery required: run /usr/bin/travo --recover-update (or the saved recovery executable)"
fi
here=$(cd "$(dirname "$0")" && pwd)
# Ask before installing anything, so a typo cannot leave a half-configured router.
if [ "$mode" = install ] && { [ "$ask_wifi" = 1 ] || [ -n "${TRAVO_WIFI_PASSWORD:-}" ]; }; then
  read_wifi_key
  case "$wifi_key" in *[!\ -~]*) die "the WiFi password must be printable ASCII" ;; esac
  [ "${#wifi_key}" -ge 8 ] && [ "${#wifi_key}" -le 63 ] || die "the WiFi password needs 8 to 63 characters"
fi
"$mode"
