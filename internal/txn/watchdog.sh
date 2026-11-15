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

# Exit 1 means a readable but unusable default. Exit 2 means no trustworthy read.
uplink_default() {
	routes=$(ip -4 route show table main default 2>/dev/null) || return 2
	printf '%s\n' "$routes" | awk -v expected="$uplink_device" '
		function ipv4(value,   octets,i) {
			if (split(value, octets, /[.]/) != 4) return 0
			for (i = 1; i <= 4; i++) {
				if (octets[i] !~ /^(0|[1-9][0-9]*)$/ || octets[i] + 0 > 255) return 0
			}
			return 1
		}
		function route(line,   fields,n,i,metric,ranked,dev,via,src,key,value,valid,identity,hop) {
			sub(/^[ \t]+/, "", line)
			n = split(line, fields, /[ \t]+/)
			i = 1
			if (fields[1] != "default") {
				if (fields[1] !~ /^(unicast|local|broadcast|multicast|throw|unreachable|prohibit|blackhole|nat|anycast)$/ || fields[2] != "default") {
					bad = 1
					return
				}
				i++
			}
			valid = i == 1 || fields[1] == "unicast"
			metric = 0
			for (++i; i <= n; i++) {
				key = fields[i]
				if (key == "") continue
				if (key ~ /^(onlink|linkdown|dead|pervasive|offload|trap|rt_offload_failed)$/) {
					if (key == "linkdown" || key == "dead") valid = 0
					continue
				}
				if (key == "nexthop") {
					if (hop && dev == "") { bad = 1; return }
					valid = 0
					hop = 1
					dev = via = src = ""
					continue
				}
				if (key !~ /^(dev|via|src|metric|proto|scope|nhid|weight|table|pref)$/) { bad = 1; return }
				value = fields[++i]
				if (value == "" || value ~ /^(dev|via|src|metric|proto|scope|nhid|weight|table|pref|nexthop|linkdown|onlink)$/) { bad = 1; return }
				if (key == "metric") {
					if (ranked || hop || value !~ /^[0-9]+$/ || value + 0 > 4294967295) { bad = 1; return }
					metric = value + 0
					ranked = 1
				} else if (key == "dev") {
					if (dev != "" || value !~ /^[[:alnum:]_.:@-]+$/) { bad = 1; return }
					dev = value
				} else if (key == "via") {
					if (via != "" || !ipv4(value)) { bad = 1; return }
					via = value
				} else if (key == "src") {
					if (src != "" || !ipv4(value)) { bad = 1; return }
					src = value
				} else if (key == "nhid" || key == "weight") {
					if (value !~ /^[0-9]+$/ || (key == "weight" && !hop)) { bad = 1; return }
					valid = 0
				} else if (value !~ /^[[:alnum:]_-]+$/) {
					bad = 1
					return
				}
			}
			if (hop && dev == "") { bad = 1; return }
			valid = valid && dev != ""
			gsub(/[ 	]+/, " ", line)
			sub(/ $/, "", line)
			identity = line
			if (!seen || metric < best) {
				seen = 1
				best = metric
				best_id = identity
				best_dev = dev
				best_valid = valid
				ambiguous = 0
			} else if (metric == best && (identity != best_id || !valid || !best_valid)) {
				ambiguous = 1
			}
		}
		NF && $1 == "nexthop" {
			if (line == "") bad = 1
			line = line " " $0
			next
		}
		NF {
			if (line != "") route(line)
			line = $0
		}
		END {
			if (line != "") route(line)
			if (bad) exit 2
			if (!seen || ambiguous || !best_valid || best_dev != expected) exit 1
		}'
}

valid_uplink_name() {
	case $1 in
		''|[!A-Za-z0-9_]*|*[!A-Za-z0-9_-]*) return 1 ;;
	esac
}

restore_uplink() {
	[ -f "$uplink_marker" ] && [ ! -L "$uplink_marker" ] || return 1
	{
		IFS= read -r uplink_logical && IFS= read -r uplink_device
	} < "$uplink_marker" || return 1
	valid_uplink_name "$uplink_logical" && valid_uplink_name "$uplink_device" || return 1
	[ "$uplink_logical" != wg0 ] && [ "$uplink_device" != wg0 ] && [ "${#uplink_device}" -le 15 ] || return 1
	printf '%s\n%s\n' "$uplink_logical" "$uplink_device" | cmp -s - "$uplink_marker" || return 1

	rc=0
	uplink_default || rc=$?
	[ "$rc" -ne 0 ] || return 0
	[ "$rc" -eq 1 ] || return 1

	object=network.interface.$uplink_logical
	info=$(ubus -t 3 call "$object" status 2>/dev/null) || return 1
	current_device=$(printf '%s\n' "$info" | jsonfilter -e '@.l3_device') || return 1
	[ "$current_device" = "$uplink_device" ] || return 1
	ubus -t 3 call "$object" down >/dev/null 2>&1 || return 1
	ubus -t 3 call "$object" up >/dev/null 2>&1 || return 1
	for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
		sleep 1
		rc=0
		uplink_default || rc=$?
		[ "$rc" -ne 0 ] || return 0
		[ "$rc" -eq 1 ] || return 1
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
uplink_marker=$dir/restore-uplink
if [ "$status" = restored ] && { [ -e "$uplink_marker" ] || [ -L "$uplink_marker" ]; }; then
	{ [ -n "$baseline" ] && restore_uplink; } || status=failed
fi
echo "$status" > "$dir/result.tmp" && mv -f "$dir/result.tmp" "$dir/result"
sync
[ "$status" = restored ]
