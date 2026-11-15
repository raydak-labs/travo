#!/bin/sh
# Transaction watchdog: restores the snapshot unless the transaction is confirmed in time.
# Runs detached from the daemon so a daemon crash or kill cannot strand the router.
# Usage: watchdog.sh <txn-dir> <config-dir> <window-s> <arm-limit-s> <uci> <reload...>
#   The window starts when the daemon writes "armed" (after commit, reload, probe), so the
#   user gets the full window. If "armed" never appears within arm-limit (daemon died
#   mid-apply), restore. window 0 = restore now (explicit rollback, startup recovery).
# Files in <txn-dir>: configs (one name per line), snap/<cfg> (pre-change copy; absent = config
# did not exist), armed, claim (first writer decides: "confirm" or "restore"), result.
# Never calls `wifi` (ath11k crash, see docs/device-facts.md §3).
set -f

dir=$1 cfgdir=$2 secs=$3 armlimit=$4 uci=$5
shift 5 2>/dev/null || exit 2
[ -n "$dir" ] && [ -d "$dir" ] && [ -n "$cfgdir" ] && [ -n "$uci" ] && [ "$#" -gt 0 ] || exit 2

if [ "$secs" -gt 0 ]; then
	i=0
	while [ ! -e "$dir/armed" ] && [ "$i" -lt "$armlimit" ]; do
		[ -e "$dir/claim" ] && exit 0
		sleep 1
		i=$((i + 1))
	done
	i=0
	while [ -e "$dir/armed" ] && [ "$i" -lt "$secs" ]; do
		[ -e "$dir/claim" ] && exit 0
		sleep 1
		i=$((i + 1))
	done
fi

wg0_link_present() {
	[ -e /sys/class/net/wg0 ]
}

quiesce_wg0() {
	wg0_link_present || return 0
	# netifd may remove the object during the call. The kernel link is the proof.
	ubus -t 3 call network.interface.wg0 down >/dev/null 2>&1 || :
	for attempt in 1 2 3 4 5 6 7 8; do
		sleep 1
		wg0_link_present || return 0
	done
	return 1
}

wg0_status() {
	# Keep ubus errors distinct from malformed or missing status fields.
	output=$(ubus -t 1 call network.interface.wg0 status 2>/dev/null) || return "$?"
	jsonfilter -s "$output" -e '@.up' 2>/dev/null || return 1
}

start_wg0() {
	for attempt in 1 2 3 4 5; do
		if ubus -t 1 call network.interface.wg0 up >/dev/null 2>&1; then
			return 0
		fi
		[ "$attempt" -eq 5 ] || sleep 1
	done
	return 1
}

wait_wg0_up() {
	for attempt in 1 2 3 4 5 6 7 8; do
		sleep 1
		if wg0_link_present && up=$(wg0_status) && [ "$up" = true ]; then
			return 0
		fi
	done
	return 1
}

wait_wg0_absent() {
	samples=0
	for attempt in 1 2 3 4 5 6 7 8; do
		sleep 1
		if wg0_link_present; then
			samples=0
			continue
		fi
		rc=0
		up=$(wg0_status) || rc=$?
		if [ "$rc" -eq 4 ] || { [ "$rc" -eq 0 ] && [ "$up" = false ]; }; then
			samples=$((samples + 1))
			[ "$samples" -lt 3 ] || return 0
		else
			samples=0
		fi
	done
	return 1
}

# noclobber makes claiming atomic: the first claimant (confirm or another restorer) wins,
# so exactly one restore ever runs. Losers exit; the daemon waits for "result".
( set -C; echo restore > "$dir/claim" ) 2>/dev/null || exit 0

status=restored
while IFS= read -r cfg; do
	case $cfg in '' | *[!a-z0-9_-]*) status=failed; continue ;; esac
	"$uci" revert "$cfg" >/dev/null 2>&1
	if [ -e "$dir/snap/$cfg" ]; then
		cp "$dir/snap/$cfg" "$cfgdir/.$cfg.restore" && mv -f "$cfgdir/.$cfg.restore" "$cfgdir/$cfg" || status=failed
	else
		rm -f "$cfgdir/$cfg" || status=failed
	fi
done < "$dir/configs"
sync
marker=$dir/restore-wg0
baseline=
if [ -e "$marker" ] || [ -L "$marker" ]; then
	if [ "$status" = restored ] && [ -f "$marker" ] && [ ! -L "$marker" ]; then
		if printf 'enabled\n' | cmp -s - "$marker"; then
			baseline=enabled
		elif printf 'absent\n' | cmp -s - "$marker"; then
			baseline=absent
		fi
	fi
	if [ -z "$baseline" ] || ! quiesce_wg0; then
		status=failed
	fi
fi
"$@" >/dev/null 2>&1 || status=failed
if [ "$status" = restored ] && [ -n "$baseline" ]; then
	if ! ubus -t 3 call network reload >/dev/null 2>&1; then
		status=failed
	elif [ "$baseline" = enabled ]; then
		{ start_wg0 && wait_wg0_up; } || status=failed
	else
		wait_wg0_absent || status=failed
	fi
fi
echo "$status" > "$dir/result.tmp" && mv -f "$dir/result.tmp" "$dir/result"
sync
[ "$status" = restored ]
