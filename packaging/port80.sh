#!/bin/sh
# Sourced by install.sh after checking package integrity.

port80_holder() {
  if [ "$(uci -q get travo-greenfield.main.listen)" = :80 ]; then
    echo self
  elif uhttpd_listen_lines | grep -q " listen_http .*:80'$"; then
    echo uhttpd
  elif port_is_listening 80; then
    echo other
  else
    echo none
  fi
}

uhttpd_listen_lines() {
  awk '
    /^config[ \t]/ {
      section = ($2 == "uhttpd") ? $3 : ""
      gsub(/\047/, "", section)
    }
    section != "" && /^[ \t]*(option|list)[ \t]+listen_https?[ \t]/ {
      print section "\t" $0
    }
  ' "${1:-/etc/config/uhttpd}"
}

uhttpd_sections() {
  awk -F '\t' '$1 != "moved" && !seen[$1]++ {print $1}' "$1"
}

uhttpd_clear_listen() {
  for key in listen_http listen_https; do
    if uci -q get "uhttpd.$1.$key" >/dev/null; then
      uci -q delete "uhttpd.$1.$key" || return 1
    fi
  done
}

uhttpd_set_listen() (
  sections=$(uhttpd_sections "$STATE_DIR/uhttpd.orig") || exit 1
  for section in $sections; do
    uhttpd_clear_listen "$section" || exit 1
    uci add_list "uhttpd.$section.listen_http=0.0.0.0:$1" &&
      uci add_list "uhttpd.$section.listen_http=[::]:$1" &&
      uci add_list "uhttpd.$section.listen_https=0.0.0.0:$2" &&
      uci add_list "uhttpd.$section.listen_https=[::]:$2" || exit 1
  done
  uci commit uhttpd && /etc/init.d/uhttpd restart 9>&-
)

uhttpd_restore_lines() (
  sections=$(uhttpd_sections "$1") || exit 1
  for section in $sections; do
    uhttpd_clear_listen "$section" || exit 1
  done
  tab=$(printf '\t')
  while IFS= read -r record; do
    section=${record%%"$tab"*}
    [ "$section" != moved ] || continue
    line=${record#*"$tab"}
    read -r kind field _ <<EOF_LINE
$line
EOF_LINE
    # Strip only the surrounding UCI quotes, preserving the original value.
    value=${line#*\'}
    value=${value%\'}
    case "$kind" in
      option) uci set "uhttpd.$section.$field=$value" || exit 1 ;;
      list) uci add_list "uhttpd.$section.$field=$value" || exit 1 ;;
      *) exit 1 ;;
    esac
  done <"$1"
  uci commit uhttpd && /etc/init.d/uhttpd restart 9>&-
)

port_is_listening() {
  netstat -ltn | awk -v port=":$1" '$4 ~ (port "$") {found=1} END {exit !found}'
}

wait_listen() (
  i=0
  until netstat -ltn | awk -v addr="$1" '$4 == addr {found=1} END {exit !found}'; do
    i=$((i + 1))
    [ "$i" -lt "$2" ] || exit 1
    sleep 1
  done
)

set_greenfield_listen() {
  uci set "travo-greenfield.main.listen=$1" || return 1
  # Off :80, uhttpd keeps or gets back :443: drop the HTTPS port enable_tls_listen set.
  if [ "$1" != :80 ] && [ "$(uci -q get travo-greenfield.main.listen_tls)" = :443 ]; then
    uci delete travo-greenfield.main.listen_tls || return 1
  fi
  uci commit travo-greenfield
}

# HTTPS on :443 (ADR 0016) once uhttpd no longer serves it. Succeeds only when it changed
# the setting. Keeps any operator value, including `off`; skips a :443 held by another
# process. A TLS listen failure is not fatal for the daemon, so this never blocks install.
enable_tls_listen() {
  [ -z "$(uci -q get travo-greenfield.main.listen_tls)" ] || return 1
  ! uhttpd_listen_lines | grep -q " listen_https .*:443'$" || return 1
  ! port_is_listening 443 || return 1
  uci set travo-greenfield.main.listen_tls=:443 && uci commit travo-greenfield
}

clear_port_swap() {
  rm -f "$PORT_SWAP_GUARD" && rm -f "$PORT_SWAP_GUARD.uhttpd"
}

port_swap_restore() {
  /etc/init.d/travo-greenfield stop 9>&- || true
  # A failed uci operation can leave staged writes that would mask the snapshot.
  if ! uci revert uhttpd ||
     ! cp "$PORT_SWAP_GUARD.uhttpd" /etc/config/uhttpd ||
     ! /etc/init.d/uhttpd restart 9>&-; then
    echo "recovery: cp $PORT_SWAP_GUARD.uhttpd /etc/config/uhttpd && /etc/init.d/uhttpd restart" >&2
    return 1
  fi
  wait_restored_uhttpd "$PORT_SWAP_GUARD.uhttpd" &&
    set_greenfield_listen 0.0.0.0:8090 &&
    /etc/init.d/travo-greenfield start 9>&- &&
    wait_healthy "${1:-}" &&
    clear_port_swap
}

recover_port_swap() {
  [ -f "$PORT_SWAP_GUARD" ] || return 0
  if [ ! -f "$PORT_SWAP_GUARD.uhttpd" ]; then
    rm -f "$PORT_SWAP_GUARD"
    return
  fi
  port_swap_restore || die "could not restore interrupted port swap; guard kept"
  echo "restored uhttpd from an interrupted install"
}

wait_original_listeners() (
  addresses=$(awk -F "\047" '$1 !~ /^moved/ {print $2}' "$1" | tr -d '[]')
  for address in $addresses; do
    wait_listen "$address" 10 || exit 1
  done
)

wait_restored_uhttpd() (
  snapshot_lines=$(mktemp) || exit 1
  trap 'rm -f "$snapshot_lines"' EXIT
  uhttpd_listen_lines "$1" >"$snapshot_lines" && wait_original_listeners "$snapshot_lines"
)

wait_moved_uhttpd() (
  for address in 0.0.0.0:8080 :::8080 0.0.0.0:8443 :::8443; do
    wait_listen "$address" 10 || exit 1
  done
)

uhttpd_expected_lines() (
  sections=$(uhttpd_sections "$1") || exit 1
  for section in $sections; do
    printf "%s\t\tlist listen_http '0.0.0.0:8080'\n" "$section"
    printf "%s\t\tlist listen_http '[::]:8080'\n" "$section"
    printf "%s\t\tlist listen_https '0.0.0.0:8443'\n" "$section"
    printf "%s\t\tlist listen_https '[::]:8443'\n" "$section"
  done
)

check_pending_uhttpd() {
  pending=$(uci changes uhttpd) || die "could not check uhttpd changes"
  [ -z "$pending" ] || die "uhttpd has uncommitted changes; commit or revert them first"
}

write_port_swap_guard() {
  cp /etc/config/uhttpd "$PORT_SWAP_GUARD.uhttpd" &&
    date +%s >"$PORT_SWAP_GUARD"
}

stop_greenfield() {
  [ ! -x /etc/init.d/travo-greenfield ] || /etc/init.d/travo-greenfield stop 9>&-
}

reset_saved_port() {
  if [ "$(uci -q get travo-greenfield.main.listen)" = :80 ]; then
    set_greenfield_listen 0.0.0.0:8090
  fi
}

uninstall_port80() (
  orig=$STATE_DIR/uhttpd.orig
  moved=$(printf 'moved\t1')
  if [ ! -f "$orig" ] || [ "$(tail -n 1 "$orig")" != "$moved" ]; then
    stop_greenfield || die "could not stop greenfield"
    reset_saved_port || die "could not reset the saved port"
    rm -f "$orig"
    exit
  fi
  comparison=$(mktemp -d) || die "cannot compare uhttpd listeners"
  trap 'rm -rf "$comparison"' EXIT
  uhttpd_listen_lines >"$comparison/current"
  uhttpd_expected_lines "$orig" >"$comparison/expected"
  if ! cmp -s "$comparison/current" "$comparison/expected"; then
    stop_greenfield || die "could not stop greenfield"
    reset_saved_port || die "could not reset the saved port"
    echo "uhttpd listen values changed since install; leaving them" >&2
    echo "uhttpd.orig kept in /etc/travo-greenfield/" >&2
    exit 0
  fi
  check_pending_uhttpd
  write_port_swap_guard || die "could not write the port-swap guard"
  trap 'port_swap_restore; exit 1' INT TERM HUP
  if ! stop_greenfield || ! reset_saved_port ||
     ! uhttpd_restore_lines "$orig" || ! wait_original_listeners "$orig"; then
    port_swap_restore || die "could not restore uhttpd listen values; recovery required"
    die "could not restore uhttpd listen values; left LuCI on the moved ports"
  fi
  clear_port_swap
  trap - INT TERM HUP
  rm -f "$orig"
)

stop_test_holder() {
  if [ -n "${test_holder_pid:-}" ]; then
    kill "$test_holder_pid" 2>/dev/null || true
    wait "$test_holder_pid" 2>/dev/null || true
    test_holder_pid=
  fi
}

swap_uhttpd() {
  check_pending_uhttpd
  for moved_port in 8080 8443; do
    if port_is_listening "$moved_port"; then
      die "port $moved_port is in use"
    fi
  done
  write_port_swap_guard || die "could not write the port-swap guard"
  trap 'stop_test_holder; port_swap_restore; exit 1' INT TERM HUP
  if ! { uhttpd_listen_lines && printf 'moved\t1\n'; } >"$STATE_DIR/uhttpd.orig" ||
     ! uhttpd_set_listen 8080 8443 || ! wait_moved_uhttpd; then
    port_swap_restore "$1" || die "uhttpd did not come up on :8080; recovery required"
    die "uhttpd did not come up on :8080; restored"
  fi
  if [ "${TRAVO_TEST_STOP_AFTER_SWAP:-0}" = 1 ]; then
    trap - INT TERM HUP
    exit 1
  fi
}

install_ports() {
  lan_ip=$(uci -q get network.lan.ipaddr || echo 192.168.1.1)
  if [ -n "$alternate_port" ]; then
    if ! set_greenfield_listen "0.0.0.0:$alternate_port" ||
       ! /etc/init.d/travo-greenfield restart 9>&- || ! wait_healthy "$1"; then
      die "greenfield did not answer on :$alternate_port"
    fi
    echo "$NAME $1 installed and healthy on http://${lan_ip%%/*}:$alternate_port/"
    return
  fi

  holder=$(port80_holder)
  case "$holder" in
    self)
      if enable_tls_listen; then
        if ! /etc/init.d/travo-greenfield restart 9>&- || ! wait_healthy "$1"; then
          die "greenfield did not answer after enabling HTTPS"
        fi
      fi
      echo "$NAME $1 installed and healthy on port 80"
      return
      ;;
    other) die "port 80 is in use by something other than uhttpd" ;;
    uhttpd) swap_uhttpd "$1" ;;
  esac

  test_holder_pid=
  if [ "${TRAVO_TEST_HOLD_80:-0}" = 1 ]; then
    /usr/sbin/uhttpd -f -p '[::]:80' -h /tmp </dev/null >/dev/null 2>&1 &
    test_holder_pid=$!
    if ! wait_listen :::80 10; then
      stop_test_holder
      [ "$holder" != uhttpd ] || port_swap_restore "$1" || die "test listener failed and recovery is required"
      die "test listener did not start"
    fi
  fi
  enable_tls_listen || true
  if ! set_greenfield_listen :80 ||
     ! /etc/init.d/travo-greenfield restart 9>&- ||
     ! wait_healthy "$1"; then
    stop_test_holder
    if [ "$holder" = uhttpd ]; then
      port_swap_restore "$1" || die "greenfield did not answer on :80; recovery required"
      die "greenfield did not answer on :80; uhttpd restored"
    fi
    if ! set_greenfield_listen 0.0.0.0:8090 ||
       ! /etc/init.d/travo-greenfield restart 9>&- || ! wait_healthy "$1"; then
      die "greenfield did not answer on :80; could not restore :8090"
    fi
    die "greenfield did not answer on :80; left on :8090"
  fi
  stop_test_holder
  if [ "$holder" = uhttpd ]; then
    clear_port_swap
    trap - INT TERM HUP
  fi
  echo "travo-greenfield on http://${lan_ip%%/*}/ (and https://${lan_ip%%/*}/ with a self-signed certificate), LuCI on http://${lan_ip%%/*}:8080/"
}
